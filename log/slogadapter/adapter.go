package slogadapter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/p4gefau1t/trojan-go/log"
)

// LogFormat represents the output format for log messages
type LogFormat int

const (
	// TextFormat outputs plain text logs
	TextFormat LogFormat = iota
	// JSONFormat outputs structured JSON logs
	JSONFormat
	// ColoredFormat outputs colored text logs for terminals
	ColoredFormat
)

// ErrorHandler interface for handling internal logging errors
type ErrorHandler interface {
	HandleError(err error, context string)
}

// DefaultErrorHandler implements ErrorHandler by logging to stderr
type DefaultErrorHandler struct{}

// HandleError logs errors to stderr without panicking
func (h *DefaultErrorHandler) HandleError(err error, context string) {
	// Never panic, always try to report the error
	if err != nil {
		fmt.Fprintf(os.Stderr, "slog adapter error [%s]: %v\n", context, err)
	}
}

// FallbackWriter wraps an io.Writer with error handling and fallback mechanisms
type FallbackWriter struct {
	primary      io.Writer
	fallback     io.Writer
	errorHandler ErrorHandler
	mu           sync.RWMutex
}

// NewFallbackWriter creates a new FallbackWriter with primary and fallback writers
func NewFallbackWriter(primary, fallback io.Writer, errorHandler ErrorHandler) *FallbackWriter {
	if errorHandler == nil {
		errorHandler = &DefaultErrorHandler{}
	}
	if fallback == nil {
		fallback = os.Stderr
	}
	return &FallbackWriter{
		primary:      primary,
		fallback:     fallback,
		errorHandler: errorHandler,
	}
}

// Write implements io.Writer with fallback error handling
func (fw *FallbackWriter) Write(p []byte) (n int, err error) {
	fw.mu.RLock()
	primary := fw.primary
	fallback := fw.fallback
	errorHandler := fw.errorHandler
	fw.mu.RUnlock()

	// Try primary writer first
	if primary != nil {
		n, err = primary.Write(p)
		if err == nil {
			return n, nil
		}

		// Report primary write error
		errorHandler.HandleError(err, "primary writer failed")
	}

	// Try fallback writer
	if fallback != nil && fallback != primary {
		n, err = fallback.Write(p)
		if err == nil {
			return n, nil
		}

		// Report fallback write error
		errorHandler.HandleError(err, "fallback writer failed")
	}

	// If both fail, discard the data to prevent blocking
	// This ensures the application continues running
	return len(p), nil
}

// SetPrimary updates the primary writer
func (fw *FallbackWriter) SetPrimary(w io.Writer) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.primary = w
}

// SetFallback updates the fallback writer
func (fw *FallbackWriter) SetFallback(w io.Writer) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.fallback = w
}

// String returns the string representation of the log format
func (f LogFormat) String() string {
	switch f {
	case TextFormat:
		return "text"
	case JSONFormat:
		return "json"
	case ColoredFormat:
		return "colored"
	default:
		return "unknown"
	}
}

// ParseLogFormat parses a string into a LogFormat
func ParseLogFormat(s string) LogFormat {
	switch s {
	case "text":
		return TextFormat
	case "json":
		return JSONFormat
	case "colored":
		return ColoredFormat
	default:
		return TextFormat // Default to text format
	}
}

// SlogAdapter implements the Logger interface using Go's standard slog package
type SlogAdapter struct {
	logger         *slog.Logger
	levelVar       *slog.LevelVar
	handler        slog.Handler
	output         io.Writer
	fallbackWriter *FallbackWriter
	errorHandler   ErrorHandler
	logLevel       int32        // atomic access for thread safety
	format         int32        // atomic access for thread safety (LogFormat)
	mu             sync.RWMutex // protects handler recreation

	// Performance optimization fields
	enabledCache sync.Map     // Cache for level enabled checks
	cacheMu      sync.RWMutex // protects cache clearing operations
	stringPool   sync.Pool    // Pool for string builders to reduce allocations
	bufferPool   sync.Pool    // Pool for byte buffers to reduce allocations
}

// NewSlogAdapter creates a new SlogAdapter with the specified configuration
func NewSlogAdapter(output io.Writer, useColor bool) *SlogAdapter {
	format := TextFormat
	if useColor {
		format = ColoredFormat
	}
	return NewSlogAdapterWithFormat(output, format)
}

