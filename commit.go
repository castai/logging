package logging

import "runtime/debug"

// commitLen bounds the returned commit prefix. Eight characters matches the
// convention used by lib/logging and most CI dashboards.
const commitLen = 8

// Commit returns the first commitLen characters of the git revision this
// binary was built from, as reported by debug.ReadBuildInfo. Returns an
// empty string when build info is unavailable (e.g. tests run via `go
// test` without `-buildvcs`, or binaries built without VCS metadata).
//
// Typical usage:
//
//	log := logging.New().With(slog.String("commit", logging.Commit()))
//
// The helper is intentionally minimal — callers who need the full hash or
// dirty flag can call debug.ReadBuildInfo() directly.
func Commit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}
		if len(s.Value) > commitLen {
			return s.Value[:commitLen]
		}
		return s.Value
	}
	return ""
}
