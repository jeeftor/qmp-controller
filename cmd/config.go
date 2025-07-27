package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/styles"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage QMP configuration files and settings",
	Long: `Manage configuration files, profiles, and settings for QMP Controller.

The config command provides tools to:
• Generate sample configuration files
• Validate existing configurations
• Manage VM profiles and connection settings
• View current configuration sources and values

Configuration files are searched in this order:
1. ~/.qmp.yaml (user config)
2. ./.qmp.yaml (project config)
3. /etc/qmp/.qmp.yaml (system config)

Environment variables (QMP_*) override config file values.
Command-line flags override both config files and environment variables.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// configInitCmd creates a sample configuration file
var configInitCmd = &cobra.Command{
	Use:   "init [config-file]",
	Short: "Create a sample configuration file",
	Long: `Generate a sample configuration file with all available options and documentation.

If no file is specified, creates ~/.qmp.yaml in the user's home directory.
The generated file includes:
• All configuration options with descriptions
• Sample VM profiles
• Common use case examples
• Environment variable mappings

Examples:
  qmp config init                    # Create ~/.qmp.yaml
  qmp config init project.yaml      # Create project.yaml
  qmp config init .qmp.yaml          # Create local project config`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var configPath string

		if len(args) > 0 {
			configPath = args[0]
		} else {
			// Default to user home directory
			home, err := os.UserHomeDir()
			if err != nil {
				logging.UserErrorf("Could not determine home directory: %v", err)
				os.Exit(1)
			}
			configPath = filepath.Join(home, ".qmp.yaml")
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			logging.UserErrorf("Could not resolve config path: %v", err)
			os.Exit(1)
		}

		// Check if file already exists
		if _, err := os.Stat(absPath); err == nil {
			logging.UserErrorf("Configuration file already exists: %s", absPath)
			logging.UserInfof("Use 'qmp config validate %s' to check the existing file", absPath)
			os.Exit(1)
		}

		// Generate sample configuration
		sampleConfig := generateSampleConfig()

		// Write to file
		if err := os.WriteFile(absPath, []byte(sampleConfig), 0644); err != nil {
			logging.UserErrorf("Failed to write configuration file: %v", err)
			os.Exit(1)
		}

		logging.Successf("Created configuration file: %s", absPath)
		logging.UserInfo("Edit the file to customize your settings")
		logging.UserInfof("Use 'qmp config validate %s' to check the configuration", absPath)
	},
}

// configValidateCmd validates a configuration file
var configValidateCmd = &cobra.Command{
	Use:   "validate [config-file]",
	Short: "Validate a configuration file",
	Long: `Validate the syntax and content of a configuration file.

If no file is specified, validates the current active configuration.
Checks for:
• Valid YAML syntax
• Required field presence
• Value type validation
• Cross-field dependencies
• File path accessibility

Examples:
  qmp config validate                # Validate current active config
  qmp config validate ~/.qmp.yaml   # Validate specific file
  qmp config validate project.yaml  # Validate project config`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var configPath string

		if len(args) > 0 {
			configPath = args[0]
			// Validate specific file
			if err := validateConfigFile(configPath); err != nil {
				logging.UserErrorf("Configuration validation failed: %v", err)
				os.Exit(1)
			}
		} else {
			// Validate current active configuration
			validateCurrentConfig()
		}
	},
}

// configShowCmd displays current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration values and sources",
	Long: `Display the current configuration with sources and effective values.

Shows:
• All configuration keys and their current values
• Source of each value (config file, environment, default)
• Configuration file locations and load status
• Environment variable overrides
• Effective configuration after all sources are merged

This is useful for debugging configuration issues and understanding
how values are resolved from multiple sources.`,
	Run: func(cmd *cobra.Command, args []string) {
		displayCurrentConfiguration()
	},
}

// configPathCmd shows configuration file search paths
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Display configuration file search paths",
	Long: `Display the paths where QMP looks for configuration files.

Shows:
• Current active configuration file (if any)
• All search paths in priority order
• File existence status for each path
• Recommended locations for different use cases

This helps understand where to place configuration files
and troubleshoot configuration loading issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		displayConfigPaths()
	},
}

// configProfilesCmd lists available profiles
var configProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available configuration profiles",
	Long: `List all available configuration profiles defined in the current configuration.

Profiles are named collections of settings that can be activated with the --profile flag.
This is useful for managing different environments (dev/staging/prod) or
different VM configurations.

