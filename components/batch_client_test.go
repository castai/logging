package components_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/castai/logging/components"
)

func Test_BufferedClient_PublishLogs(t *testing.T) {
	t.Run("should not publish logs when flush interval is not reached", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(420), components.FlushInterval(time.Hour))
		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() {
			errc <- client.Run(ctx)
			close(errc)
		}()

		err := client.IngestLogs(ctx, []components.Entry{{
			Level:   "meow",
			Message: "meow-message",
			Time:    time.Now(),
		}})
		r.NoError(err)

		sentLogs := mockAPIClient.waitForLogs(10 * time.Millisecond)
		r.Len(sentLogs, 0)

		cancel()
		r.ErrorIs(<-errc, context.Canceled)
	})

	t.Run("should publish logs when max entries per publish is reached but flush interval is not reached", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(1), components.FlushInterval(time.Hour))

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() {
			errc <- client.Run(ctx)
			close(errc)
		}()

		entries := []components.Entry{{
			Level:   "meow",
			Message: "meow-message",
			Time:    time.Now(),
		}}

		err := client.IngestLogs(ctx, entries)
		r.NoError(err)

		sentLogs := mockAPIClient.waitForLogs(time.Second)
		r.Len(sentLogs, 1)

		cancel()
		r.ErrorIs(<-errc, context.Canceled)
	})

	t.Run("should publish logs when flush interval is reached but not enough entries to publish", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(2), components.FlushInterval(time.Millisecond*10))

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() {
			errc <- client.Run(ctx)
			close(errc)
		}()

		entries := []components.Entry{{
			Level:   "meow",
			Message: "meow-message",
			Time:    time.Now(),
		}}

		err := client.IngestLogs(ctx, entries)
		r.NoError(err)

		sentLogs := mockAPIClient.waitForLogs(time.Second)
		r.Len(sentLogs, 1)

		cancel()
		r.ErrorIs(<-errc, context.Canceled)
	})

	t.Run("should publish remaining logs when no conditions are met but context is canceled", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(2), components.FlushInterval(time.Hour))

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() {
			errc <- client.Run(ctx)
			close(errc)
		}()

		entries := []components.Entry{{
			Level:   "meow",
			Message: "meow-message",
			Time:    time.Now(),
		}}

		err := client.IngestLogs(ctx, entries)
		r.NoError(err)
		<-time.After(time.Millisecond * 100)
		cancel()

		sentLogs := mockAPIClient.waitForLogs(time.Second)
		r.Len(sentLogs, 1)

		r.ErrorIs(<-errc, context.Canceled)
	})

	t.Run("should drain all buffered entries on shutdown", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(100), components.FlushInterval(time.Hour))

		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			_ = client.Run(ctx)
			close(done)
		}()

		// Ingest 10 entries
		for i := 0; i < 10; i++ {
			err := client.IngestLogs(ctx, []components.Entry{{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			}})
			r.NoError(err)
		}

		// Give time for entries to be queued
		time.Sleep(50 * time.Millisecond)

		// Cancel context - should drain buffer and flush all entries
		cancel()
		<-done

		sentLogs := mockAPIClient.getLogs()
		r.Len(sentLogs, 10, "all buffered entries should be flushed on shutdown")
	})

	t.Run("should timeout when buffer is full", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &slowAPIClient{delay: 30 * time.Second} // Very slow to keep buffer full
		client := components.NewBatchClient(
			mockAPIClient,
			components.BatchSize(1),
			components.FlushInterval(time.Second),
			components.EnqueueTimeout(time.Second),
		)

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() {
			errc <- client.Run(ctx)
			close(errc)
		}()
		defer cancel()

		// Quickly fill the buffer (capacity is BatchSize * 2 = 2)
		// and trigger processing which will block on slow API call
		for i := 0; i < 5; i++ {
			err := client.IngestLogs(ctx, []components.Entry{{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			}})
			if err != nil {
				// We expect an error when buffer is full
				r.Contains(err.Error(), "timeout")
				return
			}
			// Don't sleep between sends to fill buffer quickly
		}

		r.Fail("expected timeout error but got none")
	})
}

type apiClient struct {
	mu   sync.Mutex
	logs []components.Entry
}

func (a *apiClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logs = append(a.logs, entries...)
	return nil
}

func (a *apiClient) getLogs() []components.Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.logs)
}

