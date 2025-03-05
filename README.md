# Slog based logging for Go

This package is almost a drop in replacement for logrus. It's based on slog logger which is now part of Go standard library.

## Features

* Rate limit
* Export hook for logs export to external systems
* Logfmt text format handler with source lines support.

## Install

```
go get github.com/castai/logging
```

## Example

See logging_test.go example test.
