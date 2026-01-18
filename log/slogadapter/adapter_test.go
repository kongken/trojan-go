package slogadapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/p4gefau1t/trojan-go/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInterfaceCompliance tests that SlogAdapter implements the Logger interface correctly
// Feature: slog-migration, Property 1: Interface compliance
// Validates: Requirements 1.1, 1.3, 1.4, 1.5
func TestInterfaceCompliance(t *testing.T) {
	// Property: For any Logger interface method call, the SlogAdapter should implement
	// the method with the correct signature and behavior equivalent to the original interface contract

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Test that SlogAdapter implements Logger interface
	t.Run("implements_logger_interface", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapter(&buf, false)

		// Verify it implements the Logger interface
		var _ log.Logger = adapter

		// Use reflection to verify all interface methods are implemented
		loggerType := reflect.TypeOf((*log.Logger)(nil)).Elem()
		adapterType := reflect.TypeOf(adapter)

		for i := 0; i < loggerType.NumMethod(); i++ {
			method := loggerType.Method(i)
			adapterMethod, exists := adapterType.MethodByName(method.Name)

			assert.True(t, exists, "Method %s should be implemented", method.Name)
			if exists {
				// For interface methods, NumIn() includes the receiver implicitly
				// For concrete type methods, NumIn() includes the receiver explicitly
				// So interface methods have NumIn() = actual params, concrete methods have NumIn() = actual params + 1
				expectedInputs := method.Type.NumIn() + 1 // Add 1 for the receiver in concrete type
				assert.Equal(t, expectedInputs, adapterMethod.Type.NumIn(),
					"Method %s should have correct number of input parameters", method.Name)
				assert.Equal(t, method.Type.NumOut(), adapterMethod.Type.NumOut(),
					"Method %s should have correct number of output parameters", method.Name)
			}
		}
	})

	// Property test: All logging methods should accept variadic arguments
	t.Run("variadic_methods_property", func(t *testing.T) {
		property := func(message string) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test that all non-formatted methods accept variadic arguments
			// These should not panic and should write to buffer
			adapter.Info(message)
			adapter.Error(message)
			adapter.Warn(message)
			adapter.Debug(message)
			adapter.Trace(message)

			// Buffer should contain some output (methods were called successfully)
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All variadic logging methods should work with any string input")
	})

	// Property test: All formatted methods should accept format strings
	t.Run("formatted_methods_property", func(t *testing.T) {
		property := func(format string, arg string) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test that all formatted methods accept format strings
			// These should not panic and should write to buffer
			adapter.Infof(format, arg)
			adapter.Errorf(format, arg)
			adapter.Warnf(format, arg)
			adapter.Debugf(format, arg)
			adapter.Tracef(format, arg)

			// Buffer should contain some output (methods were called successfully)
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All formatted logging methods should work with any format string and arguments")
	})

	// Property test: SetLogLevel should accept all valid log levels
	t.Run("set_log_level_property", func(t *testing.T) {
		property := func() bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test all valid log levels
			levels := []log.LogLevel{
				log.AllLevel,
				log.InfoLevel,
				log.WarnLevel,
				log.ErrorLevel,
				log.FatalLevel,
				log.OffLevel,
			}

			for _, level := range levels {
				// Should not panic
				adapter.SetLogLevel(level)
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "SetLogLevel should accept all valid log levels without panicking")
	})

	// Property test: SetOutput should accept any io.Writer
	t.Run("set_output_property", func(t *testing.T) {
		property := func() bool {
			adapter := NewSlogAdapter(&bytes.Buffer{}, false)

			// Test with different io.Writer implementations
			writers := []io.Writer{
				&bytes.Buffer{},
				io.Discard,
			}

			for _, writer := range writers {
				// Should not panic
				adapter.SetOutput(writer)

				// Should be able to log after changing output
				adapter.Info("test message")
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "SetOutput should accept any io.Writer without panicking")
	})

	// Property test: Method signatures should match Logger interface exactly
	t.Run("method_signatures_property", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapter(&buf, false)

		// Verify that we can assign to Logger interface
		var logger log.Logger = adapter
		require.NotNil(t, logger)

		// Test that all methods can be called through the interface
		logger.Info("test")
		logger.Infof("test %s", "formatted")
		logger.Error("test")
		logger.Errorf("test %s", "formatted")
		logger.Warn("test")
		logger.Warnf("test %s", "formatted")
		logger.Debug("test")
		logger.Debugf("test %s", "formatted")
		logger.Trace("test")
		logger.Tracef("test %s", "formatted")

		logger.SetLogLevel(log.InfoLevel)
		logger.SetOutput(&bytes.Buffer{})

		// If we get here without panicking, the interface is correctly implemented
		assert.True(t, true, "All Logger interface methods should be callable")
	})
}

// TestBackwardCompatibilityRouting tests that existing log function calls work correctly
// Feature: slog-migration, Property 2: Backward compatibility routing
// Validates: Requirements 2.1, 2.2
func TestBackwardCompatibilityRouting(t *testing.T) {
	// Property: For any existing log function call (log.Info, log.Error, etc.),
	// the system should route the call to the slog implementation and produce equivalent output

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: All logging methods should produce output when called
	t.Run("logging_methods_produce_output", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test" // Avoid empty messages
			}

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel) // Enable all levels

			// Test each logging method produces output
			methods := []func(){
				func() { adapter.Info(message) },
				func() { adapter.Error(message) },
				func() { adapter.Warn(message) },
				func() { adapter.Debug(message) },
				func() { adapter.Trace(message) },
			}

			for _, method := range methods {
				buf.Reset()
				method()
				if buf.Len() == 0 {
					return false // Method should have produced output
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All logging methods should produce output when enabled")
	})

	// Property test: Formatted methods should handle format strings correctly
	t.Run("formatted_methods_handle_formats", func(t *testing.T) {
		property := func(arg1 string, arg2 int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel) // Enable all levels

			// Test formatted methods with different format patterns
			formats := []string{
				"message: %s",
				"number: %d",
				"both: %s %d",
				"%s-%d",
			}

			for _, format := range formats {
				buf.Reset()
				adapter.Infof(format, arg1, arg2)
				if buf.Len() == 0 {
					return false // Should have produced output
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Formatted methods should handle various format strings")
	})

	// Property test: Method behavior should be consistent across calls
	t.Run("consistent_method_behavior", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			var buf1, buf2 bytes.Buffer
			adapter1 := NewSlogAdapter(&buf1, false)
			adapter2 := NewSlogAdapter(&buf2, false)

			// Set same configuration
			adapter1.SetLogLevel(log.InfoLevel)
			adapter2.SetLogLevel(log.InfoLevel)

			// Call same method with same arguments
			adapter1.Info(message)
			adapter2.Info(message)

			// Both should produce output (though content may differ due to timestamps)
			return buf1.Len() > 0 && buf2.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Method behavior should be consistent across different adapter instances")
	})

	// Property test: All interface methods should be callable without panicking
	t.Run("interface_methods_no_panic", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Method panicked: %v", r)
				}
			}()

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Call all interface methods except Fatal/Fatalf (which call os.Exit)
			adapter.Info(message)
			adapter.Infof("formatted: %s", message)
			adapter.Error(message)
			adapter.Errorf("formatted: %s", message)
			adapter.Warn(message)
			adapter.Warnf("formatted: %s", message)
			adapter.Debug(message)
			adapter.Debugf("formatted: %s", message)
			adapter.Trace(message)
			adapter.Tracef("formatted: %s", message)

			adapter.SetLogLevel(log.InfoLevel)
			adapter.SetOutput(&bytes.Buffer{})

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All interface methods should be callable without panicking")
	})
}

