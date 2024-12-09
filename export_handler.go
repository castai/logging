package logging

import (
	"context"
	"log/slog"
)

type ExportHandlerConfig struct {
	MinLevel   slog.Level // Only export logs for this min log level.
	BufferSize int        // Logs channel size.
}

func NewExportHandler(cfg ExportHandlerConfig) *ExportHandler {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}

	handler := &ExportHandler{
		cfg: cfg,
		ch:  make(chan slog.Record, cfg.BufferSize),
	}
	return handler
}

// ExportHandler exports logs to separate channel available via Records()
type ExportHandler struct {
	next slog.Handler
	cfg  ExportHandlerConfig

	ch chan slog.Record
}

func (h *ExportHandler) Register(next slog.Handler) slog.Handler {
	h.next = next
	return h
}

func (h *ExportHandler) Records() <-chan slog.Record {
	return h.ch
}

func (h *ExportHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.next == nil {
		return true
	}
	return h.next.Enabled(ctx, level)
}

func (h *ExportHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= h.cfg.MinLevel {
		select {
		case h.ch <- record:
		default:
		}
	}
	if h.next == nil {
		return nil
	}
	return h.next.Handle(ctx, record)
}

func (h *ExportHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &ExportHandler{
		cfg: h.cfg,
		ch:  h.ch,
	}
	if h.next != nil {
		clone.next = h.next.WithAttrs(attrs)
	}
	return clone
}

func (h *ExportHandler) WithGroup(name string) slog.Handler {
	clone := &ExportHandler{
		cfg: h.cfg,
		ch:  h.ch,
	}
	if h.next != nil {
		clone.next = h.next.WithGroup(name)
	}
	return clone
}
