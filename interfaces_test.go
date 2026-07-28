package logging

// Compile-time assertions live in interfaces.go; this file exists so that
// `go test ./...` covers the interfaces_test.go filename referenced in the
// migration plan and gives a stable home for any future interface-focused
// tests (e.g. asserting behavior when *Logger is upcast to FieldsLogger).

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggerSatisfiesInterfaces(t *testing.T) {
	// The compile-time assertions in interfaces.go would fail the build if
	// *Logger did not satisfy the interfaces; this test additionally verifies
	// that the interface can be used at a call site and that derivation
	// preserves interface identity.
	r := require.New(t)
	var buf bytes.Buffer
	l := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: io.MultiWriter(&buf, os.Stdout),
	}))

	var fl FieldsLogger = l
	fl = fl.WithField("component", "iface-test")
	fl = fl.WithFieldAny("count", 3)
	fl = fl.WithFields(map[string]any{"role": "worker"})
	fl.Info("hello")

	out := buf.String()
	r.Contains(out, `level=info`)
	r.Contains(out, `msg=hello`)
	r.Contains(out, `component=iface-test`)
	r.Contains(out, `count=3`)
	r.Contains(out, `role=worker`)
}