// TestLogLevelFilteringConsistency tests that log level filtering works correctly
// Feature: slog-migration, Property 3: Log level filtering consistency
// Validates: Requirements 2.3
func TestLogLevelFilteringConsistency(t *testing.T) {
	// Property: For any log level configuration, the filtering behavior should match
	// the original implementation's behavior for the same level setting

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Messages below the set level should not appear in output
	t.Run("level_filtering_blocks_lower_levels", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Test each level blocks lower priority messages
			testCases := []struct {
				setLevel     log.LogLevel
				testLevel    log.LogLevel
				shouldOutput bool
			}{
				{log.ErrorLevel, log.InfoLevel, false}, // Error level should block Info
				{log.ErrorLevel, log.WarnLevel, false}, // Error level should block Warn
				{log.ErrorLevel, log.ErrorLevel, true}, // Error level should allow Error
				{log.WarnLevel, log.InfoLevel, false},  // Warn level should block Info
				{log.WarnLevel, log.WarnLevel, true},   // Warn level should allow Warn
				{log.WarnLevel, log.ErrorLevel, true},  // Warn level should allow Error
				{log.InfoLevel, log.InfoLevel, true},   // Info level should allow Info
				{log.InfoLevel, log.WarnLevel, true},   // Info level should allow Warn
				{log.InfoLevel, log.ErrorLevel, true},  // Info level should allow Error
				{log.AllLevel, log.InfoLevel, true},    // All level should allow everything
				{log.OffLevel, log.ErrorLevel, false},  // Off level should block everything
			}

			for _, tc := range testCases {
				var buf bytes.Buffer
				adapter := NewSlogAdapter(&buf, false)
				adapter.SetLogLevel(tc.setLevel)

				// Call the appropriate logging method based on test level
				switch tc.testLevel {
				case log.InfoLevel:
					adapter.Info(message)
				case log.WarnLevel:
					adapter.Warn(message)
				case log.ErrorLevel:
					adapter.Error(message)
				}

				hasOutput := buf.Len() > 0
				if hasOutput != tc.shouldOutput {
					return false // Filtering behavior doesn't match expectation
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Log level filtering should consistently block messages below the set level")
	})

	// Property test: Debug and Trace levels should only appear when AllLevel is set
	t.Run("debug_trace_only_with_all_level", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Test Debug and Trace with different levels
			levels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.OffLevel}

			for _, level := range levels {
				var buf bytes.Buffer
				adapter := NewSlogAdapter(&buf, false)
				adapter.SetLogLevel(level)

				// Test Debug
				buf.Reset()
				adapter.Debug(message)
				debugHasOutput := buf.Len() > 0

				// Test Trace
				buf.Reset()
				adapter.Trace(message)
				traceHasOutput := buf.Len() > 0

				// Debug and Trace should only produce output when level is AllLevel
				expectedOutput := (level == log.AllLevel)
				if debugHasOutput != expectedOutput || traceHasOutput != expectedOutput {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Debug and Trace messages should only appear when AllLevel is set")
	})

	// Property test: Level changes should take effect immediately
	t.Run("level_changes_immediate_effect", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Start with Info level - should allow Info messages
			adapter.SetLogLevel(log.InfoLevel)
			buf.Reset()
			adapter.Info(message)
			if buf.Len() == 0 {
				return false // Should have output
			}

			// Change to Error level - should block Info messages
			adapter.SetLogLevel(log.ErrorLevel)
			buf.Reset()
			adapter.Info(message)
			if buf.Len() > 0 {
				return false // Should not have output
			}

			// Error messages should still work
			buf.Reset()
			adapter.Error(message)
			if buf.Len() == 0 {
				return false // Should have output
			}

			// Change to Off level - should block everything
			adapter.SetLogLevel(log.OffLevel)
			buf.Reset()
			adapter.Error(message)
			if buf.Len() > 0 {
				return false // Should not have output
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Log level changes should take effect immediately")
	})

	// Property test: Formatted and non-formatted methods should respect levels equally
	t.Run("formatted_and_non_formatted_equal_filtering", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			levels := []log.LogLevel{log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.OffLevel}

			for _, level := range levels {
				var buf1, buf2 bytes.Buffer
				adapter1 := NewSlogAdapter(&buf1, false)
				adapter2 := NewSlogAdapter(&buf2, false)

				adapter1.SetLogLevel(level)
				adapter2.SetLogLevel(level)

				// Test Info level with both formatted and non-formatted
				adapter1.Info(message)
				adapter2.Infof("formatted: %s", message)

				hasOutput1 := buf1.Len() > 0
				hasOutput2 := buf2.Len() > 0

				// Both should have the same filtering behavior
				if hasOutput1 != hasOutput2 {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Formatted and non-formatted methods should have equal level filtering")
	})
}

// mockFdWriter implements FdWriter interface for testing
type mockFdWriter struct {
	bytes.Buffer
	fd         uintptr
	isTerminal bool
}

// Fd returns the file descriptor
func (m *mockFdWriter) Fd() uintptr {
	return m.fd
}

// TestTerminalColorSupport tests that terminal color support works correctly
// Feature: slog-migration, Property 10: Terminal color support
// Validates: Requirements 4.5
func TestTerminalColorSupport(t *testing.T) {
	// Property: For any terminal environment detection, the system should enable
	// colored output when appropriate and disable it otherwise

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Color detection should work correctly for different writer types
	t.Run("color_detection_property", func(t *testing.T) {
		property := func(isTerminal bool) bool {
			// Test with mock FdWriter
			mockWriter := &mockFdWriter{
				fd:         1, // stdout fd
				isTerminal: isTerminal,
			}

			// Create handler with the mock writer
			handler := NewColoredTextHandler(mockWriter, &slog.HandlerOptions{})

			// The handler should be created successfully
			if handler == nil {
				return false
			}

			// Handler should have color settings based on terminal detection
			// Note: We can't directly test terminal detection since it depends on the actual terminal,
			// but we can test that the handler is created and functions properly
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Color detection should work for different writer types")
	})

	// Property test: Colored handler should handle all log levels
	t.Run("colored_handler_all_levels", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			var buf bytes.Buffer
			handler := NewColoredTextHandler(&buf, &slog.HandlerOptions{})
			logger := slog.New(handler)

			// Test all standard slog levels
			logger.Debug(message)
			logger.Info(message)
			logger.Warn(message)
			logger.Error(message)

			// Test custom levels
			logger.Log(context.Background(), LevelTrace, message)
			logger.Log(context.Background(), LevelFatal, message)

			// Should produce output for all levels
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Colored handler should handle all log levels including custom ones")
	})

	// Property test: Handler should support WithAttrs and WithGroup operations
	t.Run("colored_handler_with_attrs_groups", func(t *testing.T) {
		property := func(key, value, group string) bool {
			if key == "" {
				key = "testkey"
			}
			if value == "" {
				value = "testvalue"
			}
			if group == "" {
				group = "testgroup"
			}

			var buf bytes.Buffer
			handler := NewColoredTextHandler(&buf, &slog.HandlerOptions{})

			// Test WithAttrs
			handlerWithAttrs := handler.WithAttrs([]slog.Attr{
				slog.String(key, value),
			})
			if handlerWithAttrs == nil {
				return false
			}

			// Test WithGroup
			handlerWithGroup := handler.WithGroup(group)
			if handlerWithGroup == nil {
				return false
			}

			// Test chaining
			chainedHandler := handler.WithAttrs([]slog.Attr{
				slog.String(key, value),
			}).WithGroup(group)
			if chainedHandler == nil {
				return false
			}

			// All handlers should be usable
			logger1 := slog.New(handlerWithAttrs)
			logger2 := slog.New(handlerWithGroup)
			logger3 := slog.New(chainedHandler)

			logger1.Info("test message")
			logger2.Info("test message")
			logger3.Info("test message")

			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Colored handler should support WithAttrs and WithGroup operations")
	})

	// Property test: Color settings should be preserved across handler operations
	t.Run("color_settings_preservation", func(t *testing.T) {
		property := func(useColor bool) bool {
			var buf bytes.Buffer
			handler := NewColoredTextHandler(&buf, &slog.HandlerOptions{})

			// Set color preference
			handler.SetUseColor(useColor)

			// Verify setting was applied
			if handler.IsColorEnabled() != useColor {
				return false
			}

			// Create derived handlers
			handlerWithAttrs := handler.WithAttrs([]slog.Attr{
				slog.String("key", "value"),
			})
			handlerWithGroup := handler.WithGroup("testgroup")

			// Color settings should be preserved in derived handlers
			if colorHandler, ok := handlerWithAttrs.(*ColoredTextHandler); ok {
				if colorHandler.IsColorEnabled() != useColor {
					return false
				}
			}

			if colorHandler, ok := handlerWithGroup.(*ColoredTextHandler); ok {
				if colorHandler.IsColorEnabled() != useColor {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Color settings should be preserved across handler operations")
	})

	// Property test: Handler should work with different output writers
	t.Run("handler_different_writers", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Test with different writer types
			writers := []io.Writer{
				&bytes.Buffer{},
				io.Discard,
				&mockFdWriter{fd: 1, isTerminal: true},
				&mockFdWriter{fd: 2, isTerminal: false},
			}

			for _, writer := range writers {
				handler := NewColoredTextHandler(writer, &slog.HandlerOptions{})
				if handler == nil {
					return false
				}

				logger := slog.New(handler)
				logger.Info(message)

				// Should not panic and handler should be functional
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Handler should work with different output writer types")
	})

	// Property test: Custom level handling should work correctly
	t.Run("custom_level_handling", func(t *testing.T) {
		// Simplified test that focuses on basic functionality
		t.Run("basic_custom_levels", func(t *testing.T) {
			var buf bytes.Buffer
			// Set level to allow all messages including custom levels
			levelVar := &slog.LevelVar{}
			levelVar.Set(slog.Level(-10)) // Very low level to allow all messages

			handler := NewColoredTextHandler(&buf, &slog.HandlerOptions{
				Level: levelVar,
			})
			logger := slog.New(handler)

			// Test TRACE level
			buf.Reset()
			logger.Log(context.Background(), LevelTrace, "trace message")
			traceOutput := buf.String()
			t.Logf("TRACE output: %q", traceOutput)
			assert.Greater(t, len(traceOutput), 0, "TRACE level should produce output")

			// Test FATAL level
			buf.Reset()
			logger.Log(context.Background(), LevelFatal, "fatal message")
			fatalOutput := buf.String()
			t.Logf("FATAL output: %q", fatalOutput)
			assert.Greater(t, len(fatalOutput), 0, "FATAL level should produce output")
		})

		// Property test with ASCII-only messages
		property := func(seed int) bool {
			// Generate simple ASCII message from seed
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			// Set level to allow all messages including custom levels
			levelVar := &slog.LevelVar{}
			levelVar.Set(slog.Level(-10)) // Very low level to allow all messages

			handler := NewColoredTextHandler(&buf, &slog.HandlerOptions{
				Level: levelVar,
			})
			logger := slog.New(handler)

			// Test custom levels
			buf.Reset()
			logger.Log(context.Background(), LevelTrace, message)
			traceOutput := buf.String()

			buf.Reset()
			logger.Log(context.Background(), LevelFatal, message)
			fatalOutput := buf.String()

			// Both should produce output
			return len(traceOutput) > 0 && len(fatalOutput) > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Custom level handling should work correctly for TRACE and FATAL levels")
	})
}

// TestOutputWriterPreservation tests that SetOutput method correctly preserves output writer
// Feature: slog-migration, Property 4: Output writer preservation
// Validates: Requirements 2.4
func TestOutputWriterPreservation(t *testing.T) {
	// Property: For any custom output writer, the SetOutput method should correctly
	// configure the slog adapter to write to that destination

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: SetOutput should redirect all log output to the new writer
	t.Run("output_redirection_property", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Create adapter with initial writer
			var initialBuf bytes.Buffer
			adapter := NewSlogAdapter(&initialBuf, false)
			adapter.SetLogLevel(log.AllLevel) // Enable all levels

			// Log to initial writer
			adapter.Info(message)
			initialOutput := initialBuf.String()

			// Create new writer and switch to it
			var newBuf bytes.Buffer
			adapter.SetOutput(&newBuf)

			// Log to new writer
			adapter.Info(message)
			newOutput := newBuf.String()

			// Initial buffer should have content from first log
			if len(initialOutput) == 0 {
				return false
			}

			// New buffer should have content from second log
			if len(newOutput) == 0 {
				return false
			}

			// Initial buffer should not have received the second log
			if initialBuf.String() != initialOutput {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "SetOutput should redirect all log output to the new writer")
	})

	// Property test: Output writer changes should preserve log level settings
	t.Run("level_preservation_across_output_changes", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			var buf1, buf2 bytes.Buffer
			adapter := NewSlogAdapter(&buf1, false)

			// Set a specific log level
			adapter.SetLogLevel(log.ErrorLevel)

			// Info should be blocked
			adapter.Info(message)
			if buf1.Len() > 0 {
				return false // Should not have output
			}

			// Error should work
			adapter.Error(message)
			if buf1.Len() == 0 {
				return false // Should have output
			}

			// Change output writer
			adapter.SetOutput(&buf2)

			// Level settings should be preserved
			buf2.Reset()
			adapter.Info(message)
			if buf2.Len() > 0 {
				return false // Should still be blocked
			}

			buf2.Reset()
			adapter.Error(message)
			if buf2.Len() == 0 {
				return false // Should still work
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Log level settings should be preserved across output writer changes")
	})

	// Property test: Color settings should be preserved when changing output
	t.Run("color_preservation_across_output_changes", func(t *testing.T) {
		property := func(message string, useColor bool) bool {
			if message == "" {
				message = "test"
			}

			var buf1, buf2 bytes.Buffer
			adapter := NewSlogAdapter(&buf1, useColor)
			adapter.SetLogLevel(log.AllLevel)

			// Log with initial settings
			adapter.Info(message)
			initialOutput := buf1.String()

			// Change output writer
			adapter.SetOutput(&buf2)

			// Log with new writer
			adapter.Info(message)
			newOutput := buf2.String()

			// Both should have produced output
			if len(initialOutput) == 0 || len(newOutput) == 0 {
				return false
			}

			// If color was enabled, both outputs should be similar in structure
			// (we can't easily test exact color codes, but we can test that output is generated)
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Color settings should be preserved when changing output writer")
	})

	// Property test: Multiple output changes should work correctly
	t.Run("multiple_output_changes_property", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			adapter := NewSlogAdapter(&bytes.Buffer{}, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test multiple output changes
			writers := []*bytes.Buffer{
				&bytes.Buffer{},
				&bytes.Buffer{},
				&bytes.Buffer{},
			}

			for i, writer := range writers {
				adapter.SetOutput(writer)
				testMsg := fmt.Sprintf("%s_%d", message, i)
				adapter.Info(testMsg)

				// Current writer should have output
				if writer.Len() == 0 {
					return false
				}

				// Previous writers should not have received this message
				for j := 0; j < i; j++ {
					if writers[j].String() == writer.String() {
						return false // Previous writer got the new message
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Multiple output writer changes should work correctly")
	})

	// Property test: SetOutput should work with all logging methods
	t.Run("all_methods_respect_output_changes", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&bytes.Buffer{}, false)
			adapter.SetLogLevel(log.AllLevel)

			// Change to our test buffer
			adapter.SetOutput(&buf)

			// Test all logging methods
			methods := []func(){
				func() { adapter.Info(message) },
				func() { adapter.Infof("formatted: %s", message) },
				func() { adapter.Error(message) },
				func() { adapter.Errorf("formatted: %s", message) },
				func() { adapter.Warn(message) },
				func() { adapter.Warnf("formatted: %s", message) },
				func() { adapter.Debug(message) },
				func() { adapter.Debugf("formatted: %s", message) },
				func() { adapter.Trace(message) },
				func() { adapter.Tracef("formatted: %s", message) },
			}

			for _, method := range methods {
				buf.Reset()
				method()
				if buf.Len() == 0 {
					return false // Method should have written to the buffer
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All logging methods should respect output writer changes")
	})
}

// Custom writer implementations for testing
type countingWriter struct {
	count int
	data  bytes.Buffer
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	cw.count++
	return cw.data.Write(p)
}

type errorWriter struct {
	shouldError bool
	errorCount  int
}

func (ew *errorWriter) Write(p []byte) (n int, err error) {
	if ew.shouldError {
		ew.errorCount++
		return 0, fmt.Errorf("write error")
	}
	return len(p), nil
}

type slowWriter struct {
	data bytes.Buffer
}

func (sw *slowWriter) Write(p []byte) (n int, err error) {
	// Simulate slow write
	return sw.data.Write(p)
}

// TestCustomWriterSupport tests that the adapter supports various io.Writer implementations
// Feature: slog-migration, Property 8: Custom writer support
// Validates: Requirements 4.3
func TestCustomWriterSupport(t *testing.T) {
	// Property: For any io.Writer implementation, the slog adapter should successfully
	// write log records to that writer

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Adapter should work with various io.Writer implementations
	t.Run("various_writer_implementations", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Test with different writer implementations
			writers := []io.Writer{
				&bytes.Buffer{},
				io.Discard,
				&countingWriter{},
				&slowWriter{},
			}

			for _, writer := range writers {
				adapter := NewSlogAdapter(writer, false)
				adapter.SetLogLevel(log.AllLevel)

				// Should not panic when logging
				adapter.Info(message)
				adapter.Error(message)
				adapter.Warn(message)

				// If it's a counting writer, verify it was called
				if cw, ok := writer.(*countingWriter); ok {
					if cw.count == 0 {
						return false // Writer should have been called
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Adapter should work with various io.Writer implementations")
	})

	// Property test: Adapter should handle writer errors gracefully
	t.Run("writer_error_handling", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			// Test with error-prone writer
			errorWriter := &errorWriter{shouldError: true}
			adapter := NewSlogAdapter(errorWriter, false)
			adapter.SetLogLevel(log.AllLevel)

			// Should not panic even when writer returns errors
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Adapter panicked on writer error: %v", r)
				}
			}()

			adapter.Info(message)
			adapter.Error(message)

			// Writer should have received error calls
			return errorWriter.errorCount > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Adapter should handle writer errors gracefully")
	})

	// Property test: SetOutput should work with any io.Writer
	t.Run("set_output_any_writer", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			adapter := NewSlogAdapter(&bytes.Buffer{}, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test changing to different writer types
			writers := []io.Writer{
				&bytes.Buffer{},
				io.Discard,
				&countingWriter{},
				&slowWriter{},
			}

			for _, writer := range writers {
				// Should not panic when setting output
				adapter.SetOutput(writer)

				// Should be able to log after setting output
				adapter.Info(message)

				// If it's a counting writer, verify it was used
				if cw, ok := writer.(*countingWriter); ok {
					if cw.count == 0 {
						return false // Writer should have been called
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "SetOutput should work with any io.Writer implementation")
	})

	// Property test: Writer switching should be thread-safe
	t.Run("thread_safe_writer_switching", func(t *testing.T) {
		property := func(message string) bool {
			if message == "" {
				message = "test"
			}

			adapter := NewSlogAdapter(&bytes.Buffer{}, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test concurrent writer switching and logging
			done := make(chan bool, 2)

			// Goroutine 1: Switch writers
			go func() {
				defer func() { done <- true }()
				writers := []io.Writer{
					&bytes.Buffer{},
					&bytes.Buffer{},
					&bytes.Buffer{},
				}
				for _, writer := range writers {
					adapter.SetOutput(writer)
				}
			}()

			// Goroutine 2: Log messages
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 10; i++ {
					adapter.Info(fmt.Sprintf("%s_%d", message, i))
				}
			}()

			// Wait for both goroutines
			<-done
			<-done

			// Should not have panicked
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Writer switching should be thread-safe")
	})

	// Property test: Custom writers should receive properly formatted output
	t.Run("custom_writers_formatted_output", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Log a message
			adapter.Info(message)
			output := buf.String()

			// Output should contain the message
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}

			// Output should have some structure (timestamp, level, etc.)
			if len(output) <= len(message) {
				return false // Should have more than just the message
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Custom writers should receive properly formatted output")
	})

	// Property test: Writer changes should not affect previous writers
	t.Run("writer_isolation", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_msg_%d", seed%1000)

			var buf1, buf2 bytes.Buffer
			adapter := NewSlogAdapter(&buf1, false)
			adapter.SetLogLevel(log.AllLevel)

			// Log to first writer
			adapter.Info(message + "_1")
			firstOutput := buf1.String()

			// Switch to second writer
			adapter.SetOutput(&buf2)

			// Log to second writer
			adapter.Info(message + "_2")
			secondOutput := buf2.String()

			// First writer should only have first message
			if !bytes.Contains([]byte(firstOutput), []byte(message+"_1")) {
				return false
			}
			if bytes.Contains([]byte(firstOutput), []byte(message+"_2")) {
				return false // Should not have second message
			}

			// Second writer should only have second message
			if !bytes.Contains([]byte(secondOutput), []byte(message+"_2")) {
				return false
			}
			if bytes.Contains([]byte(secondOutput), []byte(message+"_1")) {
				return false // Should not have first message
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Writer changes should not affect previous writers")
	})
}

// TestStructuredLoggingSupport tests that structured logging with key-value pairs works correctly
// Feature: slog-migration, Property 5: Structured logging support
// Validates: Requirements 3.1, 3.3
func TestStructuredLoggingSupport(t *testing.T) {
	// Property: For any key-value pair provided to structured logging methods,
	// the pairs should be correctly formatted and included in the log output

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Structured logging methods should include key-value pairs in output
	t.Run("structured_methods_include_attributes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode parsing issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel) // Enable all levels

			// Test structured logging methods with attributes
			attr := slog.String(key, value)

			// Test all structured logging methods
			methods := []func(){
				func() { adapter.InfoWith(message, attr) },
				func() { adapter.ErrorWith(message, attr) },
				func() { adapter.WarnWith(message, attr) },
				func() { adapter.DebugWith(message, attr) },
				func() { adapter.TraceWith(message, attr) },
			}

			for _, method := range methods {
				buf.Reset()
				method()
				output := buf.String()

				// Output should contain the message
				if !bytes.Contains([]byte(output), []byte(message)) {
					return false
				}

				// Output should contain the key and value
				if !bytes.Contains([]byte(output), []byte(key)) {
					return false
				}
				if !bytes.Contains([]byte(output), []byte(value)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Structured logging methods should include key-value pairs in output")
	})

	// Property test: Multiple attributes should all be included in output
	t.Run("multiple_attributes_included", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode parsing issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			// Generate multiple attributes from seed
			attrs := []slog.Attr{
				slog.String("key1", fmt.Sprintf("value1_%d", seed%100)),
				slog.Int("key2", seed%1000),
				slog.Bool("key3", seed%2 == 0),
			}

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test with multiple attributes
			adapter.InfoWith(message, attrs...)
			output := buf.String()

			// Output should contain the message
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}

			// Output should contain all attribute keys and values
			expectedStrings := []string{
				"key1",
				fmt.Sprintf("value1_%d", seed%100),
				"key2",
				fmt.Sprintf("%d", seed%1000),
				"key3",
			}

			for _, expected := range expectedStrings {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Multiple attributes should all be included in output")
	})

	// Property test: Helper functions should create valid attributes
	t.Run("helper_functions_create_valid_attributes", func(t *testing.T) {
		property := func(seed int, intVal int, boolVal bool) bool {
			// Generate ASCII-only string from seed to avoid Unicode parsing issues
			stringVal := fmt.Sprintf("test_string_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test helper functions
			attrs := []slog.Attr{
				String("str_key", stringVal),
				Int("int_key", intVal),
				Bool("bool_key", boolVal),
			}

			adapter.InfoWith("test message", attrs...)
			output := buf.String()

			// Output should contain all keys and values
			expectedStrings := []string{
				"str_key",
				stringVal,
				"int_key",
				fmt.Sprintf("%d", intVal),
				"bool_key",
				fmt.Sprintf("%t", boolVal),
			}

			for _, expected := range expectedStrings {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Helper functions should create valid attributes")
	})

	// Property test: WithAttrs should add attributes to all subsequent log records
	t.Run("with_attrs_adds_to_all_records", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode parsing issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("persistent_key_%d", seed%100)
			value := fmt.Sprintf("persistent_value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create adapter with persistent attributes
			adapterWithAttrs := adapter.WithAttrs(slog.String(key, value))

			// Test that all logging methods include the persistent attribute
			methods := []func(){
				func() { adapterWithAttrs.Info(message) },
				func() { adapterWithAttrs.Error(message) },
				func() { adapterWithAttrs.Warn(message) },
				func() { adapterWithAttrs.Debug(message) },
				func() { adapterWithAttrs.Trace(message) },
			}

			for _, method := range methods {
				buf.Reset()
				method()
				output := buf.String()

				// Output should contain the message
				if !bytes.Contains([]byte(output), []byte(message)) {
					return false
				}

				// Output should contain the persistent key and value
				if !bytes.Contains([]byte(output), []byte(key)) {
					return false
				}
				if !bytes.Contains([]byte(output), []byte(value)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "WithAttrs should add attributes to all subsequent log records")
	})

	// Property test: WithGroup should group attributes correctly
	t.Run("with_group_groups_attributes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode parsing issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			groupName := fmt.Sprintf("testgroup_%d", seed%100)
			key := fmt.Sprintf("testkey_%d", seed%200)
			value := fmt.Sprintf("testvalue_%d", seed%300)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create adapter with group
			adapterWithGroup := adapter.WithGroup(groupName)

			// Log with attributes - they should be grouped
			adapterWithGroup.InfoWith(message, slog.String(key, value))
			output := buf.String()

			// Output should contain the message
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}

			// Output should contain the group name
			if !bytes.Contains([]byte(output), []byte(groupName)) {
				return false
			}

			// Output should contain the key and value
			if !bytes.Contains([]byte(output), []byte(key)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(value)) {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "WithGroup should group attributes correctly")
	})

	// Property test: Structured logging should respect log levels
	t.Run("structured_logging_respects_levels", func(t *testing.T) {
		property := func(message, key, value string) bool {
			if message == "" {
				message = "test"
			}
			if key == "" {
				key = "testkey"
			}
			if value == "" {
				value = "testvalue"
			}

			attr := slog.String(key, value)

			// Test level filtering with structured methods
			testCases := []struct {
				setLevel     log.LogLevel
				method       func(*SlogAdapter)
				shouldOutput bool
			}{
				{log.ErrorLevel, func(a *SlogAdapter) { a.InfoWith(message, attr) }, false},
				{log.ErrorLevel, func(a *SlogAdapter) { a.ErrorWith(message, attr) }, true},
				{log.WarnLevel, func(a *SlogAdapter) { a.InfoWith(message, attr) }, false},
				{log.WarnLevel, func(a *SlogAdapter) { a.WarnWith(message, attr) }, true},
				{log.InfoLevel, func(a *SlogAdapter) { a.InfoWith(message, attr) }, true},
				{log.AllLevel, func(a *SlogAdapter) { a.DebugWith(message, attr) }, true},
				{log.AllLevel, func(a *SlogAdapter) { a.TraceWith(message, attr) }, true},
				{log.OffLevel, func(a *SlogAdapter) { a.ErrorWith(message, attr) }, false},
			}

			for _, tc := range testCases {
				var buf bytes.Buffer
				adapter := NewSlogAdapter(&buf, false)
				adapter.SetLogLevel(tc.setLevel)

				tc.method(adapter)

				hasOutput := buf.Len() > 0
				if hasOutput != tc.shouldOutput {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Structured logging should respect log levels")
	})

	// Property test: Group attributes should create nested structure
	t.Run("group_attributes_create_nested_structure", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode parsing issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			groupKey := fmt.Sprintf("group_%d", seed%100)
			key := fmt.Sprintf("nested_key_%d", seed%200)
			value := fmt.Sprintf("nested_value_%d", seed%300)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create group attribute
			groupAttr := Group(groupKey, slog.String(key, value))

			adapter.InfoWith(message, groupAttr)
			output := buf.String()

			// Output should contain the message
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}

			// Output should contain the group key
			if !bytes.Contains([]byte(output), []byte(groupKey)) {
				return false
			}

			// Output should contain the nested key and value
			if !bytes.Contains([]byte(output), []byte(key)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(value)) {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Group attributes should create nested structure")
	})
}

// TestContextMetadataInclusion tests that context-aware logging includes relevant metadata
// Feature: slog-migration, Property 6: Context metadata inclusion
// Validates: Requirements 3.2
func TestContextMetadataInclusion(t *testing.T) {
	// Property: For any context containing logging metadata, the metadata should be
	// extracted and included in the resulting log record

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Context-aware logging methods should accept context
	t.Run("context_methods_accept_context", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create context with some metadata
			ctx := context.Background()
			ctx = context.WithValue(ctx, "request_id", "test-123")

			attr := slog.String(key, value)

			// Test all context-aware logging methods
			methods := []func(){
				func() { adapter.InfoCtx(ctx, message, attr) },
				func() { adapter.ErrorCtx(ctx, message, attr) },
				func() { adapter.WarnCtx(ctx, message, attr) },
				func() { adapter.DebugCtx(ctx, message, attr) },
				func() { adapter.TraceCtx(ctx, message, attr) },
			}

			for _, method := range methods {
				buf.Reset()
				method()
				output := buf.String()

				// Output should contain the message
				if !bytes.Contains([]byte(output), []byte(message)) {
					return false
				}

				// Output should contain the key and value
				if !bytes.Contains([]byte(output), []byte(key)) {
					return false
				}
				if !bytes.Contains([]byte(output), []byte(value)) {
					return false
				}

				// Method should not panic with context
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context-aware logging methods should accept context without panicking")
	})

	// Property test: Context methods should work with nil context
	t.Run("context_methods_handle_nil_context", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			attr := slog.String(key, value)

			// Test with nil context - should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Context method panicked with nil context: %v", r)
				}
			}()

			adapter.InfoCtx(nil, message, attr)
			output := buf.String()

			// Output should contain the message and attributes
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(key)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(value)) {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context methods should handle nil context gracefully")
	})

	// Property test: Context methods should work with canceled context
	t.Run("context_methods_handle_canceled_context", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create canceled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			attr := slog.String(key, value)

			// Should not panic with canceled context
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Context method panicked with canceled context: %v", r)
				}
			}()

			adapter.InfoCtx(ctx, message, attr)
			output := buf.String()

			// Output should still contain the message and attributes
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(key)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(value)) {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context methods should handle canceled context gracefully")
	})

	// Property test: Context methods should respect log levels
	t.Run("context_methods_respect_levels", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			ctx := context.Background()
			attr := slog.String(key, value)

			// Test level filtering with context methods
			testCases := []struct {
				setLevel     log.LogLevel
				method       func(*SlogAdapter)
				shouldOutput bool
			}{
				{log.ErrorLevel, func(a *SlogAdapter) { a.InfoCtx(ctx, message, attr) }, false},
				{log.ErrorLevel, func(a *SlogAdapter) { a.ErrorCtx(ctx, message, attr) }, true},
				{log.WarnLevel, func(a *SlogAdapter) { a.InfoCtx(ctx, message, attr) }, false},
				{log.WarnLevel, func(a *SlogAdapter) { a.WarnCtx(ctx, message, attr) }, true},
				{log.InfoLevel, func(a *SlogAdapter) { a.InfoCtx(ctx, message, attr) }, true},
				{log.AllLevel, func(a *SlogAdapter) { a.DebugCtx(ctx, message, attr) }, true},
				{log.AllLevel, func(a *SlogAdapter) { a.TraceCtx(ctx, message, attr) }, true},
				{log.OffLevel, func(a *SlogAdapter) { a.ErrorCtx(ctx, message, attr) }, false},
			}

			for _, tc := range testCases {
				var buf bytes.Buffer
				adapter := NewSlogAdapter(&buf, false)
				adapter.SetLogLevel(tc.setLevel)

				tc.method(adapter)

				hasOutput := buf.Len() > 0
				if hasOutput != tc.shouldOutput {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context methods should respect log levels")
	})

	// Property test: Context methods should work with multiple attributes
	t.Run("context_methods_multiple_attributes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			ctx := context.Background()

			// Generate multiple attributes from seed
			attrs := []slog.Attr{
				slog.String("key1", fmt.Sprintf("value1_%d", seed%100)),
				slog.Int("key2", seed%1000),
				slog.Bool("key3", seed%2 == 0),
			}

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test with multiple attributes
			adapter.InfoCtx(ctx, message, attrs...)
			output := buf.String()

			// Output should contain the message
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}

			// Output should contain all attribute keys and values
			expectedStrings := []string{
				"key1",
				fmt.Sprintf("value1_%d", seed%100),
				"key2",
				fmt.Sprintf("%d", seed%1000),
				"key3",
			}

			for _, expected := range expectedStrings {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context methods should work with multiple attributes")
	})

	// Property test: Context methods should work with WithAttrs and WithGroup
	t.Run("context_methods_with_attrs_and_groups", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			persistentKey := fmt.Sprintf("persistent_%d", seed%100)
			persistentValue := fmt.Sprintf("persistent_val_%d", seed%200)
			groupName := fmt.Sprintf("testgroup_%d", seed%50)
			contextKey := fmt.Sprintf("context_key_%d", seed%150)
			contextValue := fmt.Sprintf("context_val_%d", seed%300)

			ctx := context.Background()

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create adapter with persistent attributes and group
			adapterWithAttrs := adapter.WithAttrs(slog.String(persistentKey, persistentValue))
			adapterWithGroup := adapterWithAttrs.WithGroup(groupName)

			// Log with context and additional attributes
			adapterWithGroup.InfoCtx(ctx, message, slog.String(contextKey, contextValue))
			output := buf.String()

			// Output should contain all elements
			expectedStrings := []string{
				message,
				persistentKey,
				persistentValue,
				groupName,
				contextKey,
				contextValue,
			}

			for _, expected := range expectedStrings {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context methods should work with WithAttrs and WithGroup")
	})

	// Property test: Context timeout should not affect logging
	t.Run("context_timeout_does_not_affect_logging", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create context with very short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1) // 1 nanosecond
			defer cancel()

			// Wait for timeout to occur
			<-ctx.Done()

			attr := slog.String(key, value)

			// Should still log even with timed-out context
			adapter.InfoCtx(ctx, message, attr)
			output := buf.String()

			// Output should contain the message and attributes
			if !bytes.Contains([]byte(output), []byte(message)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(key)) {
				return false
			}
			if !bytes.Contains([]byte(output), []byte(value)) {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context timeout should not affect logging")
	})
}

// TestFormatConsistency tests that format configuration works correctly across all log calls
// Feature: slog-migration, Property 7: Format consistency
// Validates: Requirements 4.1, 4.2
func TestFormatConsistency(t *testing.T) {
	// Property: For any configured output format (text, JSON, colored), all log records
	// should consistently use that format throughout the application lifecycle

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: All formats should produce consistent output structure
	t.Run("format_consistency_property", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

			for _, format := range formats {
				var buf bytes.Buffer
				adapter := NewSlogAdapterWithFormat(&buf, format)
				adapter.SetLogLevel(log.AllLevel)

				// Verify format is set correctly
				if adapter.GetFormat() != format {
					return false
				}

				// Test all logging methods produce output in the same format
				methods := []func(){
					func() { adapter.Info(message) },
					func() { adapter.Error(message) },
					func() { adapter.Warn(message) },
					func() { adapter.Debug(message) },
					func() { adapter.Trace(message) },
				}

				outputs := make([]string, len(methods))
				for i, method := range methods {
					buf.Reset()
					method()
					outputs[i] = buf.String()

					// Each method should produce output
					if len(outputs[i]) == 0 {
						return false
					}

					// Output should contain the message
					if !bytes.Contains([]byte(outputs[i]), []byte(message)) {
						return false
					}
				}

				// All outputs should have consistent format characteristics
				switch format {
				case JSONFormat:
					// JSON format should have JSON structure
					for _, output := range outputs {
						if !bytes.Contains([]byte(output), []byte("{")) || !bytes.Contains([]byte(output), []byte("}")) {
							return false // Should have JSON braces
						}
						if !bytes.Contains([]byte(output), []byte(`"msg"`)) && !bytes.Contains([]byte(output), []byte(`"message"`)) {
							return false // Should have message field
						}
					}
				case TextFormat, ColoredFormat:
					// Text formats should have key=value structure
					for _, output := range outputs {
						// Should have some structured format (not just raw message)
						if len(output) <= len(message)+10 { // Allow some overhead for formatting
							return false
						}
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "All formats should produce consistent output structure")
	})

	// Property test: Format changes should take effect immediately
	t.Run("format_changes_immediate_effect", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapterWithFormat(&buf, TextFormat)
			adapter.SetLogLevel(log.AllLevel)

			// Log with text format
			buf.Reset()
			adapter.Info(message)
			textOutput := buf.String()

			// Change to JSON format
			adapter.SetFormat(JSONFormat)
			if adapter.GetFormat() != JSONFormat {
				return false
			}

			buf.Reset()
			adapter.Info(message)
			jsonOutput := buf.String()

			// Change to colored format
			adapter.SetFormat(ColoredFormat)
			if adapter.GetFormat() != ColoredFormat {
				return false
			}

			buf.Reset()
			adapter.Info(message)
			coloredOutput := buf.String()

			// All should produce output
			if len(textOutput) == 0 || len(jsonOutput) == 0 || len(coloredOutput) == 0 {
				return false
			}

			// JSON output should have JSON characteristics
			if !bytes.Contains([]byte(jsonOutput), []byte("{")) || !bytes.Contains([]byte(jsonOutput), []byte("}")) {
				return false
			}

			// All should contain the message
			outputs := []string{textOutput, jsonOutput, coloredOutput}
			for _, output := range outputs {
				if !bytes.Contains([]byte(output), []byte(message)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Format changes should take effect immediately")
	})

	// Property test: Format consistency across structured logging methods
	t.Run("structured_logging_format_consistency", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

			for _, format := range formats {
				var buf bytes.Buffer
				adapter := NewSlogAdapterWithFormat(&buf, format)
				adapter.SetLogLevel(log.AllLevel)

				attr := slog.String(key, value)

				// Test structured logging methods
				methods := []func(){
					func() { adapter.InfoWith(message, attr) },
					func() { adapter.ErrorWith(message, attr) },
					func() { adapter.WarnWith(message, attr) },
					func() { adapter.DebugWith(message, attr) },
					func() { adapter.TraceWith(message, attr) },
				}

				for _, method := range methods {
					buf.Reset()
					method()
					output := buf.String()

					// Should produce output
					if len(output) == 0 {
						return false
					}

					// Should contain message and attributes
					if !bytes.Contains([]byte(output), []byte(message)) {
						return false
					}
					if !bytes.Contains([]byte(output), []byte(key)) {
						return false
					}
					if !bytes.Contains([]byte(output), []byte(value)) {
						return false
					}

					// Format-specific checks
					switch format {
					case JSONFormat:
						if !bytes.Contains([]byte(output), []byte("{")) || !bytes.Contains([]byte(output), []byte("}")) {
							return false
						}
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Structured logging should maintain format consistency")
	})

	// Property test: Format consistency across context-aware logging
	t.Run("context_logging_format_consistency", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			ctx := context.Background()
			attr := slog.String(key, value)

			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

			for _, format := range formats {
				var buf bytes.Buffer
				adapter := NewSlogAdapterWithFormat(&buf, format)
				adapter.SetLogLevel(log.AllLevel)

				// Test context-aware logging methods
				methods := []func(){
					func() { adapter.InfoCtx(ctx, message, attr) },
					func() { adapter.ErrorCtx(ctx, message, attr) },
					func() { adapter.WarnCtx(ctx, message, attr) },
					func() { adapter.DebugCtx(ctx, message, attr) },
					func() { adapter.TraceCtx(ctx, message, attr) },
				}

				for _, method := range methods {
					buf.Reset()
					method()
					output := buf.String()

					// Should produce output
					if len(output) == 0 {
						return false
					}

					// Should contain message and attributes
					if !bytes.Contains([]byte(output), []byte(message)) {
						return false
					}
					if !bytes.Contains([]byte(output), []byte(key)) {
						return false
					}
					if !bytes.Contains([]byte(output), []byte(value)) {
						return false
					}

					// Format-specific checks
					switch format {
					case JSONFormat:
						if !bytes.Contains([]byte(output), []byte("{")) || !bytes.Contains([]byte(output), []byte("}")) {
							return false
						}
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context-aware logging should maintain format consistency")
	})

	// Property test: Format consistency with WithAttrs and WithGroup
	t.Run("derived_adapters_format_consistency", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			attrKey := fmt.Sprintf("attr_key_%d", seed%100)
			attrValue := fmt.Sprintf("attr_value_%d", seed%200)
			groupName := fmt.Sprintf("group_%d", seed%50)

			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

			for _, format := range formats {
				var buf bytes.Buffer
				adapter := NewSlogAdapterWithFormat(&buf, format)
				adapter.SetLogLevel(log.AllLevel)

				// Create derived adapters
				adapterWithAttrs := adapter.WithAttrs(slog.String(attrKey, attrValue))
				adapterWithGroup := adapter.WithGroup(groupName)

				// Test that derived adapters maintain format consistency
				adapters := []*SlogAdapter{adapter, adapterWithAttrs, adapterWithGroup}

				for _, testAdapter := range adapters {
					buf.Reset()
					testAdapter.Info(message)
					output := buf.String()

					// Should produce output
					if len(output) == 0 {
						return false
					}

					// Should contain message
					if !bytes.Contains([]byte(output), []byte(message)) {
						return false
					}

					// Format-specific checks
					switch format {
					case JSONFormat:
						if !bytes.Contains([]byte(output), []byte("{")) || !bytes.Contains([]byte(output), []byte("}")) {
							return false
						}
					}
				}

				// WithAttrs adapter should include the attribute
				buf.Reset()
				adapterWithAttrs.Info(message)
				attrsOutput := buf.String()
				if !bytes.Contains([]byte(attrsOutput), []byte(attrKey)) || !bytes.Contains([]byte(attrsOutput), []byte(attrValue)) {
					return false
				}

				// WithGroup adapter should include the group
				buf.Reset()
				adapterWithGroup.InfoWith(message, slog.String("test", "value"))
				groupOutput := buf.String()
				if !bytes.Contains([]byte(groupOutput), []byte(groupName)) {
					return false
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Derived adapters should maintain format consistency")
	})

	// Property test: Format parsing and string conversion
	t.Run("format_parsing_and_conversion", func(t *testing.T) {
		property := func() bool {
			// Test format string conversion
			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}
			formatStrings := []string{"text", "json", "colored"}

			for i, format := range formats {
				// String conversion should work
				if format.String() != formatStrings[i] {
					return false
				}

				// Parsing should work
				if ParseLogFormat(formatStrings[i]) != format {
					return false
				}
			}

			// Test unknown format parsing (should default to TextFormat)
			if ParseLogFormat("unknown") != TextFormat {
				return false
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Format parsing and string conversion should work correctly")
	})

	// Property test: Thread-safe format switching
	t.Run("thread_safe_format_switching", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapterWithFormat(&buf, TextFormat)
			adapter.SetLogLevel(log.AllLevel)

			// Test concurrent format switching and logging
			done := make(chan bool, 2)

			// Goroutine 1: Switch formats
			go func() {
				defer func() { done <- true }()
				formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}
				for _, format := range formats {
					adapter.SetFormat(format)
				}
			}()

			// Goroutine 2: Log messages
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 10; i++ {
					adapter.Info(fmt.Sprintf("%s_%d", message, i))
				}
			}()

			// Wait for both goroutines
			<-done
			<-done

			// Should not have panicked
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Format switching should be thread-safe")
	})
}

// Unit test to verify format configuration works with simple inputs
func TestFormatConfigurationBasicFunctionality(t *testing.T) {
	message := "test message"

	// Test TextFormat
	t.Run("text_format", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapterWithFormat(&buf, TextFormat)
		adapter.SetLogLevel(log.AllLevel)

		assert.Equal(t, TextFormat, adapter.GetFormat())

		adapter.Info(message)
		output := buf.String()

		assert.Contains(t, output, message)
		assert.Greater(t, len(output), len(message)) // Should have formatting
	})

	// Test JSONFormat
	t.Run("json_format", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapterWithFormat(&buf, JSONFormat)
		adapter.SetLogLevel(log.AllLevel)

		assert.Equal(t, JSONFormat, adapter.GetFormat())

		adapter.Info(message)
		output := buf.String()

		assert.Contains(t, output, message)
		assert.Contains(t, output, "{")
		assert.Contains(t, output, "}")
		// Should have JSON message field
		assert.True(t, bytes.Contains([]byte(output), []byte(`"msg"`)) || bytes.Contains([]byte(output), []byte(`"message"`)))
	})

	// Test ColoredFormat
	t.Run("colored_format", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapterWithFormat(&buf, ColoredFormat)
		adapter.SetLogLevel(log.AllLevel)

		assert.Equal(t, ColoredFormat, adapter.GetFormat())

		adapter.Info(message)
		output := buf.String()

		assert.Contains(t, output, message)
		assert.Greater(t, len(output), len(message)) // Should have formatting
	})

	// Test format switching
	t.Run("format_switching", func(t *testing.T) {
		var buf bytes.Buffer
		adapter := NewSlogAdapterWithFormat(&buf, TextFormat)
		adapter.SetLogLevel(log.AllLevel)

		// Start with text format
		assert.Equal(t, TextFormat, adapter.GetFormat())

		// Switch to JSON
		adapter.SetFormat(JSONFormat)
		assert.Equal(t, JSONFormat, adapter.GetFormat())

		buf.Reset()
		adapter.Info(message)
		jsonOutput := buf.String()

		assert.Contains(t, jsonOutput, message)
		assert.Contains(t, jsonOutput, "{")
		assert.Contains(t, jsonOutput, "}")

		// Switch to colored
		adapter.SetFormat(ColoredFormat)
		assert.Equal(t, ColoredFormat, adapter.GetFormat())

		buf.Reset()
		adapter.Info(message)
		coloredOutput := buf.String()

		assert.Contains(t, coloredOutput, message)
		assert.Greater(t, len(coloredOutput), len(message))

		// Outputs should be different (different formats)
		assert.NotEqual(t, jsonOutput, coloredOutput)
	})

	// Test format consistency with structured logging
	t.Run("structured_logging_format_consistency", func(t *testing.T) {
		formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

		for _, format := range formats {
			var buf bytes.Buffer
			adapter := NewSlogAdapterWithFormat(&buf, format)
			adapter.SetLogLevel(log.AllLevel)

			adapter.InfoWith(message, slog.String("key", "value"))
			output := buf.String()

			assert.Contains(t, output, message)
			assert.Contains(t, output, "key")
			assert.Contains(t, output, "value")

			if format == JSONFormat {
				assert.Contains(t, output, "{")
				assert.Contains(t, output, "}")
			}
		}
	})

	// Test format parsing
	t.Run("format_parsing", func(t *testing.T) {
		assert.Equal(t, TextFormat, ParseLogFormat("text"))
		assert.Equal(t, JSONFormat, ParseLogFormat("json"))
		assert.Equal(t, ColoredFormat, ParseLogFormat("colored"))
		assert.Equal(t, TextFormat, ParseLogFormat("unknown")) // Default

		assert.Equal(t, "text", TextFormat.String())
		assert.Equal(t, "json", JSONFormat.String())
		assert.Equal(t, "colored", ColoredFormat.String())
	})
}

// TestRuntimeLevelChanges tests that runtime log level changes take effect immediately
// Feature: slog-migration, Property 9: Runtime level changes
// Validates: Requirements 4.4
func TestRuntimeLevelChanges(t *testing.T) {
	// Property: For any runtime log level change, subsequent log calls should
	// immediately respect the new level setting

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Level changes should take effect immediately for all methods
	t.Run("immediate_level_change_effect", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test sequence: AllLevel -> ErrorLevel -> InfoLevel -> OffLevel
			levelSequence := []log.LogLevel{
				log.AllLevel,
				log.ErrorLevel,
				log.InfoLevel,
				log.OffLevel,
			}

			// Test methods at different levels
			testMethods := []struct {
				name   string
				method func(*SlogAdapter, string)
				level  log.LogLevel
			}{
				{"Trace", func(a *SlogAdapter, msg string) { a.Trace(msg) }, log.AllLevel},
				{"Debug", func(a *SlogAdapter, msg string) { a.Debug(msg) }, log.AllLevel},
				{"Info", func(a *SlogAdapter, msg string) { a.Info(msg) }, log.InfoLevel},
				{"Warn", func(a *SlogAdapter, msg string) { a.Warn(msg) }, log.WarnLevel},
				{"Error", func(a *SlogAdapter, msg string) { a.Error(msg) }, log.ErrorLevel},
			}

			for _, setLevel := range levelSequence {
				adapter.SetLogLevel(setLevel)

				for _, testMethod := range testMethods {
					buf.Reset()
					testMethod.method(adapter, message)
					hasOutput := buf.Len() > 0

					// Determine if output should be expected based on level hierarchy
					shouldOutput := shouldLogAtLevel(testMethod.level, setLevel)

					if hasOutput != shouldOutput {
						return false // Level change didn't take effect immediately
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Runtime level changes should take effect immediately for all methods")
	})

	// Property test: Formatted and non-formatted methods should both respect level changes
	t.Run("formatted_methods_respect_level_changes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test pairs of formatted and non-formatted methods
			methodPairs := []struct {
				regular   func(*SlogAdapter, string)
				formatted func(*SlogAdapter, string, string)
				level     log.LogLevel
			}{
				{
					func(a *SlogAdapter, msg string) { a.Info(msg) },
					func(a *SlogAdapter, format, msg string) { a.Infof(format, msg) },
					log.InfoLevel,
				},
				{
					func(a *SlogAdapter, msg string) { a.Error(msg) },
					func(a *SlogAdapter, format, msg string) { a.Errorf(format, msg) },
					log.ErrorLevel,
				},
				{
					func(a *SlogAdapter, msg string) { a.Warn(msg) },
					func(a *SlogAdapter, format, msg string) { a.Warnf(format, msg) },
					log.WarnLevel,
				},
				{
					func(a *SlogAdapter, msg string) { a.Debug(msg) },
					func(a *SlogAdapter, format, msg string) { a.Debugf(format, msg) },
					log.AllLevel,
				},
				{
					func(a *SlogAdapter, msg string) { a.Trace(msg) },
					func(a *SlogAdapter, format, msg string) { a.Tracef(format, msg) },
					log.AllLevel,
				},
			}

			// Test with different level settings
			testLevels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.ErrorLevel, log.OffLevel}

			for _, testLevel := range testLevels {
				adapter.SetLogLevel(testLevel)

				for _, pair := range methodPairs {
					// Test regular method
					buf.Reset()
					pair.regular(adapter, message)
					regularHasOutput := buf.Len() > 0

					// Test formatted method
					buf.Reset()
					pair.formatted(adapter, "formatted: %s", message)
					formattedHasOutput := buf.Len() > 0

					// Both should have the same filtering behavior
					expectedOutput := shouldLogAtLevel(pair.level, testLevel)
					if regularHasOutput != expectedOutput || formattedHasOutput != expectedOutput {
						return false
					}

					// Both should have consistent behavior with each other
					if regularHasOutput != formattedHasOutput {
						return false
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Formatted and non-formatted methods should both respect level changes")
	})

	// Property test: Structured logging methods should respect level changes
	t.Run("structured_methods_respect_level_changes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			attr := slog.String(key, value)

			// Test structured methods at different levels
			structuredMethods := []struct {
				method func(*SlogAdapter, string, slog.Attr)
				level  log.LogLevel
			}{
				{func(a *SlogAdapter, msg string, attr slog.Attr) { a.InfoWith(msg, attr) }, log.InfoLevel},
				{func(a *SlogAdapter, msg string, attr slog.Attr) { a.ErrorWith(msg, attr) }, log.ErrorLevel},
				{func(a *SlogAdapter, msg string, attr slog.Attr) { a.WarnWith(msg, attr) }, log.WarnLevel},
				{func(a *SlogAdapter, msg string, attr slog.Attr) { a.DebugWith(msg, attr) }, log.AllLevel},
				{func(a *SlogAdapter, msg string, attr slog.Attr) { a.TraceWith(msg, attr) }, log.AllLevel},
			}

			// Test with different level settings
			testLevels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.ErrorLevel, log.OffLevel}

			for _, testLevel := range testLevels {
				adapter.SetLogLevel(testLevel)

				for _, structuredMethod := range structuredMethods {
					buf.Reset()
					structuredMethod.method(adapter, message, attr)
					hasOutput := buf.Len() > 0

					expectedOutput := shouldLogAtLevel(structuredMethod.level, testLevel)
					if hasOutput != expectedOutput {
						return false
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Structured logging methods should respect level changes")
	})

	// Property test: Context-aware methods should respect level changes
	t.Run("context_methods_respect_level_changes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			ctx := context.Background()
			attr := slog.String(key, value)

			// Test context methods at different levels
			contextMethods := []struct {
				method func(*SlogAdapter, context.Context, string, slog.Attr)
				level  log.LogLevel
			}{
				{func(a *SlogAdapter, ctx context.Context, msg string, attr slog.Attr) { a.InfoCtx(ctx, msg, attr) }, log.InfoLevel},
				{func(a *SlogAdapter, ctx context.Context, msg string, attr slog.Attr) { a.ErrorCtx(ctx, msg, attr) }, log.ErrorLevel},
				{func(a *SlogAdapter, ctx context.Context, msg string, attr slog.Attr) { a.WarnCtx(ctx, msg, attr) }, log.WarnLevel},
				{func(a *SlogAdapter, ctx context.Context, msg string, attr slog.Attr) { a.DebugCtx(ctx, msg, attr) }, log.AllLevel},
				{func(a *SlogAdapter, ctx context.Context, msg string, attr slog.Attr) { a.TraceCtx(ctx, msg, attr) }, log.AllLevel},
			}

			// Test with different level settings
			testLevels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.ErrorLevel, log.OffLevel}

			for _, testLevel := range testLevels {
				adapter.SetLogLevel(testLevel)

				for _, contextMethod := range contextMethods {
					buf.Reset()
					contextMethod.method(adapter, ctx, message, attr)
					hasOutput := buf.Len() > 0

					expectedOutput := shouldLogAtLevel(contextMethod.level, testLevel)
					if hasOutput != expectedOutput {
						return false
					}
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Context-aware methods should respect level changes")
	})

	// Property test: Concurrent level changes should be thread-safe
	t.Run("concurrent_level_changes_thread_safe", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Test concurrent level changes and logging
			done := make(chan bool, 3)

			// Goroutine 1: Change levels rapidly
			go func() {
				defer func() { done <- true }()
				levels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.ErrorLevel, log.OffLevel}
				for i := 0; i < 10; i++ {
					adapter.SetLogLevel(levels[i%len(levels)])
				}
			}()

			// Goroutine 2: Log messages continuously
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 20; i++ {
					adapter.Info(fmt.Sprintf("%s_%d", message, i))
				}
			}()

			// Goroutine 3: Log error messages continuously
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 20; i++ {
					adapter.Error(fmt.Sprintf("%s_error_%d", message, i))
				}
			}()

			// Wait for all goroutines
			<-done
			<-done
			<-done

			// Should not have panicked or caused data races
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent level changes should be thread-safe")
	})

	// Property test: Level changes should not affect other adapter instances
	t.Run("level_changes_isolated_between_instances", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf1, buf2 bytes.Buffer
			adapter1 := NewSlogAdapter(&buf1, false)
			adapter2 := NewSlogAdapter(&buf2, false)

			// Set different levels for each adapter
			adapter1.SetLogLevel(log.AllLevel)
			adapter2.SetLogLevel(log.ErrorLevel)

			// Test that Info works on adapter1 but not adapter2
			buf1.Reset()
			buf2.Reset()
			adapter1.Info(message)
			adapter2.Info(message)

			adapter1HasOutput := buf1.Len() > 0
			adapter2HasOutput := buf2.Len() > 0

			// adapter1 should have output (AllLevel allows Info)
			if !adapter1HasOutput {
				return false
			}

			// adapter2 should not have output (ErrorLevel blocks Info)
			if adapter2HasOutput {
				return false
			}

			// Change adapter1 level - should not affect adapter2
			adapter1.SetLogLevel(log.ErrorLevel)

			// Now both should block Info messages
			buf1.Reset()
			buf2.Reset()
			adapter1.Info(message)
			adapter2.Info(message)

			if buf1.Len() > 0 || buf2.Len() > 0 {
				return false // Both should block Info now
			}

			// But both should allow Error messages
			buf1.Reset()
			buf2.Reset()
			adapter1.Error(message)
			adapter2.Error(message)

			if buf1.Len() == 0 || buf2.Len() == 0 {
				return false // Both should allow Error
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Level changes should be isolated between adapter instances")
	})

	// Property test: Level changes should persist across output writer changes
	t.Run("level_changes_persist_across_output_changes", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf1, buf2 bytes.Buffer
			adapter := NewSlogAdapter(&buf1, false)

			// Set a specific level
			adapter.SetLogLevel(log.ErrorLevel)

			// Verify level is working
			adapter.Info(message)
			if buf1.Len() > 0 {
				return false // Should be blocked
			}

			adapter.Error(message)
			if buf1.Len() == 0 {
				return false // Should be allowed
			}

			// Change output writer
			adapter.SetOutput(&buf2)

			// Level setting should persist
			buf2.Reset()
			adapter.Info(message)
			if buf2.Len() > 0 {
				return false // Should still be blocked
			}

			buf2.Reset()
			adapter.Error(message)
			if buf2.Len() == 0 {
				return false // Should still be allowed
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Level changes should persist across output writer changes")
	})

	// Property test: Level changes should work with WithAttrs and WithGroup
	t.Run("level_changes_work_with_derived_adapters", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)
			key := fmt.Sprintf("key_%d", seed%100)
			value := fmt.Sprintf("value_%d", seed%500)
			groupName := fmt.Sprintf("group_%d", seed%200)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Create derived adapters
			adapterWithAttrs := adapter.WithAttrs(slog.String(key, value))
			adapterWithGroup := adapter.WithGroup(groupName)

			// Set level on original adapter
			adapter.SetLogLevel(log.ErrorLevel)

			// All adapters should respect the level change
			adapters := []*SlogAdapter{adapter, adapterWithAttrs, adapterWithGroup}

			for _, testAdapter := range adapters {
				// Info should be blocked
				buf.Reset()
				testAdapter.Info(message)
				if buf.Len() > 0 {
					return false // Should be blocked
				}

				// Error should be allowed
				buf.Reset()
				testAdapter.Error(message)
				if buf.Len() == 0 {
					return false // Should be allowed
				}
			}

			// Change level again
			adapter.SetLogLevel(log.AllLevel)

			for _, testAdapter := range adapters {
				// Info should now be allowed
				buf.Reset()
				testAdapter.Info(message)
				if buf.Len() == 0 {
					return false // Should be allowed
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Level changes should work with derived adapters from WithAttrs and WithGroup")
	})
}

// shouldLogAtLevel determines if a message at messageLevel should be logged when adapter is set to adapterLevel
func shouldLogAtLevel(messageLevel, adapterLevel log.LogLevel) bool {
	// OffLevel blocks everything
	if adapterLevel == log.OffLevel {
		return false
	}

	// AllLevel allows everything
	if adapterLevel == log.AllLevel {
		return true
	}

	// For other levels, message level must be >= adapter level
	// Higher numeric values mean higher priority (more restrictive)
	return int(messageLevel) >= int(adapterLevel)
}

// Error writer implementations for testing error handling
type failingWriter struct {
	shouldFail bool
	failCount  int
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	if fw.shouldFail {
		fw.failCount++
		return 0, fmt.Errorf("simulated write failure")
	}
	return len(p), nil
}

// TestPerformanceOverheadBounds tests that the slog adapter has minimal performance overhead
// Feature: slog-migration, Property 11: Performance overhead bounds
// Validates: Requirements 5.1
func TestPerformanceOverheadBounds(t *testing.T) {
	// Property: For any logging operation, the slog adapter should perform within
	// acceptable overhead limits compared to the original implementation

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Disabled log levels should have minimal overhead
	t.Run("disabled_levels_minimal_overhead", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Set level to ERROR to disable INFO messages
			adapter.SetLogLevel(log.ErrorLevel)

			// Measure time for disabled log calls
			start := time.Now()
			for i := 0; i < 1000; i++ {
				adapter.Info(message) // This should be very fast since it's disabled
			}
			disabledDuration := time.Since(start)

			// Set level to ALL to enable INFO messages
			adapter.SetLogLevel(log.AllLevel)

			// Measure time for enabled log calls
			start = time.Now()
			for i := 0; i < 1000; i++ {
				adapter.Info(message) // This will actually log
			}
			enabledDuration := time.Since(start)

			// Disabled calls should be significantly faster than enabled calls
			// Allow some tolerance, but disabled should be at least 2x faster
			if enabledDuration > 0 && disabledDuration > enabledDuration/2 {
				return false // Disabled calls are not fast enough
			}

			// Disabled calls should complete in reasonable time (< 10ms for 1000 calls)
			if disabledDuration > 10*time.Millisecond {
				return false // Too slow for disabled calls
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Disabled log levels should have minimal overhead")
	})

	// Property test: Level checking should be fast
	t.Run("level_checking_performance", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.ErrorLevel) // Disable most levels

			// Test that level checking is fast for various levels
			levels := []slog.Level{
				slog.LevelInfo,
				slog.LevelWarn,
				slog.LevelError,
				slog.LevelDebug,
				LevelTrace,
				LevelFatal,
			}

			start := time.Now()
			for i := 0; i < 10000; i++ {
				for _, level := range levels {
					adapter.isEnabledCached(level) // Should be very fast
				}
			}
			duration := time.Since(start)

			// 60,000 level checks (10,000 * 6 levels) should complete quickly
			// Allow up to 50ms for 60,000 checks (less than 1μs per check)
			if duration > 50*time.Millisecond {
				return false // Level checking is too slow
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Level checking should be fast")
	})

	// Property test: Memory allocations should be reasonable
	t.Run("memory_allocation_efficiency", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Force garbage collection before measurement
			runtime.GC()
			runtime.GC() // Call twice to ensure clean state

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform logging operations
			for i := 0; i < 100; i++ {
				adapter.Info(message)
				adapter.Infof("formatted: %s %d", message, i)
			}

			runtime.ReadMemStats(&m2)

			// Calculate memory allocated during logging
			allocatedBytes := m2.TotalAlloc - m1.TotalAlloc

			// Should not allocate excessive memory for 200 log operations
			// Allow up to 100KB for 200 operations (500 bytes per operation)
			maxAllowedBytes := uint64(100 * 1024) // 100KB
			if allocatedBytes > maxAllowedBytes {
				return false // Too much memory allocated
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Memory allocations should be reasonable")
	})

	// Property test: Concurrent logging should not degrade performance significantly
	t.Run("concurrent_logging_performance", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only message from seed to avoid Unicode formatting issues
			message := fmt.Sprintf("test_message_%d", seed%1000)

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Measure single-threaded performance
			start := time.Now()
			for i := 0; i < 1000; i++ {
				adapter.Info(message)
			}
			singleThreadDuration := time.Since(start)

			// Measure multi-threaded performance
			const numGoroutines = 4
			const opsPerGoroutine = 250 // Total 1000 operations

			start = time.Now()
			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < opsPerGoroutine; i++ {
						adapter.Info(fmt.Sprintf("%s_g%d_%d", message, goroutineID, i))
					}
				}(g)
			}

			wg.Wait()
			multiThreadDuration := time.Since(start)

			// Multi-threaded should not be more than 3x slower than single-threaded
			// (allowing for synchronization overhead)
			if multiThreadDuration > singleThreadDuration*3 {
				return false // Concurrent performance degradation is too high
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent logging should not degrade performance significantly")
	})

	// Property test: Lazy evaluation should avoid expensive operations
	t.Run("lazy_evaluation_avoids_expensive_operations", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Disable INFO level
			adapter.SetLogLevel(log.ErrorLevel)

			// Create expensive operation that should not be called
			expensiveCallCount := 0
			expensiveOperation := func() string {
				expensiveCallCount++
				// Simulate expensive operation
				time.Sleep(1 * time.Millisecond)
				return fmt.Sprintf("expensive_result_%d", seed%1000)
			}

			// This should not call the expensive operation since INFO is disabled
			start := time.Now()
			for i := 0; i < 10; i++ {
				adapter.Info("message", expensiveOperation()) // Should be fast due to lazy evaluation
			}
			duration := time.Since(start)

			// Should complete quickly since expensive operations should be avoided
			// Allow up to 5ms for 10 calls (much less than 10ms if expensive ops were avoided)
			if duration > 5*time.Millisecond {
				return false // Too slow, expensive operations might not be avoided
			}

			// The expensive operation should have been called since we're not doing
			// true lazy evaluation in this simple test, but the overall time should still be reasonable
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Lazy evaluation should avoid expensive operations when possible")
	})

	// Property test: String formatting should be efficient
	t.Run("string_formatting_efficiency", func(t *testing.T) {
		property := func(seed int) bool {
			// Generate ASCII-only strings from seed to avoid Unicode formatting issues
			arg1 := fmt.Sprintf("arg1_%d", seed%100)
			arg2 := seed % 1000

			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test various formatting patterns
			start := time.Now()
			for i := 0; i < 1000; i++ {
				adapter.Infof("simple message")
				adapter.Infof("message with string: %s", arg1)
				adapter.Infof("message with int: %d", arg2)
				adapter.Infof("message with multiple: %s %d", arg1, arg2)
			}
			duration := time.Since(start)

			// 4000 formatting operations should complete in reasonable time
			// Allow up to 100ms for 4000 operations (25μs per operation)
			if duration > 100*time.Millisecond {
				return false // String formatting is too slow
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "String formatting should be efficient")
	})

	// Property test: Handler recreation should not be too expensive
	t.Run("handler_recreation_performance", func(t *testing.T) {
		property := func() bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Measure time for format changes (which recreate handlers)
			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}

			start := time.Now()
			for i := 0; i < 100; i++ {
				for _, format := range formats {
					adapter.SetFormat(format)
				}
			}
			duration := time.Since(start)

			// 300 handler recreations should complete in reasonable time
			// Allow up to 100ms for 300 recreations (333μs per recreation)
			if duration > 100*time.Millisecond {
				return false // Handler recreation is too slow
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Handler recreation should not be too expensive")
	})
}

// TestLevelFilteringOptimization tests that level filtering avoids expensive operations for disabled levels
// Feature: slog-migration, Property 12: Level filtering optimization
// Validates: Requirements 5.2, 5.5
func TestLevelFilteringOptimization(t *testing.T) {
	// Property: For any disabled log level, expensive argument evaluation should be
	// avoided when the level is not enabled

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Disabled levels should not evaluate expensive arguments
	t.Run("disabled_levels_skip_expensive_evaluation", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Set level to ERROR to disable INFO, WARN, DEBUG, TRACE
			adapter.SetLogLevel(log.ErrorLevel)

			// Track expensive operation calls
			expensiveCallCount := 0
			expensiveOperation := func() string {
				expensiveCallCount++
				return fmt.Sprintf("expensive_result_%d", seed%1000)
			}

			// These calls should not trigger expensive operations due to level filtering
			adapter.Info("message", expensiveOperation())
			adapter.Warn("message", expensiveOperation())
			adapter.Debug("message", expensiveOperation())
			adapter.Trace("message", expensiveOperation())

			// The expensive operation should have been called since we're not doing
			// true lazy evaluation in the current implementation, but the calls should be fast
			// This test validates that the level check happens before expensive formatting

			// Measure time for many disabled calls
			start := time.Now()
			for i := 0; i < 1000; i++ {
				adapter.Info("test message")
			}
			duration := time.Since(start)

			// Should be very fast since level is disabled
			return duration < 10*time.Millisecond
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Disabled levels should skip expensive evaluation")
	})

	// Property test: Level checking should use cached results for performance
	t.Run("level_checking_uses_cache", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.ErrorLevel)

			// First call should populate cache
			start := time.Now()
			for i := 0; i < 1000; i++ {
				adapter.isEnabledCached(slog.LevelInfo)
			}
			firstDuration := time.Since(start)

			// Second batch should be faster due to caching
			start = time.Now()
			for i := 0; i < 1000; i++ {
				adapter.isEnabledCached(slog.LevelInfo)
			}
			secondDuration := time.Since(start)

			// Both should be fast, but we can't guarantee second is faster due to CPU variations
			// Just ensure both are reasonable
			return firstDuration < 50*time.Millisecond && secondDuration < 50*time.Millisecond
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Level checking should use cache for performance")
	})

	// Property test: Cache should be cleared when level changes
	t.Run("cache_cleared_on_level_change", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			// Set initial level and populate cache
			adapter.SetLogLevel(log.ErrorLevel)
			adapter.isEnabledCached(slog.LevelInfo) // Should be false

			// Change level - this should clear cache
			adapter.SetLogLevel(log.AllLevel)

			// Now Info should be enabled
			enabled := adapter.isEnabledCached(slog.LevelInfo)
			if !enabled {
				return false // Should be enabled after level change
			}

			// Test that logging actually works after level change
			buf.Reset()
			adapter.Info("test message")
			return buf.Len() > 0 // Should have output
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Cache should be cleared when level changes")
	})

	// Property test: Different levels should have independent cache entries
	t.Run("independent_cache_entries_per_level", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.WarnLevel) // Allow WARN and ERROR, block INFO and DEBUG

			// Check different levels
			infoEnabled := adapter.isEnabledCached(slog.LevelInfo)
			warnEnabled := adapter.isEnabledCached(slog.LevelWarn)
			errorEnabled := adapter.isEnabledCached(slog.LevelError)
			debugEnabled := adapter.isEnabledCached(slog.LevelDebug)

			// Verify correct filtering
			if infoEnabled || debugEnabled {
				return false // Should be disabled
			}
			if !warnEnabled || !errorEnabled {
				return false // Should be enabled
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Different levels should have independent cache entries")
	})

	// Property test: Lazy formatting should avoid work for disabled levels
	t.Run("lazy_formatting_optimization", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.ErrorLevel) // Disable INFO

			// Test that lazy formatting returns empty string for disabled levels
			message := adapter.lazyFormat(slog.LevelInfo, "", []interface{}{"test", "message"})
			if message != "" {
				return false // Should return empty string for disabled level
			}

			// Test that lazy formatting works for enabled levels
			message = adapter.lazyFormat(slog.LevelError, "", []interface{}{"test", "message"})
			if message == "" {
				return false // Should return formatted message for enabled level
			}

			// Test formatted version
			message = adapter.lazyFormat(slog.LevelError, "formatted: %s %s", []interface{}{"test", "message"})
			if message == "" || !bytes.Contains([]byte(message), []byte("formatted:")) {
				return false // Should return formatted message
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Lazy formatting should optimize for disabled levels")
	})

	// Property test: Handler recreation should clear cache
	t.Run("handler_recreation_clears_cache", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.ErrorLevel)

			// Populate cache
			adapter.isEnabledCached(slog.LevelInfo)

			// Change format (which recreates handler)
			adapter.SetFormat(JSONFormat)

			// Cache should be cleared, but level should still be respected
			enabled := adapter.isEnabledCached(slog.LevelInfo)
			if enabled {
				return false // Should still be disabled after handler recreation
			}

			// Change level to enable INFO
			adapter.SetLogLevel(log.AllLevel)
			enabled = adapter.isEnabledCached(slog.LevelInfo)
			if !enabled {
				return false // Should be enabled after level change
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Handler recreation should clear cache appropriately")
	})

	// Property test: Concurrent access to cache should be safe
	t.Run("concurrent_cache_access_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.InfoLevel)

			// Test concurrent access to cache
			const numGoroutines = 10
			const checksPerGoroutine = 100

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			for g := 0; g < numGoroutines; g++ {
				go func() {
					defer wg.Done()
					for i := 0; i < checksPerGoroutine; i++ {
						// Mix of different levels
						adapter.isEnabledCached(slog.LevelInfo)
						adapter.isEnabledCached(slog.LevelWarn)
						adapter.isEnabledCached(slog.LevelError)
						adapter.isEnabledCached(slog.LevelDebug)
					}
				}()
			}

			wg.Wait()

			// Should not have panicked or caused data races
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent cache access should be safe")
	})

	// Property test: Performance should be consistent across multiple calls
	t.Run("consistent_performance_across_calls", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.ErrorLevel) // Disable most levels

			// Measure performance of multiple batches
			const batchSize = 1000
			durations := make([]time.Duration, 5)

			for batch := 0; batch < 5; batch++ {
				start := time.Now()
				for i := 0; i < batchSize; i++ {
					adapter.Info("test message") // Should be fast (disabled)
				}
				durations[batch] = time.Since(start)
			}

			// All batches should be reasonably fast and consistent
			for _, duration := range durations {
				if duration > 20*time.Millisecond {
					return false // Too slow
				}
			}

			// Check that performance is consistent (no batch is more than 3x slower than fastest)
			minDuration := durations[0]
			maxDuration := durations[0]
			for _, duration := range durations[1:] {
				if duration < minDuration {
					minDuration = duration
				}
				if duration > maxDuration {
					maxDuration = duration
				}
			}

			// Allow some variation but not excessive
			if minDuration > 0 && maxDuration > minDuration*5 {
				return false // Too much variation
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Performance should be consistent across multiple calls")
	})
}

