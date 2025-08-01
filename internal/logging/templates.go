package logging

import "fmt"

// LogTemplate represents a logging template with standardized emoji and formatting
type LogTemplate struct {
	emoji   string
	prefix  string
	level   LogLevel
	dryRun  bool
}

// LogLevel represents the logging level for templates
type LogLevel int

const (
	LevelInfo LogLevel = iota
	LevelSuccess
	LevelWarn
	LevelError
	LevelDebug
)

// Common logging templates with standardized emojis and formats
var (
	// Action templates
	WaitTemplate = LogTemplate{emoji: "⏳", prefix: "Waiting", level: LevelInfo}
	ScreenshotTemplate = LogTemplate{emoji: "📸", prefix: "Taking screenshot", level: LevelInfo}
	SuccessTemplate = LogTemplate{emoji: "✅", prefix: "", level: LevelSuccess}
	ErrorTemplate = LogTemplate{emoji: "❌", prefix: "", level: LevelError}

	// Input/Output templates
	TypeTemplate = LogTemplate{emoji: "📝", prefix: "Typing", level: LevelInfo}
	KeyTemplate = LogTemplate{emoji: "⌨️", prefix: "Sending key", level: LevelInfo}
	VariableTemplate = LogTemplate{emoji: "📝", prefix: "Setting variable", level: LevelInfo}

	// Monitoring templates
	WatchTemplate = LogTemplate{emoji: "👁️", prefix: "Watching for", level: LevelInfo}
	SearchTemplate = LogTemplate{emoji: "🔍", prefix: "Searching for", level: LevelInfo}
	FoundTemplate = LogTemplate{emoji: "✓", prefix: "Found", level: LevelSuccess}
	NotFoundTemplate = LogTemplate{emoji: "✗", prefix: "Not found", level: LevelWarn}

	// System templates
	ExecuteTemplate = LogTemplate{emoji: "💻", prefix: "Executing", level: LevelInfo}
	ConnectTemplate = LogTemplate{emoji: "🔌", prefix: "Connecting to", level: LevelInfo}
	DisconnectTemplate = LogTemplate{emoji: "🔌", prefix: "Disconnected from", level: LevelInfo}

	// File operations
	FileTemplate = LogTemplate{emoji: "📁", prefix: "File", level: LevelInfo}
	SaveTemplate = LogTemplate{emoji: "💾", prefix: "Saved", level: LevelSuccess}
	LoadTemplate = LogTemplate{emoji: "📂", prefix: "Loading", level: LevelInfo}

	// Process templates
	StartTemplate = LogTemplate{emoji: "🚀", prefix: "Starting", level: LevelInfo}
	StopTemplate = LogTemplate{emoji: "🛑", prefix: "Stopping", level: LevelInfo}
	CompleteTemplate = LogTemplate{emoji: "✓", prefix: "Completed", level: LevelSuccess}
	FailTemplate = LogTemplate{emoji: "✗", prefix: "Failed", level: LevelError}

	// Debug templates
	DebugTemplate = LogTemplate{emoji: "🐛", prefix: "Debug", level: LevelDebug}
	BreakTemplate = LogTemplate{emoji: "🔄", prefix: "Breaking", level: LevelInfo}
	ReturnTemplate = LogTemplate{emoji: "↩", prefix: "Returning", level: LevelInfo}

	// Console templates
	ConsoleTemplate = LogTemplate{emoji: "🖥️", prefix: "Console", level: LevelInfo}
	ExitTemplate = LogTemplate{emoji: "🚪", prefix: "Exiting", level: LevelInfo}
)

// Dry-run variants (automatically generated)
var (
	DryRunWaitTemplate = LogTemplate{emoji: "⏳", prefix: "Would wait", level: LevelInfo, dryRun: true}
	DryRunScreenshotTemplate = LogTemplate{emoji: "📸", prefix: "Would take screenshot", level: LevelInfo, dryRun: true}
	DryRunTypeTemplate = LogTemplate{emoji: "📝", prefix: "Would type", level: LevelInfo, dryRun: true}
	DryRunKeyTemplate = LogTemplate{emoji: "⌨️", prefix: "Would send key", level: LevelInfo, dryRun: true}
	DryRunVariableTemplate = LogTemplate{emoji: "📝", prefix: "Would set variable", level: LevelInfo, dryRun: true}
	DryRunWatchTemplate = LogTemplate{emoji: "👁️", prefix: "Would watch for", level: LevelInfo, dryRun: true}
	DryRunExecuteTemplate = LogTemplate{emoji: "💻", prefix: "Would execute", level: LevelInfo, dryRun: true}
	DryRunConsoleTemplate = LogTemplate{emoji: "🖥️", prefix: "Would switch to console", level: LevelInfo, dryRun: true}
	DryRunExitTemplate = LogTemplate{emoji: "🚪", prefix: "Would exit", level: LevelInfo, dryRun: true}
	DryRunBreakTemplate = LogTemplate{emoji: "🔄", prefix: "Would break", level: LevelInfo, dryRun: true}
	DryRunReturnTemplate = LogTemplate{emoji: "↩", prefix: "Would return", level: LevelInfo, dryRun: true}
)

// Format formats the template with the provided message
func (t LogTemplate) Format(message string) string {
	if t.dryRun {
		return fmt.Sprintf("%s [DRY-RUN] %s: %s", t.emoji, t.prefix, message)
	}
	if t.prefix != "" {
		return fmt.Sprintf("%s %s: %s", t.emoji, t.prefix, message)
	}
	return fmt.Sprintf("%s %s", t.emoji, message)
}

