package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/umohsamuel/elcompresso/cmd/api"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	storageDomain "github.com/umohsamuel/elcompresso/internal/domain/storage"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/util"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	environmentVariables := env.LoadEnvironment()

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if environmentVariables.ProductionEnvironment {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	log := slog.New(handler).With("server", util.GetServerID())
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
	}, adapters.Compressors, store, log)
	if err != nil {
		fatal(log, "job manager setup failed", err)
	}

	services := service.NewService(service.ServiceDependencies{Adapter: adapters, Jobs: jobs, Uploads: service.NewUploads(store)})
	r := api.API(services, environmentVariables)

	srv := &http.Server{
		Addr:              environmentVariables.Port,
		Handler:           r.Engine,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No Read/WriteTimeout: uploads and downloads of large files can
		// legitimately take minutes.
	}

	go func() {
		log.Info("listening", "addr", srv.Addr, "workers", environmentVariables.Workers, "storage", environmentVariables.StorageType)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(log, "server failed", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("http shutdown", "err", err)
	}
	jobs.Shutdown(shutdownCtx)
	log.Info("stopped")
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
