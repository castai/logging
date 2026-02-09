package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/castai/logging/components"
)

var _ Handler = new(ExportHandler)

var DefaultExportHandlerConfig = ExportHandlerConfig{
	MinLevel: slog.LevelInfo,
}

type ExportHandlerConfig struct {
	MinLevel slog.Level // Only export logs for this min log level.
}

func NewExportHandler(apiClient components.APIClient, cfg ExportHandlerConfig) *ExportHandler {
	handler := &ExportHandler{
		apiClient: apiClient,
		cfg:       cfg,
		attrs:     []slog.Attr{},
		groups:    []string{},
	}
	return handler
}

// ExportHandler export logs to remote.
type ExportHandler struct {
	cfg       ExportHandlerConfig
	next      slog.Handler
	apiClient components.APIClient

	attrs  []slog.Attr
	groups []string
}

func (h *ExportHandler) Register(next slog.Handler) slog.Handler {
	h.next = next
	return h
}

func (h *ExportHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.next == nil {
		return true
	}
	return h.next.Enabled(ctx, level)
}

func (h *ExportHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	if record.Level >= h.cfg.MinLevel {
		err = h.ingestLogs(&record)
	}
	if h.next == nil {
		return err
	}
	if handleErr := h.next.Handle(ctx, record); handleErr != nil {
		err = errors.Join(err, handleErr)
	}
	return err
}

func (h *ExportHandler) ingestLogs(record *slog.Record) error {
	if len(record.Message) == 0 {
		return nil
	}
	fieldsM := make(map[string]string)

	for _, attr := range h.attrs {
		addAttrToMap(fieldsM, attr, h.groups)
	}

	record.Attrs(func(attr slog.Attr) bool {
		addAttrToMap(fieldsM, attr, h.groups)
		return true
	})

	return h.apiClient.IngestLogs(context.Background(), []components.Entry{{
		Level:   mapSlogLevel(record.Level),
		Message: record.Message,
		Time:    record.Time,
		Fields:  fieldsM,
	}})
}

func (h *ExportHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &ExportHandler{
		cfg:       h.cfg,
		apiClient: h.apiClient,
		attrs:     slices.Clone(slices.Concat(h.attrs, attrs)),
		groups:    slices.Clone(h.groups),
	}
	if h.next != nil {
		clone.next = h.next.WithAttrs(attrs)
	}
	return clone
}

func (h *ExportHandler) WithGroup(name string) slog.Handler {
	clone := &ExportHandler{
		cfg:       h.cfg,
		apiClient: h.apiClient,
		attrs:     slices.Clone(h.attrs),
		groups:    append(slices.Clone(h.groups), name),
	}
	if h.next != nil {
		clone.next = h.next.WithGroup(name)
	}
	return clone
}

func addAttrToMap(m map[string]string, attr slog.Attr, groups []string) {
	key := attr.Key
	if len(groups) > 0 {
		prefix := ""
		for _, g := range groups {
			prefix += g + "."
		}
		key = prefix + key
	}

	// Handle different value types
	val := attr.Value
	switch val.Kind() {
	case slog.KindString:
		m[key] = val.String()
	case slog.KindInt64:
		m[key] = fmt.Sprintf("%d", val.Int64())
	case slog.KindUint64:
		m[key] = fmt.Sprintf("%d", val.Uint64())
	case slog.KindFloat64:
		m[key] = fmt.Sprintf("%f", val.Float64())
	case slog.KindBool:
		m[key] = fmt.Sprintf("%t", val.Bool())
	case slog.KindTime:
		m[key] = val.Time().Format("2006-01-02T15:04:05.000Z07:00")
	case slog.KindDuration:
		m[key] = val.Duration().String()
	case slog.KindAny:
		// Handle error type specially
		if err, ok := val.Any().(error); ok {
			m[key] = err.Error()
		} else {
			m[key] = fmt.Sprintf("%v", val.Any())
		}
	case slog.KindGroup:
		// Flatten groups into the map
		for _, groupAttr := range val.Group() {
			addAttrToMap(m, groupAttr, append(groups, key))
		}
	default:
		m[key] = fmt.Sprintf("%v", val.Any())
	}
}

func mapSlogLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return string(components.LogLevelLOGLEVELERROR)
	case level >= slog.LevelWarn:
		return string(components.LogLevelLOGLEVELWARNING)
	case level >= slog.LevelInfo:
		return string(components.LogLevelLOGLEVELINFO)
	case level >= slog.LevelDebug:
		return string(components.LogLevelLOGLEVELDEBUG)
	default:
		return string(components.LogLevelLOGLEVELUNKNOWN)
	}
}
