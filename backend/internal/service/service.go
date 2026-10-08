package service

import (
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type ServiceDependencies struct {
	Adapter *adapter.Adapters
	Jobs    *JobManager
	Uploads *Uploads // nil when storage can't take direct uploads
}

type Services struct {
	Adapter      *adapter.Adapters
	Jobs         *JobManager
	Uploads      *Uploads
	Capabilities compress.Capabilities
}

func NewService(deps ServiceDependencies) *Services {
	return &Services{
		Adapter:      deps.Adapter,
		Jobs:         deps.Jobs,
		Uploads:      deps.Uploads,
		Capabilities: deps.Adapter.FFmpeg.Capabilities(),
	}
}
