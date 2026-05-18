package main

import (
	"context"
	"log"

	"github.com/umohsamuel/elcompresso/cmd/api"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/util"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	s3Client := s3.NewFromConfig(cfg)

	adapterDependencies := adapter.AdapterDependencies{
		EnvironmentVariables: environmentVariables,
		Compressor:           &compress.CompressorDependencies{},
		StorageClient:        s3Client,
	}

	adapters := adapter.NewAdapter(adapterDependencies)

	serviceDependencies := service.ServiceDependencies{
		Adapter: adapters,
	}

	services := service.NewService(serviceDependencies)

	r := api.API(services, environmentVariables)
	r.Engine.Run(environmentVariables.Port)
}
