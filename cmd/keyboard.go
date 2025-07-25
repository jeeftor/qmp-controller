package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jstein/qmp/internal/logging"
	"github.com/jstein/qmp/internal/qmp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	// Add flags for keyboard commands - use "l" as shorthand for delay
	typeTextCmd.Flags().DurationVarP(&keyDelay, "delay", "l", 0, "delay between key presses (default 50ms)")

	// Bind flags to viper
	viper.BindPFlag("keyboard.delay", typeTextCmd.Flags().Lookup("delay"))
}
