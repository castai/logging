package logging_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
	"github.com/castai/logging/components"
)

func TestExportHandler(t *testing.T) {
	r := require.New(t)

	client := &apiClient{}
	text := logging.NewTextHandler(logging.TextHandlerConfig{
		Level: slog.LevelDebug,
	})
	exportHandler := logging.NewExportHandler(client, logging.DefaultExportHandlerConfig)
	log := logging.New(text, exportHandler)

	log.Info("msg1")
	log.Warn("msg2")
	log.WithField("k", "v").Error("msg3")
	groupLogger := log.WithGroup("g")
	groupLogger.WithField("k2", "v2").Error("msg4")
	log.Debug("msg5 should not send")
	logWith := log.With(slog.String("k3", "v3"))
	logWith.Info("msg6")
	logWithNested := logWith.With(slog.String("k4", "v4"))
	logWithNested.Info("msg7")

	r.Len(client.logs, 6)
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

	log5 := client.logs[4]
	r.Equal("msg6", log5.Message)
	r.Equal("LOG_LEVEL_INFO", log5.Level)
	r.Equal(map[string]string{"k3": "v3"}, log5.Fields)
	r.NotEmpty(log5.Time)

	log6 := client.logs[5]
	r.Equal("msg7", log6.Message)
	r.Equal("LOG_LEVEL_INFO", log6.Level)
	r.Equal(map[string]string{"k3": "v3", "k4": "v4"}, log6.Fields)
	r.NotEmpty(log6.Time)
}

func TestExportHandler_Flush(t *testing.T) {
	t.Run("no-op when apiClient does not implement Flush", func(t *testing.T) {
		r := require.New(t)
		exportHandler := logging.NewExportHandler(&apiClient{}, logging.DefaultExportHandlerConfig)
		r.NoError(exportHandler.Flush(context.Background()))
	})

	t.Run("delegates to apiClient's Flush when it implements one", func(t *testing.T) {
		r := require.New(t)
		client := &flushableAPIClient{}
		exportHandler := logging.NewExportHandler(client, logging.DefaultExportHandlerConfig)

		r.NoError(exportHandler.Flush(context.Background()))
		r.True(client.flushed)
	})

	t.Run("propagates the underlying Flush error", func(t *testing.T) {
		r := require.New(t)
		client := &flushableAPIClient{err: errors.New("boom")}
		exportHandler := logging.NewExportHandler(client, logging.DefaultExportHandlerConfig)

		r.ErrorIs(exportHandler.Flush(context.Background()), client.err)
	})
}

type apiClient struct {
	logs []components.Entry
}

func (a *apiClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	a.logs = append(a.logs, entries...)
	return nil
}

type flushableAPIClient struct {
	apiClient
	flushed bool
	err     error
}

func (a *flushableAPIClient) Flush(ctx context.Context) error {
	a.flushed = true
	return a.err
}
