package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	keyDelay time.Duration
)

// Old LiveModeUI code removed - now using Bubble Tea TUI

// keyboardCmd represents the keyboard command
var keyboardCmd = &cobra.Command{
	Use:   "keyboard",
	Short: "Send keyboard input to the VM",
	Long:  `Send keyboard input to the VM, including key presses and text.`,
}

// sendKeyCmd represents the keyboard send command
var sendKeyCmd = &cobra.Command{
	Use:   "send [vmid] [key]",
	Short: "Send a single key press",
	Long: `Send a single key press to the VM.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp keyboard send 106 a
  qmp keyboard send 106 enter

  # Using environment variable
  export QMP_VM_ID=106
  qmp keyboard send a
  qmp keyboard send enter`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()

		// Check if first arg is VM ID or key
		var vmid, key string
		if len(args) == 2 {
			// Traditional format: vmid key
			vmid = args[0]
			key = args[1]
		} else if len(args) == 1 {
			// New format with env var: key
			vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1) // No args, use env var
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			vmid = vmidInfo.Value
			key = args[0]
		} else {
			fmt.Fprintf(os.Stderr, "Error: Key is required\n")
			os.Exit(1)
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		err = logging.LogOperation("send_key", vmid, func() error {
			logging.LogKeyboard(vmid, key, "single_key", 0)
			return client.SendKey(key)
		})

		if err != nil {
			fmt.Printf("Error sending key '%s' to VM %s: %v\n", key, vmid, err)
			os.Exit(1)
		}

		fmt.Printf("Sent key '%s' to VM %s\n", key, vmid)
	},
}

// typeTextCmd represents the keyboard type command
var typeTextCmd = &cobra.Command{
	Use:   "type [vmid] [text...]",
	Short: "Type a string of text",
	Long: `Type a string of text to the VM.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp keyboard type 106 "Hello World"

  # Using environment variable
  export QMP_VM_ID=106
  qmp keyboard type "Hello World"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()

		// Check if first arg is VM ID or text
		var vmid, text string
		if len(args) >= 2 {
			// Check if first arg is numeric (VM ID)
			vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
			if err == nil {
				// First arg is valid VM ID
				vmid = vmidInfo.Value
				text = strings.Join(args[1:], " ")
			} else {
				// First arg is not VM ID, try environment variable
				vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				vmid = vmidInfo.Value
				text = strings.Join(args, " ")
			}
		} else {
			// Only one arg, must be text with VM ID from env var
			vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			vmid = vmidInfo.Value
			text = args[0]
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		// Get the key delay from flag or config
		delay := getKeyDelay()

		err = logging.LogOperation("send_text", vmid, func() error {
			logging.LogKeyboard(vmid, text, "text_string", delay)
			return client.SendString(text, delay)
		})

		if err != nil {
			fmt.Printf("Error typing text to VM %s: %v\n", vmid, err)
			os.Exit(1)
		}

		fmt.Printf("Typed '%s' to VM %s with delay %v\n", text, vmid, delay)
	},
}

// liveCmd represents the keyboard live command
var liveCmd = &cobra.Command{
	Use:   "live [vmid]",
	Short: "Enter live keyboard mode",
	Long: `Enter live keyboard mode to type directly into the VM.
This mode captures all keyboard input and sends it to the VM in real-time.
Press Ctrl+^ (Ctrl+6) to exit live mode.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Supported special keys:
- Arrow keys (Up, Down, Left, Right)
- Function keys (F1-F12)
- Home, End, Page Up, Page Down
- Insert, Delete

Examples:
  # Explicit VM ID
  qmp keyboard live 106

  # Using environment variable
  export QMP_VM_ID=106
  qmp keyboard live
- Tab, Enter, Backspace, Escape
- Ctrl+key combinations

`,
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()
		vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		vmid := vmidInfo.Value

		client, err := ConnectToVM(vmid)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		// Create the Bubble Tea TUI model
		model := NewLiveTUIModel(vmid, client)

		// Start the Bubble Tea program
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

// rawCmd represents the keyboard raw command
var rawCmd = &cobra.Command{
	Use:   "raw [vmid] [json]",
	Short: "Send a raw JSON QMP command",
	Long: `Send a raw JSON QMP command directly to the VM.
This is useful for debugging and testing specific QMP structures.

Examples:
  # Send a simple key
  qmp keyboard raw 108 '{"execute":"send-key","arguments":{"keys":[{"type":"qcode","data":"a"}]}}'

  # Test Unicode character
  qmp keyboard raw 108 '{"execute":"send-key","arguments":{"keys":[{"type":"qcode","data":"U00E0"}]}}'

  # Send key combination
  qmp keyboard raw 108 '{"execute":"send-key","arguments":{"keys":[{"type":"qcode","data":"ctrl"},{"type":"qcode","data":"c"}]}}'`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		jsonStr := args[1]

		client, err := ConnectToVM(vmid)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		if err := client.SendRawJSON(jsonStr); err != nil {
			fmt.Printf("Error sending raw JSON to VM %s: %v\n", vmid, err)
			os.Exit(1)
		}

		fmt.Printf("Raw JSON sent to VM %s successfully\n", vmid)
	},
}

// testCmd represents the keyboard test command for debugging conversions
var testCmd = &cobra.Command{
	Use:   "test [vmid] [text]",
	Short: "Test Unicode to Alt code conversion without sending",
	Long: `Test Unicode to Alt code conversion and show what would be sent.
This is useful for debugging the conversion logic.

Examples:
  qmp keyboard test 108 "©"
  qmp keyboard test 108 "àáâãäå"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		text := args[1]

		var client *qmp.Client
		if socketPath := GetSocketPath(); socketPath != "" {
			client = qmp.NewWithSocketPath(vmid, socketPath)
		} else {
			client = qmp.New(vmid)
		}

		fmt.Printf("Testing conversion for: %s\n", text)
		for i, r := range text {
			fmt.Printf("Character %d: '%c' (code %d)\n", i+1, r, int(r))

			if r > 127 {
				// Test the conversion logic
				altKeys := client.TestConvertToAltCode(r)
				if len(altKeys) > 0 {
					fmt.Printf("  → Alt code sequence: %v\n", altKeys)
				} else {
					fmt.Printf("  → No Alt code mapping, would use Unicode format: U%04X\n", int(r))
				}
			} else {
				fmt.Printf("  → Regular ASCII, would send as: %c\n", r)
			}
		}
	},
}

