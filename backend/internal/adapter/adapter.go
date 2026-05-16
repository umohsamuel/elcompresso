package adapter

import (
	"github.com/umohsamuel/elcompresso/internal/adapter/compressor"
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type AdapterDependencies struct {
	EnvironmentVariables *env.EnvironmentVariables
	Compressor           *compressor.CompressorDependencies
}

type Adapters struct {
	EnvironmentVariables *env.EnvironmentVariables
	Compressor           *compressor.Compressors
}

func NewAdapter(deps AdapterDependencies) *Adapters {
	return &Adapters{
		EnvironmentVariables: deps.EnvironmentVariables,
		Compressor:           compressor.NewCompressors(*deps.Compressor),
	}
}
