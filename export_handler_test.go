package logging_test

import (
	"bytes"
	"context"
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

type apiClient struct {
	logs []components.Entry
}

func (a *apiClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	a.logs = append(a.logs, entries...)
	return nil
}

func TestExportHandler_MinLevelBelowBaseLevel(t *testing.T) {
	r := require.New(t)

	var buf bytes.Buffer
	client := &apiClient{}
	text := logging.NewTextHandler(logging.TextHandlerConfig{
		Level:  slog.LevelInfo,
		Output: &buf,
	})
	export := logging.NewExportHandler(client, logging.ExportHandlerConfig{
		MinLevel: slog.LevelDebug,
	})
	log := logging.New(text, export)

	log.Debug("debug msg")
	log.Info("info msg")
	log.Warn("warn msg")

	// The remote server receives everything from debug and up, even though
	// the stdout handler sits at info.
	r.Len(client.logs, 3)
	r.Equal("LOG_LEVEL_DEBUG", client.logs[0].Level)
	r.Equal("debug msg", client.logs[0].Message)

	// Stdout only sees info and up: the debug record must not leak into the
	// text output.
	r.Contains(buf.String(), "info msg")
	r.Contains(buf.String(), "warn msg")
	r.NotContains(buf.String(), "debug msg")
}

type denyAllHandler struct {
	handled int
}

func (d *denyAllHandler) Enabled(_ context.Context, _ slog.Level) bool { return false }

func (d *denyAllHandler) Handle(_ context.Context, _ slog.Record) error {
	d.handled++
	return nil
}

func (d *denyAllHandler) WithAttrs(_ []slog.Attr) slog.Handler { return d }

func (d *denyAllHandler) WithGroup(_ string) slog.Handler { return d }

func TestExportHandler_HonorsNextEnabledBeforeForwarding(t *testing.T) {
	r := require.New(t)

	client := &apiClient{}
	deny := &denyAllHandler{}
	export := logging.NewExportHandler(client, logging.ExportHandlerConfig{
		MinLevel: slog.LevelDebug,
	})
	log := logging.New(
		logging.HandlerFunc(func(_ slog.Handler) slog.Handler { return deny }),
		export,
	)

	log.Debug("debug msg")
	log.Info("info msg")

	// Both records are exported (MinLevel=debug)...
	r.Len(client.logs, 2)
	// ...but the handler denying them via Enabled never sees Handle calls.
	r.Zero(deny.handled)
}
