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

func resetExitHandlers(t *testing.T) {
	t.Helper()
	exitHandlersMu.Lock()
	exitHandlers = nil
	exitHandlersMu.Unlock()
	t.Cleanup(func() {
		exitHandlersMu.Lock()
		exitHandlers = nil
		exitHandlersMu.Unlock()
	})
}

func TestRegisterExitHandler_RunsInRegistrationOrder(t *testing.T) {
	resetExitHandlers(t)
	r := require.New(t)

	var order []int
	RegisterExitHandler(func() { order = append(order, 1) })
	RegisterExitHandler(func() { order = append(order, 2) })
	RegisterExitHandler(func() { order = append(order, 3) })

	runExitHandlers()

	r.Equal([]int{1, 2, 3}, order)
}

func TestRegisterExitHandler_PanicIsolatesAndDoesNotBlockOthers(t *testing.T) {
	resetExitHandlers(t)
	r := require.New(t)

	var ran []string
	RegisterExitHandler(func() { ran = append(ran, "before") })
	RegisterExitHandler(func() { panic("boom") })
	RegisterExitHandler(func() { ran = append(ran, "after") })

	r.NotPanics(func() { runExitHandlers() })
	r.Equal([]string{"before", "after"}, ran)
}

func TestRegisterExitHandler_HandlerRegisteringAnotherDoesNotRunInSamePass(t *testing.T) {
	resetExitHandlers(t)
	r := require.New(t)

	var ranLate bool
	RegisterExitHandler(func() {
		RegisterExitHandler(func() { ranLate = true })
	})

	runExitHandlers()
	r.False(ranLate, "a handler registered during this pass should not run in the same pass")

	runExitHandlers()
	r.True(ranLate, "it should run on the next pass")
}

func TestRegisterExitHandler_ConcurrentRegistration(t *testing.T) {
	resetExitHandlers(t)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RegisterExitHandler(func() {})
		}()
	}
	wg.Wait()
	runExitHandlers() // would race under -race if RegisterExitHandler weren't locked
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
	resetExitHandlers(t)
	r := require.New(t)
	log, _ := NewNullLogger()
	fl := &fakeFlusher{}

	RegisterExitHandler(NewDefaultExitHandler(log, fl, time.Second))
	runExitHandlers()

	r.Equal(1, fl.called)
}

func TestFatal_RunsRegisteredExitHandlersBeforeExit(t *testing.T) {
	// Fatal itself calls os.Exit and can't be unit tested directly; this
	// verifies the piece Fatal delegates to (runExitHandlers), which is
	// the actual behavior under test -- see also TestRegisterExitHandler_*.
	resetExitHandlers(t)
	r := require.New(t)

	var flushed bool
	RegisterExitHandler(func() { flushed = true })
	runExitHandlers()

	r.True(flushed)
}
