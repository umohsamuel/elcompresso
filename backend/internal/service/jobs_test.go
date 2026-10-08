package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/umohsamuel/elcompresso/internal/adapter/redisstore"
	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
	"github.com/umohsamuel/elcompresso/internal/domain/queue"
)

func init() { monitorInterval = 10 * time.Millisecond }

// fakeCompressor writes half of the input to the output, or blocks until
// released/canceled when block is set.
type fakeCompressor struct {
	block chan struct{}
	fail  error
}

func (f *fakeCompressor) Compress(ctx context.Context, task compress.Task, progress compress.ProgressFunc) (*compress.Result, error) {
	progress("encoding", 0.5)
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.fail != nil {
		return nil, f.fail
	}
	in, _ := os.ReadFile(task.InputPath)
	out := filepath.Join(task.WorkDir, "output.bin")
	if err := os.WriteFile(out, in[:len(in)/2], 0o644); err != nil {
		return nil, err
	}
	return &compress.Result{Path: out, Ext: "bin", ContentType: "application/octet-stream", Size: int64(len(in) / 2)}, nil
}

// memQueue behaves like the RabbitMQ queue: requeued jobs come back, jobs
// held by a stopped consumer are returned.
type memQueue struct{ ch chan string }

func newMemQueue() *memQueue { return &memQueue{ch: make(chan string, 100)} }

func (q *memQueue) Publish(_ context.Context, id string) error {
	select {
	case q.ch <- id:
		return nil
	default:
		return errors.New("queue full")
	}
}

func (q *memQueue) Consume(ctx context.Context, _ int) (<-chan queue.Delivery, error) {
	out := make(chan queue.Delivery)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-q.ch:
				d := queue.Delivery{
					JobID:   id,
					Ack:     func() error { return nil },
					Requeue: func() error { q.ch <- id; return nil },
				}
				select {
				case out <- d:
				case <-ctx.Done():
					q.ch <- id
					return
				}
			}
		}
	}()
	return out, nil
}

func (q *memQueue) Close() error { return nil }

type harness struct {
	m       *JobManager
	store   *redisstore.Store
	queue   *memQueue
	results *storage.Local
	cfg     JobManagerConfig
	c       compress.Interface
	stop    func()
}

