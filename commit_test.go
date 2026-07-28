package logging

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommit(t *testing.T) {
	r := require.New(t)
	got := Commit()

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
