# Slog based logging for Go

## Install

```
go get github.com/castai/logging
```

## Example
```go
log := logging.New(&logging.Config{
	Output: &out,
	Level:  logging.MustParseLevel("INFO"),
	RateLimiter: logging.RateLimiterConfig{
		Limit: rate.Every(10 * time.Millisecond),
		Burst: 1,
	},
})
log.Debug("slog based logger")
```