func (a *apiClient) waitForLogs(waitDuration time.Duration) []components.Entry {
	timeout := time.After(waitDuration)

	for {
		select {
		case <-time.After(time.Millisecond):
			a.mu.Lock()
			res := slices.Clone(a.logs)
			a.mu.Unlock()
			if len(res) > 0 {
				return res
			}
		case <-timeout:
			return nil
		}
	}
}

type slowAPIClient struct {
	delay time.Duration
}

func (s *slowAPIClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	time.Sleep(s.delay)
	return nil
}

func Test_BatchClient_DropWhenFull(t *testing.T) {
	entry := components.Entry{Level: "meow", Message: "meow-message", Time: time.Now()}
	entriesOf := func(n int) []components.Entry {
		res := make([]components.Entry, n)
		for i := range res {
			res[i] = entry
		}
		return res
	}

	t.Run("should never block with a full buffer", func(t *testing.T) {
		r := require.New(t)
		client := components.NewBatchClient(&apiClient{}, components.BufferSize(4), components.DropWhenFull())

		start := time.Now()
		for range 1000 {
			r.NoError(client.IngestLogs(context.Background(), entriesOf(1)))
		}

		r.Less(time.Since(start), time.Second)
		r.EqualValues(996, client.Dropped())
	})

	t.Run("should report drops once per call", func(t *testing.T) {
		r := require.New(t)
		var calls []int
		client := components.NewBatchClient(&apiClient{},
			components.BufferSize(4),
			components.DropWhenFull(),
			components.OnDrop(func(n int) { calls = append(calls, n) }),
		)

		r.NoError(client.IngestLogs(context.Background(), entriesOf(3)))
		r.Empty(calls)

		r.NoError(client.IngestLogs(context.Background(), entriesOf(10)))
		r.Equal([]int{9}, calls)
		r.EqualValues(9, client.Dropped())
	})

	t.Run("should return context error when buffer is full and context is canceled", func(t *testing.T) {
		r := require.New(t)
		client := components.NewBatchClient(&apiClient{}, components.BufferSize(1), components.DropWhenFull())
		r.NoError(client.IngestLogs(context.Background(), entriesOf(1)))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r.ErrorIs(client.IngestLogs(ctx, entriesOf(1)), context.Canceled)
		r.EqualValues(0, client.Dropped())
	})

	t.Run("should record drops made before context cancellation is observed", func(t *testing.T) {
		r := require.New(t)
		var calls []int
		client := components.NewBatchClient(&apiClient{},
			components.BufferSize(1),
			components.DropWhenFull(),
			components.OnDrop(func(n int) { calls = append(calls, n) }),
		)
		r.NoError(client.IngestLogs(context.Background(), entriesOf(1)))

		// Done() is nil (never ready) for the first two selects, then closed.
		ctx := &lateCancelCtx{Context: context.Background(), readyAfter: 2}
		r.ErrorIs(client.IngestLogs(ctx, entriesOf(5)), context.Canceled)
		r.EqualValues(2, client.Dropped())
		r.Equal([]int{2}, calls)
	})

	t.Run("should still flush entries that fit", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient,
			components.BatchSize(2), components.FlushInterval(time.Hour), components.DropWhenFull())

		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() { errc <- client.Run(ctx) }()

		r.NoError(client.IngestLogs(ctx, entriesOf(2)))
		r.Eventually(func() bool { return len(mockAPIClient.getLogs()) == 2 }, time.Second, time.Millisecond)

		r.NoError(client.IngestLogs(ctx, entriesOf(1)))
		cancel()
		r.ErrorIs(<-errc, context.Canceled)
		r.Len(mockAPIClient.getLogs(), 3)
		r.EqualValues(0, client.Dropped())
	})

	t.Run("should honor BufferSize independently of BatchSize", func(t *testing.T) {
		r := require.New(t)
		client := components.NewBatchClient(&apiClient{},
			components.BatchSize(100), components.BufferSize(3), components.DropWhenFull())
		r.NoError(client.IngestLogs(context.Background(), entriesOf(10)))
		r.EqualValues(7, client.Dropped())

		// Default buffer is BatchSize*2 when BufferSize is not set.
		client = components.NewBatchClient(&apiClient{}, components.BatchSize(5), components.DropWhenFull())
		r.NoError(client.IngestLogs(context.Background(), entriesOf(12)))
		r.EqualValues(2, client.Dropped())

		// BufferSize also applies in the default blocking mode.
		client = components.NewBatchClient(&apiClient{},
			components.BatchSize(100), components.BufferSize(3), components.EnqueueTimeout(20*time.Millisecond))
		r.NoError(client.IngestLogs(context.Background(), entriesOf(3)))
		r.Error(client.IngestLogs(context.Background(), entriesOf(1)))
	})
}

