package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

type HandlerFunc func(next slog.Handler) slog.Handler

func (h HandlerFunc) Register(next slog.Handler) slog.Handler {
	return h(next)
}

// Handler allows to chain multiple handlers.
// Order of execution is reverse to order of registration meaning first handler is executed last.
// Please make sure to check existing env variables like `JSON_LOG`.
type Handler interface {
	Register(next slog.Handler) slog.Handler
}

// defaultBaseHandler returns the base format handler used when the caller
// passes no handlers to New. JSON_LOG=true selects JSON; anything else keeps
// the text handler.
func defaultBaseHandler(useJSON bool) Handler {
	if useJSON {
		return NewJSONHandler(JSONHandlerConfig{
			Level:     slog.LevelInfo,
			Output:    os.Stdout,
			AddSource: false,
		})
	}

	return NewTextHandler(TextHandlerConfig{
		Level:     slog.LevelInfo,
		Output:    os.Stdout,
		AddSource: false,
	})
}

func MustParseLevel(lvlStr string) slog.Level {
	var lvl slog.Level
	err := lvl.UnmarshalText([]byte(lvlStr))
	if err != nil {
		panic("parsing log level from level string " + lvlStr)
	}
	return lvl
}

// New returns a new Logger.
// Default output format is Text, unless you have either passed NewJSONHandler() or set `JSON_LOG = true` env variable.
func New(handlers ...Handler) *Logger {
	isJSONSet := envJSONLog()

	if len(handlers) == 0 {
		handlers = []Handler{defaultBaseHandler(isJSONSet)}
	}

	if tz := envTimeZone(); tz != nil {
		handlers = append(handlers, NewTimeZoneHandler(tz))
	}

	slogHandler := chain(handlers)
	if slogHandler == nil {
		// Auto-insert base when only decorator handlers were given
		handlers = append([]Handler{defaultBaseHandler(isJSONSet)}, handlers...)
		slogHandler = chain(handlers)
	}

	log := slog.New(slogHandler)
	return &Logger{Log: log}
}

func chain(handlers []Handler) slog.Handler {
	var h slog.Handler
	for _, handler := range handlers {
		h = handler.Register(h)
	}

	return h
}

// Logger is a small wrapper around slog with some extra methods
// for easier migration from logrus.
type Logger struct {
	Log *slog.Logger

	// traceAttached records whether trace_id/span_id fields have already
	// been attached to this logger by attachTraceFields.
	traceAttached bool
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

func (l *Logger) Fatalf(msg string, a ...any) {
	l.doLog(slog.LevelError, msg, a...) //nolint:govet
	os.Exit(1)
}

// Println logs its arguments at error level, joined and spaced the same
// way the standard library's *log.Logger.Println does (via fmt.Sprintln,
// trailing newline trimmed since handlers terminate lines themselves).
//
// This exists so *Logger structurally satisfies the single-method
// `Println(v ...any)` interfaces several third-party packages expect —
// most notably promhttp.Logger (github.com/prometheus/client_golang's
// promhttp.HandlerOpts.ErrorLog), which logrus.Entry/Logger satisfy today
// only incidentally, via their own Println method. promhttp only ever
// calls it for genuine scrape/collection errors, hence error level here —
// this diverges from logrus, whose Println always logs at Info.
func (l *Logger) Println(v ...any) {
	l.doLog(slog.LevelError, strings.TrimSuffix(fmt.Sprintln(v...), "\n"))
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
		// Workaround to ignore go vet, see https://github.com/golang/go/issues/60529
		var format = fmt.Sprintf
		formatted := format(msg, args...)
		r := slog.NewRecord(time.Now(), lvl, formatted, pcs[0])
		_ = l.Log.Handler().Handle(ctx, r) //nolint:contextcheck
	} else {
		r := slog.NewRecord(time.Now(), lvl, msg, pcs[0])
		_ = l.Log.Handler().Handle(ctx, r) //nolint:contextcheck
	}
}

// With returns a derived logger with the given slog-style args attached.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Log: l.Log.With(args...), traceAttached: l.traceAttached}
}

// WithField returns a derived logger with a single string-valued field.
// For non-string values use WithFieldAny.
func (l *Logger) WithField(k, v string) *Logger {
	return &Logger{Log: l.Log.With(slog.String(k, v)), traceAttached: l.traceAttached}
}

// WithFieldAny returns a derived logger with a single field whose value may
// be of any type. Values are handled by slog's default attribute resolution.
func (l *Logger) WithFieldAny(k string, v any) *Logger {
	return &Logger{Log: l.Log.With(slog.Any(k, v)), traceAttached: l.traceAttached}
}

// WithFields returns a derived logger with all entries of the given map
// attached as attributes.
func (l *Logger) WithFields(fields map[string]any) *Logger {
	if len(fields) == 0 {
		return l
	}

	attrs := make([]any, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	return &Logger{Log: l.Log.With(attrs...), traceAttached: l.traceAttached}
}

// WithGroup returns a derived logger whose subsequent attributes are grouped
// under the given name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Log: l.Log.WithGroup(name), traceAttached: l.traceAttached}
}
