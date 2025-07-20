package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jeeftor/qmp/internal/logging"
	"github.com/jeeftor/qmp/internal/qmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	keyDelay time.Duration
)

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

Examples:
  # Send a single character
  qmp keyboard send 106 a

  # Send special keys
  qmp keyboard send 106 enter
  qmp keyboard send 106 esc`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		key := args[1]

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

		if err := client.SendKey(key); err != nil {
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

Example:
  qmp keyboard type 106 "Hello World"`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		// Join all remaining args to form the text
		text := strings.Join(args[1:], " ")

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
		delay := getKeyDelay()
		logging.Debug("Using key delay", "delay", delay)

		if err := client.SendString(text, delay); err != nil {
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
Press Ctrl+\ (backslash) to exit live mode.

Supported special keys:
- Arrow keys (Up, Down, Left, Right)
- Function keys (F1-F12)
- Home, End, Page Up, Page Down
- Insert, Delete
- Tab, Enter, Backspace, Escape
- Ctrl+key combinations

Example:
  qmp keyboard live 106`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]

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

		fmt.Printf("Connected to VM %s\n", vmid)
		fmt.Println("Entering live keyboard mode. Press Ctrl+\\ (backslash) to exit.")

		// Enter raw mode
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Printf("Error entering raw mode: %v\n", err)
			os.Exit(1)
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			// Just ignore signals - they'll be passed to the VM
			logging.Debug("Signal received, ignoring")
		}()

		// Read input and send to VM
		buf := make([]byte, 32) // Larger buffer to handle escape sequences
		escapeSeq := false
		escapeBuffer := make([]byte, 0, 8)

		// Track if we just sent an ESC key (for vi commands)
		justSentEsc := false

		for {
			n, err := os.Stdin.Read(buf[:1]) // Read one byte at a time
			if err != nil {
				fmt.Printf("\r\nError reading input: %v\n", err)
				break
			}

			if n > 0 {
				// Check for Ctrl+\ (ASCII 28) to exit
				if buf[0] == 28 {
					fmt.Println("\r\nExiting live keyboard mode")
					break
				}

				// Handle escape sequences for special keys
				if escapeSeq {
					escapeBuffer = append(escapeBuffer, buf[0])

					// Check if we have a complete escape sequence
					if isCompleteEscapeSequence(escapeBuffer) {
						key, err := handleEscapeSequence(escapeBuffer)
						if err != nil {
							fmt.Printf("\r\nError: %v\n", err)
						} else if key != "" {
							if err := client.SendKey(key); err != nil {
								fmt.Printf("\r\nError sending key to VM: %v\n", err)
							}
						}
						escapeSeq = false
						escapeBuffer = escapeBuffer[:0]
						justSentEsc = false
					} else if len(escapeBuffer) > 8 {
						// If the escape sequence is too long, reset and just send ESC followed by the characters
						escapeSeq = false

						// Send ESC key first
						if err := client.SendKey("esc"); err != nil {
							fmt.Printf("\r\nError sending ESC key to VM: %v\n", err)
						}

						// Then send each character individually
						for i := 1; i < len(escapeBuffer); i++ {
							key, err := handleSpecialKey(escapeBuffer[i])
							if err == nil && key != "" {
								if err := client.SendKey(key); err != nil {
									fmt.Printf("\r\nError sending key to VM: %v\n", err)
								}
							}
						}

						escapeBuffer = escapeBuffer[:0]
						justSentEsc = false
					}
				} else if buf[0] == 27 { // ESC character
					// Special handling for vi-style commands
					// First, send the ESC key immediately
					if err := client.SendKey("esc"); err != nil {
						fmt.Printf("\r\nError sending ESC key to VM: %v\n", err)
					}

					// Set flag to indicate we just sent ESC
					justSentEsc = true

					// Start tracking escape sequence in case it's a terminal control sequence
					escapeSeq = true
					escapeBuffer = append(escapeBuffer[:0], buf[0])
				} else {
					// If we just sent ESC and this is a regular character, it's likely a vi command
					if justSentEsc {
						justSentEsc = false
						// For vi commands, just send the character as is
						key, err := handleSpecialKey(buf[0])
						if err != nil {
							fmt.Printf("\r\nError: %v\n", err)
							continue
						}

						// Send key to VM
						if err := client.SendKey(key); err != nil {
							fmt.Printf("\r\nError sending key to VM: %v\n", err)
							continue
						}
					} else {
						// Handle regular keys and control characters
						key, err := handleSpecialKey(buf[0])
						if err != nil {
							fmt.Printf("\r\nError: %v\n", err)
							continue
						}

						// Send key to VM
						if err := client.SendKey(key); err != nil {
							fmt.Printf("\r\nError sending key to VM: %v\n", err)
							continue
						}
					}
				}
			}
		}
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

// handleSpecialKey converts byte input to key name for QMP
func handleSpecialKey(b byte) (string, error) {
	switch b {
	case 13: // Enter
		return "ret", nil
	case 27: // Escape
		return "esc", nil
	case 127: // Backspace
		return "backspace", nil
	case 9: // Tab
		return "tab", nil
	case 32: // Space
		return "spc", nil
	default:
		// Handle control characters (Ctrl+A through Ctrl+Z)
		if b < 32 {
			// Convert control character to corresponding key with ctrl modifier
			if b == 3 { // Ctrl+C
				return "ctrl-c", nil
			} else if b == 26 { // Ctrl+Z
				return "ctrl-z", nil
			} else if b == 4 { // Ctrl+D
				return "ctrl-d", nil
			} else if b == 1 { // Ctrl+A
				return "ctrl-a", nil
			} else if b == 5 { // Ctrl+E
				return "ctrl-e", nil
			} else if b == 23 { // Ctrl+W
				return "ctrl-w", nil
			} else if b == 20 { // Ctrl+T
				return "ctrl-t", nil
			} else if b == 18 { // Ctrl+R
				return "ctrl-r", nil
			} else if b == 21 { // Ctrl+U
				return "ctrl-u", nil
			} else if b == 11 { // Ctrl+K
				return "ctrl-k", nil
			} else if b == 12 { // Ctrl+L
				return "ctrl-l", nil
			} else {
				// For other control characters, use the letter
				ctrlKey := string('a' + (b - 1))
				return "ctrl-" + ctrlKey, nil
			}
		}

		// For regular ASCII characters, just use the character
		if b >= 32 && b <= 126 {
			return string(b), nil
		}

		// For unsupported keys
		return "", fmt.Errorf("Key not yet implemented: %d", b)
	}
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

	// Add flags for keyboard commands - use "l" as shorthand for delay
	typeTextCmd.Flags().DurationVarP(&keyDelay, "delay", "l", 0, "delay between key presses (default 50ms)")

	// Bind flags to viper
	viper.BindPFlag("keyboard.delay", typeTextCmd.Flags().Lookup("delay"))
}
