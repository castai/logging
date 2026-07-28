package logging

import "context"

// attachTraceFields is the extension point for attaching trace_id/span_id to
// loggers derived via FromContext. Phase 2 of the evictor migration replaces
// the body of this file with a pluggable TraceSpanExtractor. Until then it is
// a no-op returning the logger unchanged.
//
// The signature is stable so Phase 2 can swap the implementation without
// touching context.go.
func attachTraceFields(_ context.Context, l *Logger) *Logger {
	return l
}
