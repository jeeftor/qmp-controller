package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jstein/qmp/internal/logging"
	"github.com/jstein/qmp/internal/qmp"
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

When using SSH tunneling with the --socket flag, you may need to specify
a temporary path on the remote server using --remote-temp flag.

Examples:
  # Take a screenshot and save it as PNG
  qmp screenshot 106 screenshot.png

  # Take a screenshot with a specific format
  qmp screenshot 106 screenshot.ppm --format ppm

  # Take a screenshot with SSH tunneling
  qmp screenshot 106 screenshot.png --socket /tmp/qmp-106.sock --remote-temp /tmp/qmp-screenshot.ppm`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		outputFile := args[1]

		// Create output directory if it doesn't exist
		outputDir := filepath.Dir(outputFile)
		if outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Printf("Error creating output directory: %v\n", err)
				os.Exit(1)
			}
		}

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

		// Get format from flag, config, or file extension
		format := getScreenshotFormat(outputFile)

		// Get remote temp path from flag or config
		remotePath := getRemoteTempPath()

		var err error
		if format == "png" {
			logging.Debug("Taking screenshot in PNG format", "output", outputFile, "remoteTempPath", remotePath)
			err = client.ScreenDumpAndConvert(outputFile, remotePath)
		} else {
			logging.Debug("Taking screenshot in PPM format", "output", outputFile, "remoteTempPath", remotePath)
			err = client.ScreenDump(outputFile, remotePath)
		}

		if err != nil {
			fmt.Printf("Error taking screenshot: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Screenshot saved to %s\n", outputFile)
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

// getRemoteTempPath determines the remote temp path to use based on flag or config
func getRemoteTempPath() string {
	// Priority 1: Command line flag
	if remoteTempPath != "" {
		return remoteTempPath
	}

	// Priority 2: Config file
	if viper.IsSet("screenshot.remote_temp_path") {
		return viper.GetString("screenshot.remote_temp_path")
	}

	// Default to empty string (local temp file)
	return ""
}

func init() {
	rootCmd.AddCommand(screenshotCmd)
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "", "screenshot format (ppm, png)")
	screenshotCmd.Flags().StringVarP(&remoteTempPath, "remote-temp", "r", "", "temporary path on remote server (for SSH tunneling)")

	// Bind flags to viper
	viper.BindPFlag("screenshot.format", screenshotCmd.Flags().Lookup("format"))
	viper.BindPFlag("screenshot.remote_temp_path", screenshotCmd.Flags().Lookup("remote-temp"))
}