// Formatf formats the template with printf-style formatting
func (t LogTemplate) Formatf(format string, args ...interface{}) string {
	message := fmt.Sprintf(format, args...)
	return t.Format(message)
}

// Log logs the message using the appropriate logging function based on level
func (t LogTemplate) Log(message string) {
	formatted := t.Format(message)
	switch t.level {
	case LevelInfo:
		UserInfof(formatted)
	case LevelSuccess:
		Successf(formatted)
	case LevelWarn:
		UserWarnf(formatted)
	case LevelError:
		UserErrorf(formatted)
	case LevelDebug:
		Debug(formatted)
	}
}

// Logf logs the message using printf-style formatting
func (t LogTemplate) Logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	t.Log(message)
}

// Template helper functions for common patterns

// WaitFor logs a wait operation with duration
func WaitFor(duration string) {
	WaitTemplate.Log(duration)
}

// TakeScreenshot logs a screenshot operation
func TakeScreenshot(path, format string) {
	ScreenshotTemplate.Logf("%s (%s)", path, format)
}

// TypeText logs typing operation
func TypeText(text string) {
	TypeTemplate.Log(text)
}

// SendKey logs key sending operation
func SendKey(key string) {
	KeyTemplate.Log(key)
}

// SetVariable logs variable assignment
func SetVariable(name, value string) {
	VariableTemplate.Logf("%s=\"%s\"", name, value)
}

// WatchFor logs watch operation
func WatchFor(text string, timeout string) {
	WatchTemplate.Logf("\"%s\" (timeout: %s)", text, timeout)
}

// SearchFor logs search operation
func SearchFor(text string) {
	SearchTemplate.Logf("\"%s\"", text)
}

// Found logs successful search
func Found(text string, location string) {
	if location != "" {
		FoundTemplate.Logf("\"%s\" in %s", text, location)
	} else {
		FoundTemplate.Logf("\"%s\"", text)
	}
}

// NotFound logs unsuccessful search
func NotFound(text string) {
	NotFoundTemplate.Logf("\"%s\"", text)
}

// Execute logs command execution
func Execute(command string) {
	ExecuteTemplate.Log(command)
}

// Connect logs connection establishment
func Connect(target string) {
	ConnectTemplate.Log(target)
}

// Disconnect logs disconnection
func Disconnect(target string) {
	DisconnectTemplate.Log(target)
}

// SaveFile logs file save operation
func SaveFile(path string, details string) {
	if details != "" {
		SaveTemplate.Logf("%s (%s)", path, details)
	} else {
		SaveTemplate.Log(path)
	}
}

// LoadFile logs file load operation
func LoadFile(path string) {
	LoadTemplate.Log(path)
}

// Start logs process start
func Start(process string) {
	StartTemplate.Log(process)
}

// Stop logs process stop
func Stop(process string) {
	StopTemplate.Log(process)
}

// Complete logs successful completion
func Complete(operation string) {
	CompleteTemplate.Log(operation)
}

// Fail logs operation failure
func Fail(operation string, reason string) {
	if reason != "" {
		FailTemplate.Logf("%s: %s", operation, reason)
	} else {
		FailTemplate.Log(operation)
	}
}

// SwitchConsole logs console switching
func SwitchConsole(consoleNum interface{}) {
	ConsoleTemplate.Logf("Switching to console %v", consoleNum)
}

// Exit logs exit operation
func Exit(code interface{}) {
	ExitTemplate.Logf("with code %v", code)
}

// Break logs break operation
func Break() {
	BreakTemplate.Log("from loop")
}

// Return logs return operation
func Return() {
	ReturnTemplate.Log("from function")
}

// Dry-run helper functions

// DryRunWaitFor logs a dry-run wait operation
func DryRunWaitFor(duration string) {
	DryRunWaitTemplate.Log(duration)
}

// DryRunTakeScreenshot logs a dry-run screenshot operation
func DryRunTakeScreenshot(path, format string) {
	DryRunScreenshotTemplate.Logf("%s (%s)", path, format)
}

// DryRunTypeText logs dry-run typing operation
func DryRunTypeText(text string) {
	DryRunTypeTemplate.Log(text)
}

// DryRunSendKey logs dry-run key sending operation
func DryRunSendKey(key string) {
	DryRunKeyTemplate.Log(key)
}

// DryRunSetVariable logs dry-run variable assignment
func DryRunSetVariable(name, value string) {
	DryRunVariableTemplate.Logf("%s=\"%s\"", name, value)
}

// DryRunWatchFor logs dry-run watch operation
func DryRunWatchFor(text string, timeout string) {
	DryRunWatchTemplate.Logf("\"%s\" (timeout: %s)", text, timeout)
}

// DryRunExecute logs dry-run command execution
func DryRunExecute(command string) {
	DryRunExecuteTemplate.Log(command)
}

// DryRunSwitchConsole logs dry-run console switching
func DryRunSwitchConsole(consoleNum interface{}) {
	DryRunConsoleTemplate.Logf("%v", consoleNum)
}

// DryRunExit logs dry-run exit operation
func DryRunExit(code interface{}) {
	DryRunExitTemplate.Logf("with code %v", code)
}

// DryRunBreak logs dry-run break operation
func DryRunBreak() {
	DryRunBreakTemplate.Log("from loop")
}

// DryRunReturn logs dry-run return operation
func DryRunReturn() {
	DryRunReturnTemplate.Log("from function")
}
