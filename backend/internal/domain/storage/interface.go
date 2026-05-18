package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, filename string, file io.Reader) (string, error)

	GenerateDownloadURL(ctx context.Context, filename string, expiry time.Duration) (string, error)
}
