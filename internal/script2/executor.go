package script2

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// Execute executes a parsed script
func (e *Executor) Execute(script *Script) (*ExecutionResult, error) {
	startTime := time.Now()
	result := &ExecutionResult{
		Success:       true,
		LinesExecuted: 0,
		ExitCode:      0,
		Variables:     make(map[string]string),
	}

	// Create contextual logger
	logger := logging.NewContextualLogger(e.context.VMID, "script2_execution")
	logger.Info("Starting script execution",
		"total_lines", len(script.Lines),
		"dry_run", e.context.DryRun,
		"timeout", e.context.Timeout)

	if e.context.DryRun {
		return e.executeDryRun(script, result, startTime)
	}

	// Execute each line
	for i, line := range script.Lines {
		e.context.CurrentLine = i + 1

		// Check for timeout
		if time.Since(startTime) > e.context.Timeout {
			result.Success = false
			result.Error = fmt.Sprintf("Script execution timed out after %v", e.context.Timeout)
			break
		}

		// Skip empty lines and comments
		if line.Type == EmptyLine || line.Type == CommentLine {
			continue
		}

		logger.Debug("Executing line",
			"line_number", line.LineNumber,
			"type", line.Type.String(),
			"content", line.Content)

		if err := e.executeLine(line, logger); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("line %d: %s", line.LineNumber, err.Error())
			logger.Error("Line execution failed",
				"line_number", line.LineNumber,
				"error", err)
			break
		}

		result.LinesExecuted++
	}

	result.Duration = time.Since(startTime)
	result.Variables = e.context.Variables.GetAllVariables()

	logger.Info("Script execution completed",
		"success", result.Success,
		"lines_executed", result.LinesExecuted,
		"duration", result.Duration,
		"exit_code", result.ExitCode)

	return result, nil
}

// executeDryRun performs a dry run execution (validation only)
func (e *Executor) executeDryRun(script *Script, result *ExecutionResult, startTime time.Time) (*ExecutionResult, error) {
	logger := logging.NewContextualLogger(e.context.VMID, "script2_dry_run")

	logger.Info("🔍 Dry run execution - validating script structure")

	validationResult := e.parser.ValidateScript(script)

	// Log validation results
	if len(validationResult.Errors) > 0 {
		result.Success = false
		result.Error = fmt.Sprintf("Script validation failed with %d errors", len(validationResult.Errors))

		for _, err := range validationResult.Errors {
			logger.Error("Validation error",
				"line_number", err.LineNumber,
				"message", err.Message,
				"suggestion", err.Suggestion)
		}
	}

	for _, warning := range validationResult.Warnings {
		logger.Warn("Validation warning",
			"line_number", warning.LineNumber,
			"message", warning.Message,
			"suggestion", warning.Suggestion)
	}

	// Simulate execution for timing
	for _, line := range script.Lines {
		if line.Type != EmptyLine && line.Type != CommentLine {
			result.LinesExecuted++
		}
	}

	result.Duration = time.Since(startTime)
	result.Variables = e.context.Variables.GetAllVariables()

	logger.Info("Dry run completed",
		"valid", validationResult.Valid,
		"lines_validated", result.LinesExecuted,
		"variables_used", len(validationResult.Variables),
		"directives_found", len(validationResult.Directives))

	return result, nil
}

// executeLine executes a single parsed line
func (e *Executor) executeLine(line ParsedLine, logger *logging.ContextualLogger) error {
	switch line.Type {
	case TextLine:
		return e.executeTextLine(line, logger)
	case VariableLine:
		return e.executeVariableLine(line, logger)
	case DirectiveLine:
		return e.executeDirectiveLine(line, logger)
	case ConditionalLine:
		return e.executeConditionalLine(line, logger)
	default:
		logger.Debug("Skipping line", "type", line.Type.String())
		return nil
	}
}

