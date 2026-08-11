package components

import (
	"context"
	"errors"
	"log"
	"sync"
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

	mu      sync.Mutex
	pending []Entry
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
			b.mu.Lock()
			b.pending = append(b.pending, entry)
			full := len(b.pending) >= b.cfg.BatchSize
			b.mu.Unlock()
			if full {
				_ = b.flushPending(ctx)
			}
		case <-ticker.C:
			_ = b.flushPending(ctx)
		case <-ctx.Done():
			b.drainBuffer()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if flushErr := b.flushPending(shutdownCtx); flushErr != nil {
				return errors.Join(flushErr, ctx.Err())
			}
			return ctx.Err()
		}
	}
}

func (b *BatchClient) drainBuffer() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for {
		select {
		case entry := <-b.buffer:
			if len(entry.Message) > 0 {
				b.pending = append(b.pending, entry)
			}
		default: // empty buffer
			return
		}
	}
}

func (b *BatchClient) flushPending(ctx context.Context) error {
	b.mu.Lock()
	entries := b.pending
	b.pending = nil
	b.mu.Unlock()
	return b.flush(ctx, entries)
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

// Flush immediately sends everything buffered or pending to APIClient.
// Safe to call concurrently.
func (b *BatchClient) Flush(ctx context.Context) error {
	b.drainBuffer()
	return b.flushPending(ctx)
}
