package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
)

func TestTimeZoneHandler(t *testing.T) {
	t.Run("converts record time to UTC in text output", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewTextHandler(logging.TextHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewTimeZoneHandler(time.UTC),
		)
		log.Info("msg")
		r.Regexp(regexp.MustCompile(`time=\S+Z `), buf.String())
	})

	t.Run("converts record time in JSON output", func(t *testing.T) {
		r := require.New(t)
		loc, err := time.LoadLocation("Europe/Vilnius")
		r.NoError(err)

		var buf bytes.Buffer
		log := logging.New(
			logging.NewJSONHandler(logging.JSONHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewTimeZoneHandler(loc),
		)
		log.Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		ts, _ := m["time"].(string)
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		r.NoError(err)

		_, wantOffset := time.Now().In(loc).Zone()
		_, gotOffset := parsed.Zone()
		r.Equal(wantOffset, gotOffset)
	})

	t.Run("nil location is a no-op", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewTextHandler(logging.TextHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewTimeZoneHandler(nil),
		)
		log.Info("msg")
		r.Contains(buf.String(), "msg=msg")
	})

	t.Run("preserves WithGroup and WithAttrs", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewJSONHandler(logging.JSONHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewTimeZoneHandler(time.UTC),
		)
		log.WithGroup("g").With("k", "v").Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		g, ok := m["g"].(map[string]any)
		r.True(ok, "expected group 'g', got %T", m["g"])
		r.Equal("v", g["k"])
	})
}