// executeTextLine sends text to the VM
func (e *Executor) executeTextLine(line ParsedLine, logger *logging.ContextualLogger) error {
	text := line.ExpandedText
	if text == "" {
		return nil // Empty text, nothing to send
	}

	logger.Debug("Sending text to VM", "text", text, "length", len(text))

	// Use the existing QMP client method for sending strings
	if err := e.context.Client.SendString(text, getKeyDelay()); err != nil {
		return fmt.Errorf("failed to send text to VM: %w", err)
	}

	// Log the keyboard input for debugging
	logging.LogKeyboard(e.context.VMID, text, "text", getKeyDelay())

	return nil
}

// executeVariableLine processes variable assignments
func (e *Executor) executeVariableLine(line ParsedLine, logger *logging.ContextualLogger) error {
	if line.Variables == nil {
		return fmt.Errorf("variable line missing variable data")
	}

	for name, value := range line.Variables {
		e.context.Variables.Set(name, value)
		logger.Debug("Set variable", "name", name, "value", value)
	}

	return nil
}

// executeDirectiveLine executes directive commands
func (e *Executor) executeDirectiveLine(line ParsedLine, logger *logging.ContextualLogger) error {
	if line.Directive == nil {
		return fmt.Errorf("directive line missing directive data")
	}

	directive := line.Directive
	logger.Debug("Executing directive",
		"type", directive.Type.String(),
		"command", directive.Command)

	switch directive.Type {
	case KeySequence:
		return e.executeKeySequence(directive, logger)
	case Watch:
		return e.executeWatch(directive, logger)
	case Console:
		return e.executeConsole(directive, logger)
	case WaitDelay:
		return e.executeWait(directive, logger)
	case Exit:
		return e.executeExit(directive, logger)
	default:
		return fmt.Errorf("unsupported directive type: %s", directive.Type.String())
	}
}

// executeKeySequence sends special key sequences
func (e *Executor) executeKeySequence(directive *Directive, logger *logging.ContextualLogger) error {
	keyName := directive.KeyName
	logger.Debug("Sending key sequence", "key", keyName)

	// Map script2 key names to QMP key names
	qmpKey := mapKeyName(keyName)
	if qmpKey == "" {
		return fmt.Errorf("unsupported key sequence: %s", keyName)
	}

	// Send the key using QMP client
	if err := e.context.Client.SendKey(qmpKey); err != nil {
		return fmt.Errorf("failed to send key '%s': %w", keyName, err)
	}

	// Log the keyboard input
	logging.LogKeyboard(e.context.VMID, keyName, "key_sequence", getKeyDelay())

	return nil
}

