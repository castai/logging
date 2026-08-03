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
// Entry (Level, Message, Data, Time) so migrations from that hook are
// mechanical: rename Data -> Attrs and Level's type from logrus.Level to
// slog.Level.
type TestEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
	Time    time.Time
}

// TestHook captures records for later inspection. Register it as the base
// handler of a *Logger to observe emitted log lines in tests. It is
// concurrency-safe.
//
// Handlers produced by WithAttrs/WithGroup share the entries slice of the
// root hook via the parent pointer, so callers who hold the original hook
// still observe records emitted through derived loggers.
type TestHook struct {
	mu         sync.Mutex
	entries    []TestEntry
	baseAttrs  []slog.Attr // attributes attached via WithAttrs
	baseGroups []string    // group names attached via WithGroup
	parent     *TestHook   // non-nil on hooks derived via WithAttrs/WithGroup
}

// NewNullLogger returns a *Logger whose only handler is a TestHook. All
// emitted records are captured by the returned hook; nothing is written
// anywhere else.
//
// The returned *Logger accepts records at all levels (debug through error).
// Use hook.AllEntries() to read them back.
func NewNullLogger() (*Logger, *TestHook) {
	hook := &TestHook{}
	log := New(hook)
	return log, hook
}

// Register makes TestHook satisfy the Handler interface for use with New.
// The hook does not forward to a next handler; it acts as a terminal sink.
func (h *TestHook) Register(_ slog.Handler) slog.Handler {
	return h
}

// Enabled reports whether the given level is captured. All levels are
// captured by default so tests can assert on debug-level records emitted
// under an info-level runtime configuration.
func (h *TestHook) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle records the given slog.Record into the hook's entries slice.
func (h *TestHook) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any, r.NumAttrs()+len(h.baseAttrs))

	// Include attributes accumulated via WithAttrs, respecting groups.
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

// WithAttrs returns a shallow copy of the hook with the additional attributes
// remembered for subsequent Handle calls. The derived hook shares the
// parent's entries slice so tests reading from the original hook see records
// emitted through derived loggers.
func (h *TestHook) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TestHook{
		baseAttrs:  slices.Concat(h.baseAttrs, attrs),
		baseGroups: slices.Clone(h.baseGroups),
		parent:     h.rootHook(),
	}
}

// WithGroup returns a shallow copy of the hook with the additional group
// name appended. The derived hook shares the parent's entries slice.
func (h *TestHook) WithGroup(name string) slog.Handler {
	return &TestHook{
		baseAttrs:  slices.Clone(h.baseAttrs),
		baseGroups: append(slices.Clone(h.baseGroups), name),
		parent:     h.rootHook(),
	}
}

// AllEntries returns a copy of the captured entries in insertion order.
// Safe to call concurrently with emitting records.
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

// LastEntry returns the most recently captured entry, or nil if none.
func (h *TestHook) LastEntry() *TestEntry {
	root := h.rootHook()
	root.mu.Lock()
	defer root.mu.Unlock()
	if len(root.entries) == 0 {
		return nil
	}
	// Return a deep copy so mutating the result (including its Attrs map)
	// cannot alias or race with internal state.
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

// flattenAttr writes attr into m, honoring an active group path.
// Groups are joined with '.' to match ExportHandler's convention.
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
	// Resolve LogValuer wrappers (e.g. errors that implement slog.LogValuer).
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
