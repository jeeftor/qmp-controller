package script2

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// ExecutionMode defines the interface for different execution modes (real vs dry-run)
type ExecutionMode interface {
	// Basic operations
	SendText(text string, logger *logging.ContextualLogger) error
	SendKeySequence(keyName string, logger *logging.ContextualLogger) error
	Wait(duration time.Duration, logger *logging.ContextualLogger) error

	// Screen operations
	TakeScreenshot(path string, format string, logger *logging.ContextualLogger) error
	WatchForText(searchText string, timeout time.Duration, logger *logging.ContextualLogger) error

	// Console operations
	SwitchConsole(consoleNum int, logger *logging.ContextualLogger) error

	// System operations
	ExecuteSystemCommand(command string, logger *logging.ContextualLogger) error
	Exit(exitCode int, logger *logging.ContextualLogger) error

	// Variable operations
	SetVariable(name string, value string, variables *VariableExpander, logger *logging.ContextualLogger) error

	// Control flow operations
	Break(logger *logging.ContextualLogger) error
	Return(logger *logging.ContextualLogger) error

	// File operations
	IncludeScript(path string, logger *logging.ContextualLogger) (*Script, error)

	// Mode identification
	IsDryRun() bool
}

// RealExecution implements ExecutionMode for actual VM execution
type RealExecution struct {
	client       *qmp.Client
	context      *ExecutionContext
	trainingData string
}

// DryRunExecution implements ExecutionMode for simulation/validation
type DryRunExecution struct {
	context *ExecutionContext
}

// NewRealExecution creates a real execution mode
func NewRealExecution(client *qmp.Client, context *ExecutionContext, trainingData string) ExecutionMode {
	return &RealExecution{
		client:       client,
		context:      context,
		trainingData: trainingData,
	}
}

// NewDryRunExecution creates a dry-run execution mode
func NewDryRunExecution(context *ExecutionContext) ExecutionMode {
	return &DryRunExecution{
		context: context,
	}
}

// RealExecution implementations

func (r *RealExecution) IsDryRun() bool {
	return false
}

func (r *RealExecution) SendText(text string, logger *logging.ContextualLogger) error {
	if r.client == nil {
		return fmt.Errorf("QMP client not available")
	}

	logger.Debug("Sending text to VM", "text", text, "length", len(text))

	// Use the existing QMP client method for sending strings
	if err := r.client.SendString(text, getKeyDelay()); err != nil {
		return fmt.Errorf("failed to send text to VM: %w", err)
	}

	// Log the keyboard input for debugging
	logging.LogKeyboard(r.context.VMID, text, "text", getKeyDelay())

	return nil
}

func (r *RealExecution) SendKeySequence(keyName string, logger *logging.ContextualLogger) error {
	if r.client == nil {
		return fmt.Errorf("QMP client not available")
	}

	logger.Info("Sending key sequence", "directive", "key", "key", keyName)

	// Map script2 key names to QMP key names
	qmpKey := mapKeyName(keyName)
	if qmpKey == "" {
		return fmt.Errorf("unsupported key sequence: %s", keyName)
	}

	// Send the key using QMP client
	if err := r.client.SendKey(qmpKey); err != nil {
		return fmt.Errorf("failed to send key '%s': %w", keyName, err)
	}

	// Log the keyboard input
	logging.LogKeyboard(r.context.VMID, keyName, "key_sequence", getKeyDelay())

	return nil
}

func (r *RealExecution) Wait(duration time.Duration, logger *logging.ContextualLogger) error {
	logger.Debug("Waiting", "duration", duration)
	logging.WaitFor(duration.String())
	time.Sleep(duration)
	return nil
}

func (r *RealExecution) TakeScreenshot(path string, format string, logger *logging.ContextualLogger) error {
	if r.client == nil {
		return fmt.Errorf("QMP client not available")
	}

	logger.Info("Taking screenshot", "path", path, "format", format)
	logging.TakeScreenshot(path, format)

	// Take screenshot via QMP client - empty remoteTempPath for local execution
	var err error
	if format == "png" {
		err = r.client.ScreenDumpAndConvert(path, "")
	} else {
		err = r.client.ScreenDump(path, "")
	}

	if err != nil {
		return fmt.Errorf("failed to take screenshot: %w", err)
	}

	return nil
}

