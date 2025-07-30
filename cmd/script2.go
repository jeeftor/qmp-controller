package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jeeftor/qmp-controller/internal/args"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/script2"
	"github.com/spf13/cobra"
)

var (
	// Script2 command flags
	scriptVars          []string // --var key=value
	envFile             string   // --env-file .env
	dryRun              bool     // --dry-run
	scriptTimeout       string   // --timeout 300s
	debugMode           bool     // --debug (step mode)
	debugInteractive    bool     // --debug-interactive (TUI mode)
	debugEnhanced       bool     // --debug-enhanced (Enhanced TUI mode)
	debugConsole        bool     // --debug-console (force console mode)
	debugBreakpoints    []int    // --breakpoint line1,line2,line3
)

// script2Cmd represents the enhanced script command
var script2Cmd = &cobra.Command{
	Use:   "script2 [vmid] [script-file] [training-data-file]",
	Short: "Execute enhanced scripts with variables and conditionals",
	Long: `Execute enhanced scripts that support:

Key Features:
- Bash-style variables ($VAR, ${VAR:-default})
- Inline conditionals (<watch "text" 5s && { ... }>)
- Special key sequences (<enter>, <tab>, <ctrl+c>)
- Most lines are typed as-is (minimal directive syntax)
- Environment variable integration
- Conditional execution based on screen content
- Interactive debugging with <break> directive and TUI debugger

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.
The training data file can be provided as an argument or set via the QMP_TRAINING_DATA environment variable.

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

  # Debugging support
  echo "Starting complex process..."
  <break>    # Pause execution for debugging
  complex_command
  <watch "completed" 30s>

Examples:
  # Explicit arguments
  qmp script2 106 login.script training.json

  # Using environment variables
  export QMP_VM_ID=106
  export QMP_TRAINING_DATA=training.json
  qmp script2 login.script

  # Override variables
  qmp script2 106 login.script --var USER=admin --var PASSWORD=secret

  # Load variables from file
  qmp script2 106 login.script --env-file production.env

  # Dry run to test parsing
  qmp script2 106 login.script --dry-run

  # Debug mode with detailed logging
  qmp script2 106 login.script --debug --log-level debug

  # Interactive debugging with TUI
  qmp script2 106 login.script --debug-interactive

  # Enhanced debugging with OCR TUI
  qmp script2 106 login.script --debug-enhanced

  # Console debugging (SSH/remote friendly)
  qmp script2 106 login.script --debug-console

  # Set breakpoints on specific lines
  qmp script2 106 login.script --breakpoint 10,25,50 --debug`,
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		// Parse arguments using the new argument parser
		argParser := args.NewScriptArgumentParser()
		parsedArgs := args.ParseWithHandler(cmdArgs, argParser)

		// Extract parsed values
		vmid := parsedArgs.VMID
		scriptFile := parsedArgs.ScriptFile
		trainingDataPath := parsedArgs.TrainingData

		// Log training data source
		if trainingDataPath != "" {
			logging.Info("Using training data", "path", trainingDataPath)
		}

		// Log argument resolution source for debugging
		logging.Debug("Arguments parsed", "source", parsedArgs.Source)

		// Create contextual logger
		logger := logging.NewContextualLogger(vmid, "script2")
		timer := logging.StartTimer("script2_execution", vmid)

		logger.Info("Script2 command started",
			"script_file", scriptFile,
			"dry_run", dryRun,
			"variables", scriptVars,
			"env_file", envFile,
			"timeout", scriptTimeout)

		// Script file existence is already validated by the argument parser

		// Read script file
		scriptContent, err := os.ReadFile(scriptFile)
		if err != nil {
			logger.Error("Failed to read script file", "file", scriptFile, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "file_read",
			})
			logging.UserErrorf("Failed to read script file: %v", err)
			os.Exit(1)
		}

		// Parse timeout
		timeout, err := time.ParseDuration(scriptTimeout)
		if err != nil {
			logger.Error("Invalid timeout format", "timeout", scriptTimeout, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "timeout_parse",
			})
			logging.UserErrorf("Invalid timeout format '%s': %v", scriptTimeout, err)
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
				logging.UserErrorf("Failed to load environment file: %v", err)
				os.Exit(1)
			}
		}

		// Set command-line variable overrides
		if err := variables.SetOverrides(scriptVars); err != nil {
			logger.Error("Invalid variable override", "vars", scriptVars, "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "variable_override",
			})
			logging.UserErrorf("Invalid variable override: %v", err)
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
			logging.UserErrorf("Script parsing failed: %v", err)
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
			logging.UserInfof("🔍 Dry run mode: Validating script %s", scriptFile)

			// Create execution context for dry run
			context := &script2.ExecutionContext{
				VMID:         vmid,
				Variables:    variables,
				Timeout:      timeout,
				DryRun:       true,
				Debug:        logger.Debug != nil,
				TrainingData: trainingDataPath,
			}

			executor := script2.NewExecutor(context, logger.Debug != nil)
			executor.SetParser(parser) // Set parser for validation
			executor.SetScript(script) // Set script for function access

			// Enable debugging for dry-run if requested
			if debugMode || debugInteractive || debugEnhanced || debugConsole || len(debugBreakpoints) > 0 {
				var debugModeType script2.DebugMode
				if debugConsole {
					debugModeType = script2.DebugModeBreakpoints
					logging.UserInfo("🐛 Dry-run with console debugging - SSH/remote friendly")
				} else if debugEnhanced {
					debugModeType = script2.DebugModeInteractive
					logging.UserInfo("🔍 Dry-run with enhanced debugging - Advanced OCR TUI with performance monitoring")
				} else if debugInteractive {
					debugModeType = script2.DebugModeInteractive
					logging.UserInfo("🐛 Dry-run with interactive debugging - TUI will appear on breakpoints")
				} else if debugMode {
					debugModeType = script2.DebugModeStep
					logging.UserInfo("🐛 Dry-run with step debugging enabled")
				} else {
					debugModeType = script2.DebugModeBreakpoints
					logging.UserInfo("🐛 Dry-run with breakpoint debugging enabled")
				}

				debugger := executor.EnableDebugging(debugModeType)

				// Enable enhanced TUI if requested
				if debugEnhanced {
					debugger.EnableEnhancedTUI()
				}

				// Set initial breakpoints
				for _, lineNum := range debugBreakpoints {
					debugger.AddBreakpoint(lineNum)
					logging.UserInfof("🔴 Dry-run breakpoint set on line %d", lineNum)
				}
			}

			result, err := executor.Execute(script)
			if err != nil {
				logger.Error("Dry run execution failed", "error", err)
				timer.StopWithError(err, map[string]interface{}{
					"stage": "dry_run",
				})
				logging.UserErrorf("Dry run failed: %v", err)
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
				logging.UserErrorf("Dry run validation failed: %s", result.Error)
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
			logging.UserErrorf("Failed to connect to VM: %v", err)
			os.Exit(1)
		}
		defer client.Close()

		// Create execution context
		context := &script2.ExecutionContext{
			Client:       client,
			VMID:         vmid,
			Variables:    variables,
			Timeout:      timeout,
			DryRun:       false,
			Debug:        logger.Debug != nil,
			TrainingData: trainingDataPath,
		}

		// Execute script
		executor := script2.NewExecutor(context, logger.Debug != nil)
		executor.SetParser(parser)
		executor.SetScript(script) // Set script for function access

		// Set up debugging if requested
		if debugMode || debugInteractive || debugEnhanced || debugConsole || len(debugBreakpoints) > 0 {
			var debugModeType script2.DebugMode
			if debugConsole {
				debugModeType = script2.DebugModeBreakpoints // Force console mode
				logging.UserInfo("🐛 Console debugging enabled - perfect for SSH/remote sessions")
				logging.UserInfo("   Will break on line 1, <break> directives, and set breakpoints")
			} else if debugEnhanced {
				debugModeType = script2.DebugModeInteractive
				logging.UserInfo("🔍 Enhanced debugging enabled - Advanced OCR TUI with real-time monitoring")
				logging.UserInfo("   Enhanced features: O=full OCR, /=search, g=grid, d=diff, e=export, p=performance")
				logging.UserInfo("   Navigation: ↑↓←→/hjkl=navigate grid, r=refresh, a=auto-refresh")
			} else if debugInteractive {
				debugModeType = script2.DebugModeInteractive
				logging.UserInfo("🐛 Interactive debugging enabled - TUI will appear on line 1 and breakpoints")
				logging.UserInfo("   Use keys: c=continue, s=step, o=OCR view, w=watch progress, q=quit")

				// Check environment for SSH/remote usage
				if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
					logging.UserInfo("   📡 SSH session detected - TUI will fallback to console mode if needed")
					logging.UserInfo("   💡 Tip: Use --debug-console for guaranteed SSH compatibility")
				}
			} else if debugMode {
				debugModeType = script2.DebugModeStep
				logging.UserInfo("🐛 Step debugging enabled - will break on each line")
			} else {
				debugModeType = script2.DebugModeBreakpoints
				logging.UserInfo("🐛 Breakpoint debugging enabled")
			}

			debugger := executor.EnableDebugging(debugModeType)

			// Enable enhanced TUI if requested
			if debugEnhanced {
				debugger.EnableEnhancedTUI()
			}

			// Set initial breakpoints
			for _, lineNum := range debugBreakpoints {
				debugger.AddBreakpoint(lineNum)
				logging.UserInfof("🔴 Breakpoint set on line %d", lineNum)
			}
		}

		logging.Progress("Executing script %s on VM %s", scriptFile, vmid)

		result, err := executor.Execute(script)
		if err != nil {
			logger.Error("Script execution failed", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "script_execution",
				"lines_executed": result.LinesExecuted,
			})
			logging.UserErrorf("Script execution failed: %v", err)
			os.Exit(1)
		}

		// Report results
		if result.Success {
			duration := timer.Stop(true, map[string]interface{}{
				"lines_executed": result.LinesExecuted,
				"variables_final": len(result.Variables),
				"exit_code": result.ExitCode,
			})

			logging.Successf("Script executed successfully (%d lines, %v)",
				result.LinesExecuted, duration)
		} else {
			timer.StopWithError(fmt.Errorf(result.Error), map[string]interface{}{
				"lines_executed": result.LinesExecuted,
				"exit_code": result.ExitCode,
			})

			logging.UserErrorf("Script execution failed: %s", result.Error)
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

# Basic conditional execution (checks current screen and executes following lines)
<if-found "$ " 5s>
echo "Shell prompt is visible, continuing..."
ls -la

<if-not-found "error" 5s>
echo "No error message detected, proceeding..."
systemctl status

# Conditional execution with else blocks
<if-found "login:" 10s>
echo "Login prompt found, logging in..."
$USER
<watch "password:" 5s>
$PASSWORD
<enter>
<else>
echo "No login prompt found, trying to connect..."
ssh $USER@$TARGET_HOST

# Loop constructs for automation
<retry 3>
ping -c 1 google.com
echo "Ping successful"

<repeat 5>
echo "This will be repeated 5 times"
<wait 1s>

# While loops with OCR monitoring
<while-not-found "login:" 30s poll 2s>
echo "Waiting for login prompt..."
<wait 2s>

<while-found "loading..." 60s poll 1s>
echo "Still loading, please wait..."
<wait 3s>

# Watch for shell prompt
<watch "$ " 10s>

# Run system commands
uname -a
<wait 1s>

# Check if we need elevated access
sudo -l

# Take screenshot before attempting sudo
<screenshot "before-sudo-{timestamp}.png">

# Conditional password handling
<if-found "password" 5s>
echo "Password required for sudo"
<screenshot "password-prompt.png">
$PASSWORD
<enter>
<else>
echo "No password required, continuing with sudo commands"
<screenshot "no-password-needed.png">

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
exit

# Example of conditional exit with error codes
<if-found "error" 5s>
echo "Error detected, exiting with error code"
<exit 1>

<if-found "critical failure" 5s>
echo "Critical failure detected!"
<exit 2>

# Include functionality - execute other script files
<include "common-functions.script2">
<include "environment-setup.script2">

# Screenshot functionality for debugging and evidence
<screenshot "login-screen.png">
<screenshot "before-update_{timestamp}.png">
<screenshot "system-status.jpg">
<screenshot "debug-{datetime}.ppm">

# Screenshots with variables and timestamps
SCREENSHOT_DIR=${SCREENSHOT_DIR:-./screenshots}
<screenshot "$SCREENSHOT_DIR/final-state-{timestamp}.png">

# Function definitions for reusable code blocks
<function login_attempt>
ssh $1@$2
<watch "password:" 10s>
$3
<enter>
<wait 2s>
<end-function>

# Function with OCR and error handling
<function check_login_status>
<if-found "$ " 5s>
echo "Login successful for user: $1"
<screenshot "login-success-$1.png">
<else>
echo "Login failed for user: $1"
<screenshot "login-failed-$1.png">
<exit 1>
<end-function>

# Using functions with parameters
<call login_attempt $USER $TARGET_HOST $PASSWORD>
<call check_login_status $USER>

# Functions can be called multiple times with different parameters
<call login_attempt admin backup-server secret123>
<call check_login_status admin>

# Example of escaping directive syntax (to type literal angle brackets)
echo "To wait in a script, use: \\<wait 5s>"
echo "To exit with code 1: \\<exit 1>"`

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
		fmt.Printf("# Using environment variables (recommended):\n")
		fmt.Printf("export QMP_VM_ID=106\n")
		fmt.Printf("export QMP_TRAINING_DATA=training.json\n")
		fmt.Printf("qmp script2 my-enhanced-script.txt          # Uses env vars for VM ID and training data\n\n")
		fmt.Printf("# Using configuration profiles:\n")
		fmt.Printf("qmp --profile dev script2 my-script.txt     # Uses dev profile settings\n")
		fmt.Printf("qmp --profile prod script2 my-script.txt    # Uses production configuration\n\n")
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
		fmt.Printf("• Environment variable integration (QMP_VM_ID, QMP_TRAINING_DATA, etc.)\n")
		fmt.Printf("• Configuration profiles (--profile dev/prod/test)\n")
		fmt.Printf("• <if-found \"text\" 5s>, <if-not-found \"text\" 5s> - Conditional execution blocks\n")
		fmt.Printf("• <retry N> - Retry block N times on failure\n")
		fmt.Printf("• <repeat N> - Repeat block N times unconditionally\n")
		fmt.Printf("• <while-found \"text\" 30s poll 1s> - Loop while text is present\n")
		fmt.Printf("• <while-not-found \"text\" 30s poll 1s> - Loop until text appears\n")
		fmt.Printf("• <exit N> - Exit script with specified exit code\n")
		fmt.Printf("• <else> - Else block for if-found/if-not-found conditionals\n")
		fmt.Printf("• <include \"script.txt\"> - Include and execute another script file\n")
		fmt.Printf("• <screenshot \"filename.png\"> - Take screenshot (supports png, jpg, ppm)\n")
		fmt.Printf("• Screenshot timestamps: {timestamp}, {date}, {time}, {datetime}, {unix}\n")
		fmt.Printf("• <function name>, <end-function> - Define reusable functions\n")
		fmt.Printf("• <call function_name args...> - Call functions with parameters ($1, $2, etc.)\n")
		fmt.Printf("• \\<directive> - Escape syntax to type literal angle brackets\n")
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

	// Debugging flags
	script2Cmd.Flags().BoolVar(&debugMode, "debug", false,
		"Enable step-by-step debugging mode")

	script2Cmd.Flags().BoolVar(&debugInteractive, "debug-interactive", false,
		"Enable interactive TUI debugging mode")

	script2Cmd.Flags().BoolVar(&debugEnhanced, "debug-enhanced", false,
		"Enable enhanced TUI debugging with advanced OCR features")

	script2Cmd.Flags().BoolVar(&debugConsole, "debug-console", false,
		"Enable console debugging mode (SSH/remote friendly)")

	script2Cmd.Flags().IntSliceVar(&debugBreakpoints, "breakpoint", []int{},
		"Set breakpoints on specific line numbers (comma-separated)")

	// Add help examples
	script2Cmd.Example = `  # Execute script with enhanced features (no OCR/WATCH)
  qmp script2 106 enhanced-login.script

  # Script with WATCH commands (requires training data)
  qmp script2 106 login.script training.json

  # Enhanced debugging with advanced OCR TUI
  qmp script2 106 login.script training.json --debug-enhanced

  # Override specific variables
  qmp script2 106 login.script training.json --var USER=admin --var RETRY_COUNT=5

  # Load production environment
  qmp script2 106 deploy.script training.json --env-file production.env

  # Test script parsing without execution
  qmp script2 106 complex.script --dry-run --log-level debug`
}
