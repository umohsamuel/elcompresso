package service

import (
	"context"
	"errors"
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
	slots storage.UploadSlots
}

// NewUploads returns nil when the storage backend can't take direct uploads.
func NewUploads(store storage.Storage, slots storage.UploadSlots) *Uploads {
	du, ok := store.(storage.DirectUploader)
	if !ok {
		return nil
	}
	return &Uploads{store: du, slots: slots}
}

func (u *Uploads) Begin(ctx context.Context, fileType compress.FileType, clientIP, name string, maxBytes int64) (*Upload, error) {
	id := uuid.NewString()
	form, err := u.store.PresignUpload(ctx, id, maxBytes, UploadTTL)
	if err != nil {
		return nil, err
	}
	slot := storage.UploadSlot{FileType: fileType, ClientIP: clientIP, Name: name, MaxBytes: maxBytes}
	if err := u.slots.PutUploadSlot(ctx, id, slot, UploadTTL); err != nil {
		return nil, err
	}
	return &Upload{ID: id, DirectUpload: *form, ExpiresAt: time.Now().Add(UploadTTL).UTC()}, nil
}

// Take claims an issued upload for a job and checks what actually landed in
// storage. It can succeed only once per upload.
func (u *Uploads) Take(ctx context.Context, id string, fileType compress.FileType, clientIP string) (*PendingUpload, error) {
	slot, err := u.slots.TakeUploadSlot(ctx, id)
	if err != nil {
		return nil, err
	}
	if slot == nil {
		return nil, ErrUploadNotFound
	}
	if slot.FileType != fileType || slot.ClientIP != clientIP {
		// Not this caller's upload: leave it for the right one.
		_ = u.slots.PutUploadSlot(ctx, id, *slot, UploadTTL)
		return nil, ErrUploadNotFound
	}

	size, err := u.store.UploadSize(ctx, id)
	if err != nil {
		return nil, err
	}
	if size <= 0 || size > slot.MaxBytes {
		_ = u.store.DeleteUpload(ctx, id)
		return nil, ErrUploadNotFound
	}
	return &PendingUpload{Key: id, Name: slot.Name, Size: size}, nil
}

// Discard deletes a claimed upload whose job could not be queued.
func (u *Uploads) Discard(ctx context.Context, key string) error {
	return u.store.DeleteUpload(ctx, key)
}