func (r *RealExecution) WatchForText(searchText string, timeout time.Duration, logger *logging.ContextualLogger) error {
	logger.Info("Watching for text", "search_text", searchText, "timeout", timeout)
	logging.WatchFor(searchText, timeout.String())

	if r.client == nil {
		return fmt.Errorf("QMP client not available for watch operation")
	}

	// Create OCR capture configuration
	captureConfig := &ocr.CaptureConfig{
		TrainingDataPath: r.trainingData,
		Columns:          160,
		Rows:             50,
		CropEnabled:      false,
		Prefix:           "watch-ocr",
	}

	// Use default path if training data not specified
	if captureConfig.TrainingDataPath == "" {
		captureConfig.TrainingDataPath = "/Users/jstein/.qmp_training_data.json" // TODO: Use proper default
	}

	// Create OCR capture instance (need to import TakeTemporaryScreenshot from cmd/root.go)
	// For now, use the default implementation
	ocrCapture := ocr.NewOCRCapture(captureConfig, nil)

	pollInterval := 500 * time.Millisecond
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		// Capture current screen with OCR
		result := ocrCapture.Capture(r.client)
		if result.Error != nil {
			logger.Debug("OCR capture failed, continuing", "error", result.Error)
		} else {
			// Search for text in OCR results
			for _, line := range result.Text {
				if strings.Contains(strings.ToLower(line), strings.ToLower(searchText)) {
					logger.Info("Watch condition satisfied", "text", searchText,
						"found_in", line, "elapsed", time.Since(startTime))
					return nil
				}
			}
		}

		logger.Debug("Watch polling", "text", searchText, "elapsed", time.Since(startTime))
		time.Sleep(pollInterval)
	}

	logger.Warn("Watch condition not satisfied within timeout", "text", searchText, "timeout", timeout)
	return fmt.Errorf("watch timeout: text '%s' not found within %v", searchText, timeout)
}

func (r *RealExecution) SwitchConsole(consoleNum int, logger *logging.ContextualLogger) error {
	if r.client == nil {
		return fmt.Errorf("QMP client not available")
	}

	logger.Debug("Switching console", "console", consoleNum)
	logging.UserInfof("🖥️  Switching to console %d", consoleNum)

	// Send Ctrl+Alt+F[1-6] sequence
	fKey := fmt.Sprintf("f%d", consoleNum)
	err := r.client.SendKey("ctrl-alt-" + fKey)
	if err != nil {
		return fmt.Errorf("failed to switch to console %d: %w", consoleNum, err)
	}

	return nil
}

func (r *RealExecution) ExecuteSystemCommand(command string, logger *logging.ContextualLogger) error {
	logger.Info("Executing system command", "command", command)
	logging.Execute(command)

	// Execute system command (extracted from executeSystemCommand)
	return fmt.Errorf("ExecuteSystemCommand not implemented yet - will be extracted")
}

func (r *RealExecution) Exit(exitCode int, logger *logging.ContextualLogger) error {
	logger.Info("Script exit requested", "exit_code", exitCode)
	logging.UserInfof("🚪 Exiting with code %d", exitCode)
	return fmt.Errorf("script exit: code %d", exitCode)
}

func (r *RealExecution) SetVariable(name string, value string, variables *VariableExpander, logger *logging.ContextualLogger) error {
	logger.Debug("Setting variable", "name", name, "value", value)
	logging.SetVariable(name, value)
	variables.Set(name, value)
	return nil
}

func (r *RealExecution) Break(logger *logging.ContextualLogger) error {
	logger.Info("Break directive encountered")
	logging.Break()
	return BreakLoopError{}
}

func (r *RealExecution) Return(logger *logging.ContextualLogger) error {
	logger.Info("Executing return directive - exiting function")
	logging.Return()
	return FunctionReturnError{}
}

func (r *RealExecution) IncludeScript(path string, logger *logging.ContextualLogger) (*Script, error) {
	logger.Info("Including script file", "path", path)
	logging.UserInfof("📋 Including script: %s", path)

	// Implementation would parse and return the included script
	return nil, fmt.Errorf("IncludeScript not implemented yet - will be extracted")
}

// DryRunExecution implementations

func (d *DryRunExecution) IsDryRun() bool {
	return true
}

func (d *DryRunExecution) SendText(text string, logger *logging.ContextualLogger) error {
	logger.Info("📝 [DRY-RUN] Text input simulation", "text", text)
	logging.DryRunTypeText(text)
	return nil
}

func (d *DryRunExecution) SendKeySequence(keyName string, logger *logging.ContextualLogger) error {
	logger.Info("⌨️  [DRY-RUN] Key sequence simulation", "key", keyName)
	logging.DryRunSendKey(keyName)
	return nil
}

func (d *DryRunExecution) Wait(duration time.Duration, logger *logging.ContextualLogger) error {
	logger.Info("⏳ [DRY-RUN] Wait simulation", "duration", duration)
	logging.DryRunWaitFor(duration.String())
	return nil
}

