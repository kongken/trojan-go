package log

import (
	"bytes"
	"testing"
)

// TestSystemIntegration tests the core log package functionality
func TestSystemIntegration(t *testing.T) {
	// Test that we have a logger registered (not nil)
	if logger == nil {
		t.Fatal("Logger should not be nil")
	}

	// Test the registration mechanism
	originalLogger := logger

	// Test registering a custom logger
	customLogger := &EmptyLogger{}
	RegisterLogger(customLogger)

	if logger != customLogger {
		t.Error("Expected custom logger to be registered")
	}

	// Restore original logger
	RegisterLogger(originalLogger)

	if logger != originalLogger {
		t.Error("Expected original logger to be restored")
	}
}

// TestLogLevelConstants tests that log level constants are properly defined
func TestLogLevelConstants(t *testing.T) {
	// Test that all log levels are defined with expected values
	expectedLevels := map[LogLevel]string{
		AllLevel:   "AllLevel",
		InfoLevel:  "InfoLevel",
		WarnLevel:  "WarnLevel",
		ErrorLevel: "ErrorLevel",
		FatalLevel: "FatalLevel",
		OffLevel:   "OffLevel",
	}

	for level, name := range expectedLevels {
		if int(level) < 0 || int(level) > 5 {
			t.Errorf("Log level %s has invalid value: %d", name, int(level))
		}
	}

	// Test that levels are in ascending order
	if AllLevel >= InfoLevel || InfoLevel >= WarnLevel || WarnLevel >= ErrorLevel || ErrorLevel >= FatalLevel || FatalLevel >= OffLevel {
		t.Error("Log levels are not in expected ascending order")
	}
}

// TestRegistrationProcess tests the logger registration mechanism
func TestRegistrationProcess(t *testing.T) {
	// Save current logger
	originalLogger := logger

	// Test registering a custom logger
	customLogger := &EmptyLogger{}
	RegisterLogger(customLogger)

	if logger != customLogger {
		t.Error("Expected custom logger to be registered")
	}

	// Test that global functions use the registered logger
	var buf bytes.Buffer
	SetOutput(&buf)

	// EmptyLogger should not produce output
	Info("test message")
	output := buf.String()
	if len(output) > 0 {
		t.Errorf("Expected no output from EmptyLogger, but got: %s", output)
	}

	// Restore original logger
	RegisterLogger(originalLogger)
}

// TestBackwardCompatibility tests that existing code patterns continue working
func TestBackwardCompatibility(t *testing.T) {
	// Save original logger
	originalLogger := logger
	defer RegisterLogger(originalLogger)

	// Use EmptyLogger for testing to avoid circular imports
	testLogger := &EmptyLogger{}
	RegisterLogger(testLogger)

	// Test all the global logging functions that existing code uses
	testCases := []struct {
		name string
		fn   func()
	}{
		{"Info", func() { Info("test") }},
		{"Infof", func() { Infof("test %s", "formatted") }},
		{"Error", func() { Error("test") }},
		{"Errorf", func() { Errorf("test %s", "formatted") }},
		{"Warn", func() { Warn("test") }},
		{"Warnf", func() { Warnf("test %s", "formatted") }},
		{"Debug", func() { Debug("test") }},
		{"Debugf", func() { Debugf("test %s", "formatted") }},
		{"Trace", func() { Trace("test") }},
		{"Tracef", func() { Tracef("test %s", "formatted") }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function %s panicked: %v", tc.name, r)
				}
			}()

			tc.fn()
			// EmptyLogger doesn't produce output, so we just test that it doesn't panic
		})
	}
}

// TestOutputWriterIntegration tests that SetOutput works correctly
func TestOutputWriterIntegration(t *testing.T) {
	// Save original logger
	originalLogger := logger
	defer RegisterLogger(originalLogger)

	// Use EmptyLogger for testing
	testLogger := &EmptyLogger{}
	RegisterLogger(testLogger)

	// Test with different output writers
	var buf1, buf2 bytes.Buffer

	// Set first writer - EmptyLogger doesn't produce output but should not panic
	SetOutput(&buf1)
	Info("message1")

	// Switch to second writer - should also not panic
	SetOutput(&buf2)
	Info("message2")

	// EmptyLogger doesn't produce output, so we just verify no panics occurred
	// The real output testing is done in the integration tests
}

// TestEmptyLoggerBehavior tests the EmptyLogger implementation
func TestEmptyLoggerBehavior(t *testing.T) {
	emptyLogger := &EmptyLogger{}

	// Test that EmptyLogger methods don't panic
	emptyLogger.Info("test")
	emptyLogger.Infof("test %s", "formatted")
	emptyLogger.Error("test")
	emptyLogger.Errorf("test %s", "formatted")
	emptyLogger.Warn("test")
	emptyLogger.Warnf("test %s", "formatted")
	emptyLogger.Debug("test")
	emptyLogger.Debugf("test %s", "formatted")
	emptyLogger.Trace("test")
	emptyLogger.Tracef("test %s", "formatted")

	// Test SetLogLevel and SetOutput don't panic
	emptyLogger.SetLogLevel(InfoLevel)

	var buf bytes.Buffer
	emptyLogger.SetOutput(&buf)

	// EmptyLogger should not produce any output
	emptyLogger.Info("test message")
	if buf.Len() > 0 {
		t.Error("EmptyLogger should not produce output")
	}
}
