package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/script2"
	"github.com/spf13/cobra"
)

var (
	// Script2 command flags
	scriptVars    []string // --var key=value
	envFile       string   // --env-file .env
	dryRun        bool     // --dry-run
	scriptTimeout string   // --timeout 300s
)

// script2Cmd represents the enhanced script command
var script2Cmd = &cobra.Command{
	Use:   "script2 [vmid] [script-file]",
	Short: "Execute enhanced scripts with variables and conditionals",
	Long: `Execute enhanced scripts that support:

Key Features:
- Bash-style variables ($VAR, ${VAR:-default})
- Inline conditionals (<watch "text" 5s && { ... }>)
- Special key sequences (<enter>, <tab>, <ctrl+c>)
- Most lines are typed as-is (minimal directive syntax)
- Environment variable integration
- Conditional execution based on screen content

Script Format:
  # Variables (bash-style)
  USER=${USER:-admin}
  PASSWORD=${PASSWORD:-secret}

  # Most lines are just typed directly
  ssh $USER@server
  <watch "password:" 10s>
  $PASSWORD
  <enter>

  # Conditional execution
  <watch "$ " 5s || {
      echo "Login failed, retrying..."
      <ctrl+c>
      ssh backup@server
  }>

Examples:
  # Execute script with default variables
  qmp script2 106 login.script

  # Override variables
  qmp script2 106 login.script --var USER=admin --var PASSWORD=secret

  # Load variables from file
  qmp script2 106 login.script --env-file production.env

  # Dry run to test parsing
  qmp script2 106 login.script --dry-run

  # Debug mode with detailed logging
  qmp script2 106 login.script --debug --log-level debug`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		scriptFile := args[1]

		// Create contextual logger
		logger := logging.NewContextualLogger(vmid, "script2")
		timer := logging.StartTimer("script2_execution", vmid)

		logger.Info("Script2 command started",
			"script_file", scriptFile,
			"dry_run", dryRun,
			"variables", scriptVars,
			"env_file", envFile,
			"timeout", scriptTimeout)

		// Check if script file exists
		if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
			logger.Error("Script file not found", "file", scriptFile, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "file_check",
			})
			logging.UserError("Script file '%s' not found", scriptFile)
			os.Exit(1)
		}

		// Read script file
		scriptContent, err := os.ReadFile(scriptFile)
		if err != nil {
			logger.Error("Failed to read script file", "file", scriptFile, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "file_read",
			})
			logging.UserError("Failed to read script file: %v", err)
			os.Exit(1)
		}

		// Parse timeout
		timeout, err := time.ParseDuration(scriptTimeout)
		if err != nil {
			logger.Error("Invalid timeout format", "timeout", scriptTimeout, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "timeout_parse",
			})
			logging.UserError("Invalid timeout format '%s': %v", scriptTimeout, err)
			os.Exit(1)
		}

		// Initialize variable expander
		variables := script2.NewVariableExpander(make(map[string]string), make(map[string]string), make(map[string]string))
		variables.LoadFromEnvironment()

		// Load environment file if specified
		if envFile != "" {
			if err := variables.LoadFromFile(envFile); err != nil {
				logger.Error("Failed to load environment file", "file", envFile, "error", err)
				timer.StopWithError(err, map[string]interface{}{
					"stage": "env_file_load",
				})
				logging.UserError("Failed to load environment file: %v", err)
				os.Exit(1)
			}
		}

		// Set command-line variable overrides
		if err := variables.SetOverrides(scriptVars); err != nil {
			logger.Error("Invalid variable override", "vars", scriptVars, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "variable_override",
			})
			logging.UserError("Invalid variable override: %v", err)
			os.Exit(1)
		}

		// Create parser and parse script
		parser := script2.NewParser(variables, logger.Debug != nil)
		script, err := parser.ParseScript(string(scriptContent))
		if err != nil {
			logger.Error("Script parsing failed", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "script_parsing",
			})
			logging.UserError("Script parsing failed: %v", err)
			os.Exit(1)
		}

		script.Metadata.Filename = scriptFile
		logger.Info("Script parsed successfully",
			"total_lines", script.Metadata.TotalLines,
			"text_lines", script.Metadata.TextLines,
			"directive_lines", script.Metadata.DirectiveLines,
			"variable_lines", script.Metadata.VariableLines,
			"variables", len(script.Variables))

		// Handle dry run mode
		if dryRun {
			logging.UserInfo("🔍 Dry run mode: Validating script %s", scriptFile)

			// Create execution context for dry run
			context := &script2.ExecutionContext{
				VMID:      vmid,
				Variables: variables,
				Timeout:   timeout,
				DryRun:    true,
				Debug:     logger.Debug != nil,
			}

			executor := script2.NewExecutor(context, logger.Debug != nil)
			executor.SetParser(parser) // Set parser for validation

			result, err := executor.Execute(script)
			if err != nil {
				logger.Error("Dry run execution failed", "error", err)
				timer.StopWithError(err, map[string]interface{}{
					"stage": "dry_run",
				})
				logging.UserError("Dry run failed: %v", err)
				os.Exit(1)
			}

			// Display dry run results
			fmt.Printf("📄 Script file: %s\n", scriptFile)
			fmt.Printf("🖥️  Target VM: %s\n", vmid)
			fmt.Printf("📋 Variables: %v\n", scriptVars)
			if envFile != "" {
				fmt.Printf("🌍 Environment file: %s\n", envFile)
			}
			fmt.Printf("⏱️  Timeout: %s\n", scriptTimeout)
			fmt.Printf("📊 Lines to execute: %d\n", result.LinesExecuted)
			fmt.Printf("🔤 Variables used: %d\n", len(result.Variables))

			if result.Success {
				logging.Success("Dry run validation completed successfully")
			} else {
				logging.UserError("Dry run validation failed: %s", result.Error)
				os.Exit(1)
			}

			timer.Stop(result.Success, map[string]interface{}{
				"mode": "dry_run",
				"validated": result.Success,
				"lines_validated": result.LinesExecuted,
			})
			return
		}

		// Connect to VM for actual execution
		client, err := ConnectToVM(vmid)
		if err != nil {
			logger.Error("Failed to connect to VM", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "vm_connection",
			})
			logging.UserError("Failed to connect to VM: %v", err)
			os.Exit(1)
		}
		defer client.Close()

		// Create execution context
		context := &script2.ExecutionContext{
			Client:    client,
			VMID:      vmid,
			Variables: variables,
			Timeout:   timeout,
			DryRun:    false,
			Debug:     logger.Debug != nil,
		}

		// Execute script
		executor := script2.NewExecutor(context, logger.Debug != nil)
		executor.SetParser(parser)

		logging.Progress("Executing script %s on VM %s", scriptFile, vmid)

		result, err := executor.Execute(script)
		if err != nil {
			logger.Error("Script execution failed", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "script_execution",
				"lines_executed": result.LinesExecuted,
			})
			logging.UserError("Script execution failed: %v", err)
			os.Exit(1)
		}

		// Report results
		if result.Success {
			duration := timer.Stop(true, map[string]interface{}{
				"lines_executed": result.LinesExecuted,
				"variables_final": len(result.Variables),
				"exit_code": result.ExitCode,
			})

			logging.Success("Script executed successfully (%d lines, %v)",
				result.LinesExecuted, duration)
		} else {
			timer.StopWithError(fmt.Errorf(result.Error), map[string]interface{}{
				"lines_executed": result.LinesExecuted,
				"exit_code": result.ExitCode,
			})

			logging.UserError("Script execution failed: %s", result.Error)
			os.Exit(result.ExitCode)
		}
	},
}

