package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/p4gefau1t/trojan-go/log"
	_ "github.com/p4gefau1t/trojan-go/log/slogadapter" // Import to trigger init
)

// TestSlogAdapterIntegration tests that the SlogAdapter is properly registered as the default logger
func TestSlogAdapterIntegration(t *testing.T) {
	// Test that we can call global log functions without panics
	var buf bytes.Buffer
	log.SetOutput(&buf)

	// Test basic logging functions
	log.Info("test info message")
	log.Error("test error message")
	log.Warn("test warn message")
	log.Debug("test debug message")

	// Verify that something was written to the buffer
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected log output, but got empty buffer")
	}

	// Test formatted logging functions
	buf.Reset()
	log.Infof("test info %s", "formatted")
	log.Errorf("test error %d", 123)
	log.Warnf("test warn %v", true)
	log.Debugf("test debug %f", 3.14)

	// Verify formatted output
	output = buf.String()
	if len(output) == 0 {
		t.Error("Expected formatted log output, but got empty buffer")
	}
}

// TestLogLevelIntegration tests that log level changes work correctly
func TestLogLevelIntegration(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)

	// Set to ERROR level - should only show ERROR and FATAL
	log.SetLogLevel(log.ErrorLevel)

	// These should not appear in output
	buf.Reset()
	log.Info("info message")
	log.Warn("warn message")
	log.Debug("debug message")

	output := buf.String()
	if len(output) > 0 {
		t.Errorf("Expected no output for INFO/WARN/DEBUG at ERROR level, but got: %s", output)
	}

	// This should appear in output
	buf.Reset()
	log.Error("error message")

	output = buf.String()
	if len(output) == 0 {
		t.Error("Expected error message to appear at ERROR level")
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected 'error message' in output, but got: %s", output)
	}

	// Reset to INFO level for other tests
	log.SetLogLevel(log.InfoLevel)
}

// TestBackwardCompatibility tests that existing code patterns continue working
func TestBackwardCompatibility(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetLogLevel(log.InfoLevel)

	// Test all the global logging functions that existing code uses
	testCases := []struct {
		name string
		fn   func()
	}{
		{"Info", func() { log.Info("test") }},
		{"Infof", func() { log.Infof("test %s", "formatted") }},
		{"Error", func() { log.Error("test") }},
		{"Errorf", func() { log.Errorf("test %s", "formatted") }},
		{"Warn", func() { log.Warn("test") }},
		{"Warnf", func() { log.Warnf("test %s", "formatted") }},
		{"Debug", func() { log.Debug("test") }},
		{"Debugf", func() { log.Debugf("test %s", "formatted") }},
		{"Trace", func() { log.Trace("test") }},
		{"Tracef", func() { log.Tracef("test %s", "formatted") }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function %s panicked: %v", tc.name, r)
				}
			}()

			tc.fn()

			// Should produce some output (except for Trace/Debug which might be filtered)
			output := buf.String()
			if !strings.Contains(tc.name, "Trace") && !strings.Contains(tc.name, "Debug") {
				if len(output) == 0 {
					t.Errorf("Expected output from %s, but got empty buffer", tc.name)
				}
			}
		})
	}
}

// TestOutputWriterIntegration tests that SetOutput works correctly
func TestOutputWriterIntegration(t *testing.T) {
	// Test with different output writers
	var buf1, buf2 bytes.Buffer

	// Set first writer
	log.SetOutput(&buf1)
	log.Info("message1")

	if buf1.Len() == 0 {
		t.Error("Expected output in first buffer")
	}
	if buf2.Len() > 0 {
		t.Error("Expected no output in second buffer")
	}

	// Switch to second writer
	log.SetOutput(&buf2)
	log.Info("message2")

	// buf1 should not have new content, buf2 should have content
	buf1Content := buf1.String()
	buf2Content := buf2.String()

	if !strings.Contains(buf1Content, "message1") {
		t.Error("Expected message1 in first buffer")
	}
	if strings.Contains(buf1Content, "message2") {
		t.Error("Did not expect message2 in first buffer")
	}
	if !strings.Contains(buf2Content, "message2") {
		t.Error("Expected message2 in second buffer")
	}
}
