package benchmarks

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/castai/logging"
	"github.com/sirupsen/logrus"
)

var (
	benchErr = errors.New("boom: connection refused")

	benchFields = map[string]any{
		"user_id":     "u-123",
		"request_id":  "req-456",
		"component":   "checkout",
		"duration_ms": 42,
		"retry":       3,
		"success":     true,
		"region":      "eu-west-1",
		"tenant":      "acme-corp",
	}
)

func newCastaiJSON() *logging.Logger {
	return logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
		Level:  slog.LevelDebug,
		Output: io.Discard,
	}))
}

func newCastaiText() *logging.Logger {
	return logging.New(logging.NewTextHandler(logging.TextHandlerConfig{
		Level:  slog.LevelDebug,
		Output: io.Discard,
	}))
}

func newCastaiJSONWithSource() *logging.Logger {
	return logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
		Level:     slog.LevelDebug,
		Output:    io.Discard,
		AddSource: true,
	}))
}

func newLogrusJSON() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)
	return l
}

func newLogrusText() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	l.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	l.SetLevel(logrus.DebugLevel)
	return l
}

func newLogrusJSONWithSource() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)
	l.SetReportCaller(true)
	return l
}

// runVariants runs fn against the four standard logger configurations so
// each scenario benchmarks castai/logging and logrus under matching JSON
// and text formats.
func runCastaiVariants(b *testing.B, fn func(b *testing.B, log *logging.Logger)) {
	b.Run("castai-json", func(b *testing.B) { fn(b, newCastaiJSON()) })
	b.Run("castai-text", func(b *testing.B) { fn(b, newCastaiText()) })
}

func runLogrusVariants(b *testing.B, fn func(b *testing.B, log *logrus.Logger)) {
	b.Run("logrus-json", func(b *testing.B) { fn(b, newLogrusJSON()) })
	b.Run("logrus-text", func(b *testing.B) { fn(b, newLogrusText()) })
}

// BenchmarkNoFields measures the bare emit path: one Info call, no derived
// fields, level enabled.
func BenchmarkNoFields(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("service starting")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("service starting")
		}
	})
}

// BenchmarkOneField measures deriving a logger with a single field and
// emitting once, per iteration — the common "log.WithField(...).Info(...)"
// call-site pattern.
func BenchmarkOneField(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").Info("order placed")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").Info("order placed")
		}
	})
}

// BenchmarkManyFields measures deriving a logger with eight fields at once
// (WithFields / logrus.Fields) and emitting once per iteration.
func BenchmarkManyFields(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithFields(benchFields).Info("order placed")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithFields(logrus.Fields(benchFields)).Info("order placed")
		}
	})
}

// BenchmarkChainedFields measures building up a logger through four
// sequential single-field derivations (log.WithField(a).WithField(b)...),
// mirroring code that layers context across several call-sites before
// finally logging.
func BenchmarkChainedFields(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").
				WithField("region", "eu-west-1").
				WithField("tenant", "acme-corp").
				WithField("request_id", "req-456").
				Info("order placed")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").
				WithField("region", "eu-west-1").
				WithField("tenant", "acme-corp").
				WithField("request_id", "req-456").
				Info("order placed")
		}
	})
}

// BenchmarkReusedDerivedLogger measures the emit-only cost once field
// derivation has already happened outside the hot loop — the pattern of
// building a component-scoped logger once (e.g. in a constructor) and
// reusing it across many log calls.
func BenchmarkReusedDerivedLogger(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		derived := log.WithFields(benchFields)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			derived.Info("order placed")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		derived := log.WithFields(logrus.Fields(benchFields))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			derived.Info("order placed")
		}
	})
}

// BenchmarkErrorField measures the common "attach an error and log it"
// pattern: WithFieldAny("error", err) vs logrus's dedicated WithError.
func BenchmarkErrorField(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithFieldAny("error", benchErr).Error("request failed")
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithError(benchErr).Error("request failed")
		}
	})
}

// BenchmarkFormattedMessage measures printf-style logging (Infof), which
// costs an extra fmt.Sprintf over the plain-message path.
func BenchmarkFormattedMessage(b *testing.B) {
	runCastaiVariants(b, func(b *testing.B, log *logging.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Infof("order %s placed by %s in %dms", "ord-789", "u-123", 42)
		}
	})
	runLogrusVariants(b, func(b *testing.B, log *logrus.Logger) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Infof("order %s placed by %s in %dms", "ord-789", "u-123", 42)
		}
	})
}

// BenchmarkDisabledLevel measures the cost of a log call that is filtered
// out by level (Debug against a logger configured at Info). The "bare"
// variant isolates the level-check fast path (no derivation); the
// "with-fields" variant shows that a preceding .WithField/.WithFields call
// still pays its full derivation cost even though the eventual Debug is
// discarded — a common perf gotcha in both libraries.
func BenchmarkDisabledLevel(b *testing.B) {
	b.Run("castai-json-bare", func(b *testing.B) {
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:  slog.LevelInfo,
			Output: io.Discard,
		}))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Debug("verbose trace")
		}
	})
	b.Run("castai-json-with-fields", func(b *testing.B) {
		log := logging.New(logging.NewJSONHandler(logging.JSONHandlerConfig{
			Level:  slog.LevelInfo,
			Output: io.Discard,
		}))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").Debug("verbose trace")
		}
	})
	b.Run("logrus-json-bare", func(b *testing.B) {
		l := logrus.New()
		l.SetOutput(io.Discard)
		l.SetFormatter(&logrus.JSONFormatter{})
		l.SetLevel(logrus.InfoLevel)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			l.Debug("verbose trace")
		}
	})
	b.Run("logrus-json-with-fields", func(b *testing.B) {
		l := logrus.New()
		l.SetOutput(io.Discard)
		l.SetFormatter(&logrus.JSONFormatter{})
		l.SetLevel(logrus.InfoLevel)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			l.WithField("component", "checkout").Debug("verbose trace")
		}
	})
}

// BenchmarkWithSource measures the added cost of capturing the caller's
// source location (AddSource / ReportCaller), which both libraries support
// but neither enables by default.
func BenchmarkWithSource(b *testing.B) {
	b.Run("castai-json-source", func(b *testing.B) {
		log := newCastaiJSONWithSource()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").Info("order placed")
		}
	})
	b.Run("logrus-json-source", func(b *testing.B) {
		log := newLogrusJSONWithSource()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.WithField("component", "checkout").Info("order placed")
		}
	})
}

// BenchmarkParallelWithFields measures write-path contention (shared
// io.Discard writer, formatter mutex) when many goroutines each hold their
// own field-derived logger and log concurrently.
func BenchmarkParallelWithFields(b *testing.B) {
	b.Run("castai-json", func(b *testing.B) {
		log := newCastaiJSON()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			derived := log.WithFields(benchFields)
			for pb.Next() {
				derived.Info("order placed")
			}
		})
	})
	b.Run("logrus-json", func(b *testing.B) {
		log := newLogrusJSON()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			derived := log.WithFields(logrus.Fields(benchFields))
			for pb.Next() {
				derived.Info("order placed")
			}
		})
	})
}
