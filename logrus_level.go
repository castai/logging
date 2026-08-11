package logging

import "log/slog"

// LvlFromLogrus maps legacy logrus integer log lvl to the slog.Level.
// Please note that 0 maps to Info as it may be either panic or unset cfg value.
//
// 0=Panic -> slog.LevelInfo, 1=Fatal -> slog.LevelError, 2=Error -> slog.LevelError,
// 3=Warn -> slog.LevelWarn, 4=Info -> slog.LevelInfo, 5=Debug -> slog.LevelDebug.
func LvlFromLogrus(level int) slog.Level {
	oldLvl := level
	if level < 0 {
		level = 0
	} else if level > 5 {
		level = 5
	}

	if oldLvl != level {
		slog.Default().With("log_level", oldLvl).
			Warn("found legacy logging level, consider migrating to slog value")
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
