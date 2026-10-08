package service

import (
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type ServiceDependencies struct {
	Adapter *adapter.Adapters
	Jobs    *JobManager
}

type Services struct {
	Adapter      *adapter.Adapters
	Jobs         *JobManager
	Capabilities compress.Capabilities
}

func NewService(deps ServiceDependencies) *Services {
	return &Services{
		Adapter:      deps.Adapter,
		Jobs:         deps.Jobs,
		Capabilities: deps.Adapter.FFmpeg.Capabilities(),
	}
}
