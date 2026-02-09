package logging_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/castai/logging"
	"github.com/castai/logging/components"
)

func TestExportHandler(t *testing.T) {
	r := require.New(t)

	client := &apiClient{}
	exportHandler := logging.NewExportHandler(client, logging.DefaultExportHandlerConfig)
	log := logging.New(exportHandler)

	log.Info("msg1")
	log.Warn("msg2")
	log.WithField("k", "v").Error("msg3")
	groupLogger := log.WithGroup("g")
	groupLogger.WithField("k2", "v2").Error("msg4")
	log.Debug("msg5 should not send")

	r.Len(client.logs, 4)
	log1 := client.logs[0]
	r.Equal("msg1", log1.Message)
	r.Equal("LOG_LEVEL_INFO", log1.Level)
	r.Empty(log1.Fields)
	r.NotEmpty(log1.Time)

	log2 := client.logs[1]
	r.Equal("msg2", log2.Message)
	r.Equal("LOG_LEVEL_WARNING", log2.Level)
	r.Empty(log2.Fields)
	r.NotEmpty(log2.Time)

	log3 := client.logs[2]
	r.Equal("msg3", log3.Message)
	r.Equal("LOG_LEVEL_ERROR", log3.Level)
	r.Equal(map[string]string{"k": "v"}, log3.Fields)
	r.NotEmpty(log3.Time)

	log4 := client.logs[3]
	r.Equal("msg4", log4.Message)
	r.Equal("LOG_LEVEL_ERROR", log4.Level)
	r.Equal(map[string]string{"g.k2": "v2"}, log4.Fields)
	r.NotEmpty(log4.Time)
}

type apiClient struct {
	logs []components.Entry
}

func (a *apiClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	a.logs = append(a.logs, entries...)
	return nil
}
