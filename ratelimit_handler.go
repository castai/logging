package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

var DefaultRateLimitHandlerConfig = RateLimiterHandlerConfig{
	Limit: 100,
	Burst: 100,
}

var _ Handler = new(RateLimitHandler)

type RateLimiterHandlerConfig struct {
	Limit rate.Limit
	Burst int
}

func NewRateLimitHandler(cfg RateLimiterHandlerConfig) *RateLimitHandler {
	droppedLogsCounters := map[slog.Level]*atomic.Uint64{
		slog.LevelDebug: {},
		slog.LevelInfo:  {},
		slog.LevelWarn:  {},
		slog.LevelError: {},
	}
	logsRate := cfg.Limit
	burst := cfg.Burst
	return &RateLimitHandler{
		rt: map[slog.Level]*rate.Limiter{
			slog.LevelDebug: rate.NewLimiter(logsRate, burst),
			slog.LevelInfo:  rate.NewLimiter(logsRate, burst),
			slog.LevelWarn:  rate.NewLimiter(logsRate, burst),
			slog.LevelError: rate.NewLimiter(logsRate, burst),
		},
		droppedLogsCounters: droppedLogsCounters,
	}
}

type RateLimitHandler struct {
	next                slog.Handler
	rt                  map[slog.Level]*rate.Limiter
	droppedLogsCounters map[slog.Level]*atomic.Uint64
}

func (h *RateLimitHandler) Register(next slog.Handler) slog.Handler {
	h.next = next
	return h
}

func (h *RateLimitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.next == nil {
		return true
	}
	if !h.next.Enabled(ctx, level) {
		return false
	}
	if !h.rt[level].Allow() {
		h.droppedLogsCounters[level].Add(1)
		return false
	}
	return true
}

func (h *RateLimitHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.next == nil {
		return nil
	}
	return h.next.Handle(ctx, record)
}

func (h *RateLimitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &RateLimitHandler{
		rt:                  h.rt,
		droppedLogsCounters: h.droppedLogsCounters,
	}
	if h.next != nil {
		clone.next = h.next.WithAttrs(attrs)
	}
	return clone
}

func (h *RateLimitHandler) WithGroup(name string) slog.Handler {
	clone := &RateLimitHandler{
		rt:                  h.rt,
		droppedLogsCounters: h.droppedLogsCounters,
	}
	if h.next != nil {
		clone.next = h.next.WithGroup(name)
	}
	return clone
}

// PrintDroppedLogs prints dropped rate limit logs and resets counter to 0.
func PrintDroppedLogs(ctx context.Context, interval time.Duration, r *RateLimitHandler, printFunc func(level slog.Level, count uint64)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for level, val := range r.droppedLogsCounters {
				count := val.Load()
				if count > 0 {
					printFunc(level, count)
					val.Store(0)
				}
			}
		}
	}
}