// NewSlogAdapterWithFormat creates a new SlogAdapter with the specified format
func NewSlogAdapterWithFormat(output io.Writer, format LogFormat) *SlogAdapter {
	levelVar := &slog.LevelVar{}
	levelVar.Set(slog.LevelInfo) // Default to INFO level

	errorHandler := &DefaultErrorHandler{}
	fallbackWriter := NewFallbackWriter(output, os.Stderr, errorHandler)

	adapter := &SlogAdapter{
		levelVar:       levelVar,
		output:         output,
		fallbackWriter: fallbackWriter,
		errorHandler:   errorHandler,
		logLevel:       int32(log.InfoLevel),
		format:         int32(format),
	}

	// Initialize performance optimization pools
	adapter.initializePools()

	// Create initial handler
	adapter.recreateHandler()

	return adapter
}

// NewSlogAdapterWithErrorHandler creates a new SlogAdapter with custom error handling
func NewSlogAdapterWithErrorHandler(output io.Writer, format LogFormat, errorHandler ErrorHandler) *SlogAdapter {
	levelVar := &slog.LevelVar{}
	levelVar.Set(slog.LevelInfo) // Default to INFO level

	if errorHandler == nil {
		errorHandler = &DefaultErrorHandler{}
	}

	fallbackWriter := NewFallbackWriter(output, os.Stderr, errorHandler)

	adapter := &SlogAdapter{
		levelVar:       levelVar,
		output:         output,
		fallbackWriter: fallbackWriter,
		errorHandler:   errorHandler,
		logLevel:       int32(log.InfoLevel),
		format:         int32(format),
	}

	// Initialize performance optimization pools
	adapter.initializePools()

	// Create initial handler
	adapter.recreateHandler()

	return adapter
}

// recreateHandler creates a new handler based on current format and output settings
func (s *SlogAdapter) recreateHandler() {
	format := LogFormat(atomic.LoadInt32(&s.format))

	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level: s.levelVar,
	}

	// Use fallback writer for resilient output
	writer := s.fallbackWriter
	if writer == nil {
		// Fallback to direct output if fallback writer is not available
		writer = NewFallbackWriter(s.output, os.Stderr, s.errorHandler)
		s.fallbackWriter = writer
	}

	switch format {
	case JSONFormat:
		handler = slog.NewJSONHandler(writer, handlerOpts)
	case ColoredFormat:
		handler = NewColoredTextHandler(writer, handlerOpts)
	case TextFormat:
		fallthrough
	default:
		handler = slog.NewTextHandler(writer, handlerOpts)
	}

	s.handler = handler
	s.logger = slog.New(handler)

	// Clear enabled cache when handler changes
	s.clearEnabledCache()
}

// Performance optimization methods

// initializePools initializes the object pools for performance optimization
func (s *SlogAdapter) initializePools() {
	s.stringPool = sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 8) // Pre-allocate capacity for 8 strings
		},
	}

	s.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 256) // Pre-allocate 256 bytes
		},
	}
}

// getStringSlice gets a string slice from the pool
func (s *SlogAdapter) getStringSlice() []string {
	return s.stringPool.Get().([]string)[:0] // Reset length but keep capacity
}

// putStringSlice returns a string slice to the pool
func (s *SlogAdapter) putStringSlice(slice []string) {
	if cap(slice) <= 32 { // Only pool reasonably sized slices
		s.stringPool.Put(slice)
	}
}

// getByteBuffer gets a byte buffer from the pool
func (s *SlogAdapter) getByteBuffer() []byte {
	return s.bufferPool.Get().([]byte)[:0] // Reset length but keep capacity
}

// putByteBuffer returns a byte buffer to the pool
func (s *SlogAdapter) putByteBuffer(buf []byte) {
	if cap(buf) <= 1024 { // Only pool reasonably sized buffers
		s.bufferPool.Put(buf)
	}
}

// isEnabledCached checks if a level is enabled with caching for performance
func (s *SlogAdapter) isEnabledCached(level slog.Level) bool {
	// Use level as cache key
	s.cacheMu.RLock()
	if cached, ok := s.enabledCache.Load(level); ok {
		s.cacheMu.RUnlock()
		return cached.(bool)
	}
	s.cacheMu.RUnlock()

	// Not in cache, check and cache the result
	enabled := s.handler.Enabled(context.Background(), level)

	s.cacheMu.RLock()
	s.enabledCache.Store(level, enabled)
	s.cacheMu.RUnlock()

	return enabled
}

