package log

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// LogLevel how much log to dump
// 0: ALL; 1: INFO; 2: WARN; 3: ERROR; 4: FATAL; 5: OFF
type LogLevel int

const (
	AllLevel   LogLevel = 0
	InfoLevel  LogLevel = 1
	WarnLevel  LogLevel = 2
	ErrorLevel LogLevel = 3
	FatalLevel LogLevel = 4
	OffLevel   LogLevel = 5
)

type Logger interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Trace(v ...interface{})
	Tracef(format string, v ...interface{})
	SetLogLevel(level LogLevel)
	SetOutput(io.Writer)

	// Structured logging methods
	InfoWith(msg string, attrs ...slog.Attr)
	ErrorWith(msg string, attrs ...slog.Attr)
	WarnWith(msg string, attrs ...slog.Attr)
	DebugWith(msg string, attrs ...slog.Attr)
	TraceWith(msg string, attrs ...slog.Attr)
	FatalWith(msg string, attrs ...slog.Attr)

	// Context-aware logging methods
	InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr)
	ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr)
	WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr)
	DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr)
	TraceCtx(ctx context.Context, msg string, attrs ...slog.Attr)
	FatalCtx(ctx context.Context, msg string, attrs ...slog.Attr)

	// Attribute grouping methods
	WithAttrs(attrs ...slog.Attr) Logger
	WithGroup(name string) Logger
}

var logger Logger = &EmptyLogger{}

type EmptyLogger struct{}

func (l *EmptyLogger) SetLogLevel(LogLevel) {}

func (l *EmptyLogger) Fatal(v ...interface{}) { os.Exit(1) }

func (l *EmptyLogger) Fatalf(format string, v ...interface{}) { os.Exit(1) }

func (l *EmptyLogger) Error(v ...interface{}) {}

func (l *EmptyLogger) Errorf(format string, v ...interface{}) {}

func (l *EmptyLogger) Warn(v ...interface{}) {}

func (l *EmptyLogger) Warnf(format string, v ...interface{}) {}

func (l *EmptyLogger) Info(v ...interface{}) {}

func (l *EmptyLogger) Infof(format string, v ...interface{}) {}

func (l *EmptyLogger) Debug(v ...interface{}) {}

func (l *EmptyLogger) Debugf(format string, v ...interface{}) {}

func (l *EmptyLogger) Trace(v ...interface{}) {}

func (l *EmptyLogger) Tracef(format string, v ...interface{}) {}

func (l *EmptyLogger) SetOutput(w io.Writer) {}

// Structured logging methods
func (l *EmptyLogger) InfoWith(msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) ErrorWith(msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) WarnWith(msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) DebugWith(msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) TraceWith(msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) FatalWith(msg string, attrs ...slog.Attr) { os.Exit(1) }

// Context-aware logging methods
func (l *EmptyLogger) InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) TraceCtx(ctx context.Context, msg string, attrs ...slog.Attr) {}

func (l *EmptyLogger) FatalCtx(ctx context.Context, msg string, attrs ...slog.Attr) { os.Exit(1) }

// Attribute grouping methods
func (l *EmptyLogger) WithAttrs(attrs ...slog.Attr) Logger { return l }

func (l *EmptyLogger) WithGroup(name string) Logger { return l }

func Error(v ...interface{}) {
	logger.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

func Warn(v ...interface{}) {
	logger.Warn(v...)
}

func Warnf(format string, v ...interface{}) {
	logger.Warnf(format, v...)
}

func Info(v ...interface{}) {
	logger.Info(v...)
}

func Infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

func Debug(v ...interface{}) {
	logger.Debug(v...)
}

func Debugf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

func Trace(v ...interface{}) {
	logger.Trace(v...)
}

func Tracef(format string, v ...interface{}) {
	logger.Tracef(format, v...)
}

func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func SetLogLevel(level LogLevel) {
	logger.SetLogLevel(level)
}

func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

func RegisterLogger(l Logger) {
	logger = l
}

// Structured logging global functions
func InfoWith(msg string, attrs ...slog.Attr) {
	logger.InfoWith(msg, attrs...)
}

func ErrorWith(msg string, attrs ...slog.Attr) {
	logger.ErrorWith(msg, attrs...)
}

func WarnWith(msg string, attrs ...slog.Attr) {
	logger.WarnWith(msg, attrs...)
}

func DebugWith(msg string, attrs ...slog.Attr) {
	logger.DebugWith(msg, attrs...)
}

func TraceWith(msg string, attrs ...slog.Attr) {
	logger.TraceWith(msg, attrs...)
}

func FatalWith(msg string, attrs ...slog.Attr) {
	logger.FatalWith(msg, attrs...)
}

// Context-aware logging global functions
func InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.InfoCtx(ctx, msg, attrs...)
}

func ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.ErrorCtx(ctx, msg, attrs...)
}

func WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.WarnCtx(ctx, msg, attrs...)
}

func DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.DebugCtx(ctx, msg, attrs...)
}

func TraceCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.TraceCtx(ctx, msg, attrs...)
}

func FatalCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	logger.FatalCtx(ctx, msg, attrs...)
}

// Attribute grouping global functions
func WithAttrs(attrs ...slog.Attr) Logger {
	return logger.WithAttrs(attrs...)
}

func WithGroup(name string) Logger {
	return logger.WithGroup(name)
}
