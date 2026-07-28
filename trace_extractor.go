package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// TraceSpanExtractor pulls trace and span IDs out of a context so log
// entries derived via FromContext can be enriched with `trace_id` and
// `span_id` fields.
//
// Implementations live in the caller's tracing package (e.g. an OpenTracing
// or OTel adapter). castai/logging deliberately takes context.Context here
// rather than an opentracing.Span so the module does not have to depend on
// any tracing library.
//
// Both methods MUST be safe to call concurrently. Returning an empty string
// from either method suppresses the corresponding log field.
type TraceSpanExtractor interface {
	TraceID(ctx context.Context) string
	SpanID(ctx context.Context) string
}

// extractorHolder wraps a TraceSpanExtractor in an atomic.Pointer so
// SetTraceSpanExtractor is safe against concurrent readers on the log path.
type extractorHolder struct {
	e TraceSpanExtractor
}

var traceExtractor atomic.Pointer[extractorHolder]

// SetTraceSpanExtractor registers the process-wide extractor. Pass nil to
// disable trace/span attachment. Safe to call at any time; already-derived
// loggers will pick up the new extractor on their next FromContext call.
func SetTraceSpanExtractor(e TraceSpanExtractor) {
	if e == nil {
		traceExtractor.Store(nil)
		return
	}
	traceExtractor.Store(&extractorHolder{e: e})
}

// getTraceExtractor returns the currently-registered extractor, or nil if
// none is set.
func getTraceExtractor() TraceSpanExtractor {
	h := traceExtractor.Load()
	if h == nil {
		return nil
	}
	return h.e
}

// attachTraceFields returns a logger derived from l with trace_id/span_id
// fields attached when a TraceSpanExtractor is registered and both IDs are
// non-empty for ctx. When no extractor is registered, or when either ID is
// empty, the input logger is returned unchanged (no allocation).
func attachTraceFields(ctx context.Context, l *Logger) *Logger {
	if ctx == nil {
		return l
	}
	e := getTraceExtractor()
	if e == nil {
		return l
	}
	traceID := e.TraceID(ctx)
	spanID := e.SpanID(ctx)
	if traceID == "" && spanID == "" {
		return l
	}
	// Attach both fields when at least one is non-empty; missing IDs are
	// still attached as empty strings so tools scanning for the keys have a
	// consistent shape. To avoid that, only attach non-empty IDs:
	attrs := make([]any, 0, 4)
	if traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if spanID != "" {
		attrs = append(attrs, slog.String("span_id", spanID))
	}
	return &Logger{Log: l.Log.With(attrs...)}
}
