package logging_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
)

func TestTextHandler(t *testing.T) {
	r := require.New(t)

	t.Run("text handler log", func(t *testing.T) {
		var buf bytes.Buffer
		log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
			Level:  logging.MustParseLevel("debug"),
			Output: io.MultiWriter(&buf, os.Stdout),
		}))

		log.Debug("msg1")
		log.Info("msg2")
		log.Error("msg3")
		log.Warn("msg4")

		r.Contains(buf.String(), "level=debug msg=msg1")
		r.Contains(buf.String(), "level=info msg=msg2")
		r.Contains(buf.String(), "level=error msg=msg3")
		r.Contains(buf.String(), "level=warn msg=msg4")
	})

	t.Run("text handler with source lines", func(t *testing.T) {
		var buf bytes.Buffer
		log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
			Level:     logging.MustParseLevel("DEBUG"),
			Output:    io.MultiWriter(&buf, os.Stdout),
			AddSource: true,
		}))

		// This log should output log with source line.
		log.Info("msg1")

		// Some slog wrappers like the one in k8s runtime utils can override source field.
		// Normally this should be avoided, but we can't controll 3th party libraries.
		_ = log.Log.Handler().WithAttrs([]slog.Attr{
			{
				Key: slog.SourceKey,
				Value: slog.AnyValue(&struct {
					Random int64
				}{
					Random: 1234,
				}),
			},
		}).Handle(context.Background(), slog.Record{Message: "msg2"})

		r.Contains(buf.String(), "source=text_handler_test.go")
		r.Contains(buf.String(), "level=info source=.:0 msg=msg2")
	})
}
