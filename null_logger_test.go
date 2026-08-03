package logging

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNullLoggerCapturesRecords(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	log.Info("hello")
	log.WithField("k", "v").Errorf("boom: %d", 42)

	entries := hook.AllEntries()
	r.Len(entries, 2)

	r.Equal(slog.LevelInfo, entries[0].Level)
	r.Equal("hello", entries[0].Message)
	r.Empty(entries[0].Attrs)

	r.Equal(slog.LevelError, entries[1].Level)
	r.Equal("boom: 42", entries[1].Message)
	r.Equal("v", entries[1].Attrs["k"])
}

func TestLoggerPrintln(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	log.Println("error gathering metrics:", errors.New("boom"))

	last := hook.LastEntry()
	r.NotNil(last)
	r.Equal(slog.LevelError, last.Level)
	r.Equal("error gathering metrics: boom", last.Message)
}

func TestNullLoggerLastEntryAndReset(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	r.Nil(hook.LastEntry())

	log.Info("first")
	log.Warn("second")

	last := hook.LastEntry()
	r.NotNil(last)
	r.Equal(slog.LevelWarn, last.Level)
	r.Equal("second", last.Message)

	hook.Reset()
	r.Empty(hook.AllEntries())
	r.Nil(hook.LastEntry())
}

func TestNullLoggerCapturesAcrossDerivations(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	derived := log.WithField("component", "worker")
	derived.Info("started")

	grouped := log.WithGroup("nested")
	grouped.WithField("k", "v").Info("in-group")

	entries := hook.AllEntries()
	r.Len(entries, 2)
	r.Equal("worker", entries[0].Attrs["component"])
	r.Equal("v", entries[1].Attrs["nested.k"])
}

func TestNullLoggerCapturesAllLevels(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	log.Debug("d")
	log.Info("i")
	log.Warn("w")
	log.Error("e")

	levels := make([]slog.Level, 0, 4)
	for _, e := range hook.AllEntries() {
		levels = append(levels, e.Level)
	}
	r.Equal([]slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}, levels)
}

func TestNullLoggerLastEntryAttrsNotAliased(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	log.WithField("k", "v").Info("hello")

	last := hook.LastEntry()
	r.NotNil(last)
	last.Attrs["k"] = "mutated"
	last.Attrs["injected"] = "boom"

	again := hook.LastEntry()
	r.Equal("v", again.Attrs["k"])
	r.NotContains(again.Attrs, "injected")

	entries := hook.AllEntries()
	r.Equal("v", entries[len(entries)-1].Attrs["k"])
}

func TestNullLoggerCapturesErrorValue(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()

	err := errors.New("boom")
	log.WithFieldAny("error", err).Error("failed")

	last := hook.LastEntry()
	r.NotNil(last)
	got, ok := last.Attrs["error"].(error)
	r.True(ok, "expected the captured attribute to be an error, got %T", last.Attrs["error"])
	r.EqualError(got, "boom")
}
