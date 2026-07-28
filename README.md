# Slog based logging for Go

This package is almost a drop in replacement for logrus. It's based on slog logger which is now part of Go standard library.

## Features

* Rate limit
* Export hook for logs export to external systems
* Logfmt text format handler with source lines support.
* JSON format handler (see `NewJSONHandler`).
* Timezone rewriting handler (see `NewTimeZoneHandler`; also driven by `LOG_TIMEZONE` env var).
* Env-driven output format via `JSON_LOG=true`.
* Interfaces for consumer packages: `FieldsLogger`, `FieldLogger`, `BaseLogger` — `*Logger` satisfies them all.
* Context-aware helpers: `WithLogger`, `FromContext`, `FromContextWithField`, `FromContextWithFields`.
* Test hook: `NewNullLogger()` returns a logger that captures records for assertions in tests.
* Pluggable trace/span attach: register a `TraceSpanExtractor` to automatically enrich `FromContext` loggers with `trace_id`/`span_id` fields.
* `Commit()` helper: read the first 8 chars of the binary's git revision via `debug.ReadBuildInfo` and attach as a log field.

## Install

```
go get github.com/castai/logging
```

## Example

See `logging_test.go` for the canonical example.

## Interfaces

```go
type FieldsLogger interface {
    BaseLogger
    With(args ...any) FieldsLogger
    WithField(key, value string) FieldsLogger
    WithFieldAny(key string, value any) FieldsLogger
    WithFields(fields map[string]any) FieldsLogger
    WithGroup(name string) FieldsLogger
}

// Fields is a convenience alias for map[string]any, matching logrus's Fields.
type Fields = map[string]any
```

`*Logger` satisfies `FieldsLogger` (and `FieldLogger`, an alias). Consumer packages should depend on `FieldsLogger` in function parameters and struct fields; production code passes a `*Logger`, tests pass a `NewNullLogger()`-produced logger.

## Context helpers

```go
ctx = logging.WithLogger(ctx, log)

// Later, anywhere down the call stack:
logging.FromContext(ctx).Info("hello")

// Derive a logger with a new field and store it in a fresh ctx:
ctx, log := logging.FromContextWithField(ctx, "node_name", "node-a")
log.Warn("draining")
```

If no logger is stored in ctx, `FromContext` returns a lazily-constructed package default (identical to `New()`).

## Test hook

```go
log, hook := logging.NewNullLogger()

log.WithField("k", "v").Error("boom")

entries := hook.AllEntries()
// entries[0].Level == slog.LevelError
// entries[0].Message == "boom"
// entries[0].Attrs["k"] == "v"
```

`TestHook.Reset()`, `TestHook.LastEntry()`, `TestHook.AllEntries()` are the primary read APIs. All levels are captured regardless of runtime level filters, so tests can assert on debug records emitted under an info-level configuration.

## Trace / span attachment

`castai/logging` does not depend on any tracing library. Instead, register a `TraceSpanExtractor` from your tracing package at process init:

```go
type myExtractor struct{}

func (myExtractor) TraceID(ctx context.Context) string { /* pull from ctx */ return "" }
func (myExtractor) SpanID(ctx context.Context)  string { /* pull from ctx */ return "" }

func init() {
    logging.SetTraceSpanExtractor(myExtractor{})
}
```

When a `FromContext(ctx)` (or `FromContextWithField[s]`) call is made and the registered extractor returns non-empty IDs for `ctx`, the returned logger gets `trace_id`/`span_id` fields attached transparently. Passing `nil` to `SetTraceSpanExtractor` disables the feature.

## Commit hash

```go
log := logging.New().With(slog.String("commit", logging.Commit()))
```

`Commit()` returns the first 8 chars of `vcs.revision` from `debug.ReadBuildInfo`, or the empty string when build info is unavailable (e.g. under `go test`).
