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
