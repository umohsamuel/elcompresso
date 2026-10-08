package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/umohsamuel/elcompresso/internal/adapter/redisstore"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

func newSlots(t *testing.T) storage.UploadSlots {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { rdb.Close() })
	return redisstore.New(rdb, "test:")
}

var errMissing = errors.New("missing")

// fakeDirect is an in-memory direct-upload store. sizes holds what the
// "browser" uploaded.
type fakeDirect struct {
	storage.Storage
	sizes   map[string]int64
	deleted []string
}

func (f *fakeDirect) PresignUpload(_ context.Context, key string, _ int64, _ time.Duration) (*storage.DirectUpload, error) {
	return &storage.DirectUpload{URL: "https://bucket.example", Fields: map[string]string{"key": key}}, nil
}

func (f *fakeDirect) UploadSize(_ context.Context, key string) (int64, error) {
	if n, ok := f.sizes[key]; ok {
		return n, nil
	}
	return 0, errMissing
}

func (f *fakeDirect) FetchUpload(context.Context, string, string) error { return nil }

func (f *fakeDirect) DeleteUpload(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func TestUploadsTakeOnce(t *testing.T) {
	ctx := context.Background()
	store := &fakeDirect{sizes: map[string]int64{}}
	u := NewUploads(store, newSlots(t))
	if u == nil {
		t.Fatal("expected direct uploads to be enabled")
	}

	up, err := u.Begin(ctx, compress.FileTypeVideo, "1.1.1.1", "clip.mp4", 100)
	if err != nil {
		t.Fatal(err)
	}
	if up.Fields["key"] != up.ID {
		t.Fatalf("form not presigned for the upload id")
	}
	store.sizes[up.ID] = 42

	if _, err := u.Take(ctx, up.ID, compress.FileTypeAudio, "1.1.1.1"); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("wrong type: got %v", err)
	}
	if _, err := u.Take(ctx, up.ID, compress.FileTypeVideo, "2.2.2.2"); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("wrong client: got %v", err)
	}

	got, err := u.Take(ctx, up.ID, compress.FileTypeVideo, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Size != 42 || got.Name != "clip.mp4" || got.Key != up.ID {
		t.Fatalf("unexpected upload %+v", got)
	}

	if _, err := u.Take(ctx, up.ID, compress.FileTypeVideo, "1.1.1.1"); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("second take: got %v", err)
	}
}

func TestUploadsRejectsOversize(t *testing.T) {
	ctx := context.Background()
	store := &fakeDirect{sizes: map[string]int64{}}
	u := NewUploads(store, newSlots(t))

	up, err := u.Begin(ctx, compress.FileTypeImage, "1.1.1.1", "a.png", 10)
	if err != nil {
		t.Fatal(err)
	}
	store.sizes[up.ID] = 11

	if _, err := u.Take(ctx, up.ID, compress.FileTypeImage, "1.1.1.1"); !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("got %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != up.ID {
		t.Fatalf("oversized upload not deleted: %v", store.deleted)
	}
}

func TestNewUploadsWithoutDirectStorage(t *testing.T) {
	if NewUploads(nil, nil) != nil {
		t.Fatal("expected nil without a direct-upload store")
	}
}
