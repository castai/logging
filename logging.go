package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"
)

type HandlerFunc func(next slog.Handler) slog.Handler

func (h HandlerFunc) Register(next slog.Handler) slog.Handler {
	return h(next)
}

// Handler allows to chain multiple handlers.
// Order of execution is reverse to order of registration meaning first handler is executed last.
type Handler interface {
	Register(next slog.Handler) slog.Handler
}

func MustParseLevel(lvlStr string) slog.Level {
	var lvl slog.Level
	err := lvl.UnmarshalText([]byte(lvlStr))
	if err != nil {
		panic("parsing log level from level string " + lvlStr)
	}
	return lvl
}

func New(handlers ...Handler) *Logger {
	if len(handlers) == 0 {
		handlers = []Handler{
			NewTextHandler(TextHandlerConfig{
				Level:     slog.LevelInfo,
				Output:    os.Stdout,
				AddSource: false,
			}),
		}
	}

	// Chain handlers. Execution is in reverse order.
	var slogHandler slog.Handler
	for _, handler := range handlers {
		slogHandler = handler.Register(slogHandler)
	}

	log := slog.New(slogHandler)
	return &Logger{Log: log}
}

// Logger is a small wrapper around slog with some extra methods
// for easier migration from logrus.
type Logger struct {
	Log *slog.Logger
}

func (l *Logger) Error(msg string) {
	l.doLog(slog.LevelError, msg) //nolint:govet
}

func (l *Logger) Errorf(format string, a ...any) {
	l.doLog(slog.LevelError, format, a...)
}

func (l *Logger) Infof(format string, a ...any) {
	l.doLog(slog.LevelInfo, format, a...)
}

func (l *Logger) Info(msg string) {
	l.doLog(slog.LevelInfo, msg) //nolint:govet
}

func (l *Logger) Debug(msg string) {
	l.doLog(slog.LevelDebug, msg) //nolint:govet
}

func (l *Logger) Debugf(format string, a ...any) {
	l.doLog(slog.LevelDebug, format, a...)
}

func (l *Logger) Warn(msg string) {
	l.doLog(slog.LevelWarn, msg) //nolint:govet
}

func (l *Logger) Warnf(format string, a ...any) {
	l.doLog(slog.LevelWarn, format, a...)
}

func (l *Logger) Fatal(msg string) {
	l.doLog(slog.LevelError, msg) //nolint:govet
	os.Exit(1)
}

func (l *Logger) IsEnabled(lvl slog.Level) bool {
	ctx := context.Background()
	return l.Log.Handler().Enabled(ctx, lvl)
}

func (l *Logger) doLog(lvl slog.Level, msg string, args ...any) {
	ctx := context.Background()
	if !l.Log.Handler().Enabled(ctx, lvl) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	if len(args) > 0 {
		r := slog.NewRecord(time.Now(), lvl, fmt.Sprintf(msg, args...), pcs[0])
		_ = l.Log.Handler().Handle(ctx, r) //nolint:contextcheck
	} else {
		r := slog.NewRecord(time.Now(), lvl, msg, pcs[0])
		_ = l.Log.Handler().Handle(ctx, r) //nolint:contextcheck
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{Log: l.Log.With(args...)}
}

func (l *Logger) WithField(k, v string) *Logger {
	return &Logger{Log: l.Log.With(slog.String(k, v))}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Log: l.Log.WithGroup(name)}
}
