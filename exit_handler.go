package logging

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

var (
	exitHandlersMu sync.Mutex
	exitHandlers   []func()
)

// RegisterExitHandler registers a function to run before log.Fatal
func RegisterExitHandler(fn func()) {
	exitHandlersMu.Lock()
	defer exitHandlersMu.Unlock()
	exitHandlers = append(exitHandlers, fn)
}

func runExitHandlers() {
	exitHandlersMu.Lock()
	handlers := slices.Clone(exitHandlers)
	exitHandlersMu.Unlock()

	for _, fn := range handlers {
		runExitHandler(fn)
	}
}

func NewDefaultExitHandler(log FieldsLogger, f Flusher, timeout time.Duration) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := f.Flush(ctx); err != nil {
			log.Errorf("flush before exit failed: %v", err)
		}
	}
}

func runExitHandler(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "logging: exit handler panicked:", r)
		}
	}()
	fn()
}
