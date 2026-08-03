package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// TraceSpanExtractor pulls trace and span IDs out of a context so log
// entries derived via FromContext can be enriched with `trace_id` and
// `span_id` fields.
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

// attachTraceFields returns a logger derived from l with trace_id and/or
// span_id fields attached.
func attachTraceFields(ctx context.Context, l *Logger) *Logger {
	if ctx == nil || l.traceAttached {
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
	attrs := make([]any, 0, 2)
	if traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if spanID != "" {
		attrs = append(attrs, slog.String("span_id", spanID))
	}
	return &Logger{Log: l.Log.With(attrs...), traceAttached: true}
}
