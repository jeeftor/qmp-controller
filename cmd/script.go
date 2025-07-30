package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/script"
	"github.com/jeeftor/qmp-controller/internal/utils"
	"github.com/jeeftor/qmp-controller/internal/validation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scriptDelay time.Duration
	commentStr  string
	controlStr  string
	useTUI         bool
	useEnhancedTUI bool

	// WATCH command OCR settings
	scriptOCRConfig   = ocr.NewOCRConfig()
	watchPollInterval time.Duration
)

// scriptCmd represents the script command
var scriptCmd = &cobra.Command{
	Use:   "script [vmid] [script-file] [training-data-file]",
	Short: "Run a script of commands",
	Long: `Run a script of commands on the VM.
Each line in the script file is treated as a separate command to be executed.
Empty lines and comment lines are ignored.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.
The training data file can be provided as an argument or set via the QMP_TRAINING_DATA environment variable.

Control commands use configurable prefix (default: <#):
  <# Sleep N         - Sleep for N seconds
  <# Console N       - Switch to console N (1-6) using Ctrl+Alt+F[N]
  <# Key combo       - Send key combination (e.g., <# Key ctrl-alt-del)
  <# WATCH "string" TIMEOUT 30s - Wait for string to appear with timeout

Comments use configurable character (default: #):
  # This is a comment
  \# This types a literal # character

Examples:
  # Explicit arguments
  qmp script 106 automation.txt training.json

  # Using environment variables
  export QMP_VM_ID=106
  export QMP_TRAINING_DATA=training.json
  qmp script automation.txt

Training Data Requirements:
  Scripts containing WATCH commands or any OCR functionality require training data:
  - Provide as third argument: qmp script [vmid] [script-file] [training-data-file]
  - Or set in config file
  The script will be pre-scanned for WATCH commands and validate requirements.

Examples:
  # Basic script execution (no OCR/WATCH commands)
  qmp script 106 /path/to/script.txt

  # Script with WATCH commands (training data as argument)
  qmp script 106 script.txt training.json --columns 160 --rows 50

  # Using custom comment/control characters
  qmp script 106 script.txt training.json --comment-char "%" --control-char "<!

Script file example:
  # Comments start with # (configurable)
  <# Sleep 1
  <# Console 1
  <# Key esc
  :w
  <# Key enter
  <# WATCH "saved" TIMEOUT 10s`,
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve parameters using parameter resolver
		resolver := params.NewParameterResolver()

		// Determine parameter layout based on number of arguments
		var vmid, scriptFile, trainingDataPath string

		if len(args) == 3 {
			// Traditional format: vmid scriptfile trainingdata
			vmid = args[0]
			scriptFile = args[1]
			trainingDataPath = args[2]
		} else if len(args) == 2 {
			// Could be: vmid scriptfile OR scriptfile trainingdata
			// Try to resolve VM ID from first arg
			vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
			if err == nil {
				// First arg is valid VM ID
				vmid = vmidInfo.Value
				scriptFile = args[1]
				trainingDataPath = resolver.ResolveTrainingData([]string{}, -1) // From env or default
			} else {
				// First arg is not VM ID, try environment variable
				vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1)
				if err != nil {
					utils.ValidationError(err)
				}
				vmid = vmidInfo.Value
				scriptFile = args[0]
				trainingDataPath = args[1]
			}
		} else if len(args) == 1 {
			// Only script file, VM ID and training data from env vars
			vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1)
			if err != nil {
				utils.ValidationError(err)
			}
			vmid = vmidInfo.Value
			scriptFile = args[0]
			trainingDataPath = resolver.ResolveTrainingData([]string{}, -1)
		} else {
			utils.RequiredParameterError("Script file", "")
		}

		// Set training data path in OCR config
		if trainingDataPath != "" {
			scriptOCRConfig.TrainingDataPath = trainingDataPath
			logging.Debug("Using training data", "path", trainingDataPath)
		}

		// Get configuration values early
		commentPrefix := getCommentChar()
		controlPrefix := getControlChar()
		logging.Info("Script configuration", "comment_char", commentPrefix, "control_char", controlPrefix)
		logging.Debug("Script configuration", "comment", commentPrefix, "control", controlPrefix)

		// Sync OCR configuration from flags early (this may override the argument)
		syncScriptOCRConfig()

		// Pre-scan script file to check for WATCH commands and validate training data
		// Do this BEFORE connecting to VM to fail fast if requirements aren't met
		hasWatchCommands, err := scanScriptForWatchCommands(scriptFile)
		if err != nil {
			utils.FileSystemError("scan script file for WATCH commands", scriptFile, err)
		}

		// Validate training data is available if WATCH commands are present
		if hasWatchCommands {
			if err := validateWatchRequirements(); err != nil {
				utils.FatalErrorWithCode(err, "WATCH command requirements validation failed", utils.ExitCodeValidation)
			}

			// Additional validation for WATCH commands and remote connections
			validator := validation.NewConfigValidator()
			remoteTempPath := getRemoteTempPath()
			socketPath := viper.GetString("socket") // Get socket path from config

			socketValidation := validator.ValidateSocketPath(socketPath, remoteTempPath)
			if !socketValidation.Valid {
				logging.Error("Socket configuration validation failed for WATCH commands",
					"script_file", scriptFile,
					"socket_path", socketPath,
					"remote_temp_path", remoteTempPath,
					"validation_errors", len(socketValidation.Errors))

				fmt.Fprint(os.Stderr, validation.FormatValidationErrors(socketValidation))
				os.Exit(1)
			}

			// Log socket validation warnings
			if len(socketValidation.Warnings) > 0 {
				for _, warning := range socketValidation.Warnings {
					logging.Warn("Socket configuration warning", "warning", warning)
				}
			}
		}

		// Connect to the VM (only after validation passes)
		client, err := ConnectToVM(vmid)
		if err != nil {
			logging.Error("Failed to connect to VM", "vmid", vmid, "error", err)
			os.Exit(1)
		}
		defer client.Close()

		// Initialize the WATCH function pointer for the command pattern
		script.PerformWatchCheckFunc = performWatchCheck

		// Check if TUI mode is requested
		if useTUI || useEnhancedTUI {
			// Parse script file for TUI
			lines, err := parseScriptFile(scriptFile)
			if err != nil {
				logging.Error("Failed to parse script file for TUI mode", "script_file", scriptFile, "error", err)
				os.Exit(1)
			}

			// Create and run TUI (enhanced or standard)
			if useEnhancedTUI {
				model := NewEnhancedScriptTUIModel(vmid, client, scriptFile, lines)
				p := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					logging.Error("Enhanced TUI execution failed", "vmid", vmid, "script_file", scriptFile, "error", err)
					os.Exit(1)
				}
			} else {
				model := NewScriptTUIModel(vmid, client, scriptFile, lines)
				p := tea.NewProgram(model, tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					logging.Error("TUI execution failed", "vmid", vmid, "script_file", scriptFile, "error", err)
					os.Exit(1)
				}
			}
			return
		}

		// Original console mode execution
		// Open the script file
		file, err := os.Open(scriptFile)
		if err != nil {
			logging.Error("Failed to open script file", "script_file", scriptFile, "error", err)
			os.Exit(1)
		}
		defer file.Close()

		// Get the key delay from flag or config
		delay := getScriptDelay()
		logging.Debug("Using key delay for script", "delay", delay)

		// Create script context for command execution
		ctx := &script.ScriptContext{
			VMId:             vmid,
			ScriptFile:       scriptFile,
			TrainingDataPath: scriptOCRConfig.TrainingDataPath,
			OCRColumns:       scriptOCRConfig.Columns,
			OCRRows:          scriptOCRConfig.Rows,
			Delay:            delay,
		}

		// Process the script line by line using the command pattern
		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())

			// Skip empty lines
			if line == "" {
				continue
			}

			// Handle comments and escaped comment characters
			if processedLine, isComment := processCommentLine(line, commentPrefix); isComment {
				if processedLine == "" {
					continue // It was a pure comment, skip
				}
				line = processedLine // It had escaped comment chars, use processed version
			}

			// Update context with current line number
			ctx.LineNumber = lineNum

			// Parse the line into a command using the command pattern
			cmd, err := script.ParseScriptCommand(line, lineNum)
			if err != nil {
				logging.Error("Failed to parse script command",
					"vmid", vmid,
					"script_file", scriptFile,
					"line_number", lineNum,
					"line_content", line,
					"error", err)
				continue
			}

			// Validate the command
			if err := cmd.Validate(); err != nil {
				logging.Error("Script command validation failed",
					"vmid", vmid,
					"script_file", scriptFile,
					"line_number", lineNum,
					"command", cmd.String(),
					"error", err)
				continue
			}

			// Execute the command
			if err := cmd.Execute(client, ctx); err != nil {
				logging.Error("Script command execution failed",
					"vmid", vmid,
					"script_file", scriptFile,
					"line_number", lineNum,
					"command", cmd.String(),
					"error", err)
				continue
			}

			// For TYPE commands, send Enter after each command (maintaining backward compatibility)
			if _, isType := cmd.(*script.TypeCommand); isType {
				if err := client.SendKey("ret"); err != nil {
					logging.Error("Failed to send return key after TYPE command",
						"vmid", vmid,
						"script_file", scriptFile,
						"line_number", lineNum,
						"error", err)
					continue
				}
				// Small delay between commands
				time.Sleep(100 * time.Millisecond)
			}
		}

		if err := scanner.Err(); err != nil {
			logging.Error("Error occurred while reading script file", "script_file", scriptFile, "error", err)
			os.Exit(1)
		}

		logging.Info("Script execution completed successfully", "vmid", vmid, "script_file", scriptFile)
	},
}

