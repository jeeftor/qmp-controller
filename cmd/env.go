package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// EnvVar represents an environment variable with its metadata
type EnvVar struct {
	Name        string
	Description string
	DefaultValue string
	Category    string
	CurrentValue string
	IsSet       bool
}

// envCmd represents the env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Display environment variable configuration",
	Long: `Display all available QMP environment variables with their current values and descriptions.

This command shows:
• All supported QMP_* environment variables
• Current values (if set)
• Default values
• Variable descriptions organized by category

Environment variables override config file values but are overridden by command-line flags.`,
	Run: func(cmd *cobra.Command, args []string) {
		displayEnvironmentVariables()
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
}

// getAllEnvVars returns all supported environment variables with metadata
func getAllEnvVars() []EnvVar {
	envVars := []EnvVar{
		// Core Configuration
		{
			Name:         "QMP_LOG_LEVEL",
			Description:  "Logging level (debug, info, warn, error)",
			DefaultValue: "info",
			Category:     "Core",
		},
		{
			Name:         "QMP_SOCKET",
			Description:  "Custom socket path for SSH tunneling",
			DefaultValue: "(auto-detected)",
			Category:     "Core",
		},
		{
			Name:         "QMP_VM_ID",
			Description:  "Default VM ID for operations",
			DefaultValue: "(none)",
			Category:     "Core",
		},

		// OCR Configuration
		{
			Name:         "QMP_COLUMNS",
			Description:  "Screen width in characters",
			DefaultValue: fmt.Sprintf("%d", qmp.DEFAULT_WIDTH),
			Category:     "OCR",
		},
		{
			Name:         "QMP_ROWS",
			Description:  "Screen height in characters",
			DefaultValue: fmt.Sprintf("%d", qmp.DEFAULT_HEIGHT),
			Category:     "OCR",
		},
		{
			Name:         "QMP_TRAINING_DATA",
			Description:  "Path to OCR training data file",
			DefaultValue: qmp.GetDefaultTrainingDataPath(),
			Category:     "OCR",
		},
		{
			Name:         "QMP_IMAGE_FILE",
			Description:  "Default image file for OCR file operations",
			DefaultValue: "(none)",
			Category:     "OCR",
		},
		{
			Name:         "QMP_OUTPUT_FILE",
			Description:  "Default output file for OCR results",
			DefaultValue: "(none)",
			Category:     "OCR",
		},

		// Additional Configuration
		{
			Name:         "QMP_KEY_DELAY",
			Description:  "Keyboard input delay (e.g., 50ms, 100ms)",
			DefaultValue: "50ms",
			Category:     "Keyboard",
		},
		{
			Name:         "QMP_SCRIPT_DELAY",
			Description:  "Script execution delay between commands",
			DefaultValue: "50ms",
			Category:     "Script",
		},
		{
			Name:         "QMP_SCREENSHOT_FORMAT",
			Description:  "Default screenshot format (ppm, png)",
			DefaultValue: "png",
			Category:     "Screenshot",
		},
		{
			Name:         "QMP_COMMENT_CHAR",
			Description:  "Script comment character",
			DefaultValue: "#",
			Category:     "Script",
		},
		{
			Name:         "QMP_CONTROL_CHAR",
			Description:  "Script control command prefix",
			DefaultValue: "<#",
			Category:     "Script",
		},

		// OCR Processing Modes
		{
			Name:         "QMP_ANSI_MODE",
			Description:  "Enable ANSI bitmap output (true/false)",
			DefaultValue: "false",
			Category:     "OCR Processing",
		},
		{
			Name:         "QMP_COLOR_MODE",
			Description:  "Enable color output (true/false)",
			DefaultValue: "false",
			Category:     "OCR Processing",
		},
		{
			Name:         "QMP_FILTER_BLANK_LINES",
			Description:  "Filter out blank lines from output (true/false)",
			DefaultValue: "false",
			Category:     "OCR Processing",
		},
		{
			Name:         "QMP_SHOW_LINE_NUMBERS",
			Description:  "Show line numbers with output (true/false)",
			DefaultValue: "false",
			Category:     "OCR Processing",
		},
		{
			Name:         "QMP_RENDER_SPECIAL_CHARS",
			Description:  "Render special characters visually (true/false)",
			DefaultValue: "false",
			Category:     "OCR Processing",
		},

		// OCR Advanced Features
		{
			Name:         "QMP_SINGLE_CHAR",
			Description:  "Single character extraction mode (true/false)",
			DefaultValue: "false",
			Category:     "OCR Advanced",
		},
		{
			Name:         "QMP_CHAR_ROW",
			Description:  "Row for single character extraction (0-based)",
			DefaultValue: "0",
			Category:     "OCR Advanced",
		},
		{
			Name:         "QMP_CHAR_COL",
			Description:  "Column for single character extraction (0-based)",
			DefaultValue: "0",
			Category:     "OCR Advanced",
		},
		{
			Name:         "QMP_CROP_ROWS",
			Description:  "Crop rows range (format: \"start:end\")",
			DefaultValue: "(none)",
			Category:     "OCR Advanced",
		},
		{
			Name:         "QMP_CROP_COLS",
			Description:  "Crop columns range (format: \"start:end\")",
			DefaultValue: "(none)",
			Category:     "OCR Advanced",
		},
	}

	// Add current values and set status
	for i := range envVars {
		envVar := &envVars[i]

		// Get current value from environment
		currentValue := os.Getenv(envVar.Name)
		envVar.CurrentValue = currentValue
		envVar.IsSet = currentValue != ""

		// Also check viper for computed values
		viperKey := strings.ToLower(strings.TrimPrefix(envVar.Name, "QMP_"))
		if viper.IsSet(viperKey) {
			viperValue := viper.GetString(viperKey)
			if viperValue != "" && !envVar.IsSet {
				envVar.CurrentValue = fmt.Sprintf("%s (from config)", viperValue)
			}
		}
	}

	return envVars
}

