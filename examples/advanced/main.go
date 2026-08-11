package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/castai/logging"
	"github.com/castai/logging/components"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logging.New(
		logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:     logging.MustParseLevel("info"),
			Output:    os.Stdout,
			AddSource: false,
		}),

		// Force UTC regardless of the process timezone. Swap for any
		// *time.Location, e.g. time.LoadLocation("Europe/Vilnius").
		// You can also set your preferred timezone via `LOG_TIMEZONE = Europe/Vilnius` env variable.
		logging.NewTimeZoneHandler(time.UTC),

		// Attaches "commit" to every record
		logging.NewCommitHandler(),
	)

	log.Info("service starting")

	// create API client for sending logs to remote
	apiClient, err := components.NewAPIClient(components.Config{
		APIBaseURL:          "http://localhost:8090",
		APIKey:              "test",
		ClusterID:           "exp",
		Component:           "example-app",
		Version:             "v0",
		TLSCert:             "",
		MaxRetries:          3,
		MaxRetryBackoffWait: 3 * time.Second,
	})
	if err != nil {
		log.WithError(err).Error("export disabled: failed to create api client")
	} else {
		// create batched client
		batchClient := components.NewBatchClient(apiClient,
			components.FlushInterval(30*time.Second),
			components.BatchSize(1000),
		)
		go func() { _ = batchClient.Run(ctx) }()

		log = log.WithHandler(logging.NewExportHandler(batchClient, logging.DefaultExportHandlerConfig))
		logging.RegisterExitHandler(logging.NewDefaultExitHandler(log, batchClient, 5*time.Second))
	}

	log.WithGroup("server").With("port", 8080).Info("listening")

	log.Debug("debug msg")
	log.WithFields(map[string]any{
		"key": "value",
	}).Warn("warn msg")

	log.WithError(errors.New("some fatal error")).Fatal("fatal error message, shutting down")
}
