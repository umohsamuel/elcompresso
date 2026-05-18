package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/umohsamuel/elcompresso/internal/domain/storage"
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type Stg struct {
	Client *s3.Client
	Env    env.EnvironmentVariables
}

type StgDeps struct {
	Client *s3.Client
	Env    env.EnvironmentVariables
}

func NewStorageClient(deps StgDeps) storage.Storage {
	return &Stg{
		Client: deps.Client,
		Env:    deps.Env,
	}
}

func (s *Stg) Upload(ctx context.Context, filename string, file io.Reader) (string, error) {

	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Env.S3.AWS_BUCKET),
		Key:    aws.String(filename),
		Body:   file,
		// ACL:    "public-read",
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Env.S3.AWS_BUCKET, s.Env.S3.AWS_REGION, filename), nil
}

func (s *Stg) GenerateDownloadURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	return "hello", nil
}

// func UploadLocal(filename string, file io.Reader) (string, error) {
// 	path := filepath.Join(env.UploadPath, filename)
// 	os.MkdirAll(env.UploadPath, os.ModePerm)

// 	out, err := os.Create(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer out.Close()

// 	_, err = io.Copy(out, file)
// 	if err != nil {
// 		return "", err
// 	}

// 	return fmt.Sprintf("/uploads/%s", filename), nil
// }
