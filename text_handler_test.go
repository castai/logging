package logging_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/castai/logging"
	"github.com/stretchr/testify/require"
)

func TestTextHandler(t *testing.T) {
	r := require.New(t)

	var buf bytes.Buffer
	log := logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
		Level:  logging.MustParseLevel("DEBUG"),
		Output: io.MultiWriter(&buf, os.Stdout),
	}))

	log.Debug("msg1")
	log.Info("msg2")
	log.Error("msg3")
	log.Warn("msg4")

	r.Contains(buf.String(), "level=DEBUG msg=msg1")
	r.Contains(buf.String(), "level=INFO msg=msg2")
	r.Contains(buf.String(), "level=ERROR msg=msg3")
	r.Contains(buf.String(), "level=WARN msg=msg4")
}
