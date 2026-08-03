package logging

import "log/slog"

// Fields is a convenience alias for a bag of structured log fields.
// Used for Logrus compatibility.
type Fields = map[string]any

type FieldsLogger interface {
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

	With(args ...any) *Logger
	WithField(key, value string) *Logger
	WithFieldAny(key string, value any) *Logger
	WithFields(fields map[string]any) *Logger
	WithGroup(name string) *Logger
}

var _ FieldsLogger = (*Logger)(nil)