Examples:
  qmp config profiles                    # List all profiles
  qmp --profile dev status               # Use dev profile for status command
  qmp --profile prod ocr vm              # Use prod profile for OCR`,
	Run: func(cmd *cobra.Command, args []string) {
		listConfigProfiles()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configProfilesCmd)
}

// generateSampleConfig creates a comprehensive sample configuration
func generateSampleConfig() string {
	return `# QMP Controller Configuration File
# This file configures default settings for the QMP Controller CLI tool.
#
# Configuration Priority (highest to lowest):
# 1. Command-line flags
# 2. Environment variables (QMP_*)
# 3. Configuration file values
# 4. Built-in defaults
#
# File locations (first found is used):
# - ~/.qmp.yaml (user configuration)
# - ./.qmp.yaml (project configuration)
# - /etc/qmp/.qmp.yaml (system configuration)

# =============================================================================
# CORE SETTINGS
# =============================================================================

# Default logging level (trace, debug, info, warn, error)
log_level: info

# Default VM ID for operations (can be overridden by QMP_VM_ID environment variable)
# vm_id: 106

# Custom socket path for SSH tunneling scenarios
# Uncomment and set if using remote QMP connections
# socket: /tmp/qmp-tunnel-socket

# =============================================================================
# OCR CONFIGURATION
# =============================================================================

# Default screen dimensions for OCR operations
columns: 160          # Screen width in characters
rows: 50              # Screen height in characters

# Default training data file path
# If not set, uses ~/.qmp_training_data.json
# training_data: /path/to/custom-training.json

# Default image file for OCR file operations
# image_file: /path/to/screenshot.ppm

# Default output file for OCR results
# output_file: /path/to/ocr-output.txt

# OCR Processing Options
ansi_mode: false               # Enable ANSI bitmap output
color_mode: false              # Enable color output
filter_blank_lines: false     # Filter out blank lines from output
show_line_numbers: false      # Show line numbers with output
render_special_chars: false   # Render special characters visually

# Advanced OCR Options
single_char: false             # Single character extraction mode
char_row: 0                    # Row for single character extraction (0-based)
char_col: 0                    # Column for single character extraction (0-based)

# OCR Cropping (format: "start:end")
# crop_rows: "10:40"           # Crop rows range
# crop_cols: "20:140"          # Crop columns range

# =============================================================================
# KEYBOARD & SCRIPT SETTINGS
# =============================================================================

# Timing Configuration
key_delay: 50ms                # Keyboard input delay
script_delay: 50ms             # Script execution delay between commands

# Script Syntax Configuration
comment_char: "#"              # Character that starts comment lines
control_char: "<#"             # Prefix for control commands

# Screenshot Settings
screenshot_format: png         # Default screenshot format (ppm, png)

# =============================================================================
# VM PROFILES (Advanced Configuration)
# =============================================================================

# VM profiles allow you to define named configurations for different VMs
# Use with: qmp --profile dev status
profiles:
  # Development VM
  dev:
    vm_id: 106
    socket: /var/run/qemu-server/106.qmp
    training_data: ~/dev-training.json
    columns: 160
    rows: 50

  # Production VM (remote connection)
  prod:
    vm_id: 200
    socket: /tmp/prod-qmp-tunnel
    training_data: ~/prod-training.json
    columns: 120
    rows: 30
    screenshot_format: ppm

  # Testing VM with special OCR settings
  test:
    vm_id: 999
    training_data: ~/test-training.json
    columns: 80
    rows: 25
    ansi_mode: true
    color_mode: true
    filter_blank_lines: true

# =============================================================================
# ADVANCED SETTINGS
# =============================================================================

# Script execution configuration
script:
  # WATCH command settings (for script automation)
  watch_poll_interval: 1s       # How often to check for text during WATCH
  watch_width: 160              # Screen width for WATCH OCR
  watch_height: 50              # Screen height for WATCH OCR

  # Script safety settings
  max_script_duration: 300s     # Maximum script execution time
  confirm_destructive: true     # Confirm before running destructive operations

# Screenshot configuration
screenshot:
  # Remote screenshot settings (for SSH tunneling)
  remote_temp_path: ""          # Remote temporary file path
  compression_quality: 90       # JPEG compression quality (1-100)

# Keyboard configuration
keyboard:
  # Live mode settings
  live_history_size: 20         # Number of commands to show in live mode
  live_refresh_rate: 100ms      # Screen refresh rate in live mode

# Connection settings
connection:
  timeout: 30s                  # Connection timeout
  retry_attempts: 3             # Number of connection retry attempts
  retry_delay: 1s               # Delay between retry attempts

# =============================================================================
# ENVIRONMENT VARIABLE MAPPINGS
# =============================================================================

# All settings above can be overridden by environment variables with QMP_ prefix:
#
# Core Settings:
#   QMP_LOG_LEVEL, QMP_VM_ID, QMP_SOCKET
#
# OCR Settings:
#   QMP_COLUMNS, QMP_ROWS, QMP_TRAINING_DATA, QMP_IMAGE_FILE, QMP_OUTPUT_FILE
#   QMP_ANSI_MODE, QMP_COLOR_MODE, QMP_FILTER_BLANK_LINES, QMP_SHOW_LINE_NUMBERS
#   QMP_RENDER_SPECIAL_CHARS, QMP_SINGLE_CHAR, QMP_CHAR_ROW, QMP_CHAR_COL
#   QMP_CROP_ROWS, QMP_CROP_COLS
#
# Timing & Format:
#   QMP_KEY_DELAY, QMP_SCRIPT_DELAY, QMP_SCREENSHOT_FORMAT
#   QMP_COMMENT_CHAR, QMP_CONTROL_CHAR
#
# Example usage:
#   export QMP_VM_ID=106
#   export QMP_LOG_LEVEL=debug
#   export QMP_TRAINING_DATA=~/my-training.json
#   qmp status  # Uses VM 106 with debug logging and custom training data

# =============================================================================
# USAGE EXAMPLES
# =============================================================================

# 1. Basic OCR with custom settings:
#    qmp ocr vm 106 --columns 120 --rows 30
#
# 2. Using environment variables:
#    export QMP_VM_ID=106
#    export QMP_COLUMNS=120
#    qmp ocr vm
#
# 3. Script automation with WATCH:
#    qmp script vm 106 automation.txt training.json
#
# 4. Live keyboard mode:
#    qmp keyboard live 106
#
# 5. USB device management:
#    qmp usb list 106
#    qmp usb add 106 /dev/disk/by-id/usb-device
`
}

