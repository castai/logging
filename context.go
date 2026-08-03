package logging

import (
	"context"
	"sync"
)

type loggerCtxKey struct{}

var (
	defaultLoggerOnce sync.Once
	defaultLogger     *Logger
)

func defaultCtxLogger() *Logger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = New()
	})
	return defaultLogger
}

// WithLogger stores logger in a derived context. The stored logger is
// retrievable with FromContext.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

// FromContext returns the logger previously stored with WithLogger, or a
// package-level default if none is present. If a TraceSpanExtractor is
// registered and returns non-empty IDs for ctx, trace_id and span_id fields
// are attached to the returned logger.
func FromContext(ctx context.Context) *Logger {
	base := loggerFromCtx(ctx)
	return maybeAttachTrace(ctx, base)
}

// FromContextWithField retrieves the logger from ctx, derives a new logger
// with the additional field, stores it in a derived context, and returns both.
func FromContextWithField(ctx context.Context, key string, value any) (context.Context, *Logger) {
	base := loggerFromCtx(ctx)
	derived := base.WithFieldAny(key, value)
	derived = maybeAttachTrace(ctx, derived)
	return WithLogger(ctx, derived), derived
}

// FromContextWithFields is the multi-field variant of FromContextWithField.
func FromContextWithFields(ctx context.Context, fields map[string]any) (context.Context, *Logger) {
	base := loggerFromCtx(ctx)
	derived := base.WithFields(fields)
	derived = maybeAttachTrace(ctx, derived)
	return WithLogger(ctx, derived), derived
}

// loggerFromCtx returns the logger stored in ctx, or the package default.
func loggerFromCtx(ctx context.Context) *Logger {
	if ctx != nil {
		if v := ctx.Value(loggerCtxKey{}); v != nil {
			if l, ok := v.(*Logger); ok && l != nil {
				return l
			}
		}
	}
	return defaultCtxLogger()
}

// maybeAttachTrace returns a logger with trace_id/span_id fields attached
// when a TraceSpanExtractor is registered and yields non-empty IDs for ctx.
// It returns the input logger unchanged otherwise (no allocation).
func maybeAttachTrace(ctx context.Context, l *Logger) *Logger {
	return attachTraceFields(ctx, l)
}

// Package-level convenience helpers that combine FromContext + a log call.

// Debugf logs at debug level using the logger stored in ctx.
func Debugf(ctx context.Context, format string, args ...any) {
	FromContext(ctx).Debugf(format, args...)
}

// Debug logs at debug level using the logger stored in ctx.
func Debug(ctx context.Context, msg string) {
	FromContext(ctx).Debug(msg)
}

// Infof logs at info level using the logger stored in ctx.
func Infof(ctx context.Context, format string, args ...any) {
	FromContext(ctx).Infof(format, args...)
}

// Info logs at info level using the logger stored in ctx.
func Info(ctx context.Context, msg string) {
	FromContext(ctx).Info(msg)
}

// Warnf logs at warn level using the logger stored in ctx.
func Warnf(ctx context.Context, format string, args ...any) {
	FromContext(ctx).Warnf(format, args...)
}

// Warn logs at warn level using the logger stored in ctx.
func Warn(ctx context.Context, msg string) {
	FromContext(ctx).Warn(msg)
}

// Errorf logs at error level using the logger stored in ctx.
func Errorf(ctx context.Context, format string, args ...any) {
	FromContext(ctx).Errorf(format, args...)
}

// Error logs at error level using the logger stored in ctx.
func Error(ctx context.Context, msg string) {
	FromContext(ctx).Error(msg)
}
