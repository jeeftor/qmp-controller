package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/styles"
)

var (
	// Current log level
	currentLevel slog.Level

	// Default logger instance
	logger *slog.Logger

	// Styles for different log levels using lipgloss
	traceStyle   = styles.MutedStyle
	debugStyle   = styles.LogDebugStyle
	infoStyle    = styles.LogInfoStyle
	warnStyle    = styles.LogWarnStyle
	errorStyle   = styles.LogErrorStyle
)

// ColorTextHandler is a simple handler that adds colors to log output
type ColorTextHandler struct {
	w io.Writer
}

// NewColorTextHandler creates a new ColorTextHandler
func NewColorTextHandler(w io.Writer) *ColorTextHandler {
	return &ColorTextHandler{w: w}
}

// Custom log level for TRACE
const LevelTrace = slog.Level(-8)

// Handle handles the log record
func (h *ColorTextHandler) Handle(ctx context.Context, r slog.Record) error {
	var levelText string
	switch r.Level {
	case LevelTrace:
		levelText = traceStyle.Render("TRACE")
	case slog.LevelDebug:
		levelText = debugStyle.Render("DEBUG")
	case slog.LevelInfo:
		levelText = infoStyle.Render("INFO")
	case slog.LevelWarn:
		levelText = warnStyle.Render("WARN")
	case slog.LevelError:
		levelText = errorStyle.Render("ERROR")
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

// formatAttrValue formats a slog.Value as a string with proper styling
func formatAttrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		// Quote strings and style them
		return styles.BoldStyle.Render(fmt.Sprintf("\"%s\"", v.String()))
	case slog.KindInt64:
		return styles.InfoStyle.Render(fmt.Sprintf("%d", v.Int64()))
	case slog.KindUint64:
		return styles.InfoStyle.Render(fmt.Sprintf("%d", v.Uint64()))
	case slog.KindFloat64:
		return styles.InfoStyle.Render(fmt.Sprintf("%.2f", v.Float64()))
	case slog.KindBool:
		if v.Bool() {
			return styles.SuccessStyle.Render("true")
		}
		return styles.ErrorStyle.Render("false")
	case slog.KindDuration:
		return styles.WarningStyle.Render(v.Duration().String())
	case slog.KindTime:
		return styles.MutedStyle.Render(v.Time().Format("15:04:05.000"))
	case slog.KindAny:
		return styles.DebugStyle.Render(fmt.Sprintf("%v", v.Any()))
	default:
		return styles.MutedStyle.Render(v.String())
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
	return level >= currentLevel
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default to info
	}
}

// InitWithLevel initializes the logger with the specified log level
func InitWithLevel(level string) {
	currentLevel = parseLogLevel(level)

	handler := NewColorTextHandler(os.Stdout)
	logger = slog.New(handler)
	slog.SetDefault(logger)

	Debug("Logging initialized", "level", level, "parsed_level", currentLevel)
}

// Init initializes the logger with the specified debug level (backward compatibility)
func Init(debug bool) {
	if debug {
		InitWithLevel("debug")
	} else {
		InitWithLevel("info")
	}
}

