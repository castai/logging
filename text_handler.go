package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type TextHandlerConfig struct {
	Level     slog.Level
	Output    io.Writer
	AddSource bool
}

// NewTextHandler returns slog text handler.
func NewTextHandler(cfg TextHandlerConfig) Handler {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	var replace func(groups []string, a slog.Attr) slog.Attr
	if cfg.AddSource {
		replace = func(groups []string, a slog.Attr) slog.Attr {
			// Remove the directory from the source's filename.
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				source.File = filepath.Base(source.File)
			}
			return a
		}
	}

	return HandlerFunc(func(_ slog.Handler) slog.Handler {
		return slog.NewTextHandler(out, &slog.HandlerOptions{
			AddSource:   cfg.AddSource,
			Level:       cfg.Level,
			ReplaceAttr: replace,
		})
	})
}
