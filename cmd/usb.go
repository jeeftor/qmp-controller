package cmd

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp/internal/qmp"
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
	Long:  `Add a USB device to the VM. Type can be 'keyboard' or 'mouse'.`,
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		deviceType := args[1]
		deviceID := args[2]

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
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		deviceID := args[1]

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
