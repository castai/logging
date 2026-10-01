package components

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
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

// DropWhenFull makes IngestLogs never block: entries that do not fit in the buffer are dropped and counted.
func DropWhenFull() func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.DropWhenFull = true
	}
}

// BufferSize sets the number of buffered entries independently of BatchSize. Values <= 0 mean BatchSize*2.
func BufferSize(size int) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.BufferSize = size
	}
}

// OnDrop sets a callback invoked once per IngestLogs call with the number of entries dropped (DropWhenFull mode).
// It runs on caller goroutines, possibly concurrently, so it must be cheap, non-blocking and must not log via this client.
func OnDrop(fn func(n int)) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.OnDrop = fn
	}
}

// MaxInFlight sets the max concurrent sends; <= 1 keeps sends synchronous. Batches may arrive out of order,
// and each slot retries independently, so keep it small. Sends outliving shutdown run until RequestTimeout.
func MaxInFlight(n int) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.MaxInFlight = n
	}
}

// RequestTimeout bounds each send outside shutdown. Values <= 0 mean 45s.
func RequestTimeout(d time.Duration) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.RequestTimeout = d
	}
}

// ShutdownTimeout bounds the total time Run spends flushing after cancellation. Values <= 0 mean 10s.
func ShutdownTimeout(d time.Duration) func(*BatchClientConfig) {
	return func(config *BatchClientConfig) {
		config.ShutdownTimeout = d
	}
}

type BatchClientConfig struct {
	EnqueueTimeout time.Duration
	FlushInterval  time.Duration
	BatchSize      int
	// DropWhenFull makes IngestLogs never block; entries that do not fit are dropped.
	DropWhenFull bool
	// BufferSize is the number of buffered entries; <= 0 means BatchSize*2.
	BufferSize int
	// OnDrop is optional, called with the entries dropped by one IngestLogs call; may run concurrently.
	OnDrop func(n int)
	// MaxInFlight is the max concurrent sends; <= 1 means synchronous sends.
	MaxInFlight int
	// RequestTimeout bounds each send outside shutdown; <= 0 means 45s.
	RequestTimeout time.Duration
	// ShutdownTimeout bounds the flush after cancellation; <= 0 means 10s.
	ShutdownTimeout time.Duration
}

var _ APIClient = (*BatchClient)(nil)

type BatchClient struct {
	buffer chan Entry
	client APIClient
	cfg    BatchClientConfig

	dropped atomic.Uint64
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

	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 45 * time.Second
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	bufferSize := cfg.BatchSize * 2
	if cfg.BufferSize > 0 {
		bufferSize = cfg.BufferSize
	}

	b := &BatchClient{
		buffer: make(chan Entry, bufferSize),
		client: client,
		cfg:    cfg,
	}

	return b
}

// Dropped returns the total number of entries dropped since creation (DropWhenFull mode only).
func (b *BatchClient) Dropped() uint64 {
	return b.dropped.Load()
}

func (b *BatchClient) IngestLogs(ctx context.Context, entries []Entry) error {
	if b.cfg.DropWhenFull {
		return b.ingestOrDrop(ctx, entries)
	}

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

func (b *BatchClient) ingestOrDrop(ctx context.Context, entries []Entry) error {
	var dropped uint64
	defer func() {
		if dropped == 0 {
			return
		}
		b.dropped.Add(dropped)
		if b.cfg.OnDrop != nil {
			b.cfg.OnDrop(int(dropped)) //nolint:gosec // bounded by len(entries)
		}
	}()
	for _, entry := range entries {
		select {
		case b.buffer <- entry:
			// Successfully enqueued.
		case <-ctx.Done():
			return ctx.Err()
		default:
			dropped++
		}
	}

	return nil
}

func (b *BatchClient) Run(ctx context.Context) error {
	return b.run(ctx)
}

func (b *BatchClient) run(ctx context.Context) error {
	if b.cfg.MaxInFlight > 1 {
		return b.runConcurrent(ctx)
	}

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
				b.flushBounded(ctx, entries)
				entries = entries[:0]
			}
		case <-ticker.C:
			b.flushBounded(ctx, entries)
			entries = entries[:0]
		case <-ctx.Done():
			b.drainBuffer(&entries)
			// Use a new context with timeout for graceful shutdown.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), b.cfg.ShutdownTimeout)
			defer cancel()
			b.flush(shutdownCtx, entries)
			return ctx.Err()
		}
	}
}

func (b *BatchClient) runConcurrent(ctx context.Context) error {
	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	slots := make(chan struct{}, b.cfg.MaxInFlight)
	var wg sync.WaitGroup

	entries := make([]Entry, 0, b.cfg.BatchSize)
	for {
		select {
		case entry := <-b.buffer:
			if len(entry.Message) == 0 {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= b.cfg.BatchSize && b.send(ctx, slots, &wg, entries) {
				entries = make([]Entry, 0, b.cfg.BatchSize)
			}
		case <-ticker.C:
			if b.send(ctx, slots, &wg, entries) {
				entries = make([]Entry, 0, b.cfg.BatchSize)
			}
		case <-ctx.Done():
			b.drainBuffer(&entries)
			// One deadline covers the final flush and waiting for in-flight sends. Flush first so
			// hung in-flight sends cannot starve it; this may briefly exceed MaxInFlight by one.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), b.cfg.ShutdownTimeout)
			defer cancel()
			b.flush(shutdownCtx, entries)

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-shutdownCtx.Done():
			}
			return ctx.Err()
		}
	}
}

// send hands entries to a background send once a slot is free; false means nothing was handed off.
func (b *BatchClient) send(ctx context.Context, slots chan struct{}, wg *sync.WaitGroup, entries []Entry) bool {
	if len(entries) == 0 {
		return false
	}
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return false
	}

	wg.Go(func() {
		defer func() { <-slots }()
		b.flushBounded(context.WithoutCancel(ctx), entries)
	})

	return true
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

// flushBounded is flush with RequestTimeout applied to the single send.
func (b *BatchClient) flushBounded(ctx context.Context, e []Entry) {
	ctx, cancel := context.WithTimeout(ctx, b.cfg.RequestTimeout)
	defer cancel()
	b.flush(ctx, e)
}

func (b *BatchClient) flush(ctx context.Context, e []Entry) {
	if len(e) == 0 {
		return
	}
	if err := b.client.IngestLogs(ctx, e); err != nil {
		log.Printf("failed to publish logs: %v", err)
	}
}
