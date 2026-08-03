package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging"
)

var commitLen = 8

func TestCommitHandler(t *testing.T) {
	t.Run("attaches commit field from override", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewJSONHandler(logging.JSONHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewCommitHandler("abcd1234567890"),
		)
		log.Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		r.Equal("abcd1234", m["commit"])
	})

	t.Run("empty commit is a no-op", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewTextHandler(logging.TextHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewCommitHandler(""),
		)
		log.Info("msg")
		r.Contains(buf.String(), "msg=msg")
		r.NotContains(buf.String(), "commit=")
	})

	t.Run("resolved once as the sole handler via New's auto-inserted base", func(t *testing.T) {
		r := require.New(t)
		// No explicit base handler: New must detect the decorator-only chain
		// (chain() yields nil) and auto-insert a default base handler in
		// front, same as the existing NewTimeZoneHandler-only case.
		log := logging.New(logging.NewCommitHandler("deadbeefcafe"))
		r.NotPanics(func() { log.Info("msg") })
	})

	t.Run("preserves WithGroup and WithAttrs", func(t *testing.T) {
		r := require.New(t)
		var buf bytes.Buffer
		log := logging.New(
			logging.NewJSONHandler(logging.JSONHandlerConfig{
				Level:  slog.LevelInfo,
				Output: &buf,
			}),
			logging.NewCommitHandler("abcd1234"),
		)
		log.WithGroup("g").With("k", "v").Info("msg")

		var m map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m))
		r.Equal("abcd1234", m["commit"])
		g, ok := m["g"].(map[string]any)
		r.True(ok, "expected group 'g', got %T", m["g"])
		r.Equal("v", g["k"])
	})

	t.Run("resolved once and shared across multiple New calls", func(t *testing.T) {
		r := require.New(t)
		// The commit is resolved when NewCommitHandler is called, not when
		// the returned Handler is later registered — so a single Handler
		// value reused across several loggers only pays for one Commit()
		// lookup, and both loggers see the same value even if override
		// resolution could otherwise be call-time dependent.
		handler := logging.NewCommitHandler("cafebabe1234")

		var buf1, buf2 bytes.Buffer
		log1 := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level: slog.LevelInfo, Output: &buf1,
		}), handler)
		log2 := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level: slog.LevelInfo, Output: &buf2,
		}), handler)

		log1.Info("first")
		log2.Info("second")

		var m1, m2 map[string]any
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf1.Bytes()), &m1))
		r.NoError(json.Unmarshal(bytes.TrimSpace(buf2.Bytes()), &m2))
		r.Equal("cafebabe", m1["commit"])
		r.Equal("cafebabe", m2["commit"])
	})
}

func TestCommit(t *testing.T) {
	r := require.New(t)
	got := logging.Commit()

	// Under `go test` the vcs.revision setting may or may not be present
	// depending on the -buildvcs flag and whether the working tree is a
	// git repo. So we assert the invariant that whatever we return either
	// (a) matches what debug.ReadBuildInfo reports, capped at commitLen,
	// or (b) is empty when build info is unavailable.
	info, ok := debug.ReadBuildInfo()
	if !ok {
		r.Equal("", got, "no build info available; Commit() must return empty")
		return
	}

	expected := ""
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			if len(s.Value) > commitLen {
				expected = s.Value[:commitLen]
			} else {
				expected = s.Value
			}
			break
		}
	}
	r.Equal(expected, got)
	if expected != "" {
		r.LessOrEqual(len(got), commitLen, "commit prefix should never exceed commitLen")
	}
}

func TestCommitOverride(t *testing.T) {
	r := require.New(t)

	r.Equal("abcd1234", logging.Commit("abcd1234567890"), "override longer than commitLen must be truncated")
	r.Equal("abcd", logging.Commit("abcd"), "override shorter than commitLen must be returned as-is")

	// An empty override string falls through to the build-info path rather
	// than being treated as an explicit empty hash.
	r.Equal(logging.Commit(), logging.Commit(""))
}
