package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
	"github.com/umohsamuel/elcompresso/internal/domain/queue"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

var (
	ErrNotFound     = job.ErrNotFound
	ErrQueueFull    = errors.New("the server is busy, please try again in a few minutes")
	ErrTooManyJobs  = errors.New("too many files in progress, wait for some to finish")
	ErrNotReady     = errors.New("job has not finished yet")
	ErrShuttingDown = errors.New("the server is restarting, please try again shortly")

	errStateChanged = errors.New("job state changed")
	errCanceled     = errors.New("canceled by user")
)

var (
	// monitorInterval is how often a worker saves progress and checks for a
	// cancel request while encoding.
	monitorInterval = time.Second
	// bookkeepingTimeout bounds each job-state write.
	bookkeepingTimeout = 30 * time.Second
)

type JobManagerConfig struct {
	WorkDir        string
	Workers        int
	QueueSize      int
	JobTimeout     time.Duration
	ResultTTL      time.Duration
	MaxActivePerIP int
	// MaxAttempts caps how often a job is retried after a worker died while
	// running it (default 3).
	MaxAttempts int
}

// JobManager runs both halves of the pipeline: the API side creates jobs and
// reads their state, the worker side (Run) encodes them. Job state lives in
// a job.Store and work is handed over through a queue.Queue, so the two can
// run in separate processes and survive restarts.
type JobManager struct {
	cfg         JobManagerConfig
	compressors map[compress.FileType]compress.Interface
	storage     storage.Storage
	uploads     storage.DirectUploader // nil without direct uploads
	store       job.Store
	queue       queue.Queue
	log         *slog.Logger

	stopping atomic.Bool
}

func NewJobManager(cfg JobManagerConfig, compressors map[compress.FileType]compress.Interface, results storage.Storage, store job.Store, q queue.Queue, log *slog.Logger) (*JobManager, error) {
	if err := os.MkdirAll(cfg.WorkDir, 0o750); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	m := &JobManager{
		cfg:         cfg,
		compressors: compressors,
		storage:     results,
		store:       store,
		queue:       q,
		log:         log,
	}
	m.uploads, _ = results.(storage.DirectUploader)
	return m, nil
}

// --- API side ---------------------------------------------------------------

// PrepareWorkDir creates the scratch directory for a new job.
func (m *JobManager) PrepareWorkDir(id string) (string, error) {
	dir := filepath.Join(m.cfg.WorkDir, id)
	return dir, os.MkdirAll(dir, 0o750)
}

// CheckCapacity is a cheap pre-flight check so clients are rejected before
// uploading a large file that would not be accepted anyway.
func (m *JobManager) CheckCapacity(ctx context.Context, clientIP string) error {
	if m.stopping.Load() {
		return ErrShuttingDown
	}
	queued, _, err := m.store.Counts(ctx)
	if err != nil {
		return err
	}
	if queued >= m.cfg.QueueSize {
		return ErrQueueFull
	}
	if m.cfg.MaxActivePerIP > 0 {
		active, err := m.store.ActiveForClient(ctx, clientIP)
		if err != nil {
			return err
		}
		if active >= m.cfg.MaxActivePerIP {
			return ErrTooManyJobs
		}
	}
	return nil
}

type SubmitRequest struct {
	ID           string
	Type         compress.FileType
	ClientIP     string
	InputPath    string
	WorkDir      string
	OriginalName string
	OriginalSize int64
	Options      compress.Options
	// UploadKey marks a direct upload still in storage (see Uploads).
	UploadKey string
}

func (m *JobManager) Submit(ctx context.Context, req SubmitRequest) (job.Job, error) {
	if err := m.CheckCapacity(ctx, req.ClientIP); err != nil {
		return job.Job{}, err
	}
	rec := &job.Record{
		Job: job.Job{
			ID:           req.ID,
			Type:         req.Type,
			Status:       job.StatusQueued,
			OriginalName: req.OriginalName,
			OriginalSize: req.OriginalSize,
			Options:      req.Options,
			CreatedAt:    time.Now().UTC(),
		},
		ClientIP:  req.ClientIP,
		InputPath: req.InputPath,
		WorkDir:   req.WorkDir,
		UploadKey: req.UploadKey,
	}
	if err := m.store.Create(ctx, rec); err != nil {
		return job.Job{}, fmt.Errorf("saving job: %w", err)
	}
	if err := m.queue.Publish(ctx, rec.Job.ID); err != nil {
		_ = m.store.Delete(context.WithoutCancel(ctx), rec.Job.ID)
		m.log.Error("queue publish failed", "job", rec.Job.ID, "err", err)
		return job.Job{}, ErrQueueFull
	}
	return m.Get(ctx, rec.Job.ID)
}

