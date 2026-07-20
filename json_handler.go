package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type JSONHandlerConfig struct {
	Level     slog.Level
	Output    io.Writer
	AddSource bool
}

// NewJSONHandler returns a slog JSON handler. It is a thin wrapper around
// slog.NewJSONHandler that plugs into the logging.Handler chain and trims
// the source path to its basename (matching the text handler).
func NewJSONHandler(cfg JSONHandlerConfig) Handler {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		// Remove the directory from the source's filename.
		if cfg.AddSource {
			if a.Key == slog.SourceKey {
				if v, ok := a.Value.Any().(*slog.Source); ok {
					v.File = filepath.Base(v.File)
				}
			}
		}

		return a
	}

	return HandlerFunc(func(_ slog.Handler) slog.Handler {
		return slog.NewJSONHandler(out, &slog.HandlerOptions{
			AddSource:   cfg.AddSource,
			Level:       cfg.Level,
			ReplaceAttr: replaceAttr,
		})
	})
}
