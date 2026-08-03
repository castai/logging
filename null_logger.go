package logging

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
)

// TestEntry captures a single log record for assertions in tests.
// It intentionally mirrors the shape of github.com/sirupsen/logrus/hooks/test
type TestEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
	Time    time.Time
}

// TestHook captures records for later inspection. Register it as the base
// handler of a *Logger to observe emitted log lines in tests.
type TestHook struct {
	mu         sync.Mutex
	entries    []TestEntry
	baseAttrs  []slog.Attr // attributes attached via WithAttrs
	baseGroups []string    // group names attached via WithGroup
	parent     *TestHook   // non-nil on hooks derived via WithAttrs/WithGroup
}

func NewNullLogger() (*Logger, *TestHook) {
	hook := &TestHook{}
	log := New(hook)
	return log, hook
}

func (h *TestHook) Register(_ slog.Handler) slog.Handler {
	return h
}

func (h *TestHook) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *TestHook) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any, r.NumAttrs()+len(h.baseAttrs))

	for _, a := range h.baseAttrs {
		flattenAttr(attrs, h.baseGroups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(attrs, h.baseGroups, a)
		return true
	})

	entry := TestEntry{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   attrs,
		Time:    r.Time,
	}

	root := h.rootHook()
	root.mu.Lock()
	root.entries = append(root.entries, entry)
	root.mu.Unlock()
	return nil
}

func (h *TestHook) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TestHook{
		baseAttrs:  slices.Concat(h.baseAttrs, attrs),
		baseGroups: slices.Clone(h.baseGroups),
		parent:     h.rootHook(),
	}
}

func (h *TestHook) WithGroup(name string) slog.Handler {
	return &TestHook{
		baseAttrs:  slices.Clone(h.baseAttrs),
		baseGroups: append(slices.Clone(h.baseGroups), name),
		parent:     h.rootHook(),
	}
}

func (h *TestHook) AllEntries() []TestEntry {
	root := h.rootHook()
	root.mu.Lock()
	defer root.mu.Unlock()
	out := make([]TestEntry, len(root.entries))
	copy(out, root.entries)
	for i := range out {
		out[i].Attrs = maps.Clone(out[i].Attrs)
	}
	return out
}

func (h *TestHook) LastEntry() *TestEntry {
	root := h.rootHook()
	root.mu.Lock()
	defer root.mu.Unlock()
	if len(root.entries) == 0 {
		return nil
	}
	e := root.entries[len(root.entries)-1]
	e.Attrs = maps.Clone(e.Attrs)
	return &e
}

// Reset clears all captured entries.
func (h *TestHook) Reset() {
	root := h.rootHook()
	root.mu.Lock()
	defer root.mu.Unlock()
	root.entries = nil
}

// rootHook returns the topmost TestHook that owns the entries slice.
func (h *TestHook) rootHook() *TestHook {
	root := h
	for root.parent != nil {
		root = root.parent
	}
	return root
}

func flattenAttr(m map[string]any, groups []string, attr slog.Attr) {
	key := attr.Key
	if len(groups) > 0 {
		prefix := ""
		for _, g := range groups {
			prefix += g + "."
		}
		key = prefix + key
	}

	v := attr.Value
	// Resolve LogValuer wrappers
	v = v.Resolve()

	switch v.Kind() {
	case slog.KindGroup:
		for _, ga := range v.Group() {
			flattenAttr(m, append(slices.Clone(groups), attr.Key), ga)
		}
	default:
		m[key] = v.Any()
	}
}
