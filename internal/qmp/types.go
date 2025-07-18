package qmp

import "time"

// USBDevice represents a USB device in the VM
type USBDevice struct {
	Driver string `json:"driver"`
	ID     string `json:"id"`
	Bus    string `json:"bus,omitempty"`
}

// KeyPress represents a key press event
type KeyPress struct {
	Key string `json:"keys"`
}

// KeyPresses represents multiple key press events
type KeyPresses struct {
	Keys  []string      `json:"keys"`
	Hold  time.Duration `json:"hold,omitempty"`
	Delay time.Duration `json:"delay,omitempty"`
}

// DeviceAdd represents a device addition command
type DeviceAdd struct {
	Driver string `json:"driver"`
	ID     string `json:"id"`
}

// DeviceDel represents a device removal command
type DeviceDel struct {
	ID string `json:"id"`
}

// Status represents the VM status
type Status struct {
	Running    bool   `json:"running"`
	Status     string `json:"status"`
	Singlestep bool   `json:"singlestep"`
	Pause      bool   `json:"pause"`
}

// Screenshot represents a screenshot command
type Screenshot struct {
	Filename string `json:"filename"`
}
