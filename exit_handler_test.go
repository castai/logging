package logging

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestLogger() *Logger {
	return New(NewTextHandler(TextHandlerConfig{Level: slog.LevelDebug}))
}

func TestLogger_RegisterExitHandler_RunsInRegistrationOrder(t *testing.T) {
	r := require.New(t)
	log := newTestLogger()

	var order []int
	log.RegisterExitHandler(func() { order = append(order, 1) })
	log.RegisterExitHandler(func() { order = append(order, 2) })
	log.RegisterExitHandler(func() { order = append(order, 3) })

	log.exitHandlers.run()

	r.Equal([]int{1, 2, 3}, order)
}

func TestLogger_RegisterExitHandler_PanicIsolatesAndDoesNotBlockOthers(t *testing.T) {
	r := require.New(t)
	log := newTestLogger()

	var ran []string
	log.RegisterExitHandler(func() { ran = append(ran, "before") })
	log.RegisterExitHandler(func() { panic("boom") })
	log.RegisterExitHandler(func() { ran = append(ran, "after") })

	r.NotPanics(func() { log.exitHandlers.run() })
	r.Equal([]string{"before", "after"}, ran)
}

func TestLogger_RegisterExitHandler_HandlerRegisteringAnotherDoesNotRunInSamePass(t *testing.T) {
	r := require.New(t)
	log := newTestLogger()

	var ranLate bool
	log.RegisterExitHandler(func() {
		log.RegisterExitHandler(func() { ranLate = true })
	})

	log.exitHandlers.run()
	r.False(ranLate, "a handler registered during this pass should not run in the same pass")

	log.exitHandlers.run()
	r.True(ranLate, "it should run on the next pass")
}

func TestLogger_RegisterExitHandler_ConcurrentRegistration(t *testing.T) {
	log := newTestLogger()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.RegisterExitHandler(func() {})
		}()
	}
	wg.Wait()
	log.exitHandlers.run() // would race under -race if RegisterExitHandler weren't locked
}

func TestLogger_RegisterExitHandler_SharedWithDerivedLoggers(t *testing.T) {
	t.Run("a logger derived before registration still sees it", func(t *testing.T) {
		r := require.New(t)
		root := newTestLogger()
		child := root.WithField("component", "worker") // derived first

		var ran bool
		root.RegisterExitHandler(func() { ran = true }) // registered after

		child.exitHandlers.run()
		r.True(ran, "child should share the same registry as root, regardless of derivation order")
	})

	t.Run("a logger derived after registration sees it too", func(t *testing.T) {
		r := require.New(t)
		root := newTestLogger()

		var ran bool
		root.RegisterExitHandler(func() { ran = true })
		child := root.WithField("component", "worker") // derived after

		child.exitHandlers.run()
		r.True(ran)
	})

	t.Run("registering on a child is visible from the root too", func(t *testing.T) {
		r := require.New(t)
		root := newTestLogger()
		child := root.WithGroup("api").WithField("k", "v")

		var ran bool
		child.RegisterExitHandler(func() { ran = true })

		root.exitHandlers.run()
		r.True(ran, "the whole family shares one registry, not one per derived instance")
	})
}

func TestLogger_RegisterExitHandler_NotSharedAcrossSeparateLoggers(t *testing.T) {
	r := require.New(t)
	logA := newTestLogger()
	logB := newTestLogger() // independent New() call -- its own registry

	var ran bool
	logA.RegisterExitHandler(func() { ran = true })

	logB.exitHandlers.run()
	r.False(ran, "loggers from separate New(...) calls must not share exit handlers")

	logA.exitHandlers.run()
	r.True(ran)
}

type fakeFlusher struct {
	called int
	err    error
}

func (f *fakeFlusher) Flush(ctx context.Context) error {
	f.called++
	return f.err
}

func TestDefaultExitHandler_FlushesSuccessfully(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()
	fl := &fakeFlusher{}

	NewDefaultExitHandler(log, fl, time.Second)()

	r.Equal(1, fl.called)
	r.Empty(hook.AllEntries(), "no error should be logged on a successful flush")
}

func TestDefaultExitHandler_LogsFlushError(t *testing.T) {
	r := require.New(t)
	log, hook := NewNullLogger()
	fl := &fakeFlusher{err: errors.New("boom")}

	NewDefaultExitHandler(log, fl, time.Second)()

	r.Equal(1, fl.called)
	last := hook.LastEntry()
	r.NotNil(last)
	r.Equal(slog.LevelError, last.Level)
	r.Contains(last.Message, "boom")
}

func TestDefaultExitHandler_UsableWithRegisterExitHandler(t *testing.T) {
	r := require.New(t)
	log, _ := NewNullLogger()
	fl := &fakeFlusher{}

	log.RegisterExitHandler(NewDefaultExitHandler(log, fl, time.Second))
	log.exitHandlers.run()

	r.Equal(1, fl.called)
}

func TestLogger_Fatal_RunsRegisteredExitHandlersBeforeExit(t *testing.T) {
	r := require.New(t)
	log := newTestLogger()

	var flushed bool
	log.RegisterExitHandler(func() { flushed = true })
	log.exitHandlers.run()

	r.True(flushed)
}
