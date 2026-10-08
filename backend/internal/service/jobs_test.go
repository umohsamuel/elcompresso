package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
)

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

func newManager(t *testing.T, c compress.Interface, cfg JobManagerConfig) *JobManager {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewLocal(filepath.Join(dir, "results"))
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
	m, err := NewJobManager(cfg, map[compress.FileType]compress.Interface{compress.FileTypeImage: c}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	return m
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
	return m.Submit(SubmitRequest{ID: id, Type: compress.FileTypeImage, ClientIP: ip, InputPath: in, WorkDir: dir, OriginalName: "My Photo.png", OriginalSize: 1000})
}

func waitFor(t *testing.T, m *JobManager, id string, want job.Status) job.Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, err := m.Get(id)
		if err == nil && j.Status == want {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	j, _ := m.Get(id)
	t.Fatalf("job %s: status %q, want %q (err %q)", id, j.Status, want, j.Error)
	return j
}

func TestJobLifecycle(t *testing.T) {
	m := newManager(t, &fakeCompressor{}, JobManagerConfig{})
	if _, err := submit(t, m, "a", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	j := waitFor(t, m, "a", job.StatusDone)
	if j.Result == nil || j.Result.Size != 500 || j.Result.SavedPercent != 50 || j.Result.Name != "My Photo-compressed.bin" {
		t.Fatalf("unexpected result %+v", j.Result)
	}

	d, err := m.Download(context.Background(), "a", false)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(d.Object.Body)
	d.Object.Body.Close()
	if len(b) != 500 {
		t.Fatalf("downloaded %d bytes", len(b))
	}

	// Deleting a finished job removes its result.
	if err := m.Cancel(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestJobFailureMessageHidesPaths(t *testing.T) {
	c := &fakeCompressor{}
	m := newManager(t, c, JobManagerConfig{})
	dir, _ := m.PrepareWorkDir("x")
	c.fail = errors.New("bad input " + dir + "/input.png")
	if _, err := submit(t, m, "x", "ip"); err != nil {
		t.Fatal(err)
	}
	j := waitFor(t, m, "x", job.StatusFailed)
	if j.Error != "bad input /input.png" {
		t.Fatalf("error leaked path: %q", j.Error)
	}
}

func TestPerIPLimitAndCancel(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	m := newManager(t, c, JobManagerConfig{MaxActivePerIP: 2})

	if _, err := submit(t, m, "1", "ip"); err != nil {
		t.Fatal(err)
	}
	if _, err := submit(t, m, "2", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := m.CheckCapacity("ip"); !errors.Is(err, ErrTooManyJobs) {
		t.Fatalf("expected ErrTooManyJobs, got %v", err)
	}
	if err := m.CheckCapacity("other"); err != nil {
		t.Fatalf("other IP should be allowed: %v", err)
	}

	waitFor(t, m, "1", job.StatusProcessing)
	if j, _ := m.Get("2"); j.QueuePosition != 1 {
		t.Fatalf("queue position %d", j.QueuePosition)
	}

	// Cancel the running and the queued job; both slots free up.
	if err := m.Cancel(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(context.Background(), "2"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, "1", job.StatusCanceled)
	waitFor(t, m, "2", job.StatusCanceled)
	if err := m.CheckCapacity("ip"); err != nil {
		t.Fatalf("slots not released: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	m := newManager(t, c, JobManagerConfig{QueueSize: 1})
	if _, err := submit(t, m, "1", "a"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, "1", job.StatusProcessing) // worker took it, queue is empty
	if _, err := submit(t, m, "2", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := submit(t, m, "3", "c"); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
	close(c.block)
}

func TestTimeout(t *testing.T) {
	c := &fakeCompressor{block: make(chan struct{})}
	m := newManager(t, c, JobManagerConfig{JobTimeout: 50 * time.Millisecond})
	if _, err := submit(t, m, "slow", "ip"); err != nil {
		t.Fatal(err)
	}
	if j := waitFor(t, m, "slow", job.StatusFailed); j.Error == "" {
		t.Fatal("expected timeout message")
	}
}

func TestExpiry(t *testing.T) {
	m := newManager(t, &fakeCompressor{}, JobManagerConfig{})
	if _, err := submit(t, m, "e", "ip"); err != nil {
		t.Fatal(err)
	}
	j := waitFor(t, m, "e", job.StatusDone)

	m.mu.Lock()
	past := time.Now().Add(-time.Second)
	m.jobs["e"].job.ExpiresAt = &past
	m.mu.Unlock()
	m.expire()

	if _, err := m.Get("e"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("job not expired: %v", err)
	}
	if _, err := m.storage.Open(context.Background(), j.Result.StorageKey()); err == nil {
		t.Fatal("result file not deleted")
	}
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