func (m *JobManager) Get(ctx context.Context, id string) (job.Job, error) {
	rec, err := m.store.Get(ctx, id)
	if err != nil {
		return job.Job{}, err
	}
	j := rec.Job
	if j.Status.Finished() && j.ExpiresAt != nil && time.Now().After(*j.ExpiresAt) {
		return job.Job{}, ErrNotFound
	}
	if j.Status == job.StatusQueued {
		if j.QueuePosition, err = m.store.QueuePosition(ctx, id); err != nil {
			return job.Job{}, err
		}
	}
	return j, nil
}

// Cancel stops a queued or running job, or deletes a finished job's result.
func (m *JobManager) Cancel(ctx context.Context, id string) error {
	for range 3 {
		rec, err := m.store.Get(ctx, id)
		if err != nil {
			return err
		}
		switch rec.Job.Status {
		case job.StatusQueued:
			done, err := m.store.Update(ctx, id, func(r *job.Record) error {
				if r.Job.Status != job.StatusQueued {
					return errStateChanged // a worker just picked it up
				}
				m.finish(r, job.StatusCanceled, "")
				return nil
			})
			if errors.Is(err, errStateChanged) {
				continue
			}
			if err != nil {
				return err
			}
			// The worker acks its message when it sees the job is finished.
			m.cleanup(done)
			return nil
		case job.StatusProcessing:
			return m.store.RequestCancel(ctx, id)
		default:
			if rec.ResultKey != "" {
				if err := m.storage.Delete(ctx, rec.ResultKey); err != nil {
					return err
				}
			}
			return m.store.Delete(ctx, id)
		}
	}
	return errStateChanged
}

type Download struct {
	URL      string // set for external storage (redirect)
	Object   *storage.Object
	FileName string
	Type     string
}

func (m *JobManager) Download(ctx context.Context, id string, inline bool) (*Download, error) {
	rec, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	j := rec.Job
	if j.Status != job.StatusDone || j.Result == nil {
		return nil, ErrNotReady
	}
	d := &Download{FileName: j.Result.Name, Type: j.Result.ContentType}
	url, ok, err := m.storage.DownloadURL(ctx, rec.ResultKey, j.Result.Name, inline, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	if ok {
		d.URL = url
		return d, nil
	}
	if d.Object, err = m.storage.Open(ctx, rec.ResultKey); err != nil {
		return nil, err
	}
	return d, nil
}

type Stats struct {
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Workers    int `json:"workers"`
}

func (m *JobManager) Stats(ctx context.Context) (Stats, error) {
	queued, processing, err := m.store.Counts(ctx)
	return Stats{Queued: queued, Processing: processing, Workers: m.cfg.Workers}, err
}

// StopAccepting makes the API side refuse new jobs (graceful shutdown).
func (m *JobManager) StopAccepting() { m.stopping.Store(true) }

// --- Worker side ------------------------------------------------------------

// Run consumes jobs with cfg.Workers parallel encodes until ctx ends. Jobs
// still running then are put back on the queue for the next worker.
func (m *JobManager) Run(ctx context.Context) error {
	deliveries, err := m.queue.Consume(ctx, m.cfg.Workers)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for range m.cfg.Workers {
		wg.Go(func() {
			for d := range deliveries {
				m.process(ctx, d)
			}
		})
	}
	wg.Go(func() { m.janitor(ctx) })
	wg.Wait()
	return nil
}

func (m *JobManager) process(runCtx context.Context, d queue.Delivery) {
	// Bookkeeping gets a fresh deadline per write and ignores shutdown, so
	// state is still saved after a long encode or while stopping.
	bookkeeping := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(runCtx), bookkeepingTimeout)
	}
	log := m.log.With("job", d.JobID)

	ctx, cancel := bookkeeping()
	rec, err := m.store.Update(ctx, d.JobID, func(r *job.Record) error {
		if r.Job.Status.Finished() {
			return errStateChanged // canceled while waiting
		}
		now := time.Now().UTC()
		r.Attempts++
		r.Job.Status = job.StatusProcessing
		r.Job.StartedAt = &now
		r.Job.Stage, r.Job.Progress, r.Job.Error = "", 0, ""
		return nil
	})
	cancel()
	switch {
	case errors.Is(err, job.ErrNotFound), errors.Is(err, errStateChanged):
		_ = d.Ack()
		return
	case err != nil:
		log.Error("claim job", "err", err)
		_ = d.Requeue()
		time.Sleep(time.Second) // store unavailable; don't spin
		return
	}
	log = log.With("type", rec.Job.Type, "attempt", rec.Attempts)

	// finish saves the final state and acks only once it is stored: if it
	// can't be saved, the job goes back on the queue rather than being left
	// looking busy forever.
	finish := func(status job.Status, msg string, result *job.Result) {
		ctx, cancel := bookkeeping()
		defer cancel()
		if err := m.complete(ctx, rec, status, msg, result); err != nil {
			log.Error("save job state, requeueing", "status", status, "err", err)
			_ = d.Requeue()
			time.Sleep(time.Second)
			return
		}
		_ = d.Ack()
	}

	if rec.Attempts > m.cfg.MaxAttempts {
		log.Warn("job abandoned after repeated crashes")
		finish(job.StatusFailed, "the server couldn't process this file, please try a different file or settings", nil)
		return
	}

	started := time.Now()
	log.Info("job started", "size", rec.Job.OriginalSize, "redelivered", d.Redelivered)
	result, err := m.run(runCtx, rec)

	switch {
	case err == nil:
		finish(job.StatusDone, "", result)
		log.Info("job done", "duration", time.Since(started).Round(time.Millisecond), "in", rec.Job.OriginalSize, "out", result.Size)
	case runCtx.Err() != nil:
		// Shutting down: hand the job to the next worker instead of failing it.
		ctx, cancel := bookkeeping()
		_, uerr := m.store.Update(ctx, rec.Job.ID, func(r *job.Record) error {
			if r.Job.Status != job.StatusProcessing {
				return errStateChanged
			}
			r.Job.Status, r.Job.Stage, r.Job.Progress, r.Job.StartedAt = job.StatusQueued, "", 0, nil
			r.Attempts-- // not the job's fault
			return nil
		})
		cancel()
		if uerr != nil && !errors.Is(uerr, errStateChanged) {
			log.Warn("requeue job", "err", uerr)
		}
		_ = d.Requeue()
		log.Info("job requeued for shutdown")
	case errors.Is(err, errCanceled):
		finish(job.StatusCanceled, "", nil)
		log.Info("job canceled")
	case errors.Is(err, context.DeadlineExceeded):
		finish(job.StatusFailed, fmt.Sprintf("took longer than %s - try a faster speed, lower resolution or a shorter clip", m.cfg.JobTimeout), nil)
		log.Warn("job timed out")
	default:
		finish(job.StatusFailed, strings.ReplaceAll(err.Error(), rec.WorkDir, ""), nil)
		log.Warn("job failed", "err", err)
	}
}