// scanScriptForWatchCommands scans a script file to detect if it contains WATCH commands
func scanScriptForWatchCommands(scriptFile string) (bool, error) {
	file, err := os.Open(scriptFile)
	if err != nil {
		return false, fmt.Errorf("failed to open script file: %v", err)
	}
	defer file.Close()

	commentPrefix := getCommentChar()
	controlPrefix := getControlChar()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle comments and escaped comment characters
		if processedLine, isComment := processCommentLine(line, commentPrefix); isComment {
			if processedLine == "" {
				continue // It was a pure comment, skip
			}
			line = processedLine // It had escaped comment chars, use processed version
		}

		// Check if this line is a control command
		if strings.HasPrefix(line, controlPrefix) {
			// Remove the control prefix and parse the command
			commandPart := strings.TrimSpace(line[len(controlPrefix):])
			if strings.HasPrefix(strings.ToUpper(commandPart), "WATCH ") {
				logging.Debug("Found WATCH command in script", "line", line)
				return true, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading script file: %v", err)
	}

	return false, nil
}

// validateWatchRequirements checks that all requirements for WATCH commands are met
func validateWatchRequirements() error {
	// Check training data path
	if scriptOCRConfig.TrainingDataPath == "" {
		return fmt.Errorf("WATCH commands require training data file")
	}

	// Check that training data file exists
	if _, err := os.Stat(scriptOCRConfig.TrainingDataPath); os.IsNotExist(err) {
		return fmt.Errorf("training data file does not exist: %s", scriptOCRConfig.TrainingDataPath)
	}

	// Check screen dimensions
	if scriptOCRConfig.Columns <= 0 || scriptOCRConfig.Rows <= 0 {
		return fmt.Errorf("WATCH commands require valid screen dimensions (columns: %d, rows: %d)",
			scriptOCRConfig.Columns, scriptOCRConfig.Rows)
	}

	logging.Debug("WATCH requirements validated",
		"trainingData", scriptOCRConfig.TrainingDataPath,
		"columns", scriptOCRConfig.Columns,
		"rows", scriptOCRConfig.Rows)

	return nil
}

// syncScriptOCRConfig populates the script OCR config from command flags and config
func syncScriptOCRConfig() {
	// Update OCR config from flags and config values
	// Training data path is only set via command line argument or config file (no flag)

	if widthFlag := viper.GetInt("script.watch_width"); widthFlag > 0 {
		scriptOCRConfig.Columns = widthFlag
	}

	if heightFlag := viper.GetInt("script.watch_height"); heightFlag > 0 {
		scriptOCRConfig.Rows = heightFlag
	}

	// Check config file for training data if not provided as argument
	if scriptOCRConfig.TrainingDataPath == "" {
		if configPath := viper.GetString("ocr.training_data"); configPath != "" {
			scriptOCRConfig.TrainingDataPath = configPath
			logging.Debug("Training data path from config", "path", configPath)
		}
	}
}

// getScriptDelay determines the key delay to use based on flag or config
func getScriptDelay() time.Duration {
	// Priority 1: Command line flag
	if scriptDelay > 0 {
		return scriptDelay
	}

	// Priority 2: Config file
	if viper.IsSet("keyboard.delay") {
		// Use the same delay setting as keyboard by default
		return time.Duration(viper.GetInt("keyboard.delay")) * time.Millisecond
	}

	// Default to 50ms
	return 50 * time.Millisecond
}

// isValidKeyCombo checks if a string looks like a valid key combination
func isValidKeyCombo(combo string) bool {
	// Allow single keys and combinations with - or +
	if len(combo) == 0 {
		return false
	}

	// Common single keys
	singleKeys := map[string]bool{
		"esc": true, "escape": true, "enter": true, "return": true, "ret": true,
		"tab": true, "space": true, "spc": true, "backspace": true, "delete": true,
		"up": true, "down": true, "left": true, "right": true,
		"home": true, "end": true, "pgup": true, "pgdn": true,
		"insert": true, "caps_lock": true, "num_lock": true, "scroll_lock": true,
	}

	// Function keys f1-f12
	for i := 1; i <= 12; i++ {
		singleKeys[fmt.Sprintf("f%d", i)] = true
	}

	// Single character keys (a-z, 0-9, punctuation)
	if len(combo) == 1 {
		return true
	}

	// Check if it's in our single keys list
	if singleKeys[strings.ToLower(combo)] {
		return true
	}

	// Check if it contains modifiers (likely a combination)
	modifiers := []string{"ctrl", "alt", "shift", "cmd", "super", "meta", "control"}
	for _, mod := range modifiers {
		if strings.Contains(strings.ToLower(combo), mod) {
			return true
		}
	}

	// If it contains - or +, assume it's a key combo
	if strings.Contains(combo, "-") || strings.Contains(combo, "+") {
		return true
	}

	return false
}

// getCommentChar returns the comment character from flag or config
func getCommentChar() string {
	if commentStr != "" {
		return commentStr
	}
	if viper.IsSet("script.comment_char") {
		return viper.GetString("script.comment_char")
	}
	return "#"
}

// getControlChar returns the control character prefix from flag or config
func getControlChar() string {
	if controlStr != "" {
		return controlStr
	}
	if viper.IsSet("script.control_char") {
		return viper.GetString("script.control_char")
	}
	return "<! "
}

// processCommentLine handles comment processing and escape sequences
// Returns (processedLine, isComment)
func processCommentLine(line, commentPrefix string) (string, bool) {
	// Check if line starts with comment prefix
	if strings.HasPrefix(line, commentPrefix) {
		return "", true // Pure comment line
	}

	// Check for escaped comment characters (e.g., \# -> #)
	escaped := "\\" + commentPrefix
	if strings.Contains(line, escaped) {
		// Replace escaped comment chars with literal chars
		return strings.ReplaceAll(line, escaped, commentPrefix), false
	}

	return line, false
}

// performWatchCheck takes a screenshot, runs OCR, and searches for the target string
func performWatchCheck(client *qmp.Client, watchString, trainingDataPath string, width, height int) (bool, error) {
	// Take temporary screenshot using centralized helper
	tmpFile, err := TakeTemporaryScreenshot(client, "qmp-watch")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Process the screenshot with OCR
	result, err := ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), trainingDataPath, width, height)
	if err != nil {
		return false, fmt.Errorf("error processing screenshot for OCR: %v", err)
	}

	// Search for the target string
	searchConfig := ocr.SearchConfig{
		IgnoreCase:  false, // Could be made configurable
		FirstOnly:   true,  // Just need to know if it exists
		Quiet:       true,  // Don't output search results
		LineNumbers: false,
	}

	searchResults := ocr.FindString(result, watchString, searchConfig)
	return searchResults.Found, nil
}

