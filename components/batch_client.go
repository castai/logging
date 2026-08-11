package components

import (
	"context"
	"errors"
	"log"
	"time"
)

func EnqueueTimeout(timeout time.Duration) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.EnqueueTimeout = timeout
	}
}

func FlushInterval(interval time.Duration) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.FlushInterval = interval
	}
}

func BatchSize(size int) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.BatchSize = size
	}
}

type BatchClientConfig struct {
	EnqueueTimeout time.Duration
	FlushInterval  time.Duration
	BatchSize      int
}

var _ APIClient = (*BatchClient)(nil)

type BatchClient struct {
	buffer chan Entry
	client APIClient
	cfg    BatchClientConfig
}

func NewBatchClient(client APIClient, opts ...func(*BatchClientConfig)) *BatchClient {
	cfg := BatchClientConfig{
		EnqueueTimeout: 5 * time.Second,
		FlushInterval:  5 * time.Second,
		BatchSize:      100,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &BatchClient{
		buffer: make(chan Entry, cfg.BatchSize*2),
		client: client,
		cfg:    cfg,
	}
}

func (b *BatchClient) IngestLogs(ctx context.Context, entries []Entry) error {
	enqTimeout := time.After(b.cfg.EnqueueTimeout)
	for _, entry := range entries {
		select {
		case b.buffer <- entry:
			// Successfully enqueued.
		case <-ctx.Done():
			return ctx.Err()
		case <-enqTimeout:
			return errors.New("timeout: buffer is full, cannot enqueue log entry")
		}
	}

	return nil
}

func (b *BatchClient) Run(ctx context.Context) error {
	return b.run(ctx)
}

func (b *BatchClient) run(ctx context.Context) error {
	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	buffer := b.buffer

	entries := make([]Entry, 0, b.cfg.BatchSize)
	for {
		select {
		case entry, ok := <-buffer:
			if !ok {
				buffer = nil
				continue
			}
			if len(entry.Message) == 0 {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= b.cfg.BatchSize {
				_ = b.flush(ctx, entries)
				entries = entries[:0]
			}
		case <-ticker.C:
			_ = b.flush(ctx, entries)
			entries = entries[:0]
		case <-ctx.Done():
			b.drainBuffer(&entries)
			// Use a new context with timeout for graceful shutdown.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if flushErr := b.flush(shutdownCtx, entries); flushErr != nil {
				// Join rather than replace: callers should still be able
				// to see this was a cancellation-triggered shutdown (via
				// errors.Is(err, context.Canceled)) as well as that the
				// final flush didn't make it out.
				return errors.Join(flushErr, ctx.Err())
			}
			return ctx.Err()
		}
	}
}

func (b *BatchClient) drainBuffer(entries *[]Entry) {
	for {
		select {
		case entry, ok := <-b.buffer:
			if !ok {
				return
			}
			if len(entry.Message) > 0 {
				*entries = append(*entries, entry)
			}
		default:
			// Buffer is empty.
			return
		}
	}
}

func (b *BatchClient) flush(ctx context.Context, e []Entry) error {
	if len(e) == 0 {
		return nil
	}
	if err := b.client.IngestLogs(ctx, e); err != nil {
		log.Printf("failed to publish logs: %v", err)
		return err
	}

	return nil
}
