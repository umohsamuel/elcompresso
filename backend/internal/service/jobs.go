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
	"time"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

var (
	ErrNotFound     = errors.New("job not found or expired")
	ErrQueueFull    = errors.New("the server is busy, please try again in a few minutes")
	ErrTooManyJobs  = errors.New("too many files in progress, wait for some to finish")
	ErrNotReady     = errors.New("job has not finished yet")
	ErrShuttingDown = errors.New("the server is restarting, please try again shortly")
)

type JobManagerConfig struct {
	WorkDir        string
	Workers        int
	QueueSize      int
	JobTimeout     time.Duration
	ResultTTL      time.Duration
	MaxActivePerIP int
}

type entry struct {
	job       job.Job
	inputPath string
	workDir   string
	clientIP  string
	// uploadKey is set for direct uploads: the input is still in storage and
	// is fetched to inputPath when the job starts.
	uploadKey string
	cancel    context.CancelFunc
}

type JobManager struct {
	cfg         JobManagerConfig
	compressors map[compress.FileType]compress.Interface
	storage     storage.Storage
	uploads     storage.DirectUploader // nil without direct uploads
	log         *slog.Logger

	mu       sync.RWMutex
	jobs     map[string]*entry
	active   map[string]int // unfinished jobs per client IP
	queue    chan *entry
	stopping bool

	baseCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

func NewJobManager(cfg JobManagerConfig, compressors map[compress.FileType]compress.Interface, store storage.Storage, log *slog.Logger) (*JobManager, error) {
	// Jobs live in memory, so anything left on disk is from a previous run.
	if err := os.RemoveAll(cfg.WorkDir); err != nil {
		return nil, fmt.Errorf("clean work dir: %w", err)
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o750); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &JobManager{
		cfg:         cfg,
		compressors: compressors,
		storage:     store,
		log:         log,
		jobs:        map[string]*entry{},
		active:      map[string]int{},
		queue:       make(chan *entry, cfg.QueueSize),
		baseCtx:     ctx,
		stop:        cancel,
	}
	m.uploads, _ = store.(storage.DirectUploader)
	for range cfg.Workers {
		m.wg.Go(m.worker)
	}
	m.wg.Go(m.janitor)
	return m, nil
}

// PrepareWorkDir creates the scratch directory for a new job.
func (m *JobManager) PrepareWorkDir(id string) (string, error) {
	dir := filepath.Join(m.cfg.WorkDir, id)
	return dir, os.MkdirAll(dir, 0o750)
}

// CheckCapacity is a cheap pre-flight check so clients are rejected before
// uploading a large file that would not be accepted anyway.
func (m *JobManager) CheckCapacity(clientIP string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	switch {
	case m.stopping:
		return ErrShuttingDown
	case len(m.queue) >= cap(m.queue):
		return ErrQueueFull
	case m.cfg.MaxActivePerIP > 0 && m.active[clientIP] >= m.cfg.MaxActivePerIP:
		return ErrTooManyJobs
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

func (m *JobManager) Submit(req SubmitRequest) (job.Job, error) {
	e := &entry{
		job: job.Job{
			ID:           req.ID,
			Type:         req.Type,
			Status:       job.StatusQueued,
			OriginalName: req.OriginalName,
			OriginalSize: req.OriginalSize,
			Options:      req.Options,
			CreatedAt:    time.Now().UTC(),
		},
		inputPath: req.InputPath,
		workDir:   req.WorkDir,
		clientIP:  req.ClientIP,
		uploadKey: req.UploadKey,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping {
		return job.Job{}, ErrShuttingDown
	}
	if m.cfg.MaxActivePerIP > 0 && m.active[req.ClientIP] >= m.cfg.MaxActivePerIP {
		return job.Job{}, ErrTooManyJobs
	}
	select {
	case m.queue <- e:
	default:
		return job.Job{}, ErrQueueFull
	}
	m.jobs[e.job.ID] = e
	m.active[req.ClientIP]++
	return m.snapshotLocked(e), nil
}

func (m *JobManager) Get(id string) (job.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.jobs[id]
	if !ok {
		return job.Job{}, ErrNotFound
	}
	return m.snapshotLocked(e), nil
}

func (m *JobManager) snapshotLocked(e *entry) job.Job {
	j := e.job
	if j.Result != nil {
		r := *j.Result
		j.Result = &r
	}
	if j.Status == job.StatusQueued {
		j.QueuePosition = 1
		for _, other := range m.jobs {
			if other.job.Status == job.StatusQueued && other.job.CreatedAt.Before(j.CreatedAt) {
				j.QueuePosition++
			}
		}
	}
	return j
}

// Cancel stops a queued or running job, or deletes a finished job's result.
func (m *JobManager) Cancel(ctx context.Context, id string) error {
	m.mu.Lock()
	e, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	switch e.job.Status {
	case job.StatusQueued:
		m.finishLocked(e, job.StatusCanceled, "")
	case job.StatusProcessing:
		if e.cancel != nil {
			e.cancel() // the worker marks it canceled and cleans up
		}
	default:
		delete(m.jobs, id)
		var key string
		if e.job.Result != nil {
			key = e.job.Result.StorageKey()
		}
		m.mu.Unlock()
		if key != "" {
			return m.storage.Delete(ctx, key)
		}
		return nil
	}
	m.mu.Unlock()
	return nil
}

type Download struct {
	URL      string // set for external storage (redirect)
	Object   *storage.Object
	FileName string
	Type     string
}

func (m *JobManager) Download(ctx context.Context, id string, inline bool) (*Download, error) {
	j, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if j.Status != job.StatusDone || j.Result == nil {
		return nil, ErrNotReady
	}
	key := j.Result.StorageKey()
	d := &Download{FileName: j.Result.Name, Type: j.Result.ContentType}
	url, ok, err := m.storage.DownloadURL(ctx, key, j.Result.Name, inline, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	if ok {
		d.URL = url
		return d, nil
	}
	if d.Object, err = m.storage.Open(ctx, key); err != nil {
		return nil, err
	}
	return d, nil
}

type Stats struct {
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Workers    int `json:"workers"`
}

func (m *JobManager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := Stats{Workers: m.cfg.Workers}
	for _, e := range m.jobs {
		switch e.job.Status {
		case job.StatusQueued:
			s.Queued++
		case job.StatusProcessing:
			s.Processing++
		}
	}
	return s
}

// Shutdown stops accepting work, cancels running encodes and waits for the
// workers to exit.
func (m *JobManager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()
	m.stop()

	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (m *JobManager) worker() {
	for {
		select {
		case <-m.baseCtx.Done():
			return
		case e := <-m.queue:
			m.process(e)
		}
	}
}

func (m *JobManager) process(e *entry) {
	ctx, cancel := context.WithTimeout(m.baseCtx, m.cfg.JobTimeout)
	defer cancel()

	m.mu.Lock()
	if e.job.Status != job.StatusQueued { // canceled while waiting
		m.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	e.job.Status = job.StatusProcessing
	e.job.StartedAt = &now
	e.cancel = cancel
	m.mu.Unlock()

	log := m.log.With("job", e.job.ID, "type", e.job.Type)
	log.Info("job started", "size", e.job.OriginalSize)

	progress := func(stage string, fraction float64) {
		m.mu.Lock()
		e.job.Stage = stage
		e.job.Progress = math.Round(min(max(fraction, 0), 1)*1000) / 10
		m.mu.Unlock()
	}

	var err error
	if e.uploadKey != "" {
		progress("preparing", 0)
		err = m.uploads.FetchUpload(ctx, e.uploadKey, e.inputPath)
	}

	var res *compress.Result
	if err == nil {
		res, err = m.compressors[e.job.Type].Compress(ctx, compress.Task{
			InputPath:    e.inputPath,
			WorkDir:      e.workDir,
			OriginalName: e.job.OriginalName,
			Options:      e.job.Options,
		}, progress)
	}

	var result *job.Result
	if err == nil {
		progress("saving", 1)
		result, err = m.store(ctx, e, res)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case err == nil:
		e.job.Result = result
		m.finishLocked(e, job.StatusDone, "")
		log.Info("job done", "duration", time.Since(now).Round(time.Millisecond), "in", e.job.OriginalSize, "out", result.Size)
	case errors.Is(err, context.Canceled) && m.baseCtx.Err() == nil:
		m.finishLocked(e, job.StatusCanceled, "")
		log.Info("job canceled")
	case m.baseCtx.Err() != nil:
		m.finishLocked(e, job.StatusFailed, "the server restarted while processing this file, please try again")
	case errors.Is(err, context.DeadlineExceeded):
		m.finishLocked(e, job.StatusFailed, fmt.Sprintf("took longer than %s - try a faster speed, lower resolution or a shorter clip", m.cfg.JobTimeout))
		log.Warn("job timed out")
	default:
		msg := strings.ReplaceAll(err.Error(), e.workDir, "")
		m.finishLocked(e, job.StatusFailed, msg)
		log.Warn("job failed", "err", err)
	}
}

func (m *JobManager) store(ctx context.Context, e *entry, res *compress.Result) (*job.Result, error) {
	name := outputName(e.job.OriginalName, res.Ext)
	key := e.job.ID + "/" + name
	if err := m.storage.Save(ctx, key, res.Path, res.ContentType); err != nil {
		return nil, fmt.Errorf("saving result: %w", err)
	}
	r := &job.Result{
		Name:        name,
		Size:        res.Size,
		ContentType: res.ContentType,
		DownloadURL: "/api/v1/jobs/" + e.job.ID + "/download",
		Info:        res.Info,
		Larger:      res.Size >= e.job.OriginalSize,
	}
	if e.job.OriginalSize > 0 {
		r.SavedPercent = math.Round((1-float64(res.Size)/float64(e.job.OriginalSize))*1000) / 10
	}
	r.SetStorageKey(key)
	return r, nil
}

// finishLocked moves a job to a terminal state and releases its resources.
// Callers must hold m.mu.
func (m *JobManager) finishLocked(e *entry, status job.Status, errMsg string) {
	now := time.Now().UTC()
	expires := now.Add(m.cfg.ResultTTL)
	e.job.Status = status
	e.job.Error = errMsg
	e.job.FinishedAt = &now
	e.job.ExpiresAt = &expires
	e.job.Stage = ""
	if status == job.StatusDone {
		e.job.Progress = 100
	}
	e.cancel = nil
	if m.active[e.clientIP]--; m.active[e.clientIP] <= 0 {
		delete(m.active, e.clientIP)
	}
	_ = os.RemoveAll(e.workDir)
	if key := e.uploadKey; key != "" && m.uploads != nil {
		e.uploadKey = ""
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.uploads.DeleteUpload(ctx, key); err != nil {
				m.log.Warn("failed to delete upload", "key", key, "err", err)
			}
		}()
	}
}

func (m *JobManager) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-m.baseCtx.Done():
			return
		case <-t.C:
			m.expire()
		}
	}
}

func (m *JobManager) expire() {
	now := time.Now()
	var keys []string
	m.mu.Lock()
	for id, e := range m.jobs {
		if e.job.ExpiresAt != nil && now.After(*e.job.ExpiresAt) {
			if e.job.Result != nil {
				keys = append(keys, e.job.Result.StorageKey())
			}
			delete(m.jobs, id)
		}
	}
	m.mu.Unlock()

	for _, key := range keys {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := m.storage.Delete(ctx, key); err != nil {
			m.log.Warn("failed to delete expired result", "key", key, "err", err)
		}
		cancel()
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
