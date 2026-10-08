// Command elcompresso runs the HTTP API, the encoding worker, or both:
//
//	elcompresso api      HTTP API only: accepts uploads, creates jobs, serves status
//	elcompresso worker   consumes jobs from RabbitMQ and runs ffmpeg
//	elcompresso          both in one process (local development)
//
// Job state is kept in Redis and work is handed over through RabbitMQ, so the
// two halves can run in separate containers and jobs survive restarts.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/umohsamuel/elcompresso/cmd/api"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/adapter/rabbitmq"
	"github.com/umohsamuel/elcompresso/internal/adapter/redisstore"
	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	storageDomain "github.com/umohsamuel/elcompresso/internal/domain/storage"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/util"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

func main() {
	mode := "all"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if mode != "all" && mode != "api" && mode != "worker" {
		fmt.Fprintf(os.Stderr, "usage: elcompresso [api|worker]\n")
		os.Exit(2)
	}
	runAPI, runWorker := mode != "worker", mode != "api"

	environmentVariables := env.LoadEnvironment()

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if environmentVariables.ProductionEnvironment {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	log := slog.New(handler).With("server", util.GetServerID(), "mode", mode)
	slog.SetDefault(log)

	ctx := context.Background()

	ff, err := ffmpeg.New(ctx, environmentVariables.FFmpegPath, environmentVariables.FFprobePath, environmentVariables.FFmpegThreads)
	if err != nil {
		fatal(log, "ffmpeg is required", err)
	}
	log.Info("ffmpeg ready", "version", ff.Version, "capabilities", ff.Capabilities())

	store, err := newStorage(ctx, environmentVariables)
	if err != nil {
		fatal(log, "storage setup failed", err)
	}

	rdb, err := connectRetry(ctx, log, "redis", func() (*redis.Client, error) {
		return redisstore.Connect(ctx, environmentVariables.RedisURL)
	})
	if err != nil {
		fatal(log, "redis unavailable", err)
	}
	defer rdb.Close()
	jobStore := redisstore.New(rdb, environmentVariables.RedisPrefix)

	queue, err := connectRetry(ctx, log, "rabbitmq", func() (*rabbitmq.Queue, error) {
		return rabbitmq.Connect(environmentVariables.RabbitMQURL, environmentVariables.QueueName)
	})
	if err != nil {
		fatal(log, "rabbitmq unavailable", err)
	}
	defer queue.Close()

	adapters := adapter.NewAdapter(adapter.AdapterDependencies{
		EnvironmentVariables: environmentVariables,
		FFmpeg:               ff,
		Storage:              store,
	})

	jobs, err := service.NewJobManager(service.JobManagerConfig{
		WorkDir:        filepath.Join(environmentVariables.DataDir, "work"),
		Workers:        environmentVariables.Workers,
		QueueSize:      environmentVariables.QueueSize,
		JobTimeout:     environmentVariables.JobTimeout,
		ResultTTL:      environmentVariables.ResultTTL,
		MaxActivePerIP: environmentVariables.MaxActiveJobsPerIP,
	}, adapters.Compressors, store, jobStore, queue, log)
	if err != nil {
		fatal(log, "job manager setup failed", err)
	}

	runCtx, stopRun := context.WithCancel(ctx)
	var wg sync.WaitGroup

	if runWorker {
		wg.Go(func() {
			log.Info("worker started", "workers", environmentVariables.Workers)
			if err := jobs.Run(runCtx); err != nil {
				fatal(log, "worker failed", err)
			}
		})
	}

	var srv *http.Server
	if runAPI {
		services := service.NewService(service.ServiceDependencies{
			Adapter: adapters,
			Jobs:    jobs,
			Uploads: service.NewUploads(store, jobStore),
		})
		r := api.API(services, environmentVariables)
		srv = &http.Server{
			Addr:              environmentVariables.Port,
			Handler:           r.Engine,
			ReadHeaderTimeout: 15 * time.Second,
			IdleTimeout:       120 * time.Second,
			// No Read/WriteTimeout: uploads and downloads of large files can
			// legitimately take minutes.
		}
		go func() {
			log.Info("listening", "addr", srv.Addr, "storage", environmentVariables.StorageType)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fatal(log, "server failed", err)
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		log.Info("shutting down")
	case err := <-queue.Closed():
		// Exit and let the supervisor restart us; unacknowledged jobs are
		// redelivered by RabbitMQ.
		fatal(log, "lost rabbitmq connection", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	jobs.StopAccepting()
	if srv != nil {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("http shutdown", "err", err)
		}
	}
	// Running encodes are stopped and their jobs put back on the queue.
	stopRun()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		log.Warn("workers did not stop in time")
	}
	log.Info("stopped")
}

// connectRetry retries a dependency for a while: in Docker Compose the API
// and worker can start before Redis or RabbitMQ is ready.
func connectRetry[T any](ctx context.Context, log *slog.Logger, name string, connect func() (T, error)) (T, error) {
	var (
		v   T
		err error
	)
	for attempt := 1; attempt <= 30; attempt++ {
		if v, err = connect(); err == nil {
			return v, nil
		}
		log.Warn("waiting for "+name, "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return v, err
}

func newStorage(ctx context.Context, e *env.EnvironmentVariables) (storageDomain.Storage, error) {
	if e.StorageType == "s3" {
		var opts []func(*awsConfig.LoadOptions) error
		if e.S3.Region != "" {
			opts = append(opts, awsConfig.WithRegion(e.S3.Region))
		}
		// Credentials come from the default chain: env vars, shared config or
		// the EC2 instance role (recommended).
		cfg, err := awsConfig.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return nil, err
		}
		return storage.NewS3(s3.NewFromConfig(cfg), e.S3.Bucket, e.S3.Prefix, e.S3.UploadPrefix), nil
	}
	return storage.NewLocal(filepath.Join(e.DataDir, "results"))
}

func fatal(log *slog.Logger, msg string, err error) {
	log.Error(msg, "err", err)
	os.Exit(1)
}
