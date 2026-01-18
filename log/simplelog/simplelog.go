package simplelog

import (
	"context"
	"io"
	golog "log"
	"log/slog"
	"os"

	"github.com/p4gefau1t/trojan-go/log"
)

func init() {
	log.RegisterLogger(&SimpleLogger{})
}

type SimpleLogger struct {
	logLevel log.LogLevel
}

func (l *SimpleLogger) SetLogLevel(level log.LogLevel) {
	l.logLevel = level
}

func (l *SimpleLogger) Fatal(v ...interface{}) {
	if l.logLevel <= log.FatalLevel {
		golog.Fatal(v...)
	}
	os.Exit(1)
}

func (l *SimpleLogger) Fatalf(format string, v ...interface{}) {
	if l.logLevel <= log.FatalLevel {
		golog.Fatalf(format, v...)
	}
	os.Exit(1)
}

func (l *SimpleLogger) Error(v ...interface{}) {
	if l.logLevel <= log.ErrorLevel {
		golog.Println(v...)
	}
}

func (l *SimpleLogger) Errorf(format string, v ...interface{}) {
	if l.logLevel <= log.ErrorLevel {
		golog.Printf(format, v...)
	}
}

func (l *SimpleLogger) Warn(v ...interface{}) {
	if l.logLevel <= log.WarnLevel {
		golog.Println(v...)
	}
}

func (l *SimpleLogger) Warnf(format string, v ...interface{}) {
	if l.logLevel <= log.WarnLevel {
		golog.Printf(format, v...)
	}
}

func (l *SimpleLogger) Info(v ...interface{}) {
	if l.logLevel <= log.InfoLevel {
		golog.Println(v...)
	}
}

func (l *SimpleLogger) Infof(format string, v ...interface{}) {
	if l.logLevel <= log.InfoLevel {
		golog.Printf(format, v...)
	}
}

func (l *SimpleLogger) Debug(v ...interface{}) {
	if l.logLevel <= log.AllLevel {
		golog.Println(v...)
	}
}

func (l *SimpleLogger) Debugf(format string, v ...interface{}) {
	if l.logLevel <= log.AllLevel {
		golog.Printf(format, v...)
	}
}

func (l *SimpleLogger) Trace(v ...interface{}) {
	if l.logLevel <= log.AllLevel {
		golog.Println(v...)
	}
}

func (l *SimpleLogger) Tracef(format string, v ...interface{}) {
	if l.logLevel <= log.AllLevel {
		golog.Printf(format, v...)
	}
}

func (l *SimpleLogger) SetOutput(io.Writer) {
	// do nothing
}

// Structured logging methods - simple implementation that ignores attributes
func (l *SimpleLogger) InfoWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.InfoLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) ErrorWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.ErrorLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) WarnWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.WarnLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) DebugWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.AllLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) TraceWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.AllLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) FatalWith(msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.FatalLevel {
		golog.Println(msg)
	}
	os.Exit(1)
}

// Context-aware logging methods - simple implementation that ignores context and attributes
func (l *SimpleLogger) InfoCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.InfoLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) ErrorCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.ErrorLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) WarnCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.WarnLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) DebugCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.AllLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) TraceCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.AllLevel {
		golog.Println(msg)
	}
}

func (l *SimpleLogger) FatalCtx(ctx context.Context, msg string, attrs ...slog.Attr) {
	if l.logLevel <= log.FatalLevel {
		golog.Println(msg)
	}
	os.Exit(1)
}

// Attribute grouping methods - simple implementation that returns self
func (l *SimpleLogger) WithAttrs(attrs ...slog.Attr) log.Logger {
	return l
}

func (l *SimpleLogger) WithGroup(name string) log.Logger {
	return l
}
