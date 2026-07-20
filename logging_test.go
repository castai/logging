package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"testing"
	"time"

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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	errg, ctx := errgroup.WithContext(ctx)

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
			Level:     logging.MustParseLevel("debug"),
			Output:    io.MultiWriter(&buf, os.Stdout),
			AddSource: false,
		}))

		log.Errorf("something wrong: %v", errors.New("ups"))
		serverLog := log.WithField("component", "server")
		serverLog.Info("with component")
		serverLog.Info("more server logs")
		r.Contains(buf.String(), `level=error msg="something wrong: ups"`)
		r.Contains(buf.String(), `level=info msg="with component" component=server`)
		r.Contains(buf.String(), `level=info msg="more server logs" component=server`)
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
		log.With(slog.String("k1", "v1")).Error("msg4")
		r.Contains(buf.String(), `level=info msg="msg custom 3 custom 2 custom 1"`)
	})

	t.Run("print logs with JSON handler", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:  logging.MustParseLevel("info"),
			Output: io.MultiWriter(&buf, os.Stdout),
		}))

		log.WithField("component", "server").Info("hello")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		r.Equal("INFO", m["level"])
		r.Equal("hello", m["msg"])
		r.Equal("server", m["component"])
	})

	t.Run("timezone handler converts record time to UTC", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewJSONHandler(logging.JSONHandlerConfig{
				Level:  logging.MustParseLevel("info"),
				Output: &buf,
			}),
			logging.NewTimeZoneHandler(time.UTC),
		)

		log.Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		ts, _ := m["time"].(string)
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		r.NoError(err)
		_, offset := parsed.Zone()
		r.Equal(0, offset)
	})

	t.Run("LOG_TIMEZONE env applies to explicit JSON handler", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("JSON_LOG", "")
		t.Setenv("LOG_TIMEZONE", "Europe/Vilnius")

		var buf bytes.Buffer
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:  logging.MustParseLevel("info"),
			Output: &buf,
		}))

		log.Info("hello")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		ts, _ := m["time"].(string)
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		r.NoError(err)

		loc, err := time.LoadLocation("Europe/Vilnius")
		r.NoError(err)
		_, want := time.Now().In(loc).Zone()
		_, got := parsed.Zone()
		r.Equal(want, got)
	})

	t.Run("JSON_LOG env does not override explicit text handler", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("JSON_LOG", "true")
		t.Setenv("LOG_TIMEZONE", "")

		var buf bytes.Buffer
		log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
			Level:  logging.MustParseLevel("info"),
			Output: &buf,
		}))

		log.Info("hello")

		out := buf.String()
		r.Contains(out, "level=info")
		r.Contains(out, "msg=hello")
		r.NotContains(out, `"msg":"hello"`)
	})

	t.Run("invalid JSON_LOG env panics", func(t *testing.T) {
		t.Setenv("JSON_LOG", "notabool")
		t.Setenv("LOG_TIMEZONE", "")
		require.Panics(t, func() {
			logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
				Level:  logging.MustParseLevel("info"),
				Output: &bytes.Buffer{},
			}))
		})
	})

	t.Run("decorator-only handler chain gets default base handler", func(t *testing.T) {
		r := require.New(t)
		t.Setenv("JSON_LOG", "true")
		t.Setenv("LOG_TIMEZONE", "")

		// Only a decorator handler is passed. New must auto-insert the
		// default base handler (JSON, because JSON_LOG=true) so the chain
		// has something to terminate at.
		log := logging.New(logging.NewTimeZoneHandler(time.UTC))
		r.NotPanics(func() { log.Info("hi") })
	})

	t.Run("invalid LOG_TIMEZONE env panics", func(t *testing.T) {
		t.Setenv("JSON_LOG", "")
		t.Setenv("LOG_TIMEZONE", "Not/AZone")
		require.Panics(t, func() {
			logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
				Level:  logging.MustParseLevel("info"),
				Output: &bytes.Buffer{},
			}))
		})
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
