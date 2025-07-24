package script2

import (
	"fmt"
	"os"
	"os/exec"
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
	case ConditionalIfFound:
		return e.executeConditionalIfFound(directive, logger)
	case ConditionalIfNotFound:
		return e.executeConditionalIfNotFound(directive, logger)
	case Retry:
		return e.executeRetry(directive, logger)
	case Repeat:
		return e.executeRepeat(directive, logger)
	case WhileFound:
		return e.executeWhileFound(directive, logger)
	case WhileNotFound:
		return e.executeWhileNotFound(directive, logger)
	case Include:
		return e.executeInclude(directive, logger)
	case Screenshot:
		return e.executeScreenshot(directive, logger)
	case FunctionCall:
		return e.executeFunctionCall(directive, logger)
	case FunctionDef, EndFunction:
		// Function definitions are handled during parsing, should not be executed
		logger.Debug("Skipping function definition directive (handled during parsing)")
		return nil
	case Else:
		// Else directives are handled by their parent conditional, should not be executed directly
		logger.Debug("Skipping else directive (handled by parent conditional)")
		return nil
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

// executeConditionalIfFound executes if-found conditional logic
func (e *Executor) executeConditionalIfFound(directive *Directive, logger *logging.ContextualLogger) error {
	searchText := directive.SearchText
	timeout := directive.Timeout

	logger.Info("Executing if-found conditional", "text", searchText, "timeout", timeout)

	// Perform OCR search to check if text is found
	found, err := e.performOCRSearch(searchText, logger)
	if err != nil {
		logger.Warn("OCR search failed in if-found conditional", "error", err)
		// If OCR fails, we consider it as "not found" and don't execute
		found = false
	}

	if found {
		logger.Info("Condition met: text found on screen", "text", searchText)
		logging.UserInfo("✓ Found '%s' on screen - executing if-found block (%d lines)", searchText, len(directive.Block))

		// Execute the conditional block
		if len(directive.Block) > 0 {
			return e.executeConditionalBlock(directive.Block, logger)
		}
	} else {
		logger.Info("Condition not met: text not found on screen", "text", searchText)

		// Execute else block if present
		if len(directive.ElseBlock) > 0 {
			logging.UserInfo("✗ Text '%s' not found - executing else block (%d lines)", searchText, len(directive.ElseBlock))
			return e.executeConditionalBlock(directive.ElseBlock, logger)
		} else {
			logging.UserInfo("✗ Text '%s' not found - skipping if-found block (%d lines)", searchText, len(directive.Block))
		}
	}

	return nil
}

// executeConditionalIfNotFound executes if-not-found conditional logic
func (e *Executor) executeConditionalIfNotFound(directive *Directive, logger *logging.ContextualLogger) error {
	searchText := directive.SearchText
	timeout := directive.Timeout

	logger.Info("Executing if-not-found conditional", "text", searchText, "timeout", timeout)

	// Perform OCR search to check if text is found
	found, err := e.performOCRSearch(searchText, logger)
	if err != nil {
		logger.Warn("OCR search failed in if-not-found conditional", "error", err)
		// If OCR fails, we consider it as "not found"
		found = false
	}

	if !found {
		logger.Info("Condition met: text not found on screen", "text", searchText)
		logging.UserInfo("✓ Text '%s' not found - executing if-not-found block (%d lines)", searchText, len(directive.Block))

		// Execute the conditional block
		if len(directive.Block) > 0 {
			return e.executeConditionalBlock(directive.Block, logger)
		}
	} else {
		logger.Info("Condition not met: text found on screen", "text", searchText)

		// Execute else block if present
		if len(directive.ElseBlock) > 0 {
			logging.UserInfo("✗ Found '%s' on screen - executing else block (%d lines)", searchText, len(directive.ElseBlock))
			return e.executeConditionalBlock(directive.ElseBlock, logger)
		} else {
			logging.UserInfo("✗ Found '%s' on screen - skipping if-not-found block (%d lines)", searchText, len(directive.Block))
		}
	}

	return nil
}

// executeConditionalBlock executes a block of parsed lines (for conditional execution)
func (e *Executor) executeConditionalBlock(block []ParsedLine, logger *logging.ContextualLogger) error {
	logger.Debug("Executing conditional block", "lines", len(block))

	for _, line := range block {
		logger.Debug("Executing conditional block line",
			"line_number", line.LineNumber,
			"type", line.Type.String(),
			"content", line.Content)

		if err := e.executeLine(line, logger); err != nil {
			return fmt.Errorf("error executing conditional block line %d: %w", line.LineNumber, err)
		}
	}

	return nil
}

// performOCRSearch performs OCR on current screen and searches for text
func (e *Executor) performOCRSearch(searchText string, logger *logging.ContextualLogger) (bool, error) {
	// Take temporary screenshot
	tempFile, err := TakeTemporaryScreenshot(e.context.Client, "script2-conditional")
	if err != nil {
		return false, fmt.Errorf("failed to take screenshot for conditional: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	// Use training data from context, fall back to default if not provided
	trainingDataPath := e.context.TrainingData
	if trainingDataPath == "" {
		trainingDataPath = qmp.GetDefaultTrainingDataPath()
		logger.Debug("Using default training data for conditional", "path", trainingDataPath)
	}

	width := 160  // TODO: Make configurable from context
	height := 50  // TODO: Make configurable from context

	// Process the screenshot with OCR
	results, err := ocr.ProcessScreenshotWithTrainingData(tempFile.Name(), trainingDataPath, width, height)
	if err != nil {
		return false, fmt.Errorf("OCR processing failed for conditional: %w", err)
	}

	// Search for the target text by joining all lines
	screenText := strings.Join(results.Text, "\n")
	found := strings.Contains(screenText, searchText)

	logger.Debug("OCR search completed for conditional",
		"search_text", searchText,
		"found", found,
		"screen_lines", len(results.Text))

	return found, nil
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

// executeRetry executes a retry block with specified count
func (e *Executor) executeRetry(directive *Directive, logger *logging.ContextualLogger) error {
	retryCount := directive.RetryCount
	logger.Info("Executing retry block", "retry_count", retryCount, "block_lines", len(directive.Block))

	for attempt := 1; attempt <= retryCount; attempt++ {
		logger.Info("Retry attempt", "attempt", attempt, "max", retryCount)
		logging.UserInfo("🔄 Retry attempt %d of %d", attempt, retryCount)

		// Execute the block
		err := e.executeConditionalBlock(directive.Block, logger)
		if err == nil {
			logger.Info("Retry block succeeded", "attempt", attempt)
			logging.UserInfo("✓ Retry block succeeded on attempt %d", attempt)
			return nil
		}

		logger.Warn("Retry attempt failed", "attempt", attempt, "error", err)
		if attempt < retryCount {
			logging.UserInfo("✗ Attempt %d failed, retrying...", attempt)
			time.Sleep(1 * time.Second) // Brief pause between retries
		}
	}

	return fmt.Errorf("retry block failed after %d attempts", retryCount)
}

// executeRepeat executes a repeat block specified number of times
func (e *Executor) executeRepeat(directive *Directive, logger *logging.ContextualLogger) error {
	repeatCount := directive.RepeatCount
	logger.Info("Executing repeat block", "repeat_count", repeatCount, "block_lines", len(directive.Block))
	logging.UserInfo("🔁 Repeating block %d times", repeatCount)

	for iteration := 1; iteration <= repeatCount; iteration++ {
		logger.Info("Repeat iteration", "iteration", iteration, "max", repeatCount)
		logging.UserInfo("🔁 Iteration %d of %d", iteration, repeatCount)

		// Execute the block
		if err := e.executeConditionalBlock(directive.Block, logger); err != nil {
			logger.Error("Repeat iteration failed", "iteration", iteration, "error", err)
			return fmt.Errorf("repeat block failed on iteration %d: %w", iteration, err)
		}

		// Brief pause between iterations (except the last one)
		if iteration < repeatCount {
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Info("Repeat block completed successfully", "iterations", repeatCount)
	logging.UserInfo("✓ Repeat block completed successfully (%d iterations)", repeatCount)
	return nil
}

// executeWhileFound executes a block while text is found on screen
func (e *Executor) executeWhileFound(directive *Directive, logger *logging.ContextualLogger) error {
	searchText := directive.SearchText
	timeout := directive.Timeout
	pollInterval := directive.PollInterval

	logger.Info("Executing while-found loop",
		"text", searchText,
		"timeout", timeout,
		"poll_interval", pollInterval,
		"block_lines", len(directive.Block))
	logging.UserInfo("🔄 While-found loop: searching for '%s' (timeout: %v)", searchText, timeout)

	startTime := time.Now()
	iteration := 0

	for time.Since(startTime) < timeout {
		iteration++

		// Check if text is found on screen
		found, err := e.performOCRSearch(searchText, logger)
		if err != nil {
			logger.Warn("OCR search failed in while-found", "iteration", iteration, "error", err)
			time.Sleep(pollInterval)
			continue
		}

		if !found {
			logger.Info("While-found condition no longer met", "text", searchText, "iterations", iteration)
			logging.UserInfo("✓ While-found loop completed: '%s' no longer found (%d iterations)", searchText, iteration)
			return nil
		}

		logger.Debug("While-found condition met, executing block", "iteration", iteration)

		// Execute the block
		if err := e.executeConditionalBlock(directive.Block, logger); err != nil {
			logger.Error("While-found block execution failed", "iteration", iteration, "error", err)
			return fmt.Errorf("while-found block failed on iteration %d: %w", iteration, err)
		}

		// Wait for poll interval before next check
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("while-found loop timed out after %v (text '%s' still found)", timeout, searchText)
}

// executeWhileNotFound executes a block while text is not found on screen
func (e *Executor) executeWhileNotFound(directive *Directive, logger *logging.ContextualLogger) error {
	searchText := directive.SearchText
	timeout := directive.Timeout
	pollInterval := directive.PollInterval

	logger.Info("Executing while-not-found loop",
		"text", searchText,
		"timeout", timeout,
		"poll_interval", pollInterval,
		"block_lines", len(directive.Block))
	logging.UserInfo("🔄 While-not-found loop: waiting for '%s' to appear (timeout: %v)", searchText, timeout)

	startTime := time.Now()
	iteration := 0

	for time.Since(startTime) < timeout {
		iteration++

		// Check if text is found on screen
		found, err := e.performOCRSearch(searchText, logger)
		if err != nil {
			logger.Warn("OCR search failed in while-not-found", "iteration", iteration, "error", err)
			time.Sleep(pollInterval)
			continue
		}

		if found {
			logger.Info("While-not-found condition no longer met", "text", searchText, "iterations", iteration)
			logging.UserInfo("✓ While-not-found loop completed: '%s' found (%d iterations)", searchText, iteration)
			return nil
		}

		logger.Debug("While-not-found condition met, executing block", "iteration", iteration)

		// Execute the block
		if err := e.executeConditionalBlock(directive.Block, logger); err != nil {
			logger.Error("While-not-found block execution failed", "iteration", iteration, "error", err)
			return fmt.Errorf("while-not-found block failed on iteration %d: %w", iteration, err)
		}

		// Wait for poll interval before next check
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("while-not-found loop timed out after %v (text '%s' still not found)", timeout, searchText)
}

// executeInclude executes an included script file
func (e *Executor) executeInclude(directive *Directive, logger *logging.ContextualLogger) error {
	includePath := directive.IncludePath
	logger.Info("Including script file", "path", includePath)
	logging.UserInfo("📄 Including script: %s", includePath)

	// Read the included script file
	scriptContent, err := os.ReadFile(includePath)
	if err != nil {
		logger.Error("Failed to read included script", "path", includePath, "error", err)
		return fmt.Errorf("failed to read included script '%s': %w", includePath, err)
	}

	// Create a new parser with the same variable context
	parser := NewParser(e.context.Variables, e.debug)

	// Parse the included script
	includedScript, err := parser.ParseScript(string(scriptContent))
	if err != nil {
		logger.Error("Failed to parse included script", "path", includePath, "error", err)
		return fmt.Errorf("failed to parse included script '%s': %w", includePath, err)
	}

	logger.Info("Parsed included script",
		"path", includePath,
		"total_lines", includedScript.Metadata.TotalLines,
		"text_lines", includedScript.Metadata.TextLines,
		"directive_lines", includedScript.Metadata.DirectiveLines)

	// Store original line context
	originalLine := e.context.CurrentLine

	// Execute each line of the included script
	for i, line := range includedScript.Lines {
		// Update current line context
		e.context.CurrentLine = i + 1

		logger.Debug("Executing included script line",
			"included_file", includePath,
			"line_number", line.LineNumber,
			"type", line.Type.String(),
			"content", line.Content)

		// Skip empty lines and comments
		if line.Type == EmptyLine || line.Type == CommentLine {
			continue
		}

		if err := e.executeLine(line, logger); err != nil {
			logger.Error("Included script line execution failed",
				"included_file", includePath,
				"line_number", line.LineNumber,
				"error", err)
			// Restore original line context
			e.context.CurrentLine = originalLine
			return fmt.Errorf("included script '%s' line %d: %s", includePath, line.LineNumber, err.Error())
		}
	}

	// Restore original line context
	e.context.CurrentLine = originalLine

	logger.Info("Included script executed successfully", "path", includePath, "lines_executed", len(includedScript.Lines))
	logging.UserInfo("✓ Included script '%s' completed successfully", includePath)

	return nil
}

// executeScreenshot takes a screenshot and saves it to the specified file
func (e *Executor) executeScreenshot(directive *Directive, logger *logging.ContextualLogger) error {
	screenshotPath := directive.ScreenshotPath
	format := directive.ScreenshotFormat

	logger.Info("Taking screenshot", "path", screenshotPath, "format", format)
	logging.UserInfo("📸 Taking screenshot: %s (%s)", screenshotPath, format)

	// Expand variables in the screenshot path
	expandedPath, err := e.context.Variables.Expand(screenshotPath)
	if err != nil {
		logger.Error("Failed to expand screenshot path", "path", screenshotPath, "error", err)
		return fmt.Errorf("failed to expand screenshot path '%s': %w", screenshotPath, err)
	}

	// Handle timestamp substitution in filename
	expandedPath = e.expandTimestamp(expandedPath)

	logger.Debug("Screenshot details",
		"original_path", screenshotPath,
		"expanded_path", expandedPath,
		"format", format)

	// Take screenshot based on format
	switch format {
	case "ppm":
		// Direct PPM screenshot (fastest)
		if err := e.context.Client.ScreenDump(expandedPath, ""); err != nil {
			logger.Error("Failed to take PPM screenshot", "path", expandedPath, "error", err)
			return fmt.Errorf("failed to take PPM screenshot '%s': %w", expandedPath, err)
		}

	case "png", "jpg":
		// Take PPM first, then convert
		tempFile, err := TakeTemporaryScreenshot(e.context.Client, "script2-screenshot")
		if err != nil {
			logger.Error("Failed to take temporary screenshot", "error", err)
			return fmt.Errorf("failed to take temporary screenshot: %w", err)
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}()

		// Convert using ImageMagick convert command
		convertCmd := fmt.Sprintf("convert \"%s\" \"%s\"", tempFile.Name(), expandedPath)
		if err := e.executeSystemCommand(convertCmd, logger); err != nil {
			logger.Error("Failed to convert screenshot",
				"temp_file", tempFile.Name(),
				"target_path", expandedPath,
				"format", format,
				"error", err)
			return fmt.Errorf("failed to convert screenshot to %s format: %w", format, err)
		}

	default:
		return fmt.Errorf("unsupported screenshot format: %s (supported: ppm, png, jpg)", format)
	}

	// Verify file was created
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		logger.Error("Screenshot file was not created", "path", expandedPath)
		return fmt.Errorf("screenshot file was not created: %s", expandedPath)
	}

	// Get file size for logging
	if fileInfo, err := os.Stat(expandedPath); err == nil {
		logger.Info("Screenshot saved successfully",
			"path", expandedPath,
			"format", format,
			"size_bytes", fileInfo.Size())
		logging.UserInfo("✓ Screenshot saved: %s (%s format, %d bytes)",
			expandedPath, format, fileInfo.Size())
	} else {
		logger.Info("Screenshot saved successfully", "path", expandedPath, "format", format)
		logging.UserInfo("✓ Screenshot saved: %s (%s format)", expandedPath, format)
	}

	return nil
}

// executeSystemCommand executes a system command (helper for screenshot conversion)
func (e *Executor) executeSystemCommand(command string, logger *logging.ContextualLogger) error {
	if e.context.DryRun {
		logger.Info("Dry run: would execute system command", "command", command)
		return nil
	}

	logger.Debug("Executing system command", "command", command)

	// Use shell to execute the command
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("System command failed",
			"command", command,
			"error", err,
			"output", string(output))
		return fmt.Errorf("system command failed: %w (output: %s)", err, string(output))
	}

	if len(output) > 0 {
		logger.Debug("System command output", "command", command, "output", string(output))
	}

	return nil
}

// expandTimestamp replaces timestamp placeholders in filenames
func (e *Executor) expandTimestamp(filename string) string {
	now := time.Now()

	// Replace common timestamp patterns
	filename = strings.ReplaceAll(filename, "{timestamp}", now.Format("20060102_150405"))
	filename = strings.ReplaceAll(filename, "{date}", now.Format("20060102"))
	filename = strings.ReplaceAll(filename, "{time}", now.Format("150405"))
	filename = strings.ReplaceAll(filename, "{datetime}", now.Format("2006-01-02_15-04-05"))
	filename = strings.ReplaceAll(filename, "{unix}", fmt.Sprintf("%d", now.Unix()))

	return filename
}

// executeFunctionCall executes a function call with parameter passing
func (e *Executor) executeFunctionCall(directive *Directive, logger *logging.ContextualLogger) error {
	functionName := directive.FunctionName
	args := directive.FunctionArgs

	logger.Info("Calling function", "name", functionName, "args", args)
	logging.UserInfo("📞 Calling function: %s(%s)", functionName, strings.Join(args, ", "))

	// Check if function exists
	if e.script == nil {
		return fmt.Errorf("no script context available for function call")
	}

	function, exists := e.script.Functions[functionName]
	if !exists {
		return fmt.Errorf("function '%s' is not defined", functionName)
	}

	logger.Debug("Function found",
		"name", functionName,
		"defined_at", function.LineStart,
		"body_lines", len(function.Lines))

	// Create function call context
	callContext := &FunctionCallContext{
		FunctionName: functionName,
		Parameters:   args,
		LocalVars:    make(map[string]string),
		CallLine:     e.context.CurrentLine,
	}

	// Set up positional parameters ($1, $2, $3, etc.)
	for i, arg := range args {
		paramName := fmt.Sprintf("%d", i+1)
		// Expand variables in the argument before setting it as a parameter
		expandedArg, err := e.context.Variables.Expand(arg)
		if err != nil {
			logger.Error("Failed to expand function argument",
				"function", functionName,
				"arg_index", i+1,
				"arg", arg,
				"error", err)
			return fmt.Errorf("failed to expand function argument %d '%s': %w", i+1, arg, err)
		}
		callContext.LocalVars[paramName] = expandedArg
		logger.Debug("Set function parameter",
			"function", functionName,
			"param", paramName,
			"value", expandedArg)
	}

	// Push function context onto stack
	e.context.FunctionStack = append(e.context.FunctionStack, callContext)
	defer func() {
		// Pop function context when done
		if len(e.context.FunctionStack) > 0 {
			e.context.FunctionStack = e.context.FunctionStack[:len(e.context.FunctionStack)-1]
		}
	}()

	// Create a temporary variable expander with function parameters
	originalVariables := e.context.Variables
	functionVariables := e.createFunctionVariableExpander(callContext)
	e.context.Variables = functionVariables
	defer func() {
		// Restore original variables
		e.context.Variables = originalVariables
	}()

	// Execute function body
	logger.Info("Executing function body", "name", functionName, "lines", len(function.Lines))

	for _, line := range function.Lines {
		logger.Debug("Executing function line",
			"function", functionName,
			"line_number", line.LineNumber,
			"type", line.Type.String(),
			"content", line.Content)

		if err := e.executeLine(line, logger); err != nil {
			logger.Error("Function execution failed",
				"function", functionName,
				"line_number", line.LineNumber,
				"error", err)
			return fmt.Errorf("function '%s' line %d: %w", functionName, line.LineNumber, err)
		}
	}

	logger.Info("Function completed successfully", "name", functionName)
	logging.UserInfo("✓ Function '%s' completed successfully", functionName)

	return nil
}

// createFunctionVariableExpander creates a variable expander with function parameters and local scope
func (e *Executor) createFunctionVariableExpander(callContext *FunctionCallContext) *VariableExpander {
	// Create new maps for function scope
	environment := make(map[string]string)
	variables := make(map[string]string)
	overrides := make(map[string]string)

	// Copy environment variables (shared)
	for k, v := range e.context.Variables.environment {
		environment[k] = v
	}

	// Copy script variables (shared)
	for k, v := range e.context.Variables.variables {
		variables[k] = v
	}

	// Copy command-line overrides (shared)
	for k, v := range e.context.Variables.overrides {
		overrides[k] = v
	}

	// Add function parameters as local overrides (highest priority)
	for k, v := range callContext.LocalVars {
		overrides[k] = v
	}

	return NewVariableExpander(environment, variables, overrides)
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
