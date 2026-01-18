package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/log/slogadapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealLogOutputVerification tests real log output with various formats and destinations
// Requirements: 7.2
func TestRealLogOutputVerification(t *testing.T) {
	tests := []struct {
		name   string
		format slogadapter.LogFormat
	}{
		{"text_format", slogadapter.TextFormat},
		{"json_format", slogadapter.JSONFormat},
		{"colored_format", slogadapter.ColoredFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			adapter := slogadapter.NewSlogAdapterWithFormat(&buf, tt.format)

			// Register as global logger
			log.RegisterLogger(adapter)

			// Test basic logging
			log.Info("test info message")
			log.Error("test error message")
			log.Warn("test warning message")

			output := buf.String()
			require.NotEmpty(t, output, "Should produce log output")

			// Verify all messages are present
			assert.Contains(t, output, "test info message")
			assert.Contains(t, output, "test error message")
			assert.Contains(t, output, "test warning message")

			// Format-specific verification
			switch tt.format {
			case slogadapter.JSONFormat:
				// Verify JSON structure
				lines := strings.Split(strings.TrimSpace(output), "\n")
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						continue
					}
					var jsonObj map[string]interface{}
					err := json.Unmarshal([]byte(line), &jsonObj)
					assert.NoError(t, err, "Should be valid JSON: %s", line)

					// Check required fields
					assert.Contains(t, jsonObj, "time")
					assert.Contains(t, jsonObj, "level")
					assert.Contains(t, jsonObj, "msg")
				}
			case slogadapter.TextFormat, slogadapter.ColoredFormat:
				// Verify text structure contains level indicators
				assert.Contains(t, output, "level=INFO")
				assert.Contains(t, output, "level=ERROR")
				assert.Contains(t, output, "level=WARN")
			}
		})
	}
}

// TestVariousOutputDestinations tests logging to different output destinations
// Requirements: 7.2
func TestVariousOutputDestinations(t *testing.T) {
	t.Run("memory_buffer", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := slogadapter.NewSlogAdapter(&buf, false)
		log.RegisterLogger(adapter)

		log.Info("buffer test message")

		output := buf.String()
		assert.Contains(t, output, "buffer test message")
	})

	t.Run("discard_writer", func(t *testing.T) {
		adapter := slogadapter.NewSlogAdapter(io.Discard, false)
		log.RegisterLogger(adapter)

		// Should not panic
		log.Info("discard test message")
		log.Error("discard error message")
	})

	t.Run("temporary_file", func(t *testing.T) {
		// Create temporary file
		tmpFile, err := os.CreateTemp("", "slog_test_*.log")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		adapter := slogadapter.NewSlogAdapter(tmpFile, false)
		log.RegisterLogger(adapter)

		testMessage := "temporary file test message"
		log.Info(testMessage)

		// Flush and read back
		tmpFile.Sync()
		tmpFile.Seek(0, 0)

		content, err := io.ReadAll(tmpFile)
		require.NoError(t, err)

		assert.Contains(t, string(content), testMessage)
	})

	t.Run("stdout_stderr", func(t *testing.T) {
		// Test with stdout
		adapter := slogadapter.NewSlogAdapter(os.Stdout, false)
		log.RegisterLogger(adapter)

		// Should not panic
		log.Info("stdout test message")

		// Test with stderr
		adapter.SetOutput(os.Stderr)
		log.Error("stderr test message")
	})
}

// TestStructuredLoggingIntegration tests structured logging in real scenarios
// Requirements: 7.2
func TestStructuredLoggingIntegration(t *testing.T) {
	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapterWithFormat(&buf, slogadapter.JSONFormat)
	log.RegisterLogger(adapter)

	// Test structured logging with various attribute types
	adapter.InfoWith("user action",
		slogadapter.String("user_id", "12345"),
		slogadapter.String("action", "login"),
		slogadapter.Int("attempt", 1),
		slogadapter.Bool("success", true),
	)

	output := buf.String()
	require.NotEmpty(t, output)

	// Parse JSON to verify structure
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// Verify structured data
	assert.Equal(t, "user action", logEntry["msg"])
	assert.Equal(t, "12345", logEntry["user_id"])
	assert.Equal(t, "login", logEntry["action"])
	assert.Equal(t, float64(1), logEntry["attempt"]) // JSON numbers are float64
	assert.Equal(t, true, logEntry["success"])
}

// TestContextAwareLogging tests context-aware logging functionality
// Requirements: 7.2
func TestContextAwareLogging(t *testing.T) {
	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapterWithFormat(&buf, slogadapter.JSONFormat)
	log.RegisterLogger(adapter)

	// Create context with values
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")

	// Test context-aware logging
	adapter.InfoCtx(ctx, "processing request",
		slogadapter.String("endpoint", "/api/users"),
		slogadapter.Int("status_code", 200),
	)

	output := buf.String()
	require.NotEmpty(t, output)

	// Parse and verify
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "processing request", logEntry["msg"])
	assert.Equal(t, "/api/users", logEntry["endpoint"])
	assert.Equal(t, float64(200), logEntry["status_code"])
}