// SetOutput sets the output writer for the logger
func SetOutput(w io.Writer) {
	handler := NewColorTextHandler(w)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// Trace logs a trace message (most verbose level)
func Trace(msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
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
	// Print a carriage return first to ensure clean line
	fmt.Print("\r")
	if args != nil {
		Trace("Sending QMP command", "command", cmd, "args", args)
	} else {
		Trace("Sending QMP command", "command", cmd)
	}
}

// LogResponse logs a QMP response with pretty formatting
func LogResponse(resp interface{}) {
	// Print a carriage return first to ensure clean line
	fmt.Print("\r")
	Trace("Received QMP response", "response", resp)
}

// Structured logging functions for common use cases

// LogOperation logs the start and end of operations with timing
func LogOperation(operation string, vmid string, fn func() error) error {
	start := time.Now()
	Debug("Starting operation",
		"operation", operation,
		"vmid", vmid,
		"start_time", start)

	err := fn()
	duration := time.Since(start)

	if err != nil {
		Error("Operation failed",
			"operation", operation,
			"vmid", vmid,
			"duration", duration,
			"error", err)
	} else {
		Debug("Operation completed",
			"operation", operation,
			"vmid", vmid,
			"duration", duration)
	}

	return err
}

// LogConnection logs connection events with details
func LogConnection(vmid string, socketPath string, success bool, err error) {
	if success {
		Debug("QMP connection established",
			"vmid", vmid,
			"socket_path", socketPath,
			"connected", true)
	} else {
		Error("QMP connection failed",
			"vmid", vmid,
			"socket_path", socketPath,
			"connected", false,
			"error", err)
	}
}

// LogScreenshot logs screenshot operations with metadata
func LogScreenshot(vmid string, outputPath string, format string, size int64, duration time.Duration) {
	Info("Screenshot captured",
		"vmid", vmid,
		"output_path", outputPath,
		"format", format,
		"size_bytes", size,
		"duration", duration)
}

// LogOCR logs OCR operations with performance metrics
func LogOCR(vmid string, imagePath string, charactersFound int, duration time.Duration, success bool) {
	if success {
		Info("OCR processing completed",
			"vmid", vmid,
			"image_path", imagePath,
			"characters_found", charactersFound,
			"duration", duration,
			"success", true)
	} else {
		Warn("OCR processing failed",
			"vmid", vmid,
			"image_path", imagePath,
			"duration", duration,
			"success", false)
	}
}

// LogScript logs script execution events
func LogScript(vmid string, scriptFile string, lineNumber int, command string, status string, err error) {
	if err != nil {
		Error("Script command failed",
			"vmid", vmid,
			"script_file", scriptFile,
			"line_number", lineNumber,
			"command", command,
			"status", status,
			"error", err)
	} else {
		Debug("Script command executed",
			"vmid", vmid,
			"script_file", scriptFile,
			"line_number", lineNumber,
			"command", command,
			"status", status)
	}
}

// LogWatch logs WATCH command monitoring with search results
func LogWatch(vmid string, searchString string, found bool, attempts int, duration time.Duration) {
	if found {
		Info("WATCH condition satisfied",
			"vmid", vmid,
			"search_string", searchString,
			"found", true,
			"attempts", attempts,
			"duration", duration)
	} else {
		Warn("WATCH condition timeout",
			"vmid", vmid,
			"search_string", searchString,
			"found", false,
			"attempts", attempts,
			"duration", duration)
	}
}

// LogPerformance logs performance metrics for operations
func LogPerformance(operation string, vmid string, metrics map[string]interface{}) {
	attrs := []any{
		"operation", operation,
		"vmid", vmid,
	}

	// Add all metrics as key-value pairs
	for key, value := range metrics {
		attrs = append(attrs, key, value)
	}

	Debug("Performance metrics", attrs...)
}

// LogKeyboard logs keyboard input with timing
func LogKeyboard(vmid string, input string, inputType string, delay time.Duration) {
	Debug("Keyboard input sent",
		"vmid", vmid,
		"input", input,
		"input_type", inputType,
		"delay", delay)
}

// WithFields creates a logger with pre-set fields for contextual logging
func WithFields(fields map[string]interface{}) *slog.Logger {
	attrs := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		attrs = append(attrs, key, value)
	}
	return logger.With(attrs...)
}

// ContextualLogger provides context-aware logging for specific operations
type ContextualLogger struct {
	logger *slog.Logger
	vmid   string
	operation string
}

// NewContextualLogger creates a new contextual logger for a specific VM and operation
func NewContextualLogger(vmid string, operation string) *ContextualLogger {
	contextLogger := logger.With(
		"vmid", vmid,
		"operation", operation,
	)

	return &ContextualLogger{
		logger: contextLogger,
		vmid: vmid,
		operation: operation,
	}
}

// Debug logs a debug message with context
func (cl *ContextualLogger) Debug(msg string, args ...any) {
	cl.logger.Debug(msg, args...)
}

// Info logs an info message with context
func (cl *ContextualLogger) Info(msg string, args ...any) {
	cl.logger.Info(msg, args...)
}

// Warn logs a warning message with context
func (cl *ContextualLogger) Warn(msg string, args ...any) {
	cl.logger.Warn(msg, args...)
}

// Error logs an error message with context
func (cl *ContextualLogger) Error(msg string, args ...any) {
	cl.logger.Error(msg, args...)
}

// WithMetrics logs a message with performance metrics
func (cl *ContextualLogger) WithMetrics(level slog.Level, msg string, metrics map[string]interface{}, args ...any) {
	allArgs := make([]any, 0, len(args)+len(metrics)*2)
	allArgs = append(allArgs, args...)

	for key, value := range metrics {
		allArgs = append(allArgs, key, value)
	}

	cl.logger.Log(context.Background(), level, msg, allArgs...)
}

// User-facing logging functions that replace fmt.Printf calls
// These provide consistent formatting while maintaining structured logging

// Success logs a successful operation with green styling
func Success(msg string, args ...any) {
	// For user-facing success messages, we want them visible at info level
	if len(args) > 0 {
		Info("✓ "+msg, args...)
	} else {
		Info("✓ " + msg)
	}
}

// UserInfo logs informational messages that users should see
func UserInfo(msg string, args ...any) {
	if len(args) > 0 {
		Info(msg, args...)
	} else {
		Info(msg)
	}
}

// UserWarn logs warning messages that users should see
func UserWarn(msg string, args ...any) {
	if len(args) > 0 {
		Warn(msg, args...)
	} else {
		Warn(msg)
	}
}

// UserError logs error messages that users should see
func UserError(msg string, args ...any) {
	if len(args) > 0 {
		Error(msg, args...)
	} else {
		Error(msg)
	}
}

// Progress logs progress information for long-running operations
func Progress(msg string, args ...any) {
	if len(args) > 0 {
		Info("→ "+msg, args...)
	} else {
		Info("→ " + msg)
	}
}

