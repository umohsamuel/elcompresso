package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

var ErrUploadNotFound = errors.New("upload not found or expired, please upload the file again")

// UploadTTL is how long a browser has to finish a direct upload and create
// the job for it.
const UploadTTL = time.Hour

// Upload is a direct-to-storage upload slot handed to the browser.
type Upload struct {
	ID string `json:"id"`
	storage.DirectUpload
	ExpiresAt time.Time `json:"expires_at"`
}

type pendingUpload struct {
	fileType compress.FileType
	clientIP string
	name     string
	maxBytes int64
	expires  time.Time
}

// PendingUpload is an issued upload claimed by a job.
type PendingUpload struct {
	Key  string
	Name string
	Size int64
}

// Uploads issues presigned direct uploads and lets each one be turned into
// exactly one job. Objects that are never claimed are removed by the bucket's
// lifecycle rule.
type Uploads struct {
	store storage.DirectUploader

	mu      sync.Mutex
	pending map[string]pendingUpload
}

// NewUploads returns nil when the storage backend can't take direct uploads.
func NewUploads(store storage.Storage) *Uploads {
	du, ok := store.(storage.DirectUploader)
	if !ok {
		return nil
	}
	return &Uploads{store: du, pending: map[string]pendingUpload{}}
}

func (u *Uploads) Begin(ctx context.Context, fileType compress.FileType, clientIP, name string, maxBytes int64) (*Upload, error) {
	id := uuid.NewString()
	expires := time.Now().Add(UploadTTL)
	form, err := u.store.PresignUpload(ctx, id, maxBytes, UploadTTL)
	if err != nil {
		return nil, err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	for k, p := range u.pending {
		if now.After(p.expires) {
			delete(u.pending, k)
		}
	}
	u.pending[id] = pendingUpload{fileType: fileType, clientIP: clientIP, name: name, maxBytes: maxBytes, expires: expires}
	return &Upload{ID: id, DirectUpload: *form, ExpiresAt: expires.UTC()}, nil
}

// Take claims an issued upload for a job and checks what actually landed in
// storage. It can succeed only once per upload.
func (u *Uploads) Take(ctx context.Context, id string, fileType compress.FileType, clientIP string) (*PendingUpload, error) {
	u.mu.Lock()
	p, ok := u.pending[id]
	if ok && (p.fileType != fileType || p.clientIP != clientIP || time.Now().After(p.expires)) {
		ok = false
	}
	if ok {
		delete(u.pending, id)
	}
	u.mu.Unlock()
	if !ok {
		return nil, ErrUploadNotFound
	}

	size, err := u.store.UploadSize(ctx, id)
	if err != nil {
		return nil, err
	}
	if size <= 0 || size > p.maxBytes {
		_ = u.store.DeleteUpload(ctx, id)
		return nil, ErrUploadNotFound
	}
	return &PendingUpload{Key: id, Name: p.name, Size: size}, nil
}

// Discard deletes a claimed upload whose job could not be queued.
func (u *Uploads) Discard(ctx context.Context, key string) error {
	return u.store.DeleteUpload(ctx, key)
}
