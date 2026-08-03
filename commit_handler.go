package logging

import (
	"log/slog"
	"runtime/debug"
)

const commitLen = 8

// Commit returns the first commitLen characters of a git revision.
// With no argument, it reads vcs.revision from debug.ReadBuildInfo.
// Returns an empty string when build info is unavailable.
func Commit(override ...string) string {
	if len(override) > 0 && override[0] != "" {
		return truncateCommit(override[0])
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}
		return truncateCommit(s.Value)
	}
	return ""
}

func truncateCommit(hash string) string {
	if len(hash) > commitLen {
		return hash[:commitLen]
	}
	return hash
}

// NewCommitHandler returns a chain handler that attaches a "commit" field to every log entry.
func NewCommitHandler(override ...string) Handler {
	commit := Commit(override...)
	return HandlerFunc(func(next slog.Handler) slog.Handler {
		if next == nil {
			return next
		}
		if commit == "" {
			return next
		}
		return next.WithAttrs([]slog.Attr{slog.String("commit", commit)})
	})
}