// validateConfigFile validates a specific configuration file
func validateConfigFile(configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Create a new viper instance for validation
	v := viper.New()
	v.SetConfigFile(configPath)

	// Try to read the config file
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %v", err)
	}

	logging.Successf("Configuration file is valid: %s", configPath)

	// Show some basic info about the config
	allKeys := v.AllKeys()
	sort.Strings(allKeys)

	fmt.Printf("\n%s\n", styles.SectionStyle.Render("📋 Configuration Summary"))
	fmt.Printf("File: %s\n", configPath)
	fmt.Printf("Keys found: %d\n\n", len(allKeys))

	if len(allKeys) > 0 {
		fmt.Printf("%s\n", styles.SectionStyle.Render("🔑 Configuration Keys"))
		for _, key := range allKeys {
			value := v.Get(key)
			fmt.Printf("  %s: %v\n", styles.KeyStyle.Render(key), styles.ValueStyle.Render(fmt.Sprintf("%v", value)))
		}
	}

	return nil
}

// validateCurrentConfig validates the currently active configuration
func validateCurrentConfig() {
	fmt.Printf("%s\n\n", styles.HeaderStyle.Render("🔍 Current Configuration Validation"))

	// Check if a config file is currently loaded
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fmt.Printf("Active config file: %s\n\n", styles.SuccessStyle.Render(configFile))
		if err := validateConfigFile(configFile); err != nil {
			logging.UserErrorf("Current configuration is invalid: %v", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("%s\n\n", styles.WarningStyle.Render("No configuration file is currently loaded"))
		fmt.Printf("Using defaults and environment variables only.\n")
		fmt.Printf("Use 'qmp config init' to create a configuration file.\n\n")
	}

	// Show environment variable overrides
	showEnvironmentOverrides()

	logging.Success("Configuration validation completed")
}

// displayCurrentConfiguration shows all current config values and sources
func displayCurrentConfiguration() {
	fmt.Printf("%s\n\n", styles.HeaderStyle.Render("⚙️  Current Configuration"))

	// Show active config file
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fmt.Printf("📁 Active config file: %s\n", styles.SuccessStyle.Render(configFile))
	} else {
		fmt.Printf("📁 Active config file: %s\n", styles.MutedStyle.Render("none"))
	}
	fmt.Println()

	// Get all current configuration
	allSettings := viper.AllSettings()
	if len(allSettings) == 0 {
		fmt.Printf("%s\n", styles.WarningStyle.Render("No configuration values found"))
		return
	}

	// Organize settings by category
	categories := map[string][]string{
		"Core":           {"log_level", "vm_id", "socket"},
		"OCR":            {"columns", "rows", "training_data", "image_file", "output_file"},
		"OCR Processing": {"ansi_mode", "color_mode", "filter_blank_lines", "show_line_numbers", "render_special_chars"},
		"OCR Advanced":   {"single_char", "char_row", "char_col", "crop_rows", "crop_cols"},
		"Timing":         {"key_delay", "script_delay"},
		"Script":         {"comment_char", "control_char"},
		"Screenshot":     {"screenshot_format"},
	}

	// Display each category
	for category, keys := range categories {
		hasValues := false
		for _, key := range keys {
			if viper.IsSet(key) {
				hasValues = true
				break
			}
		}

		if !hasValues {
			continue
		}

		fmt.Printf("%s\n", styles.SectionStyle.Render(fmt.Sprintf("📂 %s", category)))

		for _, key := range keys {
			if viper.IsSet(key) {
				value := viper.Get(key)
				source := getConfigSource(key)

				fmt.Printf("  %s: %s %s\n",
					styles.KeyStyle.Render(key),
					styles.ValueStyle.Render(fmt.Sprintf("%v", value)),
					styles.MutedStyle.Render(fmt.Sprintf("(%s)", source)))
			}
		}
		fmt.Println()
	}

	// Show any other settings not in categories
	fmt.Printf("%s\n", styles.SectionStyle.Render("📂 Other Settings"))
	allKeys := viper.AllKeys()
	sort.Strings(allKeys)

	categoryKeys := make(map[string]bool)
	for _, keys := range categories {
		for _, key := range keys {
			categoryKeys[key] = true
		}
	}

	hasOther := false
	for _, key := range allKeys {
		if !categoryKeys[key] {
			hasOther = true
			value := viper.Get(key)
			source := getConfigSource(key)

			fmt.Printf("  %s: %s %s\n",
				styles.KeyStyle.Render(key),
				styles.ValueStyle.Render(fmt.Sprintf("%v", value)),
				styles.MutedStyle.Render(fmt.Sprintf("(%s)", source)))
		}
	}

	if !hasOther {
		fmt.Printf("  %s\n", styles.MutedStyle.Render("None"))
	}
	fmt.Println()
}

