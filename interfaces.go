package logging

import "log/slog"

// Fields is a convenience alias for a bag of structured log fields.
// It matches the shape used by callers migrating from logrus and from
// github.com/castai/kubecast/lib/logging.
type Fields = map[string]any

// BaseLogger is the minimum surface needed by callers that only emit logs
// without further derivation. All methods mirror the corresponding *Logger
// methods.
type BaseLogger interface {
	Debug(msg string)
	Debugf(format string, args ...any)
	Info(msg string)
	Infof(format string, args ...any)
	Warn(msg string)
	Warnf(format string, args ...any)
	Error(msg string)
	Errorf(format string, args ...any)
	Fatal(msg string)
	Fatalf(format string, args ...any)
	IsEnabled(lvl slog.Level) bool
}

// FieldsLogger is the interface most consumer packages should depend on.
// It extends BaseLogger with derivation methods that return the same
// interface, so tests can substitute a fake implementation.
type FieldsLogger interface {
	BaseLogger
	With(args ...any) FieldsLogger
	WithField(key, value string) FieldsLogger
	WithFieldAny(key string, value any) FieldsLogger
	WithFields(fields map[string]any) FieldsLogger
	WithGroup(name string) FieldsLogger
}

// FieldLogger is a compatibility alias for callers migrating from
// github.com/castai/kubecast/lib/logging, which distinguished FieldLogger
// (logrus-style) from FieldsLogger. Under this package they are the same.
type FieldLogger = FieldsLogger

// Compile-time assertion that *Logger satisfies the interfaces.
var (
	_ BaseLogger   = (*Logger)(nil)
	_ FieldsLogger = (*Logger)(nil)
	_ FieldLogger  = (*Logger)(nil)
)