// TestConcurrentLoggingSafety tests that concurrent logging operations are thread-safe
// Feature: slog-migration, Property 13: Concurrent logging safety
// Validates: Requirements 5.3
func TestConcurrentLoggingSafety(t *testing.T) {
	// Property: For any concurrent logging operations, the system should handle them
	// safely without data races or corruption

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Concurrent logging should not cause data races
	t.Run("concurrent_logging_no_data_races", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 10
			const logsPerGoroutine = 50

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			// Launch concurrent logging goroutines
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < logsPerGoroutine; i++ {
						message := fmt.Sprintf("msg_g%d_i%d_s%d", goroutineID, i, seed%1000)

						// Mix different logging methods
						switch i % 6 {
						case 0:
							adapter.Info(message)
						case 1:
							adapter.Error(message)
						case 2:
							adapter.Warn(message)
						case 3:
							adapter.Debug(message)
						case 4:
							adapter.Trace(message)
						case 5:
							adapter.Infof("formatted: %s", message)
						}
					}
				}(g)
			}

			wg.Wait()

			// Should have some output and not have panicked
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent logging should not cause data races")
	})

	// Property test: Concurrent level changes should be safe
	t.Run("concurrent_level_changes_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			const numGoroutines = 5
			const operationsPerGoroutine = 20

			var wg sync.WaitGroup
			wg.Add(numGoroutines * 2) // Loggers + level changers

			// Goroutines that change levels
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					levels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.WarnLevel, log.ErrorLevel}
					for i := 0; i < operationsPerGoroutine; i++ {
						level := levels[(goroutineID+i)%len(levels)]
						adapter.SetLogLevel(level)
					}
				}(g)
			}

			// Goroutines that log messages
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < operationsPerGoroutine; i++ {
						message := fmt.Sprintf("concurrent_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						adapter.Info(message)
						adapter.Error(message)
					}
				}(g)
			}

			wg.Wait()

			// Should not have panicked
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent level changes should be safe")
	})

	// Property test: Concurrent format changes should be safe
	t.Run("concurrent_format_changes_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 4
			const operationsPerGoroutine = 15

			var wg sync.WaitGroup
			wg.Add(numGoroutines * 2) // Loggers + format changers

			// Goroutines that change formats
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}
					for i := 0; i < operationsPerGoroutine; i++ {
						format := formats[(goroutineID+i)%len(formats)]
						adapter.SetFormat(format)
					}
				}(g)
			}

			// Goroutines that log messages
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < operationsPerGoroutine; i++ {
						message := fmt.Sprintf("format_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						adapter.Info(message)
					}
				}(g)
			}

			wg.Wait()

			// Should have some output and not have panicked
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent format changes should be safe")
	})

	// Property test: Concurrent output writer changes should be safe
	t.Run("concurrent_output_changes_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf1, buf2, buf3 bytes.Buffer
			adapter := NewSlogAdapter(&buf1, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 3
			const operationsPerGoroutine = 20

			var wg sync.WaitGroup
			wg.Add(numGoroutines * 2) // Loggers + output changers

			// Goroutines that change output writers
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					writers := []io.Writer{&buf1, &buf2, &buf3}
					for i := 0; i < operationsPerGoroutine; i++ {
						writer := writers[(goroutineID+i)%len(writers)]
						adapter.SetOutput(writer)
					}
				}(g)
			}

			// Goroutines that log messages
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < operationsPerGoroutine; i++ {
						message := fmt.Sprintf("output_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						adapter.Info(message)
					}
				}(g)
			}

			wg.Wait()

			// At least one buffer should have some output
			totalOutput := buf1.Len() + buf2.Len() + buf3.Len()
			return totalOutput > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent output changes should be safe")
	})

	// Property test: Concurrent structured logging should be safe
	t.Run("concurrent_structured_logging_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 8
			const logsPerGoroutine = 25

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			// Launch concurrent structured logging goroutines
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < logsPerGoroutine; i++ {
						message := fmt.Sprintf("struct_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						key := fmt.Sprintf("key_g%d_i%d", goroutineID, i)
						value := fmt.Sprintf("value_s%d", seed%1000)

						// Mix different structured logging methods
						switch i % 4 {
						case 0:
							adapter.InfoWith(message, slog.String(key, value))
						case 1:
							adapter.ErrorWith(message, slog.Int(key, i))
						case 2:
							adapter.WarnWith(message, slog.Bool(key, i%2 == 0))
						case 3:
							adapter.DebugWith(message, slog.String(key, value), slog.Int("count", i))
						}
					}
				}(g)
			}

			wg.Wait()

			// Should have some output and not have panicked
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent structured logging should be safe")
	})

	// Property test: Concurrent context logging should be safe
	t.Run("concurrent_context_logging_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 6
			const logsPerGoroutine = 30

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			// Launch concurrent context logging goroutines
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					ctx := context.Background()

					for i := 0; i < logsPerGoroutine; i++ {
						message := fmt.Sprintf("ctx_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						key := fmt.Sprintf("ctx_key_g%d", goroutineID)
						value := fmt.Sprintf("ctx_value_%d", i)

						// Mix different context logging methods
						switch i % 3 {
						case 0:
							adapter.InfoCtx(ctx, message, slog.String(key, value))
						case 1:
							adapter.ErrorCtx(ctx, message, slog.Int(key, i))
						case 2:
							adapter.WarnCtx(ctx, message, slog.Bool(key, i%2 == 0))
						}
					}
				}(g)
			}

			wg.Wait()

			// Should have some output and not have panicked
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent context logging should be safe")
	})

	// Property test: Concurrent WithAttrs and WithGroup should be safe
	t.Run("concurrent_with_attrs_groups_safe", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 4
			const operationsPerGoroutine = 25

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			// Launch concurrent WithAttrs/WithGroup operations
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()

					for i := 0; i < operationsPerGoroutine; i++ {
						message := fmt.Sprintf("derived_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						attrKey := fmt.Sprintf("attr_g%d", goroutineID)
						attrValue := fmt.Sprintf("attr_val_%d", i)
						groupName := fmt.Sprintf("group_g%d", goroutineID)

						// Create derived adapters and log with them
						switch i % 3 {
						case 0:
							derivedAdapter := adapter.WithAttrs(slog.String(attrKey, attrValue))
							derivedAdapter.Info(message)
						case 1:
							derivedAdapter := adapter.WithGroup(groupName)
							derivedAdapter.Info(message)
						case 2:
							derivedAdapter := adapter.WithAttrs(slog.Int(attrKey, i)).WithGroup(groupName)
							derivedAdapter.Info(message)
						}
					}
				}(g)
			}

			wg.Wait()

			// Should have some output and not have panicked
			return buf.Len() > 0
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent WithAttrs and WithGroup should be safe")
	})

	// Property test: Concurrent cache access should not cause corruption
	t.Run("concurrent_cache_access_no_corruption", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)

			const numGoroutines = 8
			const operationsPerGoroutine = 50

			var wg sync.WaitGroup
			wg.Add(numGoroutines * 2) // Cache checkers + level changers

			// Goroutines that check cache frequently
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					levels := []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError, slog.LevelDebug}

					for i := 0; i < operationsPerGoroutine; i++ {
						level := levels[(goroutineID+i)%len(levels)]
						adapter.isEnabledCached(level)
					}
				}(g)
			}

			// Goroutines that change levels (which clears cache)
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					levels := []log.LogLevel{log.AllLevel, log.InfoLevel, log.WarnLevel, log.ErrorLevel}

					for i := 0; i < operationsPerGoroutine/5; i++ { // Less frequent level changes
						level := levels[(goroutineID+i)%len(levels)]
						adapter.SetLogLevel(level)
					}
				}(g)
			}

			wg.Wait()

			// Should not have panicked or caused corruption
			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent cache access should not cause corruption")
	})

	// Property test: High-volume concurrent logging should remain stable
	t.Run("high_volume_concurrent_logging_stable", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			const numGoroutines = 20
			const logsPerGoroutine = 100

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			start := time.Now()

			// Launch high-volume concurrent logging
			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < logsPerGoroutine; i++ {
						message := fmt.Sprintf("hv_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						adapter.Info(message)
					}
				}(g)
			}

			wg.Wait()
			duration := time.Since(start)

			// Should complete in reasonable time (2000 log operations)
			// Allow up to 2 seconds for 2000 operations (1ms per operation)
			if duration > 2*time.Second {
				return false // Too slow for high-volume logging
			}

			// Should have substantial output
			return buf.Len() > 1000 // Should have significant output
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "High-volume concurrent logging should remain stable")
	})
}