// TestLogLevelIntegrationReal tests log level filtering in real scenarios
// Requirements: 7.2
func TestLogLevelIntegrationReal(t *testing.T) {
	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapter(&buf, false)
	log.RegisterLogger(adapter)

	// Set to ERROR level
	adapter.SetLogLevel(log.ErrorLevel)

	// Log messages at different levels
	log.Debug("debug message - should not appear")
	log.Info("info message - should not appear")
	log.Warn("warn message - should not appear")
	log.Error("error message - should appear")

	output := buf.String()

	// Only error message should appear
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.NotContains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

// TestConcurrentLoggingIntegration tests concurrent logging in real scenarios
// Requirements: 7.2
func TestConcurrentLoggingIntegration(t *testing.T) {
	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapter(&buf, false)
	log.RegisterLogger(adapter)

	const numGoroutines = 10
	const messagesPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	// Start concurrent loggers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < messagesPerGoroutine; j++ {
				log.Info(fmt.Sprintf("goroutine_%d_message_%d", id, j))
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	output := buf.String()

	// Verify we got messages from all goroutines
	for i := 0; i < numGoroutines; i++ {
		assert.Contains(t, output, fmt.Sprintf("goroutine_%d_message_0", i))
	}

	// Count total messages (approximate - some may be interleaved)
	messageCount := strings.Count(output, "goroutine_")
	assert.Equal(t, numGoroutines*messagesPerGoroutine, messageCount)
}

// TestErrorHandlingIntegration tests error handling in real scenarios
// Requirements: 7.2
func TestErrorHandlingIntegration(t *testing.T) {
	t.Run("failing_writer", func(t *testing.T) {
		// Create a writer that fails
		failingWriter := &failingWriter{shouldFail: true}
		adapter := slogadapter.NewSlogAdapter(failingWriter, false)
		log.RegisterLogger(adapter)

		// Should not panic even with failing writer
		assert.NotPanics(t, func() {
			log.Info("test message")
			log.Error("test error")
		})
	})

	t.Run("nil_writer_recovery", func(t *testing.T) {
		// Start with valid writer
		var buf bytes.Buffer
		adapter := slogadapter.NewSlogAdapter(&buf, false)
		log.RegisterLogger(adapter)

		log.Info("initial message")
		assert.Contains(t, buf.String(), "initial message")

		// Switch to failing writer
		failingWriter := &failingWriter{shouldFail: true}
		adapter.SetOutput(failingWriter)

		// Should not panic
		assert.NotPanics(t, func() {
			log.Error("error with failing writer")
		})

		// Switch back to working writer
		var buf2 bytes.Buffer
		adapter.SetOutput(&buf2)

		log.Info("recovery message")
		assert.Contains(t, buf2.String(), "recovery message")
	})
}

// TestBackwardCompatibilityIntegration tests that existing code patterns work
// Requirements: 7.2
func TestBackwardCompatibilityIntegration(t *testing.T) {
	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapter(&buf, false)
	log.RegisterLogger(adapter)

	// Test all existing global functions
	log.Info("info message")
	log.Infof("info formatted: %s", "test")
	log.Error("error message")
	log.Errorf("error formatted: %d", 123)
	log.Warn("warn message")
	log.Warnf("warn formatted: %v", true)
	log.Debug("debug message")
	log.Debugf("debug formatted: %f", 3.14)
	log.Trace("trace message")
	log.Tracef("trace formatted: %s", "trace")

	output := buf.String()

	// Verify all messages are present
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "info formatted: test")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "error formatted: 123")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "warn formatted: true")

	// Debug and trace may not appear depending on level
	if strings.Contains(output, "debug message") {
		assert.Contains(t, output, "debug formatted: 3.14")
	}
	if strings.Contains(output, "trace message") {
		assert.Contains(t, output, "trace formatted: trace")
	}
}

// TestPerformanceIntegration tests performance in realistic scenarios
// Requirements: 7.2
func TestPerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	var buf bytes.Buffer
	adapter := slogadapter.NewSlogAdapter(&buf, false)
	log.RegisterLogger(adapter)

	const numMessages = 10000

	start := time.Now()

	for i := 0; i < numMessages; i++ {
		log.Info("performance test message", i)
	}

	duration := time.Since(start)

	// Should complete within reasonable time (adjust threshold as needed)
	maxDuration := time.Second * 5
	assert.Less(t, duration, maxDuration,
		"Logging %d messages took %v, expected less than %v",
		numMessages, duration, maxDuration)

	// Verify all messages were logged
	output := buf.String()
	messageCount := strings.Count(output, "performance test message")
	assert.Equal(t, numMessages, messageCount)
}

// Helper types for testing

type failingWriter struct {
	shouldFail bool
	errorCount int
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	if fw.shouldFail {
		fw.errorCount++
		return 0, fmt.Errorf("write error")
	}
	return len(p), nil
}
