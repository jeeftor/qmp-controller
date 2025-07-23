package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scriptDelay   time.Duration
	commentChar   string
	controlChar   string

	// WATCH command OCR settings
	watchTrainingData string
	watchWidth        int
	watchHeight       int
	watchPollInterval time.Duration
)

// scriptCmd represents the script command
var scriptCmd = &cobra.Command{
	Use:   "script [vmid] [file]",
	Short: "Run a script of commands",
	Long: `Run a script of commands on the VM.
Each line in the script file is treated as a separate command to be executed.
Empty lines and comment lines are ignored.

Control commands use configurable prefix (default: <#):
  <# Sleep N         - Sleep for N seconds
  <# Console N       - Switch to console N (1-6) using Ctrl+Alt+F[N]
  <# Key combo       - Send key combination (e.g., <# Key ctrl-alt-del)
  <# WATCH "string" TIMEOUT 30s - Wait for string to appear with timeout

Comments use configurable character (default: #):
  # This is a comment
  \# This types a literal # character

Examples:
  qmp script 106 /path/to/script.txt
  qmp script 106 script.txt --comment-char "%" --control-char "<!

Script file example:
  # Comments start with # (configurable)
  <# Sleep 1
  <# Console 1
  <# Key esc
  :w
  <# Key enter
  <# WATCH "saved" TIMEOUT 10s`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		scriptFile := args[1]

		// Get configuration values
		commentPrefix := getCommentChar()
		controlPrefix := getControlChar()
		logging.Debug("Script configuration", "comment", commentPrefix, "control", controlPrefix)

		// Open the script file
		file, err := os.Open(scriptFile)
		if err != nil {
			fmt.Printf("Error opening script file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		// Connect to the VM
		var client *qmp.Client
		if socketPath := GetSocketPath(); socketPath != "" {
			client = qmp.NewWithSocketPath(vmid, socketPath)
		} else {
			client = qmp.New(vmid)
		}

		if err := client.Connect(); err != nil {
			fmt.Printf("Error connecting to VM %s: %v\n", vmid, err)
			os.Exit(1)
		}
		defer client.Close()

		// Get the key delay from flag or config
		delay := getScriptDelay()
		logging.Debug("Using key delay for script", "delay", delay)

		// Process the script line by line
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

			// Check for control commands using configurable prefix
			if strings.HasPrefix(line, controlPrefix) {
				command := strings.TrimSpace(line[len(controlPrefix):]) // Remove control prefix
				parts := strings.Fields(command)
				if len(parts) > 0 {
					switch strings.ToLower(parts[0]) {
					case "sleep":
						if len(parts) != 2 {
							fmt.Printf("Line %d: Invalid sleep command format. Use <# Sleep N>\n", lineNum)
							continue
						}
						var seconds float64
						_, err := fmt.Sscanf(parts[1], "%f", &seconds)
						if err != nil {
							fmt.Printf("Line %d: Invalid sleep duration: %v\n", lineNum, err)
							continue
						}
						sleepDuration := time.Duration(seconds * float64(time.Second))
						logging.Debug("Sleeping", "duration", sleepDuration)
						time.Sleep(sleepDuration)
					case "key":
						// Handle key combinations like <# Key ctrl-alt-del>
						if len(parts) < 2 {
							fmt.Printf("Line %d: Invalid key command format. Use <# Key combination>\n", lineNum)
							continue
						}
						keyCombo := strings.Join(parts[1:], " ")
						logging.Info("Sending key combination", "keys", keyCombo)
						if err := client.SendKey(keyCombo); err != nil {
							fmt.Printf("Line %d: Error sending key combination '%s': %v\n", lineNum, keyCombo, err)
							continue
						}
					case "watch":
						// Handle WATCH commands like <# WATCH "string" TIMEOUT 30s>
						if err := processWatchCommand(parts, lineNum, client); err != nil {
							fmt.Printf("Line %d: %v\n", lineNum, err)
							continue
						}
					case "console":
						// Handle console switching like <# Console 1>
						if len(parts) != 2 {
							fmt.Printf("Line %d: Invalid console command format. Use <# Console 1-6>\n", lineNum)
							continue
						}
						consoleNum := parts[1]
						// Validate console number
						if consoleNum < "1" || consoleNum > "6" {
							fmt.Printf("Line %d: Console number must be between 1 and 6, got: %s\n", lineNum, consoleNum)
							continue
						}
						// Build the F-key name (f1, f2, etc.)
						fKey := fmt.Sprintf("f%s", consoleNum)
						// Send Ctrl+Alt+F[1-6] combination
						logging.Info("Switching to console", "console", consoleNum)
						if err := client.SendKeyCombo([]string{"ctrl", "alt", fKey}); err != nil {
							fmt.Printf("Line %d: Error switching to console %s: %v\n", lineNum, consoleNum, err)
							continue
						}
					default:
						fmt.Printf("Line %d: Unknown control command: %s\n", lineNum, parts[0])
						fmt.Printf("Available commands: Sleep, Key, Console, Watch\n")
					}
					continue
				}
			}

			// Regular line - send as keyboard input
			logging.Info("Executing line", "line", line)
			if err := client.SendString(line, delay); err != nil {
				fmt.Printf("Line %d: Error sending text: %v\n", lineNum, err)
				continue
			}

			// Send Enter after each command
			if err := client.SendKey("ret"); err != nil {
				fmt.Printf("Line %d: Error sending return key: %v\n", lineNum, err)
				continue
			}

			// Small delay between commands
			time.Sleep(100 * time.Millisecond)
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading script file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Script execution completed for VM %s\n", vmid)
	},
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
	if commentChar != "" {
		return commentChar
	}
	if viper.IsSet("script.comment_char") {
		return viper.GetString("script.comment_char")
	}
	return "#"
}

// getControlChar returns the control character prefix from flag or config
func getControlChar() string {
	if controlChar != "" {
		return controlChar
	}
	if viper.IsSet("script.control_char") {
		return viper.GetString("script.control_char")
	}
	return "<#"
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

// processWatchCommand handles WATCH commands with timeout support
func processWatchCommand(parts []string, lineNum int, client *qmp.Client) error {
	// Expected format: WATCH "string" TIMEOUT 30s
	if len(parts) < 4 {
		return fmt.Errorf("Invalid WATCH command format. Use <# WATCH \"string\" TIMEOUT 30s")
	}

	// Parse the watch string (should be quoted)
	watchString := ""
	timeoutStr := ""

	// Find the quoted string
	commandStr := strings.Join(parts[1:], " ")
	if !strings.Contains(commandStr, "\"") {
		return fmt.Errorf("WATCH string must be quoted")
	}

	// Extract quoted string
	startQuote := strings.Index(commandStr, "\"")
	endQuote := strings.Index(commandStr[startQuote+1:], "\"")
	if endQuote == -1 {
		return fmt.Errorf("WATCH string must be properly quoted")
	}

	watchString = commandStr[startQuote+1 : startQuote+1+endQuote]
	remaining := strings.TrimSpace(commandStr[startQuote+1+endQuote+1:])

	// Parse TIMEOUT
	timeoutParts := strings.Fields(remaining)
	if len(timeoutParts) != 2 || strings.ToLower(timeoutParts[0]) != "timeout" {
		return fmt.Errorf("WATCH command must specify TIMEOUT")
	}

	timeoutStr = timeoutParts[1]
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("Invalid timeout duration '%s': %v", timeoutStr, err)
	}

	logging.Info("Executing WATCH command", "string", watchString, "timeout", timeout)

	// Get WATCH configuration
	trainingData := getWatchTrainingData()
	width := getWatchWidth()
	height := getWatchHeight()
	pollInterval := getWatchPollInterval()

	if trainingData == "" {
		return fmt.Errorf("WATCH command requires training data. Use --watch-training-data flag or set in config")
	}

	if width <= 0 || height <= 0 {
		return fmt.Errorf("WATCH command requires valid screen dimensions. Use --watch-width and --watch-height flags")
	}

	logging.Debug("WATCH configuration", "trainingData", trainingData, "width", width, "height", height, "pollInterval", pollInterval)

	// Check for remote socket usage - WATCH mode is incompatible with remote connections
	if getRemoteTempPath() != "" {
		return fmt.Errorf("WATCH command is not compatible with remote socket connections (--socket or --remote-temp). Screenshots are saved remotely but OCR processing requires local file access")
	}

	// Implement the WATCH loop
	startTime := time.Now()
	for time.Since(startTime) < timeout {
		found, err := performWatchCheck(client, watchString, trainingData, width, height)
		if err != nil {
			logging.Debug("WATCH check error", "error", err)
			// Continue trying on errors
		} else if found {
			logging.Info("WATCH string found", "string", watchString, "elapsed", time.Since(startTime))
			fmt.Printf("✓ Found '%s' after %v\n", watchString, time.Since(startTime).Round(time.Millisecond))
			return nil
		}

		// Sleep before next check
		time.Sleep(pollInterval)
	}

	// Timeout reached
	logging.Info("WATCH timeout reached", "string", watchString, "timeout", timeout)
	return fmt.Errorf("WATCH timeout: '%s' not found within %v", watchString, timeout)
}

// performWatchCheck takes a screenshot, runs OCR, and searches for the target string
func performWatchCheck(client *qmp.Client, watchString, trainingDataPath string, width, height int) (bool, error) {
	// Create a temporary file for the screenshot
	tmpFile, err := os.CreateTemp("", "qmp-watch-*.ppm")
	if err != nil {
		return false, fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Get remote temp path from flag or config
	remotePath := getRemoteTempPath()

	// Take a screenshot
	logging.Debug("Taking screenshot for WATCH", "output", tmpFile.Name(), "remoteTempPath", remotePath)
	if err := client.ScreenDump(tmpFile.Name(), remotePath); err != nil {
		return false, fmt.Errorf("error taking screenshot: %v", err)
	}

	// Process the screenshot with OCR
	result, err := ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), trainingDataPath, width, height, false)
	if err != nil {
		return false, fmt.Errorf("error processing screenshot for OCR: %v", err)
	}

	// Search for the target string
	searchConfig := ocr.SearchConfig{
		IgnoreCase:  false, // Could be made configurable
		FirstOnly:   true,  // Just need to know if it exists
		Quiet:       true,  // Don't output search results
		Debug:       false,
		LineNumbers: false,
	}

	searchResults := ocr.FindString(result, watchString, searchConfig)
	return searchResults.Found, nil
}

