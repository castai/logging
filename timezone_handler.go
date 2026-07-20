package logging

import (
	"context"
	"log/slog"
	"time"
)

// NewTimeZoneHandler returns a chain handler that converts each record's
// timestamp into the given location before forwarding to the next handler.
// A nil location makes the handler a no-op.
func NewTimeZoneHandler(loc *time.Location) Handler {
	return HandlerFunc(func(next slog.Handler) slog.Handler {
		if loc == nil || next == nil {
			return next
		}
		return &tzHandler{loc: loc, next: next}
	})
}

type tzHandler struct {
	loc  *time.Location
	next slog.Handler
}

func (h *tzHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.next.Enabled(ctx, lvl)
}

func (h *tzHandler) Handle(ctx context.Context, r slog.Record) error {
	if !r.Time.IsZero() {
		r.Time = r.Time.In(h.loc)
	}
	return h.next.Handle(ctx, r)
}

func (h *tzHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &tzHandler{loc: h.loc, next: h.next.WithAttrs(attrs)}
}

func (h *tzHandler) WithGroup(name string) slog.Handler {
	return &tzHandler{loc: h.loc, next: h.next.WithGroup(name)}
}
