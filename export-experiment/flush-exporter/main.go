package main

import (
	"context"
	"time"

	"github.com/castai/logging"
	"github.com/castai/logging/components"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logging.New()

	apiClient, err := components.NewAPIClient(components.Config{
		APIBaseURL: "http://localhost:8090",
		APIKey:     "test",
		ClusterID:  "exp",
		Component:  "flush-exporter",
		Version:    "v0",
	})
	if err != nil {
		panic(err)
	}

	batchClient := components.NewBatchClient(apiClient,
		components.FlushInterval(30*time.Second),
		components.BatchSize(1000),
	)
	go func() { _ = batchClient.Run(ctx) }()

	log = log.WithHandler(logging.NewExportHandler(batchClient, logging.DefaultExportHandlerConfig))
	log.RegisterExitHandler(logging.NewDefaultExitHandler(log, batchClient, 5*time.Second))

	for i := 1; i <= 5; i++ {
		log.Infof("flush-exporter message %d", i)
	}

	log.Fatal("flush-exporter fatal exit")
}
