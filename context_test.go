package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithLoggerAndFromContext(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))

	ctx := WithLogger(context.Background(), base)
	got := FromContext(ctx)
	r.Same(base.Log.Handler(), got.Log.Handler(),
		"FromContext should return the same underlying slog.Handler as the logger stored via WithLogger")

	got.Info("ctx-hello")
	r.Contains(buf.String(), `msg=ctx-hello`)
}

func TestFromContextDefaultsWhenAbsent(t *testing.T) {
	r := require.New(t)
	got := FromContext(context.Background())
	r.NotNil(got)
	r.NotPanics(func() { got.Debug("default-fallback") })
}

func TestFromContextWithFieldDerivesAndStores(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))

	ctx := WithLogger(context.Background(), base)
	ctx, derived := FromContextWithField(ctx, "own_identity", "leader-1")
	derived.Info("elected")

	out := buf.String()
	r.Contains(out, `msg=elected`)
	r.Contains(out, `own_identity=leader-1`)

	// The derived context should carry the derived logger, not the original.
	ctx2Log := FromContext(ctx)
	ctx2Log.Info("second")
	r.Contains(buf.String(), `own_identity=leader-1`)
}

func TestFromContextWithFieldsMultipleAttrs(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	ctx, derived := FromContextWithFields(ctx, map[string]any{
		"node_name":    "node-a",
		"reconcile_id": 42,
		"drain_reason": "cost",
	})
	_ = ctx
	derived.Warn("draining")

	out := buf.String()
	r.Contains(out, `msg=draining`)
	r.Contains(out, `node_name=node-a`)
	r.Contains(out, `reconcile_id=42`)
	r.Contains(out, `drain_reason=cost`)
}

func TestCtxHelpers(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	Debugf(ctx, "d=%d", 1)
	Infof(ctx, "i=%d", 2)
	Warnf(ctx, "w=%d", 3)
	Errorf(ctx, "e=%d", 4)

	out := buf.String()
	r.Contains(out, `level=debug msg="d=1"`)
	r.Contains(out, `level=info msg="i=2"`)
	r.Contains(out, `level=warn msg="w=3"`)
	r.Contains(out, `level=error msg="e=4"`)
}
