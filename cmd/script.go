package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeeftor/qmp/internal/logging"
	"github.com/jeeftor/qmp/internal/qmp"
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
  <sleep N>    - Sleep for N seconds

Example:
  qmp script 106 /path/to/script.txt`,
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
					default:
						fmt.Printf("Line %d: Unknown special command: %s\n", lineNum, parts[0])
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

func init() {
	rootCmd.AddCommand(scriptCmd)
	scriptCmd.Flags().DurationVarP(&scriptDelay, "delay", "l", 0, "delay between key presses (default 50ms)")

	// Bind flags to viper
	viper.BindPFlag("script.delay", scriptCmd.Flags().Lookup("delay"))
}