// getConfigSource determines where a configuration value came from
func getConfigSource(key string) string {
	// Check if environment variable exists
	envKey := "QMP_" + strings.ToUpper(key)
	if os.Getenv(envKey) != "" {
		return "environment"
	}

	// Check if config file is loaded and contains the key
	if viper.ConfigFileUsed() != "" {
		// This is a bit of a hack since Viper doesn't provide direct source info
		v := viper.New()
		v.SetConfigFile(viper.ConfigFileUsed())
		if err := v.ReadInConfig(); err == nil {
			if v.IsSet(key) {
				return "config file"
			}
		}
	}

	return "default"
}

// displayConfigPaths shows configuration file search paths
func displayConfigPaths() {
	fmt.Printf("%s\n\n", styles.HeaderStyle.Render("📍 Configuration File Paths"))

	// Current active config
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fmt.Printf("🟢 Active: %s\n", styles.SuccessStyle.Render(configFile))
	} else {
		fmt.Printf("🔴 Active: %s\n", styles.MutedStyle.Render("none"))
	}
	fmt.Println()

	// Search paths in order
	fmt.Printf("%s\n", styles.SectionStyle.Render("🔍 Search Paths (in priority order)"))

	searchPaths := []struct {
		path        string
		description string
	}{
		{filepath.Join(os.Getenv("HOME"), ".qmp.yaml"), "User configuration"},
		{"./.qmp.yaml", "Project configuration"},
		{"/etc/qmp/.qmp.yaml", "System configuration"},
	}

	for i, sp := range searchPaths {
		exists := ""
		if _, err := os.Stat(sp.path); err == nil {
			exists = styles.SuccessStyle.Render("✓ exists")
		} else {
			exists = styles.MutedStyle.Render("✗ not found")
		}

		fmt.Printf("  %d. %s\n", i+1, sp.path)
		fmt.Printf("     %s - %s\n", exists, sp.description)
		fmt.Println()
	}

	// Usage tips
	fmt.Printf("%s\n", styles.SectionStyle.Render("💡 Tips"))
	fmt.Printf("• Use 'qmp config init' to create a user configuration\n")
	fmt.Printf("• Use 'qmp config init .qmp.yaml' for project-specific settings\n")
	fmt.Printf("• Use '--config path/to/config.yaml' to specify a custom config file\n")
	fmt.Printf("• Environment variables (QMP_*) override config file values\n")
}