// clearEnabledCache safely clears the enabled cache
func (s *SlogAdapter) clearEnabledCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// Clear all entries from the cache
	s.enabledCache.Range(func(key, value interface{}) bool {
		s.enabledCache.Delete(key)
		return true
	})
}

// lazyFormat performs lazy formatting only if logging is enabled
func (s *SlogAdapter) lazyFormat(level slog.Level, format string, args []interface{}) string {
	if !s.isEnabledCached(level) {
		return "" // Don't format if logging is disabled
	}

	if format == "" {
		// Use fmt.Sprint for variadic arguments
		return fmt.Sprint(args...)
	}

	// Use fmt.Sprintf for formatted arguments
	return fmt.Sprintf(format, args...)
}

// SetLogLevel sets the logging level
func (s *SlogAdapter) SetLogLevel(level log.LogLevel) {
	atomic.StoreInt32(&s.logLevel, int32(level))
	s.levelVar.Set(mapLogLevelToSlog(level))

	// Clear enabled cache when level changes
	s.clearEnabledCache()
}

// SetFormat sets the log output format and recreates the handler
func (s *SlogAdapter) SetFormat(format LogFormat) {
	s.mu.Lock()
	defer s.mu.Unlock()

	atomic.StoreInt32(&s.format, int32(format))
	s.recreateHandler()
}

// GetFormat returns the current log format
func (s *SlogAdapter) GetFormat() LogFormat {
	return LogFormat(atomic.LoadInt32(&s.format))
}

// SetOutput sets the output writer
func (s *SlogAdapter) SetOutput(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.output = w

	// Update the fallback writer's primary writer
	if s.fallbackWriter != nil {
		s.fallbackWriter.SetPrimary(w)
	} else {
		s.fallbackWriter = NewFallbackWriter(w, os.Stderr, s.errorHandler)
	}

	s.recreateHandler()
}

// SetErrorHandler sets the error handler for internal logging errors
func (s *SlogAdapter) SetErrorHandler(handler ErrorHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if handler == nil {
		handler = &DefaultErrorHandler{}
	}

	s.errorHandler = handler

	// Update fallback writer with new error handler
	if s.fallbackWriter != nil {
		s.fallbackWriter = NewFallbackWriter(s.output, os.Stderr, handler)
		s.recreateHandler()
	}
}

// SetFallbackWriter sets the fallback writer for when primary writer fails
func (s *SlogAdapter) SetFallbackWriter(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fallbackWriter != nil {
		s.fallbackWriter.SetFallback(w)
	} else {
		s.fallbackWriter = NewFallbackWriter(s.output, w, s.errorHandler)
		s.recreateHandler()
	}
}

// Fatal logs a fatal message and exits the program
func (s *SlogAdapter) Fatal(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(LevelFatal) {
		os.Exit(1) // Still exit even if logging is disabled
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(LevelFatal, "", v)
	if message != "" {
		logger.Log(context.Background(), LevelFatal, message)
	}
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits the program
func (s *SlogAdapter) Fatalf(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(LevelFatal) {
		os.Exit(1) // Still exit even if logging is disabled
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(LevelFatal, format, v)
	if message != "" {
		logger.Log(context.Background(), LevelFatal, message)
	}
	os.Exit(1)
}

// Error logs an error message
func (s *SlogAdapter) Error(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelError) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(slog.LevelError, "", v)
	if message != "" {
		logger.Error(message)
	}
}

// Errorf logs a formatted error message
func (s *SlogAdapter) Errorf(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelError) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(slog.LevelError, format, v)
	if message != "" {
		logger.Error(message)
	}
}

// Warn logs a warning message
func (s *SlogAdapter) Warn(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelWarn) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(slog.LevelWarn, "", v)
	if message != "" {
		logger.Warn(message)
	}
}

// Warnf logs a formatted warning message
func (s *SlogAdapter) Warnf(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelWarn) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(slog.LevelWarn, format, v)
	if message != "" {
		logger.Warn(message)
	}
}

// Info logs an informational message
func (s *SlogAdapter) Info(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelInfo) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(slog.LevelInfo, "", v)
	if message != "" {
		logger.Info(message)
	}
}

// Infof logs a formatted informational message
func (s *SlogAdapter) Infof(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelInfo) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(slog.LevelInfo, format, v)
	if message != "" {
		logger.Info(message)
	}
}

