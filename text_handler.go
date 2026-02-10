package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var DefaultTextHandlerConfig = TextHandlerConfig{
	Level:     MustParseLevel("INFO"),
	Output:    os.Stdout,
	AddSource: true,
}

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
	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			switch a.Value.Any().(type) {
			case slog.Level:
				a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
			}
		}

		// Remove the directory from the source's filename.
		if cfg.AddSource {
			if a.Key == slog.SourceKey {
				switch v := a.Value.Any().(type) {
				case *slog.Source:
					v.File = filepath.Base(v.File)
				}
			}
		}
		return a
	}

	return HandlerFunc(func(_ slog.Handler) slog.Handler {
		return slog.NewTextHandler(out, &slog.HandlerOptions{
			AddSource:   cfg.AddSource,
			Level:       cfg.Level,
			ReplaceAttr: replaceAttr,
		})
	})
}
