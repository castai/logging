package components_test

import (
	"context"
	"errors"
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

func TestBatchClient_Flush(t *testing.T) {
	t.Run("sends buffered entries without Run being started", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(100), components.FlushInterval(time.Hour))

		for i := 0; i < 5; i++ {
			err := client.IngestLogs(context.Background(), []components.Entry{{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			}})
			r.NoError(err)
		}

		r.NoError(client.Flush(context.Background()))
		r.Len(mockAPIClient.getLogs(), 5)
	})

	t.Run("is a no-op when nothing is buffered", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(100), components.FlushInterval(time.Hour))

		r.NoError(client.Flush(context.Background()))
		r.Empty(mockAPIClient.getLogs())
	})

	t.Run("safe to call while Run is still active", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &apiClient{}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(100), components.FlushInterval(time.Hour))

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			_ = client.Run(ctx)
			close(done)
		}()
		defer func() {
			cancel()
			<-done
		}()

		for i := 0; i < 3; i++ {
			err := client.IngestLogs(ctx, []components.Entry{{
				Level:   "info",
				Message: "test message",
				Time:    time.Now(),
			}})
			r.NoError(err)
		}

		// Give Run's select loop a chance to potentially race for the same
		// entries; either way none should be lost or duplicated.
		time.Sleep(10 * time.Millisecond)
		r.NoError(client.Flush(ctx))
		r.Len(mockAPIClient.getLogs(), 3)
	})

	t.Run("propagates the underlying client's error", func(t *testing.T) {
		r := require.New(t)
		mockAPIClient := &erroringAPIClient{err: errors.New("boom")}
		client := components.NewBatchClient(mockAPIClient, components.BatchSize(100), components.FlushInterval(time.Hour))

		r.NoError(client.IngestLogs(context.Background(), []components.Entry{{
			Level:   "info",
			Message: "test message",
			Time:    time.Now(),
		}}))

		r.ErrorIs(client.Flush(context.Background()), mockAPIClient.err)
	})
}

type erroringAPIClient struct {
	err error
}

func (e *erroringAPIClient) IngestLogs(ctx context.Context, entries []components.Entry) error {
	return e.err
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