// displayEnvironmentVariables shows all environment variables organized by category
func displayEnvironmentVariables() {
	envVars := getAllEnvVars()

	// Group by category
	categories := make(map[string][]EnvVar)
	for _, envVar := range envVars {
		categories[envVar.Category] = append(categories[envVar.Category], envVar)
	}

	// Sort categories for consistent display
	categoryNames := make([]string, 0, len(categories))
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	// Header
	fmt.Println(styles.HeaderStyle.Render("🌍 QMP Environment Variables"))
	fmt.Println()

	// Summary
	setCount := 0
	for _, envVar := range envVars {
		if envVar.IsSet {
			setCount++
		}
	}

	fmt.Printf("%s %d/%d environment variables are currently set\n\n",
		styles.SuccessStyle.Render("ℹ️"), setCount, len(envVars))

	// Display by category
	for _, categoryName := range categoryNames {
		categoryVars := categories[categoryName]

		// Sort variables within category
		sort.Slice(categoryVars, func(i, j int) bool {
			return categoryVars[i].Name < categoryVars[j].Name
		})

		fmt.Println(styles.SectionStyle.Render(fmt.Sprintf("📂 %s", categoryName)))
		fmt.Println()

		for _, envVar := range categoryVars {
			displayEnvVar(envVar)
		}
		fmt.Println()
	}

	// Footer with usage examples
	fmt.Println(styles.SectionStyle.Render("💡 Usage Examples"))
	fmt.Println()
	fmt.Println("  # Set common OCR defaults")
	fmt.Println(styles.CodeStyle.Render("  export QMP_COLUMNS=160"))
	fmt.Println(styles.CodeStyle.Render("  export QMP_ROWS=50"))
	fmt.Println(styles.CodeStyle.Render("  export QMP_TRAINING_DATA=~/my-training.json"))
	fmt.Println()
	fmt.Println("  # Use flexible argument ordering")
	fmt.Println(styles.CodeStyle.Render("  qmp ocr vm 106 output.txt  # training data from QMP_TRAINING_DATA"))
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println(styles.CodeStyle.Render("  export QMP_LOG_LEVEL=debug"))
	fmt.Println()
	fmt.Println("  # Configuration management")
	fmt.Println(styles.CodeStyle.Render("  qmp config init               # Generate comprehensive config file"))
	fmt.Println(styles.CodeStyle.Render("  qmp --profile dev status      # Use profile settings"))
	fmt.Println()

	logging.Success("Environment variable configuration displayed successfully")
}

// displayEnvVar formats and displays a single environment variable
func displayEnvVar(envVar EnvVar) {
	nameStyle := styles.KeyStyle
	if envVar.IsSet {
		nameStyle = styles.SuccessStyle
	}

	// Variable name with status indicator
	statusIndicator := "○"
	if envVar.IsSet {
		statusIndicator = "●"
	}

	fmt.Printf("  %s %s\n",
		nameStyle.Render(statusIndicator),
		nameStyle.Render(envVar.Name))

	// Description
	fmt.Printf("    %s\n", styles.DescriptionStyle.Render(envVar.Description))

	// Current value
	if envVar.IsSet {
		fmt.Printf("    %s %s\n",
			styles.LabelStyle.Render("Current:"),
			styles.ValueStyle.Render(envVar.CurrentValue))
	} else {
		fmt.Printf("    %s %s\n",
			styles.LabelStyle.Render("Current:"),
			styles.MutedStyle.Render("(not set)"))
	}

	// Default value
	fmt.Printf("    %s %s\n",
		styles.LabelStyle.Render("Default:"),
		styles.DefaultStyle.Render(envVar.DefaultValue))

	fmt.Println()
}
