package storage

import (
	"context"
	"io"
	"time"
)

type Object struct {
	Body    io.ReadSeekCloser
	ModTime time.Time
}

type Storage interface {
	// Save takes ownership of the file at localPath and stores it under key.
	Save(ctx context.Context, key, localPath, contentType string) error

	// DownloadURL returns an external URL to fetch the object from. ok is
	// false when the backend has no external URL and the object must be
	// streamed through Open instead.
	DownloadURL(ctx context.Context, key, downloadName string, inline bool, expiry time.Duration) (url string, ok bool, err error)

	Open(ctx context.Context, key string) (*Object, error)

	Delete(ctx context.Context, key string) error
}

// DirectUpload is a presigned HTML form the browser posts a file to: the
// fields go first, then the file as "file".
type DirectUpload struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
}

// DirectUploader is implemented by backends the browser can upload to
// directly (S3), so large files never pass through the API. Upload keys live
// in their own namespace, separate from result keys.
type DirectUploader interface {
	// PresignUpload returns a form that accepts one file of 1..maxBytes bytes
	// under key until expiry.
	PresignUpload(ctx context.Context, key string, maxBytes int64, expiry time.Duration) (*DirectUpload, error)

	// UploadSize returns the size of an uploaded file, or ErrNotFound.
	UploadSize(ctx context.Context, key string) (int64, error)

	// FetchUpload copies an uploaded file to localPath.
	FetchUpload(ctx context.Context, key, localPath string) error

	DeleteUpload(ctx context.Context, key string) error
}