// script2SampleCmd shows a sample script with all available features
var script2SampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Display a sample script showcasing all enhanced script2 features",
	Long: `Display a comprehensive sample script that demonstrates all available
features of the enhanced script2 command including:

- Bash-style variables ($VAR, ${VAR:-default}, ${VAR:=default}, ${VAR:+value})
- Most lines typed directly (minimal directive syntax)
- Special key sequences (<enter>, <tab>, <ctrl+c>, etc.)
- Watch directives with OCR integration
- Console switching
- Environment variable integration
- Conditional execution capabilities

This sample is kept up-to-date with all implemented features.`,
	Run: func(cmd *cobra.Command, args []string) {
		sample := `# Sample Enhanced Script2 - Advanced Scripting Features
# This demonstrates all features available in the enhanced script2 command
#
# Key differences from original script:
# - Bash-style variables with expansion
# - Most lines typed directly (no special prefix needed)
# - Minimal directive syntax
# - Environment variable integration

# Variable definitions (bash-style)
USER=${USER:-admin}
PASSWORD=${PASSWORD:-secret123}
TARGET_HOST=${TARGET_HOST:-remote-server}
RETRY_COUNT=${RETRY_COUNT:-3}
TIMEOUT=${TIMEOUT:-30}

# Environment variables can be loaded from files or command line
# Use: qmp script2 106 sample.script training.json --env-file production.env
# Use: qmp script2 106 sample.script training.json --var USER=myuser --var PASSWORD=mypass

# Most lines are just typing - no special syntax needed
ssh $USER@$TARGET_HOST

# Watch directive - wait for text with OCR (requires training data)
<watch "password:" ${TIMEOUT}s>

# Type password using variable
$PASSWORD

# Special key sequences use angle bracket syntax
<enter>

# Wait for login to complete
<wait 2s>

# Console switching (Ctrl+Alt+F1 through F6)
<console 1>

# More typing with variables
echo "Logged in as: $USER"
echo "Target: $TARGET_HOST"

# Complex variable expansion examples
BACKUP_USER=${BACKUP_USER:-${USER}_backup}
echo "Backup user would be: $BACKUP_USER"

# Conditional variable usage
LOG_LEVEL=${LOG_LEVEL:+debug}
${LOG_LEVEL:+echo "Debug mode enabled"}

# Watch for shell prompt
<watch "$ " 10s>

# Run system commands
uname -a
<wait 1s>

# Check if we need elevated access
sudo -l
<watch "password" 5s>
$PASSWORD
<enter>

# Wait for sudo to complete
<watch "$ " 10s>

# Run privileged command
sudo systemctl status
<wait 2s>

# Switch console for monitoring
<console 2>

# Wait for login prompt on new console
<watch "login:" 15s>

# Use different credentials if available
${BACKUP_USER:-$USER}
<watch "password:" 10s>
${BACKUP_PASSWORD:-$PASSWORD}
<enter>

# Monitor system while main session runs
<watch "$ " 10s>
htop
<wait 5s>
<ctrl+c>

# Return to main console
<console 1>

# Complex watch with longer timeout
<watch "Process completed successfully" 60s>

# Variable-driven commands
echo "Script completed for user: $USER on host: $TARGET_HOST"
<enter>

# Exit sequences
<ctrl+d>
<wait 1s>
exit`

		fmt.Println("=== Sample Enhanced Script2 ===")
		fmt.Println()
		fmt.Println(sample)
		fmt.Println()
		fmt.Println("=== Usage Examples ===")
		fmt.Println()
		fmt.Printf("# Save sample to file:\n")
		fmt.Printf("qmp script2 sample > my-enhanced-script.txt\n\n")
		fmt.Printf("# Execute script (no OCR/WATCH commands):\n")
		fmt.Printf("qmp script2 106 my-enhanced-script.txt\n\n")
		fmt.Printf("# Execute with WATCH commands (requires training data):\n")
		fmt.Printf("qmp script2 106 my-enhanced-script.txt training.json\n\n")
		fmt.Printf("# Override variables:\n")
		fmt.Printf("qmp script2 106 my-enhanced-script.txt training.json --var USER=admin --var PASSWORD=secure123\n\n")
		fmt.Printf("# Load environment file:\n")
		fmt.Printf("qmp script2 106 my-enhanced-script.txt training.json --env-file production.env\n\n")
		fmt.Printf("# Dry run validation:\n")
		fmt.Printf("qmp script2 106 my-enhanced-script.txt --dry-run\n\n")
		fmt.Printf("=== Key Features ===\n")
		fmt.Printf("• Bash-style variables: $VAR, ${VAR:-default}, ${VAR:=default}, ${VAR:+value}\n")
		fmt.Printf("• Most lines typed directly (no special prefix needed)\n")
		fmt.Printf("• <watch \"text\" 30s> - Wait for screen text with OCR\n")
		fmt.Printf("• <console N> - Switch to console 1-6\n")
		fmt.Printf("• <enter>, <tab>, <ctrl+c> - Special key sequences\n")
		fmt.Printf("• <wait 5s> - Pause execution\n")
		fmt.Printf("• Environment variable integration (--env-file, --var)\n")
		fmt.Printf("• Dry run validation (--dry-run)\n")
		fmt.Printf("• Advanced variable expansion with defaults and conditionals\n")
	},
}

func init() {
	rootCmd.AddCommand(script2Cmd)
	script2Cmd.AddCommand(script2SampleCmd)

	// Variable override flags
	script2Cmd.Flags().StringArrayVar(&scriptVars, "var", []string{},
		"Set script variables (format: key=value)")

	// Environment configuration
	script2Cmd.Flags().StringVar(&envFile, "env-file", "",
		"Load environment variables from file")

	// Execution control
	script2Cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Parse and validate script without executing VM commands")

	script2Cmd.Flags().StringVar(&scriptTimeout, "timeout", "300s",
		"Overall script execution timeout")

	// Add help examples
	script2Cmd.Example = `  # Execute script with enhanced features (no OCR/WATCH)
  qmp script2 106 enhanced-login.script

  # Script with WATCH commands (requires training data)
  qmp script2 106 login.script training.json

  # Override specific variables
  qmp script2 106 login.script training.json --var USER=admin --var RETRY_COUNT=5

  # Load production environment
  qmp script2 106 deploy.script training.json --env-file production.env

  # Test script parsing without execution
  qmp script2 106 complex.script --dry-run --log-level debug`
}