// Helper functions to get WATCH configuration values
func getWatchTrainingData() string {
	if watchTrainingData != "" {
		return watchTrainingData
	}
	if viper.IsSet("script.watch_training_data") {
		return viper.GetString("script.watch_training_data")
	}
	return ""
}

func getWatchWidth() int {
	if watchWidth > 0 {
		return watchWidth
	}
	if viper.IsSet("script.watch_width") {
		return viper.GetInt("script.watch_width")
	}
	return 80 // Default
}

func getWatchHeight() int {
	if watchHeight > 0 {
		return watchHeight
	}
	if viper.IsSet("script.watch_height") {
		return viper.GetInt("script.watch_height")
	}
	return 25 // Default
}

func getWatchPollInterval() time.Duration {
	if watchPollInterval > 0 {
		return watchPollInterval
	}
	if viper.IsSet("script.watch_poll_interval") {
		return viper.GetDuration("script.watch_poll_interval")
	}
	return 1 * time.Second // Default: check every second
}

func init() {
	rootCmd.AddCommand(scriptCmd)
	scriptCmd.Flags().DurationVarP(&scriptDelay, "delay", "l", 0, "delay between key presses (default 50ms)")
	scriptCmd.Flags().StringVar(&commentChar, "comment-char", "#", "character that starts comment lines")
	scriptCmd.Flags().StringVar(&controlChar, "control-char", "<#", "prefix for control commands")

	// WATCH command flags
	scriptCmd.Flags().StringVar(&watchTrainingData, "watch-training-data", "", "training data file for WATCH command OCR")
	scriptCmd.Flags().IntVar(&watchWidth, "watch-width", 80, "screen width for WATCH command OCR")
	scriptCmd.Flags().IntVar(&watchHeight, "watch-height", 25, "screen height for WATCH command OCR")
	scriptCmd.Flags().DurationVar(&watchPollInterval, "watch-poll-interval", time.Second, "poll interval for WATCH command")

	// Bind flags to viper
	viper.BindPFlag("script.delay", scriptCmd.Flags().Lookup("delay"))
	viper.BindPFlag("script.comment_char", scriptCmd.Flags().Lookup("comment-char"))
	viper.BindPFlag("script.control_char", scriptCmd.Flags().Lookup("control-char"))
	viper.BindPFlag("script.watch_training_data", scriptCmd.Flags().Lookup("watch-training-data"))
	viper.BindPFlag("script.watch_width", scriptCmd.Flags().Lookup("watch-width"))
	viper.BindPFlag("script.watch_height", scriptCmd.Flags().Lookup("watch-height"))
	viper.BindPFlag("script.watch_poll_interval", scriptCmd.Flags().Lookup("watch-poll-interval"))
}
