package logging_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"testing"

	"github.com/castai/logging"
	"github.com/castai/logging/components"
)

func ExampleLogger() {
	ingestClient, err := components.NewAPIClient(components.Config{
		APIBaseURL: "https://api.cast.ai",
		APIKey:     "<api-key>",
		ClusterID:  "<cluster-id>",
		Component:  "castware",
		Version:    "<version>",
	})
	if err != nil {
		// Handle err ...
		return
	}

	var errg errgroup.Group

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	ingestClientBatchClient := components.NewBatchClient(ingestClient)
	errg.Go(func() error {
		return ingestClientBatchClient.Run(ctx)
	})

	errg.Go(func() error {
		text := logging.NewTextHandler(logging.DefaultTextHandlerConfig)
		export := logging.NewExportHandler(ingestClientBatchClient, logging.DefaultExportHandlerConfig)
		log := logging.New(text, export)

		// Log logs
		log.Infof("debug message with format value %s", "hello")
		log.WithField("component", "agent").Errorf("something failed: %v", "unknown")

		return nil
	})

	if err := errg.Wait(); err != nil {
		// Hanlde err.
	}
}

func TestLogger(t *testing.T) {
	t.Run("print logs with default options", func(t *testing.T) {
		log := logging.New()

		log.Errorf("something wrong: %v", errors.New("ups"))
		serverLog := log.WithField("component", "server")
		serverLog.Info("with component")
		serverLog.Info("more server logs")
	})

	t.Run("print logs with text handler", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
			Level:     logging.MustParseLevel("DEBUG"),
			Output:    io.MultiWriter(&buf, os.Stdout),
			AddSource: false,
		}))

		log.Errorf("something wrong: %v", errors.New("ups"))
		serverLog := log.WithField("component", "server")
		serverLog.Info("with component")
		serverLog.Info("more server logs")
		r.Contains(buf.String(), `level=ERROR msg="something wrong: ups"`)
		r.Contains(buf.String(), `level=INFO msg="with component" component=server`)
		r.Contains(buf.String(), `level=INFO msg="more server logs" component=server`)
	})

	t.Run("chain handlers", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		opts := []logging.Handler{
			logging.NewTextHandler(logging.TextHandlerConfig{
				Output: io.MultiWriter(&buf, os.Stdout),
				Level:  logging.MustParseLevel("DEBUG"),
			}),
			logging.HandlerFunc(func(next slog.Handler) slog.Handler {
				return &customHandler{name: "custom 1", next: next}
			}),
			logging.HandlerFunc(func(next slog.Handler) slog.Handler {
				return &customHandler{name: "custom 2", next: next}
			}),
			logging.HandlerFunc(func(next slog.Handler) slog.Handler {
				return &customHandler{name: "custom 3", next: next}
			}),
		}
		log := logging.New(opts...)

		log.Info("msg")
		log.WithField("k", "v").Debug("msg2")
		log.WithGroup("group").Debug("msg3")
		r.Contains(buf.String(), `level=INFO msg="msg custom 3 custom 2 custom 1"`)
	})
}

type customHandler struct {
	name string
	next slog.Handler
}

func (c *customHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.next.Enabled(ctx, level)
}

func (c *customHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Message += " " + c.name
	return c.next.Handle(ctx, record)
}

func (c *customHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c.next.WithAttrs(attrs)
}

func (c *customHandler) WithGroup(name string) slog.Handler {
	return c.next.WithGroup(name)
}
