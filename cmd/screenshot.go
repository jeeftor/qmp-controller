package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jstein/qmp/internal/qmp"
	"github.com/spf13/cobra"
)

var (
	screenshotOutput string
	format           string
)

// screenshotCmd represents the screenshot command
var screenshotCmd = &cobra.Command{
	Use:   "screenshot [vmid]",
	Short: "Take a screenshot of the VM",
	Long: `Take a screenshot of the virtual machine's display.

Supported formats: ppm (default), png (requires ImageMagick)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]

		// Set default output filename if not provided
		if screenshotOutput == "" {
			screenshotOutput = fmt.Sprintf("screenshot_%s_%d.%s",
				vmid,
				time.Now().Unix(),
				format,
			)
		}

		// Ensure output directory exists
		outputDir := filepath.Dir(screenshotOutput)
		if outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Printf("Error creating output directory: %v\n", err)
				os.Exit(1)
			}
		}

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

		var err error
		if strings.ToLower(format) == "png" {
			err = client.ScreenDumpAndConvert(screenshotOutput)
		} else {
			err = client.ScreenDump(screenshotOutput)
		}

		if err != nil {
			fmt.Printf("Error taking screenshot: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Screenshot saved to %s\n", screenshotOutput)
	},
}

func init() {
	rootCmd.AddCommand(screenshotCmd)

	// Add flags for screenshot command
	screenshotCmd.Flags().StringVarP(&screenshotOutput, "output", "o", "", "output file path")
	screenshotCmd.Flags().StringVarP(&format, "format", "f", "ppm", "output format (ppm, png)")
}
