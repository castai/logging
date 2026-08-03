package logging

import (
	"log/slog"
	"runtime/debug"
)

// commitLen bounds the returned commit prefix
const commitLen = 8

// NewCommitHandler returns a chain handler that attaches a "commit" field to every record.
func NewCommitHandler(override ...string) Handler {
	return HandlerFunc(func(next slog.Handler) slog.Handler {
		if next == nil {
			return next
		}
		commit := Commit(override...)
		if commit == "" {
			return next
		}
		return next.WithAttrs([]slog.Attr{slog.String("commit", commit)})
	})
}

// Commit returns the first commitLen characters of a git revision.
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
