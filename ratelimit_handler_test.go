package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
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