func (d *DryRunExecution) TakeScreenshot(path string, format string, logger *logging.ContextualLogger) error {
	logger.Info("📸 [DRY-RUN] Screenshot simulation", "path", path, "format", format)
	logging.DryRunTakeScreenshot(path, format)
	return nil
}

func (d *DryRunExecution) WatchForText(searchText string, timeout time.Duration, logger *logging.ContextualLogger) error {
	logger.Info("👁️ [DRY-RUN] Watch directive simulation", "search_text", searchText, "timeout", timeout)
	logging.DryRunWatchFor(searchText, timeout.String())
	return nil
}

func (d *DryRunExecution) SwitchConsole(consoleNum int, logger *logging.ContextualLogger) error {
	logger.Info("🖥️  [DRY-RUN] Console switch simulation", "console", consoleNum)
	logging.DryRunSwitchConsole(consoleNum)
	return nil
}

func (d *DryRunExecution) ExecuteSystemCommand(command string, logger *logging.ContextualLogger) error {
	logger.Info("💻 [DRY-RUN] System command simulation", "command", command)
	logging.DryRunExecute(command)
	return nil
}

func (d *DryRunExecution) Exit(exitCode int, logger *logging.ContextualLogger) error {
	logger.Info("🚪 [DRY-RUN] Exit simulation", "exit_code", exitCode)
	logging.DryRunExit(exitCode)
	return fmt.Errorf("script exit: code %d", exitCode)
}

func (d *DryRunExecution) SetVariable(name string, value string, variables *VariableExpander, logger *logging.ContextualLogger) error {
	logger.Info("📝 [DRY-RUN] Variable assignment simulation", "name", name, "value", value)
	logging.DryRunSetVariable(name, value)
	// Actually set the variable in dry-run mode for better simulation
	variables.Set(name, value)
	return nil
}

func (d *DryRunExecution) Break(logger *logging.ContextualLogger) error {
	logger.Info("🔄 [DRY-RUN] Break simulation")
	logging.DryRunBreak()
	return BreakLoopError{}
}

func (d *DryRunExecution) Return(logger *logging.ContextualLogger) error {
	logger.Info("↩ [DRY-RUN] Return simulation")
	logging.DryRunReturn()
	return FunctionReturnError{}
}

func (d *DryRunExecution) IncludeScript(path string, logger *logging.ContextualLogger) (*Script, error) {
	logger.Info("📋 [DRY-RUN] Include simulation", "path", path)
	logging.UserInfof("📋 [DRY-RUN] Would include script: %s", path)

	// For dry-run, actually parse the script for validation
	return nil, fmt.Errorf("IncludeScript simulation not implemented yet")
}

// Helper functions

// mapKeyName maps script2 key names to QMP key names
func mapKeyName(scriptKey string) string {
	keyMap := map[string]string{
		"enter":     "ret",
		"tab":       "tab",
		"space":     "spc",
		"escape":    "esc",
		"esc":       "esc",
		"backspace": "backspace",
		"delete":    "delete",
		"home":      "home",
		"end":       "end",
		"pageup":    "pgup",
		"pagedown":  "pgdn",
		"insert":    "insert",
		"up":        "up",
		"down":      "down",
		"left":      "left",
		"right":     "right",
		"f1":        "f1",
		"f2":        "f2",
		"f3":        "f3",
		"f4":        "f4",
		"f5":        "f5",
		"f6":        "f6",
		"f7":        "f7",
		"f8":        "f8",
		"f9":        "f9",
		"f10":       "f10",
		"f11":       "f11",
		"f12":       "f12",
	}

	// Check exact match first
	if qmpKey, exists := keyMap[strings.ToLower(scriptKey)]; exists {
		return qmpKey
	}

	// Handle ctrl+key combinations (e.g., "ctrl+c" -> "ctrl-c")
	if strings.HasPrefix(strings.ToLower(scriptKey), "ctrl+") {
		key := strings.ToLower(scriptKey[5:]) // Remove "ctrl+" prefix
		return "ctrl-" + key
	}

	// Handle alt+key combinations (e.g., "alt+f4" -> "alt-f4")
	if strings.HasPrefix(strings.ToLower(scriptKey), "alt+") {
		key := strings.ToLower(scriptKey[4:]) // Remove "alt+" prefix
		return "alt-" + key
	}

	return "" // Unsupported key
}

// getKeyDelay returns the key delay (TODO: make configurable)
func getKeyDelay() time.Duration {
	return 50 * time.Millisecond
}