// Debug logs a debug message
func (s *SlogAdapter) Debug(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelDebug) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(slog.LevelDebug, "", v)
	if message != "" {
		logger.Debug(message)
	}
}

// Debugf logs a formatted debug message
func (s *SlogAdapter) Debugf(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(slog.LevelDebug) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(slog.LevelDebug, format, v)
	if message != "" {
		logger.Debug(message)
	}
}

// Trace logs a trace message
func (s *SlogAdapter) Trace(v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(LevelTrace) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprint when disabled
	message := s.lazyFormat(LevelTrace, "", v)
	if message != "" {
		logger.Log(context.Background(), LevelTrace, message)
	}
}

// Tracef logs a formatted trace message
func (s *SlogAdapter) Tracef(format string, v ...interface{}) {
	// Check level first to avoid expensive operations if disabled
	if !s.isEnabledCached(LevelTrace) {
		return
	}

	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()

	// Use lazy formatting to avoid expensive Sprintf when disabled
	message := s.lazyFormat(LevelTrace, format, v)
	if message != "" {
		logger.Log(context.Background(), LevelTrace, message)
	}
}

// Structured logging methods for key-value pair logging

// InfoWith logs an informational message with structured attributes
func (s *SlogAdapter) InfoWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// ErrorWith logs an error message with structured attributes
func (s *SlogAdapter) ErrorWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

// WarnWith logs a warning message with structured attributes
func (s *SlogAdapter) WarnWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
}

// DebugWith logs a debug message with structured attributes
func (s *SlogAdapter) DebugWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), slog.LevelDebug, msg, attrs...)
}

// TraceWith logs a trace message with structured attributes
func (s *SlogAdapter) TraceWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), LevelTrace, msg, attrs...)
}

// FatalWith logs a fatal message with structured attributes and exits
func (s *SlogAdapter) FatalWith(msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(context.Background(), LevelFatal, msg, attrs...)
	os.Exit(1)
}

// Context-aware logging methods

// InfoCtx logs an informational message with context
func (s *SlogAdapter) InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

// ErrorCtx logs an error message with context
func (s *SlogAdapter) ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

// WarnCtx logs a warning message with context
func (s *SlogAdapter) WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

// DebugCtx logs a debug message with context
func (s *SlogAdapter) DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

// TraceCtx logs a trace message with context
func (s *SlogAdapter) TraceCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, LevelTrace, msg, attrs...)
}

// FatalCtx logs a fatal message with context and exits
func (s *SlogAdapter) FatalCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	logger.LogAttrs(ctx, LevelFatal, msg, attrs...)
	os.Exit(1)
}

// WithAttrs returns a new SlogAdapter with the given attributes added to all log records
func (s *SlogAdapter) WithAttrs(attrs ...slog.Attr) *SlogAdapter {
	s.mu.RLock()
	newHandler := s.handler.WithAttrs(attrs)
	s.mu.RUnlock()

	newLogger := slog.New(newHandler)

	return &SlogAdapter{
		logger:         newLogger,
		levelVar:       s.levelVar,
		handler:        newHandler,
		output:         s.output,
		fallbackWriter: s.fallbackWriter,
		errorHandler:   s.errorHandler,
		logLevel:       s.logLevel,
		format:         s.format,
	}
}

// WithGroup returns a new SlogAdapter with the given group name added to all log records
func (s *SlogAdapter) WithGroup(name string) *SlogAdapter {
	s.mu.RLock()
	newHandler := s.handler.WithGroup(name)
	s.mu.RUnlock()

	newLogger := slog.New(newHandler)

	return &SlogAdapter{
		logger:         newLogger,
		levelVar:       s.levelVar,
		handler:        newHandler,
		output:         s.output,
		fallbackWriter: s.fallbackWriter,
		errorHandler:   s.errorHandler,
		logLevel:       s.logLevel,
		format:         s.format,
	}
}

// Helper methods for creating common attributes

// String creates a string attribute
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int creates an int attribute
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Int64 creates an int64 attribute
func Int64(key string, value int64) slog.Attr {
	return slog.Int64(key, value)
}

// Float64 creates a float64 attribute
func Float64(key string, value float64) slog.Attr {
	return slog.Float64(key, value)
}

// Bool creates a bool attribute
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Any creates an attribute with any value
func Any(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

// Group creates a group attribute
func Group(key string, attrs ...slog.Attr) slog.Attr {
	// Convert slog.Attr to any for slog.Group
	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}
	return slog.Group(key, anyAttrs...)
}
