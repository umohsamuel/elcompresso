package main

import (
	"log"

	"github.com/umohsamuel/elcompresso/cmd/api"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/adapter/compressor"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/util"
)

var (
	environmentVariables = env.LoadEnvironment()
	serverID             string
)

func init() {
	serverID = util.GetServerID()
	log.Println("Server ID:", serverID)
}

func main() {

	adapterDependencies := adapter.AdapterDependencies{
		EnvironmentVariables: environmentVariables,
		Compressor:           &compressor.CompressorDependencies{},
	}

	adapters := adapter.NewAdapter(adapterDependencies)

	serviceDependencies := service.ServiceDependencies{
		Adapter: adapters,
	}

	services := service.NewService(serviceDependencies)

	r := api.API(services, environmentVariables)
	r.Engine.Run(environmentVariables.Port)
}