// showEnvironmentOverrides displays current environment variable overrides
func showEnvironmentOverrides() {
	fmt.Printf("%s\n", styles.SectionStyle.Render("🌍 Environment Variable Overrides"))

	// Check for QMP_* environment variables
	envVars := []string{
		"QMP_LOG_LEVEL", "QMP_VM_ID", "QMP_SOCKET",
		"QMP_COLUMNS", "QMP_ROWS", "QMP_TRAINING_DATA", "QMP_IMAGE_FILE", "QMP_OUTPUT_FILE",
		"QMP_ANSI_MODE", "QMP_COLOR_MODE", "QMP_FILTER_BLANK_LINES", "QMP_SHOW_LINE_NUMBERS",
		"QMP_RENDER_SPECIAL_CHARS", "QMP_SINGLE_CHAR", "QMP_CHAR_ROW", "QMP_CHAR_COL",
		"QMP_CROP_ROWS", "QMP_CROP_COLS", "QMP_KEY_DELAY", "QMP_SCRIPT_DELAY",
		"QMP_SCREENSHOT_FORMAT", "QMP_COMMENT_CHAR", "QMP_CONTROL_CHAR",
	}

	overrides := make(map[string]string)
	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			overrides[envVar] = value
		}
	}

	if len(overrides) == 0 {
		fmt.Printf("  %s\n", styles.MutedStyle.Render("No environment variable overrides found"))
	} else {
		for envVar, value := range overrides {
			fmt.Printf("  %s = %s\n",
				styles.KeyStyle.Render(envVar),
				styles.ValueStyle.Render(value))
		}
	}
	fmt.Println()
}

// listConfigProfiles displays all available configuration profiles
func listConfigProfiles() {
	fmt.Printf("%s\n\n", styles.HeaderStyle.Render("👥 Configuration Profiles"))

	// Check if a config file is loaded
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		fmt.Printf("%s\n", styles.WarningStyle.Render("No configuration file loaded"))
		fmt.Printf("Use 'qmp config init' to create a configuration file with sample profiles.\n")
		return
	}

	// Get all profiles
	profilesMap := viper.GetStringMap("profiles")
	if len(profilesMap) == 0 {
		fmt.Printf("%s\n", styles.WarningStyle.Render("No profiles defined in configuration"))
		fmt.Printf("Add profiles to your configuration file: %s\n", configFile)
		fmt.Printf("Use 'qmp config init --help' for examples.\n")
		return
	}

	fmt.Printf("📁 Config file: %s\n", styles.SuccessStyle.Render(configFile))
	fmt.Printf("🔢 Total profiles: %d\n\n", len(profilesMap))

	// Display each profile
	profileNames := make([]string, 0, len(profilesMap))
	for name := range profilesMap {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)

	for _, profileName := range profileNames {
		profileKey := fmt.Sprintf("profiles.%s", profileName)
		profileSettings := viper.GetStringMap(profileKey)

		fmt.Printf("%s\n", styles.SectionStyle.Render(fmt.Sprintf("📋 Profile: %s", profileName)))

		if len(profileSettings) == 0 {
			fmt.Printf("  %s\n", styles.MutedStyle.Render("(no settings)"))
		} else {
			// Sort settings for consistent display
			settingKeys := make([]string, 0, len(profileSettings))
			for key := range profileSettings {
				settingKeys = append(settingKeys, key)
			}
			sort.Strings(settingKeys)

			for _, key := range settingKeys {
				value := profileSettings[key]
				fmt.Printf("  %s: %s\n",
					styles.KeyStyle.Render(key),
					styles.ValueStyle.Render(fmt.Sprintf("%v", value)))
			}
		}
		fmt.Println()
	}

	// Usage examples
	fmt.Printf("%s\n", styles.SectionStyle.Render("💡 Usage Examples"))
	if len(profileNames) > 0 {
		exampleProfile := profileNames[0]
		fmt.Printf("• Use profile: %s\n", styles.CodeStyle.Render(fmt.Sprintf("qmp --profile %s status", exampleProfile)))
		fmt.Printf("• With OCR: %s\n", styles.CodeStyle.Render(fmt.Sprintf("qmp --profile %s ocr vm", exampleProfile)))
		fmt.Printf("• List USB: %s\n", styles.CodeStyle.Render(fmt.Sprintf("qmp --profile %s usb list", exampleProfile)))
	}
	fmt.Printf("• Create profile: Add to %s under 'profiles:' section\n", filepath.Base(configFile))
}
