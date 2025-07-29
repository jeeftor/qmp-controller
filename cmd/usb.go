package cmd

import (
	"fmt"

	"github.com/jeeftor/qmp-controller/internal/args"
	"github.com/jeeftor/qmp-controller/internal/ui"
	"github.com/jeeftor/qmp-controller/internal/utils"
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
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		// Parse arguments using the new argument parser
		argParser := args.NewUSBArgumentParser()
		parsedArgs := args.ParseWithHandler(append([]string{"list"}, cmdArgs...), argParser)

		// List command doesn't need VM ID, but if provided, use it
		vmid := ""
		if parsedArgs.VMID != "" {
			vmid = parsedArgs.VMID
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()

		devices, err := client.QueryUSBDevices()
		if err != nil {
			utils.FatalError(err, "listing USB devices")
		}

		ui.USBDeviceMessage(vmid, "listed", "", "")
		if len(devices) == 0 {
			ui.InfoMessage("No USB devices connected")
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
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		// Parse arguments using the new argument parser
		argParser := args.NewUSBArgumentParser()
		parsedArgs := args.ParseWithHandler(append([]string{"add"}, cmdArgs...), argParser)

		// Extract parsed values
		vmid := parsedArgs.VMID
		if len(parsedArgs.RemainingArgs) < 2 {
			utils.ValidationError(fmt.Errorf("device type and device ID are required"))
		}
		deviceType := parsedArgs.RemainingArgs[0]
		deviceID := parsedArgs.RemainingArgs[1]

		client, err := ConnectToVM(vmid)
		if err != nil {
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()
		switch deviceType {
		case "keyboard":
			err = client.AddUSBKeyboard(deviceID)
		case "mouse":
			err = client.AddUSBMouse(deviceID)
		default:
			utils.ValidationError(fmt.Errorf("unknown device type: %s. Supported types: keyboard, mouse", deviceType))
		}

		if err != nil {
			utils.FatalError(err, fmt.Sprintf("adding USB %s", deviceType))
		}

		ui.USBDeviceMessage(vmid, "added", deviceType, deviceID)
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
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		// Parse arguments using the new argument parser
		argParser := args.NewUSBArgumentParser()
		parsedArgs := args.ParseWithHandler(append([]string{"remove"}, cmdArgs...), argParser)

		// Extract parsed values
		vmid := parsedArgs.VMID
		if len(parsedArgs.RemainingArgs) < 1 {
			utils.ValidationError(fmt.Errorf("device ID is required"))
		}
		deviceID := parsedArgs.RemainingArgs[0]

		client, err := ConnectToVM(vmid)
		if err != nil {
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()

		if err := client.RemoveDevice(deviceID); err != nil {
			utils.FatalError(err, fmt.Sprintf("removing device %s", deviceID))
		}

		ui.USBDeviceMessage(vmid, "removed", "", deviceID)
	},
}

func init() {
	rootCmd.AddCommand(usbCmd)
	usbCmd.AddCommand(listUSBCmd)
	usbCmd.AddCommand(addUSBCmd)
	usbCmd.AddCommand(removeUSBCmd)
}
