package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
)

func TestJSONHandler(t *testing.T) {
	t.Run("emits valid JSON with level and msg", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:  slog.LevelInfo,
			Output: &buf,
		}))

		log.Info("hello")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		r.Equal("INFO", m["level"])
		r.Equal("hello", m["msg"])
	})

	t.Run("AddSource emits basename-only source.file", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:     slog.LevelInfo,
			Output:    &buf,
			AddSource: true,
		}))

		log.Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		src, ok := m["source"].(map[string]any)
		r.True(ok, "source must be an object, got %T", m["source"])
		file, _ := src["file"].(string)
		r.NotContains(file, "/", "file should be basename only, got %q", file)
		r.True(strings.HasSuffix(file, ".go"), "file should end with .go, got %q", file)
	})
}
