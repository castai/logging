package logging

import (
	"log/slog"
	"strings"
)

// LvlFromLogrus maps legacy logrus integer log lvl to the slog.Level.
// Please note that 0 maps to Info as it may be either panic or unset cfg value.
//
// 0=Panic -> slog.LevelInfo, 1=Fatal -> slog.LevelError, 2=Error -> slog.LevelError,
// 3=Warn -> slog.LevelWarn, 4=Info -> slog.LevelInfo, 5=Debug -> slog.LevelDebug.
func LvlFromLogrus(level int) slog.Level {
	if level < 0 {
		level = 0
	} else if level > 5 {
		level = 5
	}

	switch level {
	case 0: // logrus.PanicLevel + zero unset value
		return slog.LevelInfo
	case 1: // logrus.FatalLevel
		return slog.LevelError
	case 2: // logrus.ErrorLevel
		return slog.LevelError
	case 3: // logrus.WarnLevel
		return slog.LevelWarn
	case 4: // logrus.InfoLevel
		return slog.LevelInfo
	case 5: // logrus.DebugLevel
		return slog.LevelDebug
	default:
		return slog.LevelDebug
	}
}

// LvlFromString parses a case-insensitive level name into a slog.Level.
func LvlFromString(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "fatal":
		return slog.LevelError
	case "error":
		return slog.LevelError
	case "warn", "warning":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "trace":
		return slog.LevelDebug
	default:
		return LvlFromLogrus(0)
	}
}

// LvlFromAny converts a level config value into a slog.Level, accepting
// either the historic logrus-style int LvlFromLogrus or a level name string LvlFromString.
func LvlFromAny(v any) slog.Level {
	switch val := v.(type) {
	case string:
		return LvlFromString(val)
	case int:
		return LvlFromLogrus(val)
	default:
		return slog.LevelInfo
	}
}
