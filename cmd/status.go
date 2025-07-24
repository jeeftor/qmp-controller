package cmd

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [vmid]",
	Short: "Query VM status",
	Long:  `Query the current status of a QEMU virtual machine using QMP.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]

		// Start timer and create contextual logger
		timer := logging.StartTimer("status_query", vmid)
		logger := logging.NewContextualLogger(vmid, "status")

		logger.Debug("Status command started")

		client, err := ConnectToVM(vmid)
		if err != nil {
			logger.Error("Failed to connect to VM", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "connection",
			})
			os.Exit(1)
		}
		defer client.Close()

		status, err := client.QueryStatus()
		if err != nil {
			logger.Error("Failed to query VM status", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "status_query",
			})
			os.Exit(1)
		}

		// Extract key status values for structured logging
		running, runningOk := status["running"]
		statusValue, statusOk := status["status"]

		// Log structured status information
		statusMetrics := map[string]interface{}{
			"running_present": runningOk,
			"status_present":  statusOk,
		}

		if runningOk {
			statusMetrics["running"] = running
		}
		if statusOk {
			statusMetrics["status"] = statusValue
		}

		logger.Info("VM status retrieved",
			"running", running,
			"status", statusValue,
			"full_response", status)

		duration := timer.Stop(true, statusMetrics)

		// Display user-friendly output
		logging.Result("Status for VM %s", vmid)
		fmt.Printf("  Running: %v\n", running)
		fmt.Printf("  Status: %v\n", statusValue)

		// Debug output if requested
		debugFlag, _ := cmd.Flags().GetBool("debug")
		if debugFlag {
			logger.Debug("Debug output requested", "full_status", status)
			fmt.Printf("Debug - Full status response: %+v\n", status)
			fmt.Printf("Debug - Query duration: %v\n", duration)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	// Here you will define your flags and configuration settings.
}