// consoleCmd represents the keyboard console command for virtual terminal switching
var consoleCmd = &cobra.Command{
	Use:   "console [vmid] [1-6]",
	Short: "Switch to virtual terminal console",
	Long: `Switch to a virtual terminal console (F1-F6) using Ctrl+Alt+F[1-6].
This is useful for switching between different virtual terminals in Linux VMs.

Examples:
  qmp keyboard console 108 1   # Switch to console 1 (Ctrl+Alt+F1)
  qmp keyboard console 108 6   # Switch to console 6 (Ctrl+Alt+F6)`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		consoleNum := args[1]

		// Validate console number
		if consoleNum < "1" || consoleNum > "6" {
			fmt.Printf("Error: Console number must be between 1 and 6, got: %s\n", consoleNum)
			os.Exit(1)
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		// Build the F-key name (f1, f2, etc.)
		fKey := fmt.Sprintf("f%s", consoleNum)

		// Send Ctrl+Alt+F[1-6] combination
		if err := client.SendKeyCombo([]string{"ctrl", "alt", fKey}); err != nil {
			fmt.Printf("Error switching to console %s on VM %s: %v\n", consoleNum, vmid, err)
			os.Exit(1)
		}

		fmt.Printf("Switched to console %s on VM %s (Ctrl+Alt+%s)\n", consoleNum, vmid, strings.ToUpper(fKey))
	},
}

// isCompleteEscapeSequence checks if an escape sequence is complete
func isCompleteEscapeSequence(seq []byte) bool {
	if len(seq) < 2 {
		return false
	}

	// For vi-style commands (ESC followed by a single character),
	// we don't treat them as escape sequences
	if len(seq) == 2 && seq[0] == 27 && seq[1] != '[' && seq[1] != 'O' {
		return false
	}

	// Most common escape sequences
	if seq[0] == 27 && seq[1] == '[' {
		// ESC [ ... sequences
		if len(seq) >= 3 {
			last := seq[len(seq)-1]
			// Terminal sequences typically end with a letter
			if (last >= 'A' && last <= 'Z') || (last >= 'a' && last <= 'z') {
				return true
			}

			// Special case for Home/End on some terminals
			if len(seq) >= 4 && seq[2] == '1' && seq[3] == '~' {
				return true // Home
			}
			if len(seq) >= 4 && seq[2] == '4' && seq[3] == '~' {
				return true // End
			}
			if len(seq) >= 4 && seq[2] == '5' && seq[3] == '~' {
				return true // Page Up
			}
			if len(seq) >= 4 && seq[2] == '6' && seq[3] == '~' {
				return true // Page Down
			}
		}
	} else if seq[0] == 27 && seq[1] == 'O' {
		// ESC O ... sequences (function keys on some terminals)
		if len(seq) >= 3 {
			return true
		}
	}

	return false
}