// Result logs operation results with formatting
func Result(msg string, args ...any) {
	if len(args) > 0 {
		Info("» "+msg, args...)
	} else {
		Info("» " + msg)
	}
}

// Interactive logs messages for interactive prompts
func Interactive(msg string, args ...any) {
	// Interactive messages bypass structured logging to maintain UX
	if len(args) > 0 {
		fmt.Printf(msg+"\n", args...)
	} else {
		fmt.Printf("%s\n", msg)
	}
}

// InteractivePrompt logs prompts without newlines
func InteractivePrompt(msg string, args ...any) {
	// Interactive prompts bypass structured logging to maintain UX
	if len(args) > 0 {
		fmt.Printf(msg, args...)
	} else {
		fmt.Print(msg)
	}
}

// Performance monitoring and metrics

// Timer represents a timing measurement
type Timer struct {
	operation string
	vmid      string
	startTime time.Time
	logger    *ContextualLogger
}

// StartTimer creates and starts a new timer for performance monitoring
func StartTimer(operation string, vmid string) *Timer {
	contextLogger := NewContextualLogger(vmid, operation)
	timer := &Timer{
		operation: operation,
		vmid:      vmid,
		startTime: time.Now(),
		logger:    contextLogger,
	}

	timer.logger.Debug("Operation started", "start_time", timer.startTime)
	return timer
}

// Stop stops the timer and logs the duration with optional metrics
func (t *Timer) Stop(success bool, metrics map[string]interface{}) time.Duration {
	duration := time.Since(t.startTime)

	logMetrics := map[string]interface{}{
		"duration": duration,
		"success":  success,
	}

	// Merge additional metrics if provided
	if metrics != nil {
		for k, v := range metrics {
			logMetrics[k] = v
		}
	}

	if success {
		t.logger.WithMetrics(slog.LevelInfo, "Operation completed successfully", logMetrics)
	} else {
		t.logger.WithMetrics(slog.LevelError, "Operation failed", logMetrics)
	}

	return duration
}

// StopWithError stops the timer and logs an error
func (t *Timer) StopWithError(err error, metrics map[string]interface{}) time.Duration {
	duration := time.Since(t.startTime)

	logMetrics := map[string]interface{}{
		"duration": duration,
		"success":  false,
		"error":    err.Error(),
	}

	// Merge additional metrics if provided
	if metrics != nil {
		for k, v := range metrics {
			logMetrics[k] = v
		}
	}

	t.logger.WithMetrics(slog.LevelError, "Operation failed with error", logMetrics)
	return duration
}

// LogProgress logs intermediate progress during long operations
func (t *Timer) LogProgress(msg string, progress float64, args ...any) {
	elapsed := time.Since(t.startTime)
	progressArgs := append([]any{
		"elapsed", elapsed,
		"progress_percent", progress,
	}, args...)

	t.logger.Info("Progress: "+msg, progressArgs...)
}

// Specialized logging functions for common operations

// LogCommandExecution logs command execution with structured data
func LogCommandExecution(vmid string, command string, args []string, exitCode int, duration time.Duration, output string) {
	logger := NewContextualLogger(vmid, "command_execution")

	metrics := map[string]interface{}{
		"command":   command,
		"args":      args,
		"exit_code": exitCode,
		"duration":  duration,
		"output_size": len(output),
	}

	if exitCode == 0 {
		logger.WithMetrics(slog.LevelInfo, "Command executed successfully", metrics)
	} else {
		logger.WithMetrics(slog.LevelError, "Command execution failed", metrics, "output", output)
	}
}

// LogResourceUsage logs system resource usage metrics
func LogResourceUsage(vmid string, operation string, memoryMB float64, cpuPercent float64, diskMB float64) {
	logger := NewContextualLogger(vmid, operation)

	metrics := map[string]interface{}{
		"memory_mb":    memoryMB,
		"cpu_percent":  cpuPercent,
		"disk_mb":      diskMB,
	}

	logger.WithMetrics(slog.LevelDebug, "Resource usage metrics", metrics)
}

// LogNetworkOperation logs network-related operations with timing and data metrics
func LogNetworkOperation(vmid string, operation string, bytes int64, duration time.Duration, success bool, endpoint string) {
	logger := NewContextualLogger(vmid, "network_"+operation)

	metrics := map[string]interface{}{
		"bytes":      bytes,
		"duration":   duration,
		"success":    success,
		"endpoint":   endpoint,
		"bytes_per_sec": float64(bytes) / duration.Seconds(),
	}

	if success {
		logger.WithMetrics(slog.LevelInfo, "Network operation completed", metrics)
	} else {
		logger.WithMetrics(slog.LevelError, "Network operation failed", metrics)
	}
}

// GetLogger returns the default logger instance for advanced usage
func GetLogger() *slog.Logger {
	return logger
}

// GetCurrentLogLevel returns the current log level
func GetCurrentLogLevel() slog.Level {
	return currentLevel
}

// IsLevelEnabled checks if a specific log level is enabled
func IsLevelEnabled(level slog.Level) bool {
	return level >= currentLevel
}
