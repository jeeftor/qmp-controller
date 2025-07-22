package cmd

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp-controller/internal/qmp"
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

		status, err := client.QueryStatus()
		if err != nil {
			fmt.Printf("Error querying status for VM %s: %v\n", vmid, err)
			os.Exit(1)
		}

		fmt.Printf("Status for VM %s:\n", vmid)
		fmt.Printf("  Running: %v\n", status["running"])
		fmt.Printf("  Status: %v\n", status["status"])

		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			fmt.Printf("Debug - Full status response: %+v\n", status)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	// Here you will define your flags and configuration settings.
}
