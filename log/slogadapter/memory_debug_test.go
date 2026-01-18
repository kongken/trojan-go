package slogadapter

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"testing"

	"github.com/p4gefau1t/trojan-go/log"
)

// TestMemoryUsageDebug helps understand actual memory usage patterns
func TestMemoryUsageDebug(t *testing.T) {
	var buf bytes.Buffer
	adapter := NewSlogAdapter(&buf, false)
	adapter.SetLogLevel(log.AllLevel)

	// Measure memory usage over multiple batches
	const batchSize = 100
	const numBatches = 5

	for batch := 0; batch < numBatches; batch++ {
		// Force GC before measurement
		runtime.GC()
		runtime.GC()

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Perform logging operations
		for i := 0; i < batchSize; i++ {
			message := fmt.Sprintf("batch_%d_msg_%d", batch, i)
			adapter.Info(message)
			adapter.Infof("formatted: %s", message)
			adapter.ErrorWith("structured", slog.String("key", message))
		}

		runtime.ReadMemStats(&m2)
		allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
		allocatedObjects := m2.Mallocs - m1.Mallocs

		t.Logf("Batch %d: %d bytes, %d objects", batch, allocatedBytes, allocatedObjects)
	}
}

// TestLazyFormattingDebug helps understand allocation patterns for disabled levels
func TestLazyFormattingDebug(t *testing.T) {
	var buf bytes.Buffer
	adapter := NewSlogAdapter(&buf, false)
	adapter.SetLogLevel(log.ErrorLevel) // Disable INFO level

	// Force garbage collection before measurement
	runtime.GC()
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform many disabled log operations
	for i := 0; i < 1000; i++ {
		message := fmt.Sprintf("disabled_message_%d", i)
		adapter.Info(message) // Should be disabled and fast
	}

	runtime.ReadMemStats(&m2)

	allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
	allocatedObjects := m2.Mallocs - m1.Mallocs

	t.Logf("Disabled operations: %d bytes, %d objects for 1000 calls", allocatedBytes, allocatedObjects)
}
