package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/castai/logging"
	"golang.org/x/time/rate"
)

func TestRateLimiterHandler(t *testing.T) {
	rateLimitHandler := logging.NewRateLimitHandler(logging.RateLimiterHandlerConfig{
		Limit: rate.Every(10 * time.Millisecond),
		Burst: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logging.PrintDroppedLogs(ctx, 1*time.Millisecond, rateLimitHandler, func(level slog.Level, count uint64) {
		fmt.Println("dropped", level, count)
	})

	var buf bytes.Buffer
	log := logging.New(
		logging.NewTextHandler(logging.TextHandlerConfig{Output: io.MultiWriter(&buf, os.Stdout)}),
		rateLimitHandler,
	)

	for i := 0; i < 10; i++ {
		log.WithField("component", "test").Info("test")
		time.Sleep(8 * time.Millisecond)
	}

	actualLinesCount := countLogLines(&buf)
	if actualLinesCount > 9 || actualLinesCount < 1 {
		t.Errorf("got %d lines, want at least 1 but no more than 10", actualLinesCount)
	}
}

func countLogLines(buf *bytes.Buffer) int {
	var n int
	for _, b := range buf.Bytes() {
		if b == '\n' {
			n++
		}
	}
	return n
}

func TestRateLimiterHandler_EnabledIsPure(t *testing.T) {
	rl := logging.NewRateLimitHandler(logging.RateLimiterHandlerConfig{
		Limit: 0,
		Burst: 1,
	})

	// Enabled must not consume rate tokens: repeated calls return the same
	// answer without draining the burst.
	for i := 0; i < 5; i++ {
		if !rl.Enabled(context.Background(), slog.LevelInfo) {
			t.Fatal("Enabled with no next handler should return true")
		}
	}

	var buf bytes.Buffer
	log := logging.New(
		logging.NewTextHandler(logging.TextHandlerConfig{Level: slog.LevelInfo, Output: &buf}),
		rl,
	)
	log.Info("test")
	if n := countLogLines(&buf); n != 1 {
		t.Errorf("got %d lines, want 1: Enabled calls must not consume the burst token", n)
	}
}

func TestRateLimiterHandler_DropsInHandle(t *testing.T) {
	rl := logging.NewRateLimitHandler(logging.RateLimiterHandlerConfig{
		Limit: 0, // no sustained rate: only the initial burst gets through
		Burst: 1,
	})

	var buf bytes.Buffer
	log := logging.New(
		logging.NewTextHandler(logging.TextHandlerConfig{Level: slog.LevelInfo, Output: &buf}),
		rl,
	)
	for i := 0; i < 4; i++ {
		log.Info("test")
	}
	if n := countLogLines(&buf); n != 1 {
		t.Errorf("got %d lines, want 1 (burst of 1)", n)
	}

	// The three dropped records are counted and observable via
	// PrintDroppedLogs. All drops already happened, so the first tick must
	// report them all.
	var total uint64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logging.PrintDroppedLogs(ctx, 5*time.Millisecond, rl, func(level slog.Level, count uint64) {
		atomic.AddUint64(&total, count)
	})

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadUint64(&total) == 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := atomic.LoadUint64(&total); got != 3 {
		t.Errorf("dropped counter total = %d, want 3", got)
	}
}

func TestRateLimiterHandler_CustomLevelDoesNotPanic(t *testing.T) {
	rl := logging.NewRateLimitHandler(logging.RateLimiterHandlerConfig{
		Limit: rate.Inf,
		Burst: 1,
	})

	var buf bytes.Buffer
	log := logging.New(
		logging.NewTextHandler(logging.TextHandlerConfig{Level: slog.LevelDebug, Output: &buf}),
		rl,
	)

	// slog.Level(12) is between Info and Warn: bucketed to Warn. Previously
	// this panicked with a nil-map lookup on h.rt[level].
	log.Log.Log(context.Background(), slog.Level(12), "custom level msg")

	if !bytes.Contains(buf.Bytes(), []byte("custom level msg")) {
		t.Errorf("custom-level record missing from output: %q", buf.String())
	}
}
