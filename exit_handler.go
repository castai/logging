package logging

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

type exitHandlerRegistry struct {
	mu       sync.Mutex
	handlers []func()
}

func newExitHandlerRegistry() *exitHandlerRegistry {
	return &exitHandlerRegistry{}
}

func (r *exitHandlerRegistry) register(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, fn)
}

func (r *exitHandlerRegistry) run() {
	r.mu.Lock()
	handlers := slices.Clone(r.handlers)
	r.mu.Unlock()

	for _, fn := range handlers {
		runExitHandler(fn)
	}
}

func runExitHandler(fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintln(os.Stderr, "logging: exit handler panicked:", rec)
		}
	}()
	fn()
}

// NewDefaultExitHandler returns a default exit handler flushing func.
func NewDefaultExitHandler(log FieldsLogger, f Flusher, timeout time.Duration) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := f.Flush(ctx); err != nil {
			log.Errorf("flush before exit failed: %v", err)
		}
	}
}