// run fetches the input if needed, encodes it and stores the result.
func (m *JobManager) run(runCtx context.Context, rec *job.Record) (*job.Result, error) {
	ctx, cancel := context.WithCancelCause(runCtx)
	defer cancel(nil)
	ctx, cancelTimeout := context.WithTimeout(ctx, m.cfg.JobTimeout)
	defer cancelTimeout()

	if err := os.MkdirAll(rec.WorkDir, 0o750); err != nil {
		return nil, err
	}

	// Progress is saved and cancel requests are checked on a timer rather
	// than on every ffmpeg progress line.
	var (
		mu       sync.Mutex
		stage    string
		fraction float64
		dirty    bool
	)
	progress := func(s string, f float64) {
		mu.Lock()
		stage, fraction, dirty = s, f, true
		mu.Unlock()
	}
	stopMonitor := make(chan struct{})
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		t := time.NewTicker(monitorInterval)
		defer t.Stop()
		for {
			select {
			case <-stopMonitor:
				return
			case <-t.C:
			}
			if yes, _ := m.store.CancelRequested(ctx, rec.Job.ID); yes {
				cancel(errCanceled)
			}
			mu.Lock()
			s, f, changed := stage, fraction, dirty
			dirty = false
			mu.Unlock()
			if changed {
				_, _ = m.store.Update(ctx, rec.Job.ID, func(r *job.Record) error {
					if r.Job.Status != job.StatusProcessing {
						return errStateChanged
					}
					r.Job.Stage = s
					r.Job.Progress = math.Round(min(max(f, 0), 1)*1000) / 10
					return nil
				})
			}
		}
	}()
	defer func() {
		close(stopMonitor)
		<-monitorDone
	}()

	result, err := m.encode(ctx, rec, progress)
	if cause := context.Cause(ctx); errors.Is(cause, errCanceled) && err != nil {
		return nil, errCanceled
	}
	return result, err
}

