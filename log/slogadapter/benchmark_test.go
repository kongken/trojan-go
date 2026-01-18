package slogadapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/log/golog"
	"github.com/p4gefau1t/trojan-go/log/simplelog"
)

// Benchmark configuration
const (
	benchmarkIterations = 1000
	concurrentWorkers   = 10
	messageSize         = 100
)

// Test message generators
func generateMessage(size int) string {
	return fmt.Sprintf("benchmark message %s", string(make([]byte, size-20)))
}

func generateLargeMessage() string {
	return generateMessage(1024) // 1KB message
}

func generateSmallMessage() string {
	return generateMessage(50) // 50 byte message
}

// Benchmark helpers
type benchmarkWriter struct {
	io.Writer
}

func (bw *benchmarkWriter) Fd() uintptr {
	return 1 // stdout
}

func newBenchmarkWriter() *benchmarkWriter {
	return &benchmarkWriter{Writer: io.Discard}
}

// BenchmarkSlogAdapterVsGolog compares SlogAdapter performance with golog
func BenchmarkSlogAdapterVsGolog(b *testing.B) {
	message := generateSmallMessage()

	b.Run("SlogAdapter_Info", func(b *testing.B) {
		writer := newBenchmarkWriter()
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message)
		}
	})

	b.Run("Golog_Info", func(b *testing.B) {
		writer := newBenchmarkWriter()
		logger := golog.New(writer)
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message)
		}
	})

	b.Run("SlogAdapter_Infof", func(b *testing.B) {
		writer := newBenchmarkWriter()
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Infof("formatted message: %s", message)
		}
	})

	b.Run("Golog_Infof", func(b *testing.B) {
		writer := newBenchmarkWriter()
		logger := golog.New(writer)
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Infof("formatted message: %s", message)
		}
	})
}

// BenchmarkSlogAdapterVsSimplelog compares SlogAdapter performance with simplelog
func BenchmarkSlogAdapterVsSimplelog(b *testing.B) {
	message := generateSmallMessage()

	b.Run("SlogAdapter_Info", func(b *testing.B) {
		writer := newBenchmarkWriter()
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message)
		}
	})

	b.Run("Simplelog_Info", func(b *testing.B) {
		logger := &simplelog.SimpleLogger{}
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message)
		}
	})

	b.Run("SlogAdapter_Error", func(b *testing.B) {
		writer := newBenchmarkWriter()
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.ErrorLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Error(message)
		}
	})

	b.Run("Simplelog_Error", func(b *testing.B) {
		logger := &simplelog.SimpleLogger{}
		logger.SetLogLevel(log.ErrorLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Error(message)
		}
	})
}

// BenchmarkLogLevels tests performance across different log levels
func BenchmarkLogLevels(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	levels := []struct {
		name  string
		level log.LogLevel
	}{
		{"AllLevel", log.AllLevel},
		{"InfoLevel", log.InfoLevel},
		{"WarnLevel", log.WarnLevel},
		{"ErrorLevel", log.ErrorLevel},
		{"OffLevel", log.OffLevel},
	}

	for _, level := range levels {
		b.Run("SlogAdapter_"+level.name, func(b *testing.B) {
			adapter := NewSlogAdapter(writer, false)
			adapter.SetLogLevel(level.level)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				adapter.Info(message)
				adapter.Warn(message)
				adapter.Error(message)
				adapter.Debug(message)
				adapter.Trace(message)
			}
		})

		b.Run("Golog_"+level.name, func(b *testing.B) {
			logger := golog.New(writer)
			logger.SetLogLevel(level.level)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info(message)
				logger.Warn(message)
				logger.Error(message)
				logger.Debug(message)
				logger.Trace(message)
			}
		})
	}
}

// BenchmarkMessageSizes tests performance with different message sizes
func BenchmarkMessageSizes(b *testing.B) {
	writer := newBenchmarkWriter()

	sizes := []struct {
		name string
		msg  string
	}{
		{"Small_50B", generateMessage(50)},
		{"Medium_200B", generateMessage(200)},
		{"Large_1KB", generateMessage(1024)},
		{"XLarge_4KB", generateMessage(4096)},
	}

	for _, size := range sizes {
		b.Run("SlogAdapter_"+size.name, func(b *testing.B) {
			adapter := NewSlogAdapter(writer, false)
			adapter.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				adapter.Info(size.msg)
			}
		})

		b.Run("Golog_"+size.name, func(b *testing.B) {
			logger := golog.New(writer)
			logger.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info(size.msg)
			}
		})
	}
}

