package cmd

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/spf13/cobra"
)

// usbCmd represents the usb command
var usbCmd = &cobra.Command{
	Use:   "usb",
	Short: "Manage USB devices",
	Long:  `Manage USB devices attached to the virtual machine.`,
}

var listUSBCmd = &cobra.Command{
	Use:   "list [vmid]",
	Short: "List USB devices",
	Long:  `List USB devices attached to the virtual machine.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp usb list 106

  # Using environment variable
  export QMP_VM_ID=106
  qmp usb list`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()
		vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		vmid := vmidInfo.Value

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

		devices, err := client.QueryUSBDevices()
		if err != nil {
			fmt.Printf("Error listing USB devices: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("USB devices for VM %s:\n", vmid)
		if len(devices) == 0 {
			fmt.Println("No USB devices connected")
			return
		}

		for i, dev := range devices {
			fmt.Printf("Device %d: %+v\n", i+1, dev)
		}
	},
}

var addUSBCmd = &cobra.Command{
	Use:   "add [vmid] [type] [id]",
	Short: "Add a USB device",
	Long:  `Add a USB device to the VM. Type can be 'keyboard' or 'mouse'.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp usb add 106 keyboard usb-kbd1

  # Using environment variable
  export QMP_VM_ID=106
  qmp usb add keyboard usb-kbd1`,
	Args:  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()

		// Check if first arg is VM ID or device type
		var vmid, deviceType, deviceID string
		if len(args) == 3 {
			// Traditional format: vmid type id
			vmid = args[0]
			deviceType = args[1]
			deviceID = args[2]
		} else if len(args) == 2 {
			// New format with env var: type id
			vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1) // No args, use env var
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			vmid = vmidInfo.Value
			deviceType = args[0]
			deviceID = args[1]
		} else {
			fmt.Fprintf(os.Stderr, "Error: Invalid number of arguments\n")
			os.Exit(1)
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

		var err error
		switch deviceType {
		case "keyboard":
			err = client.AddUSBKeyboard(deviceID)
		case "mouse":
			err = client.AddUSBMouse(deviceID)
		default:
			fmt.Printf("Unknown device type: %s. Supported types: keyboard, mouse\n", deviceType)
			os.Exit(1)
		}

		if err != nil {
			fmt.Printf("Error adding USB %s: %v\n", deviceType, err)
			os.Exit(1)
		}

		fmt.Printf("Added USB %s with ID %s to VM %s\n", deviceType, deviceID, vmid)
	},
}

var removeUSBCmd = &cobra.Command{
	Use:   "remove [vmid] [id]",
	Short: "Remove a USB device",
	Long:  `Remove a USB device from the VM by its device ID.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.

Examples:
  # Explicit VM ID
  qmp usb remove 106 usb-kbd1

  # Using environment variable
  export QMP_VM_ID=106
  qmp usb remove usb-kbd1`,
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve VM ID using parameter resolver
		resolver := params.NewParameterResolver()

		// Check if first arg is VM ID or device ID
		var vmid, deviceID string
		if len(args) == 2 {
			// Traditional format: vmid deviceid
			vmid = args[0]
			deviceID = args[1]
		} else if len(args) == 1 {
			// New format with env var: deviceid
			vmidInfo, err := resolver.ResolveVMIDWithInfo([]string{}, -1) // No args, use env var
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			vmid = vmidInfo.Value
			deviceID = args[0]
		} else {
			fmt.Fprintf(os.Stderr, "Error: Device ID is required\n")
			os.Exit(1)
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

		if err := client.RemoveDevice(deviceID); err != nil {
			fmt.Printf("Error removing device %s: %v\n", deviceID, err)
			os.Exit(1)
		}

		fmt.Printf("Removed device %s from VM %s\n", deviceID, vmid)
	},
}

func init() {
	rootCmd.AddCommand(usbCmd)
	usbCmd.AddCommand(listUSBCmd)
	usbCmd.AddCommand(addUSBCmd)
	usbCmd.AddCommand(removeUSBCmd)
}