func newHarness(t *testing.T, c compress.Interface, cfg JobManagerConfig) *harness {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	dir := t.TempDir()
	results, err := storage.NewLocal(filepath.Join(dir, "results"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.WorkDir = filepath.Join(dir, "work")
	if cfg.Workers == 0 {
		cfg.Workers = 1
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 10
	}
	if cfg.JobTimeout == 0 {
		cfg.JobTimeout = time.Minute
	}
	if cfg.ResultTTL == 0 {
		cfg.ResultTTL = time.Hour
	}
	h := &harness{store: redisstore.New(rdb, "test:"), queue: newMemQueue(), results: results, cfg: cfg, c: c}
	h.start(t)
	return h
}

// start runs a fresh manager (a "new process") against the same Redis,
// queue and files.
func (h *harness) start(t *testing.T) {
	t.Helper()
	m, err := NewJobManager(h.cfg, map[compress.FileType]compress.Interface{compress.FileTypeImage: h.c}, h.results, h.store, h.queue, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { _ = m.Run(ctx) })
	var once sync.Once
	h.m = m
	h.stop = func() { once.Do(func() { cancel(); wg.Wait() }) }
	t.Cleanup(h.stop)
}

func submit(t *testing.T, m *JobManager, id, ip string) (job.Job, error) {
	t.Helper()
	dir, err := m.PrepareWorkDir(id)
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "input.png")
	if err := os.WriteFile(in, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	return m.Submit(context.Background(), SubmitRequest{ID: id, Type: compress.FileTypeImage, ClientIP: ip, InputPath: in, WorkDir: dir, OriginalName: "My Photo.png", OriginalSize: 1000})
}

func waitFor(t *testing.T, m *JobManager, id string, want job.Status) job.Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, err := m.Get(context.Background(), id)
		if err == nil && j.Status == want {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	j, _ := m.Get(context.Background(), id)
	t.Fatalf("job %s: status %q, want %q (err %q)", id, j.Status, want, j.Error)
	return j
}

func TestJobLifecycle(t *testing.T) {
	h := newHarness(t, &fakeCompressor{}, JobManagerConfig{})
	if _, err := submit(t, h.m, "a", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	j := waitFor(t, h.m, "a", job.StatusDone)
	if j.Result == nil || j.Result.Size != 500 || j.Result.SavedPercent != 50 || j.Result.Name != "My Photo-compressed.bin" {
		t.Fatalf("unexpected result %+v", j.Result)
	}

	d, err := h.m.Download(context.Background(), "a", false)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(d.Object.Body)
	d.Object.Body.Close()
	if len(b) != 500 {
		t.Fatalf("downloaded %d bytes", len(b))
	}

	// Deleting a finished job removes its result.
	if err := h.m.Cancel(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.m.Get(context.Background(), "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestJobFailureMessageHidesPaths(t *testing.T) {
	c := &fakeCompressor{}
	h := newHarness(t, c, JobManagerConfig{})
	dir, _ := h.m.PrepareWorkDir("x")
	c.fail = errors.New("bad input " + dir + "/input.png")
	if _, err := submit(t, h.m, "x", "ip"); err != nil {
		t.Fatal(err)
	}
	j := waitFor(t, h.m, "x", job.StatusFailed)
	if j.Error != "bad input /input.png" {
		t.Fatalf("error leaked path: %q", j.Error)
	}
}

func TestPerIPLimitAndCancel(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	h := newHarness(t, c, JobManagerConfig{MaxActivePerIP: 2})
	ctx := context.Background()

	if _, err := submit(t, h.m, "1", "ip"); err != nil {
		t.Fatal(err)
	}
	if _, err := submit(t, h.m, "2", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.CheckCapacity(ctx, "ip"); !errors.Is(err, ErrTooManyJobs) {
		t.Fatalf("expected ErrTooManyJobs, got %v", err)
	}
	if err := h.m.CheckCapacity(ctx, "other"); err != nil {
		t.Fatalf("other IP should be allowed: %v", err)
	}

	waitFor(t, h.m, "1", job.StatusProcessing)
	if j, _ := h.m.Get(ctx, "2"); j.QueuePosition != 1 {
		t.Fatalf("queue position %d", j.QueuePosition)
	}

	// Cancel the running and the queued job; both slots free up.
	if err := h.m.Cancel(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Cancel(ctx, "2"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.m, "1", job.StatusCanceled)
	waitFor(t, h.m, "2", job.StatusCanceled)
	if err := h.m.CheckCapacity(ctx, "ip"); err != nil {
		t.Fatalf("slots not released: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	h := newHarness(t, c, JobManagerConfig{QueueSize: 1})
	if _, err := submit(t, h.m, "1", "a"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.m, "1", job.StatusProcessing) // worker took it, queue is empty
	if _, err := submit(t, h.m, "2", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := submit(t, h.m, "3", "c"); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
	close(c.block)
}

func TestTimeout(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	h := newHarness(t, c, JobManagerConfig{JobTimeout: 50 * time.Millisecond})
	if _, err := submit(t, h.m, "slow", "ip"); err != nil {
		t.Fatal(err)
	}
	if j := waitFor(t, h.m, "slow", job.StatusFailed); j.Error == "" {
		t.Fatal("expected timeout message")
	}
}

func TestExpiry(t *testing.T) {
	h := newHarness(t, &fakeCompressor{}, JobManagerConfig{})
	ctx := context.Background()
	if _, err := submit(t, h.m, "e", "ip"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.m, "e", job.StatusDone)
	rec, err := h.store.Update(ctx, "e", func(r *job.Record) error {
		past := time.Now().Add(-time.Second)
		r.Job.ExpiresAt = &past
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	h.m.expire(ctx)

	if _, err := h.store.Get(ctx, "e"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("job not expired: %v", err)
	}
	if _, err := h.results.Open(ctx, rec.ResultKey); err == nil {
		t.Fatal("result file not deleted")
	}
}

// A job running when its worker shuts down goes back on the queue and is
// finished by the next worker.
func TestJobSurvivesWorkerRestart(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	h := newHarness(t, c, JobManagerConfig{})
	if _, err := submit(t, h.m, "r", "ip"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.m, "r", job.StatusProcessing)

	h.stop()
	rec, err := h.store.Get(context.Background(), "r")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Job.Status != job.StatusQueued || rec.Attempts != 0 {
		t.Fatalf("after shutdown: status %q attempts %d", rec.Job.Status, rec.Attempts)
	}

	close(c.block)
	h.start(t)
	waitFor(t, h.m, "r", job.StatusDone)
}

// A job that keeps killing its worker is given up on instead of looping.
func TestJobAbandonedAfterRepeatedCrashes(t *testing.T) {
	h := newHarness(t, &fakeCompressor{}, JobManagerConfig{MaxAttempts: 2})
	ctx := context.Background()
	h.stop()

	rec := &job.Record{
		Job:      job.Job{ID: "crashy", Type: compress.FileTypeImage, Status: job.StatusProcessing, CreatedAt: time.Now()},
		ClientIP: "ip",
		WorkDir:  filepath.Join(h.cfg.WorkDir, "crashy"),
		Attempts: 2, // two workers already died on it
	}
	if err := h.store.Create(ctx, rec); err != nil {
		t.Fatal(err)
	}
	_ = h.queue.Publish(ctx, "crashy")

	h.start(t)
	j := waitFor(t, h.m, "crashy", job.StatusFailed)
	if j.Error == "" {
		t.Fatal("expected an explanation")
	}
	if n, _ := h.store.ActiveForClient(ctx, "ip"); n != 0 {
		t.Fatalf("active count not released: %d", n)
	}
}

// Regression: the final state used to be saved with a deadline that started
// when the job was picked up, so encodes longer than it were never marked done.
func TestLongEncodeIsSaved(t *testing.T) {
	old := bookkeepingTimeout
	bookkeepingTimeout = 50 * time.Millisecond
	t.Cleanup(func() { bookkeepingTimeout = old })

	c := &fakeCompressor{block: make(chan struct{})}
	h := newHarness(t, c, JobManagerConfig{})
	if _, err := submit(t, h.m, "long", "ip"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.m, "long", job.StatusProcessing)
	time.Sleep(4 * bookkeepingTimeout)
	close(c.block)
	waitFor(t, h.m, "long", job.StatusDone)
}

func TestOutputName(t *testing.T) {
	cases := map[string]string{
		"holiday.MOV":         "holiday-compressed.mp4",
		"a/b/../evil.mp4":     "evil-compressed.mp4",
		`we:ird*na"me?.mkv`:   "we_ird_na_me_-compressed.mp4",
		".mp4":                "file-compressed.mp4",
		"photo.with.dots.jpg": "photo.with.dots-compressed.mp4",
	}
	for in, want := range cases {
		if got := outputName(in, "mp4"); got != want {
			t.Errorf("outputName(%q) = %q, want %q", in, got, want)
		}
	}
}