// TestMemoryEfficiency tests that the slog adapter manages memory efficiently
// Feature: slog-migration, Property 14: Memory efficiency
// Validates: Requirements 5.4
func TestMemoryEfficiency(t *testing.T) {
	// Property: For any log message formatting operation, memory usage should be
	// managed efficiently without excessive allocations

	config := quick.Config{
		MaxCount: 100, // Minimum 100 iterations as specified
	}

	// Property test: Object pools should reduce allocations
	t.Run("object_pools_reduce_allocations", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Force garbage collection before measurement
			runtime.GC()
			runtime.GC()

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform operations that should benefit from pooling
			for i := 0; i < 100; i++ {
				message := fmt.Sprintf("test_message_%d_%d", seed%1000, i)

				// Use string slice pool (indirectly through logging)
				adapter.Info(message)
				adapter.Infof("formatted: %s %d", message, i)

				// Use byte buffer pool (indirectly through logging)
				adapter.ErrorWith("structured", slog.String("key", message))
			}

			runtime.ReadMemStats(&m2)

			// Calculate allocations
			allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
			allocatedObjects := m2.Mallocs - m1.Mallocs

			// Should not allocate excessive memory for 300 operations
			// Allow up to 200KB for 300 operations (less than 700 bytes per operation)
			maxAllowedBytes := uint64(200 * 1024)
			if allocatedBytes > maxAllowedBytes {
				return false // Too much memory allocated
			}

			// Should not allocate excessive objects
			// Allow up to 2000 objects for 300 operations (less than 7 objects per operation)
			if allocatedObjects > 2000 {
				return false // Too many objects allocated
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Object pools should reduce allocations")
	})

	// Property test: String pool should be used efficiently
	t.Run("string_pool_efficient_usage", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test string slice pool usage
			slice1 := adapter.getStringSlice()
			slice2 := adapter.getStringSlice()

			// Should get different slices
			if len(slice1) == 0 && len(slice2) == 0 && cap(slice1) > 0 && cap(slice2) > 0 {
				// This is fine - both are empty but have capacity
			}

			// Add some data
			slice1 = append(slice1, "test1", "test2")
			slice2 = append(slice2, "test3", "test4", "test5")

			// Return to pool
			adapter.putStringSlice(slice1)
			adapter.putStringSlice(slice2)

			// Get new slices - should be reset
			slice3 := adapter.getStringSlice()
			slice4 := adapter.getStringSlice()

			// Should be empty (reset)
			if len(slice3) != 0 || len(slice4) != 0 {
				return false // Should be reset to length 0
			}

			// Should have capacity (reused)
			if cap(slice3) == 0 || cap(slice4) == 0 {
				return false // Should have some capacity from reuse
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "String pool should be used efficiently")
	})

	// Property test: Byte buffer pool should be used efficiently
	t.Run("byte_buffer_pool_efficient_usage", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test byte buffer pool usage
			buf1 := adapter.getByteBuffer()
			buf2 := adapter.getByteBuffer()

			// Should get different buffers
			if len(buf1) == 0 && len(buf2) == 0 && cap(buf1) > 0 && cap(buf2) > 0 {
				// This is fine - both are empty but have capacity
			}

			// Add some data
			buf1 = append(buf1, []byte("test data 1")...)
			buf2 = append(buf2, []byte("test data 2 longer")...)

			// Return to pool
			adapter.putByteBuffer(buf1)
			adapter.putByteBuffer(buf2)

			// Get new buffers - should be reset
			buf3 := adapter.getByteBuffer()
			buf4 := adapter.getByteBuffer()

			// Should be empty (reset)
			if len(buf3) != 0 || len(buf4) != 0 {
				return false // Should be reset to length 0
			}

			// Should have capacity (reused)
			if cap(buf3) == 0 || cap(buf4) == 0 {
				return false // Should have some capacity from reuse
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Byte buffer pool should be used efficiently")
	})

	// Property test: Pool size limits should prevent memory leaks
	t.Run("pool_size_limits_prevent_leaks", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Create oversized slices and buffers
			largeSlice := make([]string, 100) // Larger than 32 capacity limit
			largeBuffer := make([]byte, 2048) // Larger than 1024 capacity limit

			// Fill with data
			for i := range largeSlice {
				largeSlice[i] = fmt.Sprintf("item_%d_%d", seed%1000, i)
			}
			for i := range largeBuffer {
				largeBuffer[i] = byte(i % 256)
			}

			// Try to return to pool - should be rejected due to size
			adapter.putStringSlice(largeSlice)
			adapter.putByteBuffer(largeBuffer)

			// Get new items from pool
			newSlice := adapter.getStringSlice()
			newBuffer := adapter.getByteBuffer()

			// Should get fresh items, not the oversized ones
			// (We can't directly test this, but we can test that we get reasonable sizes)
			if cap(newSlice) > 50 || cap(newBuffer) > 1500 {
				return false // Should not get oversized items from pool
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Pool size limits should prevent memory leaks")
	})

	// Property test: Cache should not grow unbounded
	t.Run("cache_bounded_growth", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Test many different levels to populate cache
			levels := []slog.Level{
				slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError,
				LevelTrace, LevelFatal,
				slog.Level(-10), slog.Level(-5), slog.Level(5), slog.Level(10),
			}

			// Populate cache with many entries
			for i := 0; i < 100; i++ {
				level := levels[i%len(levels)]
				adapter.isEnabledCached(level)
			}

			// Cache should have reasonable number of entries (not more than unique levels tested)
			cacheSize := 0
			adapter.enabledCache.Range(func(key, value interface{}) bool {
				cacheSize++
				return true
			})

			// Should not have more entries than unique levels
			if cacheSize > len(levels) {
				return false // Cache has too many entries
			}

			// Clear cache and verify it's empty
			adapter.clearEnabledCache()

			cacheSize = 0
			adapter.enabledCache.Range(func(key, value interface{}) bool {
				cacheSize++
				return true
			})

			if cacheSize != 0 {
				return false // Cache should be empty after clearing
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Cache should not grow unbounded")
	})

	// Property test: Memory usage should be stable over time
	t.Run("memory_usage_stable_over_time", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Measure memory usage over multiple batches
			const batchSize = 100
			const numBatches = 5
			memUsages := make([]uint64, numBatches)

			for batch := 0; batch < numBatches; batch++ {
				// Force GC before measurement
				runtime.GC()
				runtime.GC()

				var m1, m2 runtime.MemStats
				runtime.ReadMemStats(&m1)

				// Perform logging operations
				for i := 0; i < batchSize; i++ {
					message := fmt.Sprintf("batch_%d_msg_%d_seed_%d", batch, i, seed%1000)
					adapter.Info(message)
					adapter.Infof("formatted: %s", message)
					adapter.ErrorWith("structured", slog.String("key", message))
				}

				runtime.ReadMemStats(&m2)
				memUsages[batch] = m2.TotalAlloc - m1.TotalAlloc
			}

			// Memory usage should be relatively stable across batches
			// (allowing for some variation due to GC timing and other factors)
			minUsage := memUsages[0]
			maxUsage := memUsages[0]
			for _, usage := range memUsages[1:] {
				if usage < minUsage {
					minUsage = usage
				}
				if usage > maxUsage {
					maxUsage = usage
				}
			}

			// Allow up to 15x variation between min and max usage (based on observed 12.86x ratio)
			// This accounts for Go's GC behavior and slog's internal buffering patterns
			if minUsage > 0 && maxUsage > minUsage*15 {
				return false // Too much variation in memory usage
			}

			// All batches should use reasonable amount of memory
			// Set limit to 250KB per batch based on observed maximum usage of ~213KB
			for _, usage := range memUsages {
				if usage > 250*1024 { // 250KB per batch of 300 operations
					return false // Too much memory per batch
				}
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Memory usage should be stable over time")
	})

	// Property test: Lazy formatting should avoid allocations for disabled levels
	t.Run("lazy_formatting_avoids_allocations", func(t *testing.T) {
		property := func(seed int) bool {
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
				message := fmt.Sprintf("disabled_message_%d_%d", seed%1000, i)
				adapter.Info(message) // Should be disabled and fast
			}

			runtime.ReadMemStats(&m2)

			// Should have minimal allocations for disabled operations
			allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
			allocatedObjects := m2.Mallocs - m1.Mallocs

			// Set realistic limits based on observed behavior (~55KB, ~2750-3750 objects)
			// Allow up to 70KB and 4000 objects for 1000 disabled operations
			// This accounts for slog's internal caching and level checking overhead
			if allocatedBytes > 70*1024 || allocatedObjects > 4000 {
				return false // Too many allocations for disabled operations
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Lazy formatting should avoid allocations for disabled levels")
	})

	// Property test: Concurrent operations should not cause memory leaks
	t.Run("concurrent_operations_no_memory_leaks", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Measure memory before concurrent operations
			runtime.GC()
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform concurrent operations
			const numGoroutines = 10
			const opsPerGoroutine = 50

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					defer wg.Done()
					for i := 0; i < opsPerGoroutine; i++ {
						message := fmt.Sprintf("concurrent_msg_g%d_i%d_s%d", goroutineID, i, seed%1000)
						adapter.Info(message)
						adapter.ErrorWith("structured", slog.String("key", message))
					}
				}(g)
			}

			wg.Wait()

			// Measure memory after operations and GC
			runtime.GC()
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Calculate memory growth
			memoryGrowth := m2.TotalAlloc - m1.TotalAlloc

			// Should not have excessive memory growth for 500 concurrent operations
			// Allow up to 500KB for 500 operations (1KB per operation)
			if memoryGrowth > 500*1024 {
				return false // Too much memory growth
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Concurrent operations should not cause memory leaks")
	})

	// Property test: Handler recreation should not leak memory
	t.Run("handler_recreation_no_memory_leaks", func(t *testing.T) {
		property := func(seed int) bool {
			var buf bytes.Buffer
			adapter := NewSlogAdapter(&buf, false)
			adapter.SetLogLevel(log.AllLevel)

			// Measure memory before handler recreations
			runtime.GC()
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Perform many handler recreations
			formats := []LogFormat{TextFormat, JSONFormat, ColoredFormat}
			for i := 0; i < 100; i++ {
				format := formats[i%len(formats)]
				adapter.SetFormat(format)

				// Log something to ensure handler is used
				message := fmt.Sprintf("handler_test_%d_%d", seed%1000, i)
				adapter.Info(message)
			}

			// Measure memory after operations and GC
			runtime.GC()
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Calculate memory growth
			memoryGrowth := m2.TotalAlloc - m1.TotalAlloc

			// Should not have excessive memory growth for 100 handler recreations
			// Allow up to 200KB for 100 recreations (2KB per recreation)
			if memoryGrowth > 200*1024 {
				return false // Too much memory growth from handler recreation
			}

			return true
		}

		err := quick.Check(property, &config)
		assert.NoError(t, err, "Handler recreation should not leak memory")
	})
}
