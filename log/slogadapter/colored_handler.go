package slogadapter

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/MatusOllah/slogcolor"
	"golang.org/x/term"
)

// FdWriter interface extends existing io.Writer with file descriptor function
// support (compatible with golog's FdWriter)
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

// ColoredTextHandler is a custom slog handler that provides colored output
// similar to the current golog implementation
type ColoredTextHandler struct {
	handler  slog.Handler
	useColor bool
	mu       sync.RWMutex
}

// NewColoredTextHandler creates a new colored text handler
func NewColoredTextHandler(w io.Writer, opts *slog.HandlerOptions) *ColoredTextHandler {
	// Detect if we should use color based on terminal detection
	useColor := false
	if fdw, ok := w.(FdWriter); ok {
		useColor = term.IsTerminal(int(fdw.Fd()))
	} else if w == os.Stdout || w == os.Stderr {
		// Check if stdout/stderr are terminals
		if w == os.Stdout {
			useColor = term.IsTerminal(int(os.Stdout.Fd()))
		} else if w == os.Stderr {
			useColor = term.IsTerminal(int(os.Stderr.Fd()))
		}
	}

	var handler slog.Handler
	if useColor {
		// Use slogcolor for colored output with custom level names
		colorOpts := &slogcolor.Options{
			Level:   opts.Level,
			NoColor: false,
		}
		handler = slogcolor.NewHandler(w, colorOpts)
	} else {
		// Use standard text handler for non-terminal output
		handler = slog.NewTextHandler(w, opts)
	}

	return &ColoredTextHandler{
		handler:  handler,
		useColor: useColor,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *ColoredTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle handles the Record
func (h *ColoredTextHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Map custom levels to appropriate display names
	switch r.Level {
	case LevelTrace:
		// Create a new record with TRACE level name
		newRecord := slog.NewRecord(r.Time, slog.LevelDebug, r.Message, r.PC)
		newRecord.AddAttrs(slog.String("level", "TRACE"))
		r.Attrs(func(a slog.Attr) bool {
			if a.Key != "level" { // Don't duplicate level
				newRecord.AddAttrs(a)
			}
			return true
		})
		return h.handler.Handle(ctx, newRecord)
	case LevelFatal:
		// Create a new record with FATAL level name
		newRecord := slog.NewRecord(r.Time, slog.LevelError, r.Message, r.PC)
		newRecord.AddAttrs(slog.String("level", "FATAL"))
		r.Attrs(func(a slog.Attr) bool {
			if a.Key != "level" { // Don't duplicate level
				newRecord.AddAttrs(a)
			}
			return true
		})
		return h.handler.Handle(ctx, newRecord)
	}

	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments
func (h *ColoredTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColoredTextHandler{
		handler:  h.handler.WithAttrs(attrs),
		useColor: h.useColor,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups
func (h *ColoredTextHandler) WithGroup(name string) slog.Handler {
	return &ColoredTextHandler{
		handler:  h.handler.WithGroup(name),
		useColor: h.useColor,
	}
}

// SetUseColor enables or disables color output
func (h *ColoredTextHandler) SetUseColor(useColor bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.useColor = useColor
}

// IsColorEnabled returns whether color output is enabled
func (h *ColoredTextHandler) IsColorEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.useColor
}