func (m *JobManager) encode(ctx context.Context, rec *job.Record, progress compress.ProgressFunc) (*job.Result, error) {
	if rec.UploadKey != "" {
		if m.uploads == nil {
			return nil, errors.New("direct uploads are not configured on this worker")
		}
		progress("preparing", 0)
		if err := m.uploads.FetchUpload(ctx, rec.UploadKey, rec.InputPath); err != nil {
			return nil, err
		}
	}

	res, err := m.compressors[rec.Job.Type].Compress(ctx, compress.Task{
		InputPath:    rec.InputPath,
		WorkDir:      rec.WorkDir,
		OriginalName: rec.Job.OriginalName,
		Options:      rec.Job.Options,
	}, progress)
	if err != nil {
		return nil, err
	}

	progress("saving", 1)
	name := outputName(rec.Job.OriginalName, res.Ext)
	rec.ResultKey = rec.Job.ID + "/" + name
	if err := m.storage.Save(ctx, rec.ResultKey, res.Path, res.ContentType); err != nil {
		return nil, fmt.Errorf("saving result: %w", err)
	}
	r := &job.Result{
		Name:        name,
		Size:        res.Size,
		ContentType: res.ContentType,
		DownloadURL: "/api/v1/jobs/" + rec.Job.ID + "/download",
		Info:        res.Info,
		Larger:      res.Size >= rec.Job.OriginalSize,
	}
	if rec.Job.OriginalSize > 0 {
		r.SavedPercent = math.Round((1-float64(res.Size)/float64(rec.Job.OriginalSize))*1000) / 10
	}
	return r, nil
}

// complete records a final state and, once stored, releases the job's files.
func (m *JobManager) complete(ctx context.Context, rec *job.Record, status job.Status, msg string, result *job.Result) error {
	done, err := m.store.Update(ctx, rec.Job.ID, func(r *job.Record) error {
		if result != nil {
			r.Job.Result = result
			r.ResultKey = rec.ResultKey
		}
		m.finish(r, status, msg)
		return nil
	})
	if errors.Is(err, job.ErrNotFound) {
		m.cleanup(rec) // deleted meanwhile; nothing left to record
		return nil
	}
	if err != nil {
		return err
	}
	m.cleanup(done)
	return nil
}

// finish moves a record to a terminal state. It only edits the record, so it
// can run inside a store update.
func (m *JobManager) finish(r *job.Record, status job.Status, msg string) {
	now := time.Now().UTC()
	expires := now.Add(m.cfg.ResultTTL)
	r.Job.Status = status
	r.Job.Error = msg
	r.Job.FinishedAt = &now
	r.Job.ExpiresAt = &expires
	r.Job.Stage = ""
	if status == job.StatusDone {
		r.Job.Progress = 100
	}
}

// cleanup deletes a finished job's scratch files and its uploaded input.
func (m *JobManager) cleanup(rec *job.Record) {
	if rec.WorkDir != "" {
		_ = os.RemoveAll(rec.WorkDir)
	}
	if rec.UploadKey != "" && m.uploads != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.uploads.DeleteUpload(ctx, rec.UploadKey); err != nil {
			m.log.Warn("failed to delete upload", "key", rec.UploadKey, "err", err)
		}
	}
}

func (m *JobManager) janitor(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.expire(ctx)
			m.sweepWorkDir(ctx)
		}
	}
}

// expire deletes results that are past their expiry.
func (m *JobManager) expire(ctx context.Context) {
	ids, err := m.store.Expired(ctx, time.Now(), 100)
	if err != nil {
		m.log.Warn("list expired jobs", "err", err)
		return
	}
	for _, id := range ids {
		rec, err := m.store.Get(ctx, id)
		if err == nil && rec.ResultKey != "" {
			if err := m.storage.Delete(ctx, rec.ResultKey); err != nil {
				m.log.Warn("failed to delete expired result", "key", rec.ResultKey, "err", err)
				continue
			}
		}
		if err := m.store.Delete(ctx, id); err != nil {
			m.log.Warn("failed to delete expired job", "job", id, "err", err)
		}
	}
}

// sweepWorkDir removes scratch directories left behind by jobs that no
// longer exist (e.g. an upload that failed halfway).
func (m *JobManager) sweepWorkDir(ctx context.Context) {
	entries, err := os.ReadDir(m.cfg.WorkDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-2 * m.cfg.JobTimeout)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !e.IsDir() || info.ModTime().After(cutoff) {
			continue
		}
		rec, err := m.store.Get(ctx, e.Name())
		if errors.Is(err, job.ErrNotFound) || (err == nil && rec.Job.Status.Finished()) {
			_ = os.RemoveAll(filepath.Join(m.cfg.WorkDir, e.Name()))
		}
	}
}

// outputName turns "My Holiday.MOV" into "My Holiday-compressed.mp4".
func outputName(original, ext string) string {
	base := strings.TrimSuffix(filepath.Base(original), filepath.Ext(original))
	base = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, base)
	if base == "" || base == "." {
		base = "file"
	}
	if r := []rune(base); len(r) > 120 {
		base = string(r[:120])
	}
	return base + "-compressed." + ext
}