// BenchmarkOutputFormats tests performance across different output formats
func BenchmarkOutputFormats(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	formats := []struct {
		name   string
		format LogFormat
	}{
		{"Text", TextFormat},
		{"JSON", JSONFormat},
		{"Colored", ColoredFormat},
	}

	for _, format := range formats {
		b.Run("SlogAdapter_"+format.name, func(b *testing.B) {
			adapter := NewSlogAdapterWithFormat(writer, format.format)
			adapter.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				adapter.Info(message)
			}
		})
	}

	// Compare with golog colored output
	b.Run("Golog_Colored", func(b *testing.B) {
		logger := golog.New(writer).WithColor()
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message)
		}
	})
}

// BenchmarkStructuredLogging tests performance of structured logging features
func BenchmarkStructuredLogging(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_BasicLogging", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message)
		}
	})

	b.Run("SlogAdapter_StructuredLogging_SingleAttr", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.InfoWith(message, slog.String("key", "value"))
		}
	})

	b.Run("SlogAdapter_StructuredLogging_MultipleAttrs", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.InfoWith(message,
				slog.String("key1", "value1"),
				slog.Int("key2", 42),
				slog.Bool("key3", true),
				slog.Float64("key4", 3.14),
			)
		}
	})

	b.Run("SlogAdapter_WithAttrs", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)
		adapterWithAttrs := adapter.WithAttrs(
			slog.String("service", "benchmark"),
			slog.String("version", "1.0.0"),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapterWithAttrs.Info(message)
		}
	})

	b.Run("SlogAdapter_WithGroup", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)
		adapterWithGroup := adapter.WithGroup("benchmark")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapterWithGroup.InfoWith(message, slog.String("key", "value"))
		}
	})

	b.Run("SlogAdapter_ContextLogging", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)
		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.InfoCtx(ctx, message, slog.String("key", "value"))
		}
	})
}

// BenchmarkConcurrentLogging tests performance under concurrent load
func BenchmarkConcurrentLogging(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_Concurrent", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				adapter.Info(message)
			}
		})
	})

	b.Run("Golog_Concurrent", func(b *testing.B) {
		logger := golog.New(writer)
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(message)
			}
		})
	})

	b.Run("SlogAdapter_ConcurrentStructured", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				adapter.InfoWith(message, slog.String("worker", "test"))
			}
		})
	})
}

// BenchmarkLevelFiltering tests performance of level filtering optimization
func BenchmarkLevelFiltering(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_DisabledLevel_Info", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.ErrorLevel) // Disable Info level

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message) // Should be filtered out
		}
	})

	b.Run("Golog_DisabledLevel_Info", func(b *testing.B) {
		logger := golog.New(writer)
		logger.SetLogLevel(log.ErrorLevel) // Disable Info level

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message) // Should be filtered out
		}
	})

	b.Run("SlogAdapter_DisabledLevel_Infof", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.ErrorLevel) // Disable Info level

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Infof("expensive formatting: %s %d %v", message, i, time.Now()) // Should be filtered out
		}
	})

	b.Run("Golog_DisabledLevel_Infof", func(b *testing.B) {
		logger := golog.New(writer)
		logger.SetLogLevel(log.ErrorLevel) // Disable Info level

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Infof("expensive formatting: %s %d %v", message, i, time.Now()) // Should be filtered out
		}
	})

	b.Run("SlogAdapter_DisabledLevel_Structured", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.ErrorLevel) // Disable Info level

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.InfoWith(message,
				slog.String("key1", "value1"),
				slog.Int("key2", i),
				slog.Any("key3", time.Now()),
			) // Should be filtered out
		}
	})
}

// BenchmarkMemoryUsage tests memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_MemoryUsage", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})

	b.Run("Golog_MemoryUsage", func(b *testing.B) {
		logger := golog.New(writer)
		logger.SetLogLevel(log.InfoLevel)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})

	b.Run("SlogAdapter_StructuredMemoryUsage", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.InfoWith(message,
				slog.String("key1", "value1"),
				slog.Int("key2", i),
			)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
	})
}

// BenchmarkOutputWriters tests performance with different output writers
func BenchmarkOutputWriters(b *testing.B) {
	message := generateSmallMessage()

	writers := []struct {
		name   string
		writer io.Writer
	}{
		{"Discard", io.Discard},
		{"Buffer", &bytes.Buffer{}},
	}

	for _, w := range writers {
		b.Run("SlogAdapter_"+w.name, func(b *testing.B) {
			adapter := NewSlogAdapter(w.writer, false)
			adapter.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				adapter.Info(message)
			}
		})

		b.Run("Golog_"+w.name, func(b *testing.B) {
			var logger *golog.Logger
			if fdWriter, ok := w.writer.(golog.FdWriter); ok {
				logger = golog.New(fdWriter)
			} else {
				// For non-FdWriter, use a wrapper
				logger = golog.New(newBenchmarkWriter())
				logger.SetOutput(w.writer)
			}
			logger.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				logger.Info(message)
			}
		})
	}
}

