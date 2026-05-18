package adapter

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress"
	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	storageDomain "github.com/umohsamuel/elcompresso/internal/domain/storage"
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type AdapterDependencies struct {
	EnvironmentVariables *env.EnvironmentVariables
	Compressor           *compress.CompressorDependencies
	StorageClient        *s3.Client
}

type Adapters struct {
	EnvironmentVariables *env.EnvironmentVariables
	Compressor           *compress.Compressors
	Storage              storageDomain.Storage
}

func NewAdapter(deps AdapterDependencies) *Adapters {
	return &Adapters{
		EnvironmentVariables: deps.EnvironmentVariables,
		Compressor:           compress.NewCompressors(*deps.Compressor),
		Storage: storage.NewStorageClient(storage.StgDeps{
			Client: deps.StorageClient,
			Env:    *deps.EnvironmentVariables,
		}),
	}
}
