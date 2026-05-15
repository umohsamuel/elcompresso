package adapter

import (
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type AdapterDependencies struct {
	EnvironmentVariables *env.EnvironmentVariables
}

type Adapters struct {
	EnvironmentVariables *env.EnvironmentVariables
}

func NewAdapter(deps AdapterDependencies) *Adapters {
	return &Adapters{
		EnvironmentVariables: deps.EnvironmentVariables,
	}
}
