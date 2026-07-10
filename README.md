# Slog based logging for Go

This package is almost a drop in replacement for logrus. It's based on slog logger which is now part of Go standard
library.

## Features

* Rate limit
* Export hook for logs export to external systems
* Logfmt text format handler with source lines support.

## Install

```
go get github.com/castai/logging
```

## Example

See `examples` directory for examples.

### Env vars honored by `New`

`New` reads two environment variables:

| Env var        | Values                                                         | Effect                                                                     |
|----------------|----------------------------------------------------------------|----------------------------------------------------------------------------|
| `JSON_LOG`     | anything `strconv.ParseBool` accepts, e.g. `true, false, 1, 0` | Selects the **default** base handler when no handlers are passed to `New`. |
| `LOG_TIMEZONE` | IANA name, e.g. `Europe/Vilnius`, `America/Lima`               | Appends a timezone handler that converts record timestamps.                |

`JSON_LOG` never overrides an explicit handler you pass to `New` — it only
picks the default when you pass none. `LOG_TIMEZONE` always applies, so you
can mix an explicit format with an env-driven timezone:

```go
// Env-only: JSON_LOG / LOG_TIMEZONE fully drive the setup.
log := logging.New()

// Mixed: explicit JSON format, timezone from env.
log := logging.New(logging.NewJSONHandler(logging.DefaultJSONHandlerConfig))

// Fully explicit: env vars for format are ignored; LOG_TIMEZONE would still apply.
log := logging.New(
logging.NewTextHandler(logging.DefaultTextHandlerConfig),
logging.NewTimeZoneHandler(time.UTC),
)
```

Invalid values (`JSON_LOG=notabool`, unknown `LOG_TIMEZONE`) cause `New` to
panic — same convention as `MustParseLevel`. Loading a zone requires tzdata
to be present in the runtime image; on scratch/distroless containers add a
blank import of `time/tzdata`.
