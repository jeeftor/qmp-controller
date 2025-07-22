package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/embedded"
	"github.com/spf13/cobra"
)

var (
	outputDir   string
	listOnly    bool
	forceExtract bool
)

// generateScriptsCmd represents the generatescripts command
var generateScriptsCmd = &cobra.Command{
	Use:   "generatescripts",
	Short: "Extract embedded training scripts to local directory",
	Long: `Extract all embedded OCR training scripts to a local directory.

This command extracts the training scripts that are embedded in the binary,
making them available for use on air-gapped systems without needing to
transfer multiple script files.

The extracted scripts include:
- training_pipeline.sh - Complete automated training pipeline
- send_ansi.sh - Send ANSI escape sequences via QMP
- clear_screen_simple.sh - Reliable screen clearing
- structured_ocr_training.sh - Display structured character sets
- generate_ocr_training_chars.sh - Generate character sequences
- And more...

Examples:
  # Extract scripts to ./scripts directory
  qmp generatescripts

  # Extract scripts to custom directory
  qmp generatescripts --output /tmp/qmp-scripts

  # List available scripts without extracting
  qmp generatescripts --list

  # Force overwrite existing files
  qmp generatescripts --force`,
	Run: func(cmd *cobra.Command, args []string) {
		if listOnly {
			// List embedded scripts
			scripts, err := embedded.ListEmbeddedScripts()
			if err != nil {
				fmt.Printf("Error listing embedded scripts: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Embedded training scripts:")
			for _, script := range scripts {
				fmt.Printf("  %s\n", script)
			}
			fmt.Printf("\nTotal: %d scripts\n", len(scripts))
			return
		}

		// Determine output directory
		targetDir := outputDir
		if targetDir == "" {
			// Default to ./scripts in current directory
			pwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current directory: %v\n", err)
				os.Exit(1)
			}
			targetDir = filepath.Join(pwd, "scripts")
		}

		// Check if directory exists and has files
		if !forceExtract {
			if _, err := os.Stat(targetDir); err == nil {
				// Directory exists, check if it has files
				entries, err := os.ReadDir(targetDir)
				if err == nil && len(entries) > 0 {
					fmt.Printf("Directory %s already exists and contains files.\n", targetDir)
					fmt.Println("Use --force to overwrite existing files, or specify a different --output directory.")
					os.Exit(1)
				}
			}
		}

		// Prompt user for confirmation
		fmt.Printf("Going to write scripts to %s\n", targetDir)
		fmt.Print("Proceed? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Operation cancelled.")
			os.Exit(0)
		}

		// Extract scripts
		fmt.Printf("Extracting embedded scripts to: %s\n", targetDir)
		if err := embedded.ExtractScripts(targetDir); err != nil {
			fmt.Printf("Error extracting scripts: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nScripts extracted successfully!")
		fmt.Printf("Run scripts from: cd %s\n", targetDir)
		fmt.Println("\nQuick start:")
		fmt.Printf("  cd %s\n", targetDir)
		fmt.Println("  ./training_pipeline.sh --vmid 108")
		fmt.Println("  ./clear_screen_simple.sh --vmid 108")
	},
}

func init() {
	rootCmd.AddCommand(generateScriptsCmd)

	// Add flags
	generateScriptsCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for extracted scripts (default: ./scripts)")
	generateScriptsCmd.Flags().BoolVarP(&listOnly, "list", "l", false, "List embedded scripts without extracting")
	generateScriptsCmd.Flags().BoolVar(&forceExtract, "force", false, "Force overwrite existing files")
}
