# Slog based logging for Go

This package is almost a drop in replacement for logrus. It's based on slog logger which is now part of Go standard library.

## Features

* Rate limit
* Export hook for logs export to external systems
* Logfmt text format handler with source lines support.
* JSON format handler (see `NewJSONHandler`).
* Timezone rewriting handler (see `NewTimeZoneHandler`; also driven by `LOG_TIMEZONE` env var).
* Env-driven output format via `JSON_LOG=true`.
* `FieldsLogger` interface for consumer packages — `*Logger` satisfies it.
* Context-aware helpers: `WithLogger`, `FromContext`, `FromContextWithField`, `FromContextWithFields`.
* Test hook: `NewNullLogger()` returns a logger that captures records for assertions in tests.
* Pluggable trace/span attach: register a `TraceSpanExtractor` to automatically enrich `FromContext` loggers with `trace_id`/`span_id` fields.
* `Commit()` helper: read the first 8 chars of the binary's git revision via `debug.ReadBuildInfo` and attach as a log field.
* `Println(v ...any)`, logged at error level: lets `*Logger` be passed directly where a `promhttp.Logger`-shaped (or `*log.Logger`-shaped) single-method interface is expected, e.g. `promhttp.HandlerOpts{ErrorLog: log}`.

## Install

```
go get github.com/castai/logging
```

## Example

See `logging_test.go` for the canonical example.

## Interfaces

```go
type FieldsLogger interface {
    Debug(msg string)
    Debugf(format string, args ...any)
    Info(msg string)
    Infof(format string, args ...any)
    Warn(msg string)
    Warnf(format string, args ...any)
    Error(msg string)
    Errorf(format string, args ...any)
    Fatal(msg string)
    Fatalf(format string, args ...any)
    IsEnabled(lvl slog.Level) bool

    With(args ...any) *Logger
    WithField(key, value string) *Logger
    WithFieldAny(key string, value any) *Logger
    WithFields(fields map[string]any) *Logger
    WithGroup(name string) *Logger
}

// Fields is a convenience alias for map[string]any, matching logrus's Fields.
type Fields = map[string]any
```

Derivation methods return the concrete `*Logger` (not the interface), the same way `logrus.FieldLogger.WithField` returns `*logrus.Entry` rather than the interface itself. This is what lets `*Logger` satisfy `FieldsLogger` with no adapter code, while callers still get a concrete, chainable value back. Consumer packages should depend on `FieldsLogger` in function parameters and struct fields; production code passes a `*Logger`, tests pass a `NewNullLogger()`-produced logger.

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
