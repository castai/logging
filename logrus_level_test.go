package logging_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
)

func TestLevelFromLogrusInt(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  slog.Level
	}{
		{"zero value maps to Info, not Panic", 0, slog.LevelInfo},
		{"logrus.FatalLevel maps to Error (slog has no Fatal)", 1, slog.LevelError},
		{"logrus.ErrorLevel maps to Error", 2, slog.LevelError},
		{"logrus.WarnLevel maps to Warn", 3, slog.LevelWarn},
		{"logrus.InfoLevel maps to Info", 4, slog.LevelInfo},
		{"logrus.DebugLevel maps to Debug", 5, slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, logging.LvlFromLogrus(tt.input))
		})
	}
}

func TestLevelFromLogrusInt_ClampsOutOfRangeValues(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  slog.Level
	}{
		{"logrus.TraceLevel (6) clamps to Debug, same as 5", 6, slog.LevelDebug},
		{"an arbitrary value above the range clamps to Debug", 42, slog.LevelDebug},
		{"a raw slog.LevelDebug value (-4) clamps to Info, same as 0", int(slog.LevelDebug), slog.LevelInfo},
		{"an arbitrary negative value clamps to Info, same as 0", -1, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, logging.LvlFromLogrus(tt.input))
		})
	}
}

func TestLevelFromLogrusInt_UsableAsHandlerLevel(t *testing.T) {
	r := require.New(t)

	// Mirrors the intended call site: a config's legacy logrus int level is
	// converted once and fed straight into a handler config's Level field.
	cfgLevel := 5 // logrus.DebugLevel
	log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
		Level: logging.LvlFromLogrus(cfgLevel),
	}))
	r.True(log.IsEnabled(slog.LevelDebug))
}