type lateCancelCtx struct {
	context.Context
	readyAfter int
	calls      int
}

func (c *lateCancelCtx) Done() <-chan struct{} {
	c.calls++
	if c.calls <= c.readyAfter {
		return nil
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (c *lateCancelCtx) Err() error { return context.Canceled }

type gatedClient struct {
	gate  chan struct{} // nil means never blocks
	delay time.Duration

	mu          sync.Mutex
	inFlight    int
	peak        int
	messages    []string
	deadlines   []time.Time
	ctxErrAfter []error
}

func (g *gatedClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	g.mu.Lock()
	g.inFlight++
	g.peak = max(g.peak, g.inFlight)
	if d, ok := ctx.Deadline(); ok {
		g.deadlines = append(g.deadlines, d)
	}
	g.mu.Unlock()

	if g.gate != nil {
		select {
		case <-g.gate:
		case <-ctx.Done():
		}
	}
	if g.delay > 0 {
		time.Sleep(g.delay)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.inFlight--
	g.ctxErrAfter = append(g.ctxErrAfter, ctx.Err())
	for _, e := range entries {
		g.messages = append(g.messages, e.Message)
	}
	return nil
}

func (g *gatedClient) current() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inFlight
}

func (g *gatedClient) snapshot() (peak int, msgs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.peak, slices.Clone(g.messages)
}

// hangFirstClient blocks its first `hang` calls until their context ends; later calls record
// each message with the context error seen on entry.
type hangFirstClient struct {
	hang int

	mu   sync.Mutex
	n    int
	sent map[string]error
}

func (h *hangFirstClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	h.mu.Lock()
	h.n++
	hang := h.n <= h.hang
	h.mu.Unlock()

	if hang {
		<-ctx.Done()
		return ctx.Err()
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sent == nil {
		h.sent = map[string]error{}
	}
	for _, e := range entries {
		h.sent[e.Message] = ctx.Err()
	}
	return nil
}

func (h *hangFirstClient) calls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.n
}

func (h *hangFirstClient) delivered() map[string]error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return maps.Clone(h.sent)
}

func startBatchClient(t *testing.T, c *components.BatchClient) (cancel context.CancelFunc, wait func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- c.Run(ctx) }()
	return cancel, func() error { return <-errc }
}

func namedEntries(prefix string, n int) []components.Entry {
	res := make([]components.Entry, n)
	for i := range res {
		res[i] = components.Entry{Level: "meow", Message: fmt.Sprintf("%s-%d", prefix, i), Time: time.Now()}
	}
	return res
}

