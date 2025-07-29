package cmd

import (
	"fmt"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/jeeftor/qmp-controller/internal/utils"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [vmid]",
	Short: "Query VM status",
	Long:  `Query the current status of a QEMU virtual machine using QMP.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp status 106

  # Using environment variable
  export QMP_VM_ID=106
  qmp status`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()
		vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
		if err != nil {
			utils.ValidationError(err)
		}
		vmid := vmidInfo.Value

		// Log parameter resolution for debugging
		if vmidInfo.Source != "argument" {
			logging.Debug("Parameter resolved from non-argument source",
				"vmid", vmid,
				"source", vmidInfo.Source)
		}

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
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()

		status, err := client.QueryStatus()
		if err != nil {
			logger.Error("Failed to query VM status", "error", err)
			timer.StopWithError(err, map[string]interface{}{
				"stage": "status_query",
			})
			utils.FatalError(err, "querying VM status")
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
