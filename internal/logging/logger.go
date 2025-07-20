package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/fatih/color"
)

var (
	// Debug flag to track debug state
	debugEnabled bool

	// Default logger instance
	logger *slog.Logger

	// Colors for different log levels
	infoColor    = color.New(color.FgGreen).SprintFunc()
	warnColor    = color.New(color.FgYellow).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	debugColor   = color.New(color.FgCyan).SprintFunc()
	commandColor = color.New(color.FgMagenta).SprintFunc()
)

// ColorTextHandler is a simple handler that adds colors to log output
type ColorTextHandler struct {
	w io.Writer
}

// NewColorTextHandler creates a new ColorTextHandler
func NewColorTextHandler(w io.Writer) *ColorTextHandler {
	return &ColorTextHandler{w: w}
}

// Handle handles the log record
func (h *ColorTextHandler) Handle(ctx context.Context, r slog.Record) error {
	var levelText string
	switch r.Level {
	case slog.LevelDebug:
		levelText = debugColor("DEBUG")
	case slog.LevelInfo:
		levelText = infoColor("INFO")
	case slog.LevelWarn:
		levelText = warnColor("WARN")
	case slog.LevelError:
		levelText = errorColor("ERROR")
	default:
		levelText = r.Level.String()
	}

	// Format the message
	msg := r.Message

	// Build attributes string
	var attrs string
	r.Attrs(func(a slog.Attr) bool {
		// Skip the source attribute
		if a.Key == "source" {
			return true
		}

		// Format the attribute
		attrs += " " + a.Key + "=" + formatAttrValue(a.Value)
		return true
	})

	// Write the log line with a carriage return at the beginning to ensure clean output
	_, err := fmt.Fprintf(h.w, "\r%s %s%s\n", levelText, msg, attrs)
	return err
}

// formatAttrValue formats a slog.Value as a string
func formatAttrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return fmt.Sprintf("%d", v.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", v.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%f", v.Float64())
	case slog.KindBool:
		return fmt.Sprintf("%t", v.Bool())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format("15:04:05")
	case slog.KindAny:
		return fmt.Sprintf("%v", v.Any())
	default:
		return v.String()
	}
}

// WithAttrs returns a new handler with the given attributes
func (h *ColorTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group
func (h *ColorTextHandler) WithGroup(name string) slog.Handler {
	return h
}

// Enabled reports whether the handler handles records at the given level
func (h *ColorTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if debugEnabled {
		return level >= slog.LevelDebug
	}
	return level >= slog.LevelInfo
}

// Init initializes the logger with the specified debug level
func Init(debug bool) {
	debugEnabled = debug

	handler := NewColorTextHandler(os.Stdout)
	logger = slog.New(handler)
	slog.SetDefault(logger)

	if debug {
		Debug("Debug logging enabled")
	}
}

// SetOutput sets the output writer for the logger
func SetOutput(w io.Writer) {
	handler := NewColorTextHandler(w)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// LogCommand logs a QMP command with pretty formatting
func LogCommand(cmd string, args interface{}) {
	if !debugEnabled {
		return
	}

	// Print a carriage return first to ensure clean line
	fmt.Print("\r")
	if args != nil {
		Debug("Sending QMP command", "command", cmd, "args", args)
	} else {
		Debug("Sending QMP command", "command", cmd)
	}
}

// LogResponse logs a QMP response with pretty formatting
func LogResponse(resp interface{}) {
	if !debugEnabled {
		return
	}

	// Print a carriage return first to ensure clean line
	fmt.Print("\r")
	Debug("Received QMP response", "response", resp)
}
