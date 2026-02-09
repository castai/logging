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
	wg     sync.WaitGroup
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

	b := &BatchClient{
		buffer: make(chan Entry, cfg.BatchSize*2),
		client: client,
		cfg:    cfg,
	}

	return b
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
	b.wg.Add(1)
	defer b.wg.Done()

	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	entries := make([]Entry, 0, b.cfg.BatchSize)
	for {
		select {
		case entry := <-b.buffer:
			if len(entry.Message) == 0 {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= b.cfg.BatchSize {
				b.flush(ctx, entries)
				entries = entries[:0]
			}
		case <-ticker.C:
			b.flush(ctx, entries)
			entries = entries[:0]
		case <-ctx.Done():
			b.drainBuffer(&entries)
			// Use a new context with timeout for graceful shutdown.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			b.flush(shutdownCtx, entries)
			return ctx.Err()
		}
	}
}

func (b *BatchClient) drainBuffer(entries *[]Entry) {
	for {
		select {
		case entry := <-b.buffer:
			if len(entry.Message) > 0 {
				*entries = append(*entries, entry)
			}
		default:
			// Buffer is empty.
			return
		}
	}
}

func (b *BatchClient) flush(ctx context.Context, e []Entry) {
	if len(e) == 0 {
		return
	}
	if err := b.client.IngestLogs(ctx, e); err != nil {
		log.Printf("failed to publish logs: %v", err)
	}
}
