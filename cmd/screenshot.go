package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	screenshotFormat string
	remoteTempPath   string
)

// screenshotCmd represents the screenshot command
var screenshotCmd = &cobra.Command{
	Use:   "screenshot [vmid] [output-file]",
	Short: "Take a screenshot of the VM",
	Long: `Take a screenshot of the VM and save it to a file.
The output format can be specified with the --format flag.
Supported formats: ppm, png

The VM ID and output file can be provided as arguments or set via environment variables:
- QMP_VM_ID for the VM ID
- QMP_OUTPUT_FILE for the output file

When using SSH tunneling with the --socket flag, you may need to specify
a temporary path on the remote server using --remote-temp flag.

Examples:
  # Explicit arguments
  qmp screenshot 106 screenshot.png

  # Using environment variables
  export QMP_VM_ID=106
  export QMP_OUTPUT_FILE=screenshot.png
  qmp screenshot

  # Take a screenshot with a specific format
  qmp screenshot 106 screenshot.ppm --format ppm

  # Take a screenshot with SSH tunneling
  qmp screenshot 106 screenshot.png --socket /tmp/qmp-106.sock --remote-temp /tmp/qmp-screenshot.ppm`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve parameters using parameter resolver
		resolver := params.NewParameterResolver()

		// Resolve VM ID
		vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		vmid := vmidInfo.Value

		// Resolve output file
		outputFileInfo := resolver.ResolveOutputFileWithInfo(args, 1)
		if outputFileInfo.Value == "" {
			fmt.Fprintf(os.Stderr, "Error: Output file is required: provide as argument or set QMP_OUTPUT_FILE environment variable\n")
			os.Exit(1)
		}
		outputFile := outputFileInfo.Value

		// Log parameter resolution for debugging
		if vmidInfo.Source != "argument" || outputFileInfo.Source != "argument" {
			logging.Debug("Parameters resolved from non-argument sources",
				"vmid", vmid, "vmid_source", vmidInfo.Source,
				"output_file", outputFile, "output_source", outputFileInfo.Source)
		}

		// Start timer for performance monitoring
		timer := logging.StartTimer("screenshot", vmid)

		// Create contextual logger
		logger := logging.NewContextualLogger(vmid, "screenshot")

		logger.Debug("Screenshot command started",
			"output_file", outputFile,
			"format_flag", screenshotFormat,
			"remote_temp", remoteTempPath)

		// Create output directory if it doesn't exist
		outputDir := filepath.Dir(outputFile)
		if outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				logger.Error("Failed to create output directory",
					"output_dir", outputDir,
					"error", err)
				timer.StopWithError(err, map[string]interface{}{
					"stage": "directory_creation",
				})
				os.Exit(1)
			}
			logger.Debug("Created output directory", "output_dir", outputDir)
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			logger.Error("Failed to connect to VM", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "connection",
			})
			os.Exit(1)
		}
		defer client.Close()

		// Get format from flag, config, or file extension
		format := getScreenshotFormat(outputFile)
		logger.Debug("Determined screenshot format", "format", format)

		// Take screenshot using centralized helper
		if err := TakeScreenshot(client, outputFile, format); err != nil {
			logger.Error("Failed to take screenshot", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "screenshot_capture",
				"format": format,
			})
			os.Exit(1)
		}

		// Get file size for metrics
		if stat, statErr := os.Stat(outputFile); statErr == nil {
			duration := timer.Stop(true, map[string]interface{}{
				"format": format,
				"file_size": stat.Size(),
				"output_path": outputFile,
			})

			logging.Successf("Screenshot saved to %s (%s, %d bytes, %v)",
				outputFile, format, stat.Size(), duration)
		} else {
			timer.Stop(true, map[string]interface{}{
				"format": format,
				"output_path": outputFile,
			})
			logging.Successf("Screenshot saved to %s (%s)", outputFile, format)
		}
	},
}

// getScreenshotFormat determines the format to use based on flag, config, or file extension
func getScreenshotFormat(outputFile string) string {
	// Priority 1: Command line flag
	if screenshotFormat != "" {
		return strings.ToLower(screenshotFormat)
	}

	// Priority 2: Config file
	if viper.IsSet("screenshot.format") {
		return strings.ToLower(viper.GetString("screenshot.format"))
	}

	// Priority 3: File extension
	ext := strings.ToLower(filepath.Ext(outputFile))
	if ext == ".png" {
		return "png"
	}

	// Default to PPM
	return "ppm"
}


func init() {
	rootCmd.AddCommand(screenshotCmd)
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "", "screenshot format (ppm, png)")
	screenshotCmd.Flags().StringVarP(&remoteTempPath, "remote-temp", "r", "", "temporary path on remote server (for SSH tunneling)")

	// Bind flags to viper
	viper.BindPFlag("screenshot.format", screenshotCmd.Flags().Lookup("format"))
	viper.BindPFlag("screenshot.remote_temp_path", screenshotCmd.Flags().Lookup("remote-temp"))
}
