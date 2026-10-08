package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
)

var ErrNotFound = errors.New("file not found or expired")

// ContentDisposition builds a header value that is safe for any file name,
// including non-ASCII ones (RFC 6266 / RFC 2231).
func ContentDisposition(name string, inline bool) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	if v := mime.FormatMediaType(kind, map[string]string{"filename": name}); v != "" {
		return v
	}
	return kind
}

// Local stores results on the server's disk.
type Local struct {
	dir string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &Local{dir: dir}, nil
}

func (l *Local) path(key string) (string, error) {
	p := filepath.Join(l.dir, filepath.FromSlash(key))
	if !strings.HasPrefix(p, filepath.Clean(l.dir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid storage key")
	}
	return p, nil
}

func (l *Local) Save(_ context.Context, key, localPath, _ string) error {
	dst, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	if err := os.Rename(localPath, dst); err == nil {
		return nil
	}
	// Rename fails across filesystems; fall back to copying.
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (l *Local) DownloadURL(context.Context, string, string, bool, time.Duration) (string, bool, error) {
	return "", false, nil
}

func (l *Local) Open(_ context.Context, key string) (*storage.Object, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &storage.Object{Body: f, ModTime: info.ModTime()}, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Keys are "<job id>/<file name>"; drop the job's folder once it is empty.
	if dir := filepath.Dir(p); dir != filepath.Clean(l.dir) {
		_ = os.Remove(dir)
	}
	return nil
}

// S3 stores results in a bucket and hands out short-lived presigned URLs. It
// also accepts uploads straight from the browser under uploadPrefix.
type S3 struct {
	client       *s3.Client
	presign      *s3.PresignClient
	bucket       string
	prefix       string
	uploadPrefix string
}

var _ storage.DirectUploader = (*S3)(nil)

func NewS3(client *s3.Client, bucket, prefix, uploadPrefix string) *S3 {
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: bucket, prefix: prefix, uploadPrefix: uploadPrefix}
}

func (s *S3) key(k string) string       { return s.prefix + k }
func (s *S3) uploadKey(k string) string { return s.uploadPrefix + k }

func (s *S3) Save(ctx context.Context, key, localPath, contentType string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.key(key)),
		Body:          f,
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload to s3: %w", err)
	}
	return os.Remove(localPath)
}

func (s *S3) DownloadURL(ctx context.Context, key, downloadName string, inline bool, expiry time.Duration) (string, bool, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(s.key(key)),
		ResponseContentDisposition: aws.String(ContentDisposition(downloadName, inline)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", false, fmt.Errorf("presign url: %w", err)
	}
	return req.URL, true, nil
}

func (s *S3) Open(context.Context, string) (*storage.Object, error) {
	return nil, errors.New("s3 objects are served via presigned urls")
}

func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(key))})
	return err
}

func (s *S3) PresignUpload(ctx context.Context, key string, maxBytes int64, expiry time.Duration) (*storage.DirectUpload, error) {
	req, err := s.presign.PresignPostObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.uploadKey(key)),
	}, func(o *s3.PresignPostOptions) {
		o.Expires = expiry
		// Enforced by S3 itself, so the size limit can't be bypassed.
		o.Conditions = []any{[]any{"content-length-range", 1, maxBytes}}
	})
	if err != nil {
		return nil, fmt.Errorf("presign upload: %w", err)
	}
	return &storage.DirectUpload{URL: req.URL, Fields: req.Values}, nil
}

func (s *S3) UploadSize(ctx context.Context, key string) (int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.uploadKey(key))})
	if err != nil {
		var nf *s3types.NotFound
		if errors.As(err, &nf) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("check upload: %w", err)
	}
	return aws.ToInt64(out.ContentLength), nil
}

func (s *S3) FetchUpload(ctx context.Context, key, localPath string) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.uploadKey(key))})
	if err != nil {
		var nk *s3types.NoSuchKey
		if errors.As(err, &nk) {
			return ErrNotFound
		}
		return fmt.Errorf("fetch upload: %w", err)
	}
	defer out.Body.Close()
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, out.Body); err != nil {
		f.Close()
		return fmt.Errorf("fetch upload: %w", err)
	}
	return f.Close()
}

func (s *S3) DeleteUpload(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.uploadKey(key))})
	return err
}