// executeWatch waits for text to appear on screen using OCR
func (e *Executor) executeWatch(directive *Directive, logger *logging.ContextualLogger) error {
	searchText := directive.SearchText
	timeout := directive.Timeout

	logger.Info("Watching for text", "text", searchText, "timeout", timeout)

	// Take screenshot and perform OCR search
	startTime := time.Now()
	found := false

	for time.Since(startTime) < timeout && !found {
		// Take temporary screenshot
		tempFile, err := TakeTemporaryScreenshot(e.context.Client, "script2-watch")
		if err != nil {
			logger.Warn("Failed to take screenshot for watch", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}()

		// Perform OCR on the screenshot
		// Use training data from context, fall back to default if not provided
		trainingDataPath := e.context.TrainingData
		if trainingDataPath == "" {
			trainingDataPath = qmp.GetDefaultTrainingDataPath()
			logger.Info("Using default training data location", "path", trainingDataPath)
			// Also show user-visible message
			logging.UserInfo("📂 Using default training data: %s", trainingDataPath)
		}
		width := 160  // TODO: Make configurable from context
		height := 50  // TODO: Make configurable from context

		// Process the screenshot with OCR
		results, err := ocr.ProcessScreenshotWithTrainingData(tempFile.Name(), trainingDataPath, width, height)
		if err != nil {
			logger.Warn("OCR processing failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Search for the target text by joining all lines
		screenText := strings.Join(results.Text, "\n")
		if strings.Contains(screenText, searchText) {
			found = true
			logger.Info("Watch condition satisfied",
				"text", searchText,
				"elapsed", time.Since(startTime))
			break
		}

		// Brief pause before next attempt
		time.Sleep(500 * time.Millisecond)
	}

	if !found {
		return fmt.Errorf("watch timeout: text '%s' not found within %v", searchText, timeout)
	}

	return nil
}

// executeConsole switches console
func (e *Executor) executeConsole(directive *Directive, logger *logging.ContextualLogger) error {
	consoleNum := directive.ConsoleNum
	logger.Debug("Switching console", "console", consoleNum)

	// Map console number to function key
	var fKey string
	switch consoleNum {
	case 1:
		fKey = "f1"
	case 2:
		fKey = "f2"
	case 3:
		fKey = "f3"
	case 4:
		fKey = "f4"
	case 5:
		fKey = "f5"
	case 6:
		fKey = "f6"
	default:
		return fmt.Errorf("invalid console number: %d (must be 1-6)", consoleNum)
	}

	// Send Ctrl+Alt+F[n] sequence using key combo
	keys := []string{"ctrl", "alt", fKey}
	if err := e.context.Client.SendKeyCombo(keys); err != nil {
		return fmt.Errorf("failed to switch to console %d: %w", consoleNum, err)
	}

	logger.Info("Switched to console", "console", consoleNum, "key", "ctrl+alt+"+fKey)
	return nil
}

// executeWait pauses execution
func (e *Executor) executeWait(directive *Directive, logger *logging.ContextualLogger) error {
	duration := directive.Timeout
	logger.Debug("Waiting", "duration", duration)

	time.Sleep(duration)
	return nil
}

// executeExit terminates script execution
func (e *Executor) executeExit(directive *Directive, logger *logging.ContextualLogger) error {
	exitCode := directive.ExitCode
	logger.Info("Script exit requested", "exit_code", exitCode)

	// Set exit code in context (this will be handled by the caller)
	return fmt.Errorf("script exit: code %d", exitCode)
}

// executeConditionalLine executes conditional logic
func (e *Executor) executeConditionalLine(line ParsedLine, logger *logging.ContextualLogger) error {
	// TODO: Implement conditional execution in Phase 3
	logger.Warn("Conditional execution not yet implemented", "line", line.LineNumber)
	return fmt.Errorf("conditional execution not implemented yet: %s", line.Content)
}

// Helper functions

// mapKeyName maps script2 key names to QMP key names
func mapKeyName(scriptKey string) string {
	keyMap := map[string]string{
		"enter":     "ret",
		"tab":       "tab",
		"space":     "spc",
		"escape":    "esc",
		"backspace": "backspace",
		"delete":    "delete",
		"up":        "up",
		"down":      "down",
		"left":      "left",
		"right":     "right",
		"home":      "home",
		"end":       "end",
		"page_up":   "pgup",
		"page_down": "pgdn",
	}

	// Handle function keys
	if strings.HasPrefix(scriptKey, "f") && len(scriptKey) >= 2 {
		return scriptKey // f1, f2, etc. are the same
	}

	// Handle ctrl+key combinations
	if strings.HasPrefix(scriptKey, "ctrl+") && len(scriptKey) == 6 {
		key := scriptKey[5:] // Extract the single character
		return "ctrl-" + key
	}

	// Handle alt+key combinations
	if strings.HasPrefix(scriptKey, "alt+") && len(scriptKey) == 5 {
		key := scriptKey[4:] // Extract the single character
		return "alt-" + key
	}

	// Handle shift+key combinations
	if strings.HasPrefix(scriptKey, "shift+") && len(scriptKey) == 7 {
		key := scriptKey[6:] // Extract the single character
		return "shift-" + key
	}

	// Look up in the map
	if qmpKey, exists := keyMap[scriptKey]; exists {
		return qmpKey
	}

	return "" // Unsupported key
}

// getKeyDelay returns the key delay (TODO: make configurable)
func getKeyDelay() time.Duration {
	return 50 * time.Millisecond
}

// TakeTemporaryScreenshot is a helper function (TODO: integrate with resource management)
func TakeTemporaryScreenshot(client *qmp.Client, prefix string) (*os.File, error) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", prefix+"-*.ppm")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %w", err)
	}

	// Take screenshot to temporary file
	if err := client.ScreenDump(tmpFile.Name(), ""); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("error taking screenshot: %w", err)
	}

	return tmpFile, nil
}
