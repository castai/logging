package logging_test

import (
	"log/slog"
	"testing"

	"github.com/castai/logging"
	"github.com/stretchr/testify/require"
)

func TestExportHandler(t *testing.T) {
	r := require.New(t)

	exportHandler := logging.NewExportHandler(logging.ExportHandlerConfig{
		MinLevel:   slog.LevelWarn,
		BufferSize: 2,
	})
	log := logging.New(exportHandler)

	log.Debug("msg1")
	log.Info("msg2")
	log.Warn("msg3")
	log.WithField("k", "v").Error("msg4")
	log.WithGroup("g").Error("msg5")

	// Only warn and error should be inside the export channel.
	msg1 := <-exportHandler.Records()
	r.Equal("msg3", msg1.Message)
	msg2 := <-exportHandler.Records()
	r.Equal("msg4", msg2.Message)

	// Ensure logs are not blocked.
	log.Debug("msg5")
}
