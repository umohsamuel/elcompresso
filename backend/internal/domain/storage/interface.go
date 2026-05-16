package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, key string, reader io.Reader) (url string, err error)
	GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}