// BenchmarkRuntimeLevelChanges tests performance of runtime level changes
func BenchmarkRuntimeLevelChanges(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_LevelChanges", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		levels := []log.LogLevel{log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.AllLevel}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.SetLogLevel(levels[i%len(levels)])
			adapter.Info(message)
		}
	})

	b.Run("Golog_LevelChanges", func(b *testing.B) {
		logger := golog.New(writer)
		levels := []log.LogLevel{log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.AllLevel}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.SetLogLevel(levels[i%len(levels)])
			logger.Info(message)
		}
	})
}

// BenchmarkErrorHandling tests performance of error handling scenarios
func BenchmarkErrorHandling(b *testing.B) {
	message := generateSmallMessage()

	// Error writer that always fails
	errorWriter := &benchmarkErrorWriter{shouldError: true}

	b.Run("SlogAdapter_ErrorHandling", func(b *testing.B) {
		adapter := NewSlogAdapter(errorWriter, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			adapter.Info(message) // Should handle write errors gracefully
		}
	})

	b.Run("Golog_ErrorHandling", func(b *testing.B) {
		logger := golog.New(newBenchmarkWriter())
		logger.SetOutput(errorWriter)
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			logger.Info(message) // May not handle errors as gracefully
		}
	})
}

// BenchmarkComplexScenarios tests realistic usage patterns
func BenchmarkComplexScenarios(b *testing.B) {
	writer := newBenchmarkWriter()

	b.Run("SlogAdapter_WebServerScenario", func(b *testing.B) {
		adapter := NewSlogAdapter(writer, false)
		adapter.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate web server logging pattern
			adapter.InfoWith("request started",
				slog.String("method", "GET"),
				slog.String("path", "/api/users"),
				slog.String("remote_addr", "192.168.1.100"),
				slog.String("user_agent", "Mozilla/5.0"),
			)

			adapter.DebugWith("processing request",
				slog.Int("user_id", 12345),
				slog.String("action", "fetch_profile"),
			)

			if i%10 == 0 {
				adapter.WarnWith("slow query detected",
					slog.Duration("duration", 150*time.Millisecond),
					slog.String("query", "SELECT * FROM users WHERE id = ?"),
				)
			}

			adapter.InfoWith("request completed",
				slog.Int("status_code", 200),
				slog.Duration("duration", 45*time.Millisecond),
				slog.Int("response_size", 1024),
			)
		}
	})

	b.Run("Golog_WebServerScenario", func(b *testing.B) {
		logger := golog.New(writer)
		logger.SetLogLevel(log.InfoLevel)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate web server logging pattern with traditional logging
			logger.Infof("request started method=%s path=%s remote_addr=%s user_agent=%s",
				"GET", "/api/users", "192.168.1.100", "Mozilla/5.0")

			logger.Debugf("processing request user_id=%d action=%s", 12345, "fetch_profile")

			if i%10 == 0 {
				logger.Warnf("slow query detected duration=%v query=%s",
					150*time.Millisecond, "SELECT * FROM users WHERE id = ?")
			}

			logger.Infof("request completed status_code=%d duration=%v response_size=%d",
				200, 45*time.Millisecond, 1024)
		}
	})
}

// BenchmarkConcurrentWorkers tests performance with multiple concurrent workers
func BenchmarkConcurrentWorkers(b *testing.B) {
	message := generateSmallMessage()
	writer := newBenchmarkWriter()

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("SlogAdapter_Workers_%d", workers), func(b *testing.B) {
			adapter := NewSlogAdapter(writer, false)
			adapter.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			workPerWorker := b.N / workers

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < workPerWorker; i++ {
						adapter.Info(message)
					}
				}()
			}
			wg.Wait()
		})

		b.Run(fmt.Sprintf("Golog_Workers_%d", workers), func(b *testing.B) {
			logger := golog.New(writer)
			logger.SetLogLevel(log.InfoLevel)

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			workPerWorker := b.N / workers

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < workPerWorker; i++ {
						logger.Info(message)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// benchmarkErrorWriter for testing error handling performance
type benchmarkErrorWriter struct {
	shouldError bool
	errorCount  int
}

func (ew *benchmarkErrorWriter) Write(p []byte) (n int, err error) {
	if ew.shouldError {
		ew.errorCount++
		return 0, fmt.Errorf("write error")
	}
	return len(p), nil
}