func Test_BatchClient_MaxInFlight(t *testing.T) {
	t.Run("peak concurrency is bounded and reached", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.MaxInFlight(3), components.BatchSize(1),
			components.BufferSize(32), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 10)))
		r.Eventually(func() bool { return down.current() == 3 }, 5*time.Second, time.Millisecond)

		close(down.gate)
		r.Eventually(func() bool { _, m := down.snapshot(); return len(m) == 10 }, 5*time.Second, time.Millisecond)
		cancel()
		r.ErrorIs(wait(), context.Canceled)
		peak, _ := down.snapshot()
		r.Equal(3, peak)
	})

	t.Run("no loss and no duplication", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{}
		client := components.NewBatchClient(down, components.MaxInFlight(4), components.BatchSize(7),
			components.BufferSize(1000), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		in := namedEntries("m", 500)
		r.NoError(client.IngestLogs(context.Background(), in))
		cancel()
		r.ErrorIs(wait(), context.Canceled)

		_, msgs := down.snapshot()
		want := make([]string, len(in))
		for i, e := range in {
			want[i] = e.Message
		}
		slices.Sort(want)
		slices.Sort(msgs)
		r.Equal(want, msgs)
	})

	t.Run("hot path never blocks with all slots busy", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.DropWhenFull(),
			components.BufferSize(4), components.BatchSize(1), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)
		t.Cleanup(func() {
			cancel()
			close(down.gate)
			_ = wait()
		})

		start := time.Now()
		for range 1000 {
			r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 1)))
		}
		r.Less(time.Since(start), time.Second)
		r.Positive(client.Dropped())
	})

	t.Run("shutdown is bounded when downstream hangs", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.BatchSize(1),
			components.ShutdownTimeout(100*time.Millisecond), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)
		t.Cleanup(func() { close(down.gate) })

		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 2)))
		r.Eventually(func() bool { return down.current() == 2 }, 5*time.Second, time.Millisecond)

		start := time.Now()
		cancel()
		r.ErrorIs(wait(), context.Canceled)
		r.Less(time.Since(start), time.Second)
	})

	t.Run("shutdown waits for in-flight sends", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{delay: 50 * time.Millisecond}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.BatchSize(1),
			components.BufferSize(32), components.ShutdownTimeout(2*time.Second), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 6)))
		r.Eventually(func() bool { return down.current() > 0 }, 5*time.Second, time.Millisecond)
		cancel()
		r.ErrorIs(wait(), context.Canceled)

		_, msgs := down.snapshot()
		r.Len(msgs, 6)
		r.Zero(down.current())
	})

	t.Run("cancel while waiting for a slot still flushes the pending batch", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.BatchSize(1),
			components.BufferSize(32), components.ShutdownTimeout(5*time.Second), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 3)))
		r.Eventually(func() bool { return down.current() == 2 }, 5*time.Second, time.Millisecond)

		cancel()
		close(down.gate)
		r.ErrorIs(wait(), context.Canceled)

		_, msgs := down.snapshot()
		slices.Sort(msgs)
		r.Equal([]string{"m-0", "m-1", "m-2"}, msgs)
	})

	t.Run("request timeout is applied and parent cancel does not cancel sends", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.BatchSize(1),
			components.RequestTimeout(30*time.Second), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		before := time.Now()
		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 1)))
		r.Eventually(func() bool { return down.current() == 1 }, 5*time.Second, time.Millisecond)

		cancel()
		close(down.gate)
		r.ErrorIs(wait(), context.Canceled)

		down.mu.Lock()
		defer down.mu.Unlock()
		r.Len(down.deadlines, 1)
		r.WithinDuration(before.Add(30*time.Second), down.deadlines[0], 5*time.Second)
		r.Equal([]error{nil}, down.ctxErrAfter)
	})

	t.Run("final flush is not starved by hung in-flight sends", func(t *testing.T) {
		r := require.New(t)
		down := &hangFirstClient{hang: 2}
		client := components.NewBatchClient(down, components.MaxInFlight(2), components.BatchSize(1),
			components.BufferSize(32), components.ShutdownTimeout(200*time.Millisecond), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		r.NoError(client.IngestLogs(context.Background(), namedEntries("hung", 2)))
		r.Eventually(func() bool { return down.calls() == 2 }, 5*time.Second, time.Millisecond)
		r.NoError(client.IngestLogs(context.Background(), namedEntries("late", 1)))

		cancel()
		r.ErrorIs(wait(), context.Canceled)
		r.Equal(map[string]error{"late-0": nil}, down.delivered())
	})

	t.Run("default mode applies request timeout", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{gate: make(chan struct{})}
		client := components.NewBatchClient(down, components.BatchSize(1), components.BufferSize(32),
			components.RequestTimeout(30*time.Second), components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		before := time.Now()
		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 1)))
		r.Eventually(func() bool { return down.current() == 1 }, 5*time.Second, time.Millisecond)
		close(down.gate)
		cancel()
		r.ErrorIs(wait(), context.Canceled)

		down.mu.Lock()
		defer down.mu.Unlock()
		r.Len(down.deadlines, 1)
		r.WithinDuration(before.Add(30*time.Second), down.deadlines[0], 5*time.Second)
	})

	t.Run("default mode stays synchronous", func(t *testing.T) {
		r := require.New(t)
		down := &gatedClient{delay: 5 * time.Millisecond}
		client := components.NewBatchClient(down, components.BatchSize(1), components.BufferSize(32),
			components.FlushInterval(time.Hour))
		cancel, wait := startBatchClient(t, client)

		r.NoError(client.IngestLogs(context.Background(), namedEntries("m", 10)))
		r.Eventually(func() bool { _, m := down.snapshot(); return len(m) == 10 }, 5*time.Second, time.Millisecond)
		cancel()
		r.ErrorIs(wait(), context.Canceled)
		peak, _ := down.snapshot()
		r.Equal(1, peak)
	})
}
