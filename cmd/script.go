package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scriptDelay time.Duration
)

// scriptCmd represents the script command
var scriptCmd = &cobra.Command{
	Use:   "script [vmid] [file]",
	Short: "Run a script of commands",
	Long: `Run a script of commands on the VM.
Each line in the script file is treated as a separate command to be executed.
Empty lines and lines starting with # are ignored.

Special commands can be included using <command> syntax:
  <sleep N>         - Sleep for N seconds
  <console N>       - Switch to console N (1-6) using Ctrl+Alt+F[N]
  <key combo>       - Send key combination (e.g., <key ctrl-alt-del>)
  <ctrl-t>          - Send Ctrl+T (direct key combo syntax)
  <esc>             - Send Escape key
  <shift-h>         - Send Shift+H
  <ctrl-alt-delete> - Send Ctrl+Alt+Delete

Examples:
  qmp script 106 /path/to/script.txt

Script file example:
  # Comments start with #
  <sleep 1>
  <console 1>
  <esc>
  :w
  <enter>
  <ctrl-c>`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		scriptFile := args[1]

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

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// Check for special commands enclosed in <>
			if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
				command := line[1 : len(line)-1] // Remove < and >
				parts := strings.Fields(command)
				if len(parts) > 0 {
					switch parts[0] {
					case "sleep":
						if len(parts) != 2 {
							fmt.Printf("Line %d: Invalid sleep command format. Use <sleep N>\n", lineNum)
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
						// Handle key combinations like <key ctrl-alt-del>
						if len(parts) != 2 {
							fmt.Printf("Line %d: Invalid key command format. Use <key combination>\n", lineNum)
							continue
						}
						keyCombo := parts[1]
						logging.Info("Sending key combination", "keys", keyCombo)
						if err := client.SendKey(keyCombo); err != nil {
							fmt.Printf("Line %d: Error sending key combination '%s': %v\n", lineNum, keyCombo, err)
							continue
						}
					case "console":
						// Handle console switching like <console 1>
						if len(parts) != 2 {
							fmt.Printf("Line %d: Invalid console command format. Use <console 1-6>\n", lineNum)
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
						// If it's not a known command, treat it as a key combination directly
						// This allows <ctrl-t>, <esc>, <shift-h> syntax
						keyCombo := command
						if isValidKeyCombo(keyCombo) {
							logging.Info("Sending key combination", "keys", keyCombo)
							if err := client.SendKey(keyCombo); err != nil {
								fmt.Printf("Line %d: Error sending key combination '%s': %v\n", lineNum, keyCombo, err)
								continue
							}
						} else {
							fmt.Printf("Line %d: Unknown special command: %s\n", lineNum, parts[0])
						}
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

func init() {
	rootCmd.AddCommand(scriptCmd)
	scriptCmd.Flags().DurationVarP(&scriptDelay, "delay", "l", 0, "delay between key presses (default 50ms)")

	// Bind flags to viper
	viper.BindPFlag("script.delay", scriptCmd.Flags().Lookup("delay"))
}
