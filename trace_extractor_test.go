package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubExtractor implements TraceSpanExtractor with configurable return
// values. Used to drive the attachTraceFields path from tests without
// pulling in a real tracing backend.
type stubExtractor struct {
	traceID string
	spanID  string
}

func (s stubExtractor) TraceID(_ context.Context) string { return s.traceID }
func (s stubExtractor) SpanID(_ context.Context) string  { return s.spanID }

func TestSetTraceSpanExtractor_AttachesFields(t *testing.T) {
	r := require.New(t)
	// Ensure we start from a clean slate and restore after.
	t.Cleanup(func() { SetTraceSpanExtractor(nil) })

	SetTraceSpanExtractor(stubExtractor{traceID: "trace-abc", spanID: "span-123"})

	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	FromContext(ctx).Info("traced")
	out := buf.String()
	r.Contains(out, `msg=traced`)
	r.Contains(out, `trace_id=trace-abc`)
	r.Contains(out, `span_id=span-123`)
}

func TestSetTraceSpanExtractor_NoOpWhenUnset(t *testing.T) {
	r := require.New(t)
	SetTraceSpanExtractor(nil) // Explicit clear.

	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	FromContext(ctx).Info("no-trace")
	out := buf.String()
	r.Contains(out, `msg=no-trace`)
	r.NotContains(out, `trace_id`)
	r.NotContains(out, `span_id`)
}

func TestSetTraceSpanExtractor_EmptyIDsSuppressed(t *testing.T) {
	r := require.New(t)
	t.Cleanup(func() { SetTraceSpanExtractor(nil) })

	// Only trace_id populated; span_id empty must be suppressed.
	SetTraceSpanExtractor(stubExtractor{traceID: "t-only", spanID: ""})

	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	FromContext(ctx).Info("partial")
	out := buf.String()
	r.Contains(out, `trace_id=t-only`)
	r.NotContains(out, `span_id`)
}

func TestSetTraceSpanExtractor_NoDuplicateOnRepeatedFromContext(t *testing.T) {
	r := require.New(t)
	t.Cleanup(func() { SetTraceSpanExtractor(nil) })
	SetTraceSpanExtractor(stubExtractor{traceID: "trace-abc", spanID: "span-123"})

	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	// Round-trip through FromContext/WithLogger repeatedly, as
	// FromContextWithField/FromContextWithFields do internally. Each round
	// trip must attach trace_id/span_id at most once, not once per call.
	ctx, log1 := FromContextWithField(ctx, "step", "one")
	log1.Info("first")
	ctx, log2 := FromContextWithField(ctx, "step", "two")
	log2.Info("second")
	FromContext(ctx).Info("third")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	r.Len(lines, 3)
	for _, line := range lines {
		r.Equal(1, strings.Count(line, "trace_id="), "line must not carry duplicate trace_id: %s", line)
		r.Equal(1, strings.Count(line, "span_id="), "line must not carry duplicate span_id: %s", line)
	}
}

func TestSetTraceSpanExtractor_NilCtx(t *testing.T) {
	r := require.New(t)
	t.Cleanup(func() { SetTraceSpanExtractor(nil) })
	SetTraceSpanExtractor(stubExtractor{traceID: "t", spanID: "s"})

	// FromContext(nil) must not panic. It should fall back to the package
	// default and skip trace attachment because ctx is nil.
	r.NotPanics(func() { _ = FromContext(context.Background()) })
	r.NotPanics(func() { _ = FromContext(nil) }) //nolint:staticcheck
}

func TestSetTraceSpanExtractor_ConcurrentRegistration(t *testing.T) {
	r := require.New(t)
	t.Cleanup(func() { SetTraceSpanExtractor(nil) })

	var buf bytes.Buffer
	base := New(NewTextHandler(TextHandlerConfig{
		Level:  slog.LevelInfo,
		Output: &buf,
	}))
	ctx := WithLogger(context.Background(), base)

	// Race: many goroutines register different extractors while other
	// goroutines read via FromContext. This test would trip the race
	// detector under `go test -race` if SetTraceSpanExtractor were not
	// safe.
	var wg sync.WaitGroup
	writers := 4
	readers := 4
	iters := 200
	wg.Add(writers + readers)

	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				SetTraceSpanExtractor(stubExtractor{traceID: "t", spanID: "s"})
			}
		}(i)
	}
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				FromContext(ctx).Debug("noop")
			}
		}()
	}
	wg.Wait()
	// If we reach here without a race-detector abort, the test passes.
	r.True(true)
}