// scriptSampleCmd shows a sample script with all available features
var scriptSampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Display a sample script showcasing all available features",
	Long: `Display a comprehensive sample script that demonstrates all available
features of the original script command including:

- Basic text typing
- Control commands (WATCH, Console switching, Key sequences, Sleep)
- Comment syntax and escape sequences
- Configurable comment and control characters
- WATCH command with timeout and OCR integration

This sample is kept up-to-date with all implemented features.`,
	Run: func(cmd *cobra.Command, args []string) {
		sample := `# Sample QMP Script - Original Script Command
# This demonstrates all features available in the original script command
#
# Comments start with # (configurable with --comment-char)
# Control commands start with <# (configurable with --control-char)
# To type a literal # character, use \#

# Basic text typing - each line is sent to the VM followed by Enter
ssh user@remote-server

# WATCH command - wait for text to appear on screen with timeout
# Requires training data: qmp script 106 sample.script training.json
<# WATCH "password:" TIMEOUT 10s

# Type password (this line will be typed as-is)
mypassword123

# Send special keys
<# Key enter

# Sleep/wait commands
<# Sleep 2

# Console switching (Ctrl+Alt+F1 through F6)
<# Console 1

# More complex key combinations
<# Key ctrl-c
<# Key esc
<# Key tab

# Wait for login prompt
<# WATCH "$ " TIMEOUT 30s

# Run some commands
ls -la
<# Sleep 1

# Check system status
systemctl status
<# Sleep 2

# Switch to another console
<# Console 2

# Login on second console
<# WATCH "login:" TIMEOUT 15s
admin
<# WATCH "password:" TIMEOUT 10s
adminpass

# Wait for shell prompt
<# WATCH "$ " TIMEOUT 10s

# Run monitoring command
top
<# Sleep 3
<# Key q

# Example of typing literal comment character
echo "This line has a \# character in it"

# Complex WATCH example with longer timeout
<# WATCH "Process completed successfully" TIMEOUT 60s

# End of sample script
echo "Script execution completed"
<# Key enter`

		fmt.Println("=== Sample QMP Script (Original Script Command) ===")
		fmt.Println()
		fmt.Println(sample)
		fmt.Println()
		fmt.Println("=== Usage Examples ===")
		fmt.Println()
		fmt.Printf("# Save sample to file:\n")
		fmt.Printf("qmp script sample > my-script.txt\n\n")
		fmt.Printf("# Execute script (no OCR/WATCH commands):\n")
		fmt.Printf("qmp script 106 my-script.txt\n\n")
		fmt.Printf("# Execute script with WATCH commands (requires training data):\n")
		fmt.Printf("qmp script 106 my-script.txt training.json\n\n")
		fmt.Printf("# Using environment variables (recommended):\n")
		fmt.Printf("export QMP_VM_ID=106\n")
		fmt.Printf("export QMP_TRAINING_DATA=training.json\n")
		fmt.Printf("qmp script my-script.txt                    # Uses env vars for VM ID and training data\n\n")
		fmt.Printf("# Using configuration profiles:\n")
		fmt.Printf("qmp --profile dev script my-script.txt      # Uses dev profile settings\n")
		fmt.Printf("qmp --profile prod script my-script.txt     # Uses production configuration\n\n")
		fmt.Printf("# Custom comment/control characters:\n")
		fmt.Printf("qmp script 106 my-script.txt --comment-char \"%%\" --control-char \"<!\"\n\n")
		fmt.Printf("# Interactive TUI mode:\n")
		fmt.Printf("qmp script 106 my-script.txt training.json --tui\n")
		fmt.Printf("qmp script 106 my-script.txt training.json --enhanced-tui  # Full-screen OCR viewer\n\n")
		fmt.Printf("=== Key Features ===\n")
		fmt.Printf("• Most lines typed directly to VM (followed by Enter)\n")
		fmt.Printf("• <# WATCH \"text\" TIMEOUT 30s - Wait for screen text (requires training data)\n")
		fmt.Printf("• <# Console N - Switch to console 1-6 (Ctrl+Alt+F[N])\n")
		fmt.Printf("• Environment variable support (QMP_VM_ID, QMP_TRAINING_DATA, etc.)\n")
		fmt.Printf("• Configuration profiles (--profile dev/prod/test)\n")
		fmt.Printf("• <# Key combo - Send special keys (enter, esc, ctrl-c, etc.)\n")
		fmt.Printf("• <# Sleep N - Pause execution for N seconds\n")
		fmt.Printf("• # Comments (configurable character)\n")
		fmt.Printf("• \\# Literal comment character in text\n")
		fmt.Printf("• Configurable syntax via --comment-char and --control-char flags\n")
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
	scriptCmd.AddCommand(scriptSampleCmd)
	scriptCmd.Flags().DurationVarP(&scriptDelay, "delay", "l", 0, "delay between key presses (default 50ms)")
	scriptCmd.Flags().StringVar(&commentStr, "comment-char", "#", "character that starts comment lines")
	scriptCmd.Flags().StringVar(&controlStr, "control-char", "<#", "prefix for control commands")
	scriptCmd.Flags().BoolVar(&useTUI, "tui", false, "use interactive TUI mode for script execution")
	scriptCmd.Flags().BoolVar(&useEnhancedTUI, "enhanced-tui", false, "use enhanced TUI with full-screen OCR viewer")

	// WATCH command flags
	scriptCmd.Flags().IntVarP(&scriptOCRConfig.Columns, "columns", "c", qmp.DEFAULT_WIDTH, "screen width for WATCH command OCR")
	scriptCmd.Flags().IntVarP(&scriptOCRConfig.Rows, "rows", "r", qmp.DEFAULT_HEIGHT, "screen height for WATCH command OCR")
	scriptCmd.Flags().DurationVar(&watchPollInterval, "watch-poll-interval", time.Second, "poll interval for WATCH command")

	// Bind flags to viper
	viper.BindPFlag("script.delay", scriptCmd.Flags().Lookup("delay"))
	viper.BindPFlag("script.comment_char", scriptCmd.Flags().Lookup("comment-char"))
	viper.BindPFlag("script.control_char", scriptCmd.Flags().Lookup("control-char"))
	viper.BindPFlag("script.watch_width", scriptCmd.Flags().Lookup("columns"))
	viper.BindPFlag("script.watch_height", scriptCmd.Flags().Lookup("rows"))
	viper.BindPFlag("script.watch_poll_interval", scriptCmd.Flags().Lookup("watch-poll-interval"))
}