// handleEscapeSequence converts an escape sequence to a key name for QMP
func handleEscapeSequence(seq []byte) (string, error) {
	if len(seq) < 2 {
		return "", fmt.Errorf("Invalid escape sequence")
	}

	// ESC [ ... sequences
	if seq[0] == 27 && seq[1] == '[' {
		if len(seq) >= 3 {
			switch seq[2] {
			case 'A':
				return "up", nil // Up arrow
			case 'B':
				return "down", nil // Down arrow
			case 'C':
				return "right", nil // Right arrow
			case 'D':
				return "left", nil // Left arrow
			case 'H':
				return "home", nil // Home
			case 'F':
				return "end", nil // End
			}

			// Handle more complex sequences
			if len(seq) >= 4 {
				if seq[2] == '1' && seq[3] == '~' {
					return "home", nil // Home on some terminals
				}
				if seq[2] == '4' && seq[3] == '~' {
					return "end", nil // End on some terminals
				}
				if seq[2] == '5' && seq[3] == '~' {
					return "pgup", nil // Page Up
				}
				if seq[2] == '6' && seq[3] == '~' {
					return "pgdn", nil // Page Down
				}
				if seq[2] == '2' && seq[3] == '~' {
					return "insert", nil // Insert
				}
				if seq[2] == '3' && seq[3] == '~' {
					return "delete", nil // Delete
				}

				// Function keys
				if seq[2] == '1' && seq[3] == '1' && seq[4] == '~' {
					return "f1", nil
				}
				if seq[2] == '1' && seq[3] == '2' && seq[4] == '~' {
					return "f2", nil
				}
				if seq[2] == '1' && seq[3] == '3' && seq[4] == '~' {
					return "f3", nil
				}
				if seq[2] == '1' && seq[3] == '4' && seq[4] == '~' {
					return "f4", nil
				}
				if seq[2] == '1' && seq[3] == '5' && seq[4] == '~' {
					return "f5", nil
				}
				if seq[2] == '1' && seq[3] == '7' && seq[4] == '~' {
					return "f6", nil
				}
				if seq[2] == '1' && seq[3] == '8' && seq[4] == '~' {
					return "f7", nil
				}
				if seq[2] == '1' && seq[3] == '9' && seq[4] == '~' {
					return "f8", nil
				}
				if seq[2] == '2' && seq[3] == '0' && seq[4] == '~' {
					return "f9", nil
				}
				if seq[2] == '2' && seq[3] == '1' && seq[4] == '~' {
					return "f10", nil
				}
				if seq[2] == '2' && seq[3] == '3' && seq[4] == '~' {
					return "f11", nil
				}
				if seq[2] == '2' && seq[3] == '4' && seq[4] == '~' {
					return "f12", nil
				}
			}
		}
	} else if seq[0] == 27 && seq[1] == 'O' {
		// ESC O ... sequences (function keys on some terminals)
		if len(seq) >= 3 {
			switch seq[2] {
			case 'A':
				return "up", nil // Up arrow
			case 'B':
				return "down", nil // Down arrow
			case 'C':
				return "right", nil // Right arrow
			case 'D':
				return "left", nil // Left arrow
			case 'H':
				return "home", nil // Home
			case 'F':
				return "end", nil // End
			case 'P':
				return "f1", nil // F1
			case 'Q':
				return "f2", nil // F2
			case 'R':
				return "f3", nil // F3
			case 'S':
				return "f4", nil // F4
			}
		}
	}

	return "", fmt.Errorf("Unsupported escape sequence: %v", seq)
}


// getKeyDelay determines the key delay to use based on flag or config
func getKeyDelay() time.Duration {
	// Priority 1: Command line flag
	if keyDelay > 0 {
		return keyDelay
	}

	// Priority 2: Config file
	if viper.IsSet("keyboard.delay") {
		// Convert milliseconds from config to time.Duration
		return time.Duration(viper.GetInt("keyboard.delay")) * time.Millisecond
	}

	// Default to 50ms
	return 50 * time.Millisecond
}

func init() {
	rootCmd.AddCommand(keyboardCmd)
	keyboardCmd.AddCommand(sendKeyCmd)
	keyboardCmd.AddCommand(typeTextCmd)
	keyboardCmd.AddCommand(liveCmd)
	keyboardCmd.AddCommand(rawCmd)
	keyboardCmd.AddCommand(testCmd)
	keyboardCmd.AddCommand(consoleCmd)

	// Add flags for keyboard commands - use "l" as shorthand for delay
	typeTextCmd.Flags().DurationVarP(&keyDelay, "delay", "l", 0, "delay between key presses (default 50ms)")

	// Bind flags to viper
	viper.BindPFlag("keyboard.delay", typeTextCmd.Flags().Lookup("delay"))
}
