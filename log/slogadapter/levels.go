package slogadapter

import (
	"log/slog"

	"github.com/p4gefau1t/trojan-go/log"
)

// Custom slog levels for Trace and Fatal
const (
	LevelTrace = slog.Level(-8) // Below DEBUG (-4)
	LevelFatal = slog.Level(12) // Above ERROR (8)
)

// mapLogLevelToSlog maps the current LogLevel to slog.Level
func mapLogLevelToSlog(level log.LogLevel) slog.Level {
	switch level {
	case log.AllLevel:
		return LevelTrace // Show all messages including trace
	case log.InfoLevel:
		return slog.LevelInfo
	case log.WarnLevel:
		return slog.LevelWarn
	case log.ErrorLevel:
		return slog.LevelError
	case log.FatalLevel:
		return LevelFatal
	case log.OffLevel:
		return slog.Level(100) // High level to disable all logging
	default:
		return slog.LevelInfo
	}
}

// mapSlogLevelToLogLevel maps slog.Level back to LogLevel
func mapSlogLevelToLogLevel(level slog.Level) log.LogLevel {
	switch {
	case level <= LevelTrace:
		return log.AllLevel
	case level <= slog.LevelDebug:
		return log.AllLevel
	case level <= slog.LevelInfo:
		return log.InfoLevel
	case level <= slog.LevelWarn:
		return log.WarnLevel
	case level <= slog.LevelError:
		return log.ErrorLevel
	case level <= LevelFatal:
		return log.FatalLevel
	default:
		return log.OffLevel
	}
}
