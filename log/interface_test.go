package log

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

// TestNewInterfaceMethods tests that the new structured logging methods are properly exposed
func TestNewInterfaceMethods(t *testing.T) {
	// Test that global functions exist and can be called
	t.Run("GlobalFunctions", func(t *testing.T) {
		// These should not panic
		InfoWith("test", slog.String("key", "value"))
		ErrorWith("test", slog.String("key", "value"))
		WarnWith("test", slog.String("key", "value"))
		DebugWith("test", slog.String("key", "value"))
		TraceWith("test", slog.String("key", "value"))

		ctx := context.Background()
		InfoCtx(ctx, "test", slog.String("key", "value"))
		ErrorCtx(ctx, "test", slog.String("key", "value"))
		WarnCtx(ctx, "test", slog.String("key", "value"))
		DebugCtx(ctx, "test", slog.String("key", "value"))
		TraceCtx(ctx, "test", slog.String("key", "value"))

		// Test attribute grouping
		logger := WithAttrs(slog.String("service", "test"))
		if logger == nil {
			t.Error("WithAttrs returned nil")
		}

		groupLogger := WithGroup("test")
		if groupLogger == nil {
			t.Error("WithGroup returned nil")
		}
	})
}

// TestEmptyLoggerImplementsNewMethods tests that EmptyLogger implements all new methods correctly
func TestEmptyLoggerImplementsNewMethods(t *testing.T) {
	logger := &EmptyLogger{}

	t.Run("StructuredMethods", func(t *testing.T) {
		// These should not panic
		logger.InfoWith("test", slog.String("key", "value"))
		logger.ErrorWith("test", slog.String("key", "value"))
		logger.WarnWith("test", slog.String("key", "value"))
		logger.DebugWith("test", slog.String("key", "value"))
		logger.TraceWith("test", slog.String("key", "value"))
	})

	t.Run("ContextMethods", func(t *testing.T) {
		ctx := context.Background()
		// These should not panic
		logger.InfoCtx(ctx, "test", slog.String("key", "value"))
		logger.ErrorCtx(ctx, "test", slog.String("key", "value"))
		logger.WarnCtx(ctx, "test", slog.String("key", "value"))
		logger.DebugCtx(ctx, "test", slog.String("key", "value"))
		logger.TraceCtx(ctx, "test", slog.String("key", "value"))
	})

	t.Run("AttributeMethods", func(t *testing.T) {
		// Test WithAttrs returns the same logger (no-op behavior)
		result := logger.WithAttrs(slog.String("key", "value"))
		if result != logger {
			t.Error("EmptyLogger.WithAttrs should return self")
		}

		// Test WithGroup returns the same logger (no-op behavior)
		result = logger.WithGroup("test")
		if result != logger {
			t.Error("EmptyLogger.WithGroup should return self")
		}
	})
}

// TestLoggerInterfaceCompliance tests that the Logger interface includes all expected methods
func TestLoggerInterfaceCompliance(t *testing.T) {
	var logger Logger = &EmptyLogger{}

	// Test that all methods exist on the interface
	// This is a compile-time test - if any method is missing, this won't compile

	// Original methods
	logger.Info("test")
	logger.Infof("test %s", "value")
	logger.Error("test")
	logger.Errorf("test %s", "value")
	logger.Warn("test")
	logger.Warnf("test %s", "value")
	logger.Debug("test")
	logger.Debugf("test %s", "value")
	logger.Trace("test")
	logger.Tracef("test %s", "value")
	logger.SetLogLevel(InfoLevel)
	logger.SetOutput(&bytes.Buffer{})

	// New structured methods
	logger.InfoWith("test", slog.String("key", "value"))
	logger.ErrorWith("test", slog.String("key", "value"))
	logger.WarnWith("test", slog.String("key", "value"))
	logger.DebugWith("test", slog.String("key", "value"))
	logger.TraceWith("test", slog.String("key", "value"))

	// New context methods
	ctx := context.Background()
	logger.InfoCtx(ctx, "test", slog.String("key", "value"))
	logger.ErrorCtx(ctx, "test", slog.String("key", "value"))
	logger.WarnCtx(ctx, "test", slog.String("key", "value"))
	logger.DebugCtx(ctx, "test", slog.String("key", "value"))
	logger.TraceCtx(ctx, "test", slog.String("key", "value"))

	// New attribute methods
	newLogger := logger.WithAttrs(slog.String("key", "value"))
	if newLogger == nil {
		t.Error("WithAttrs returned nil")
	}

	groupLogger := logger.WithGroup("test")
	if groupLogger == nil {
		t.Error("WithGroup returned nil")
	}
}
