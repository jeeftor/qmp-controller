package qmp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/jeeftor/qmp/internal/logging"
)

// Client represents a QMP client connection
type Client struct {
	conn       net.Conn
	vmid       string
	reader     *bufio.Reader
	socketPath string
}

// Command represents a QMP command
type Command struct {
	Execute   string      `json:"execute"`
	Arguments interface{} `json:"arguments,omitempty"`
	ID        string      `json:"id,omitempty"`
}

// Response represents a QMP response
type Response struct {
	Return interface{} `json:"return,omitempty"`
	Error  *Error      `json:"error,omitempty"`
	ID     string      `json:"id,omitempty"`
	Event  string      `json:"event,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// Error represents a QMP error
type Error struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// New creates a new QMP client
func New(vmid string) *Client {
	return &Client{vmid: vmid}
}

// NewWithSocketPath creates a new QMP client with a custom socket path
func NewWithSocketPath(vmid string, socketPath string) *Client {
	return &Client{
		vmid:       vmid,
		socketPath: socketPath,
	}
}

// Connect establishes a connection to the QMP socket
func (q *Client) Connect() error {
	var socketPath string
	if q.socketPath != "" {
		socketPath = q.socketPath
	} else {
		socketPath = fmt.Sprintf("/var/run/qemu-server/%s.qmp", q.vmid)
	}

	logging.Debug("Connecting to QMP socket", "path", socketPath)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to QMP socket: %v", err)
	}
	q.conn = conn
	q.reader = bufio.NewReader(conn)

	// Read the greeting message
	var greeting Response
	if err := q.readJSON(&greeting); err != nil {
		q.conn.Close()
		return fmt.Errorf("failed to read greeting: %v", err)
	}
	logging.LogResponse(greeting)

	// Send qmp_capabilities to enable commands
	cmd := Command{Execute: "qmp_capabilities"}
	data, err := json.Marshal(cmd)
	if err != nil {
		q.conn.Close()
		return fmt.Errorf("failed to marshal capabilities command: %v", err)
	}

	logging.LogCommand("qmp_capabilities", nil)
	if _, err := q.conn.Write(data); err != nil {
		q.conn.Close()
		return fmt.Errorf("failed to send capabilities command: %v", err)
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		q.conn.Close()
		return fmt.Errorf("failed to read capabilities response: %v", err)
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		q.conn.Close()
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	logging.Info("Connected to QMP socket", "vmid", q.vmid)
	return nil
}

// Close closes the QMP connection
func (q *Client) Close() error {
	if q.conn != nil {
		logging.Debug("Closing QMP connection", "vmid", q.vmid)
		return q.conn.Close()
	}
	return nil
}

// sendCommand sends a QMP command and returns the response
func (q *Client) sendCommand(cmd Command) (*Response, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %v", err)
	}

	logging.LogCommand(cmd.Execute, cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send command: %v", err)
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return nil, fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return &resp, nil
}

// readJSON reads a JSON object from the QMP socket
func (q *Client) readJSON(v interface{}) error {
	var fullLine []byte
	for {
		line, isPrefix, err := q.reader.ReadLine()
		if err != nil {
			return err
		}
		fullLine = append(fullLine, line...)
		if !isPrefix {
			break
		}
	}

	logging.Debug("Raw JSON received", "json", string(fullLine))
	return json.Unmarshal(fullLine, v)
}

// QueryUSBDevices returns a list of USB devices
func (q *Client) QueryUSBDevices() ([]interface{}, error) {
	cmd := Command{
		Execute: "query-usb",
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	logging.LogCommand("query-usb", nil)
	if _, err := q.conn.Write(data); err != nil {
		return nil, err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return nil, err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return nil, fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	devices, ok := resp.Return.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return devices, nil
}

// AddUSBKeyboard adds a USB keyboard to the VM
func (q *Client) AddUSBKeyboard(id string) error {
	cmd := Command{
		Execute: "device_add",
		Arguments: map[string]interface{}{
			"driver": "usb-kbd",
			"id":     id,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("device_add", cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}

// AddUSBMouse adds a USB mouse to the VM
func (q *Client) AddUSBMouse(id string) error {
	cmd := Command{
		Execute: "device_add",
		Arguments: map[string]interface{}{
			"driver": "usb-mouse",
			"id":     id,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("device_add", cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}

// RemoveDevice removes a device from the VM
func (q *Client) RemoveDevice(deviceID string) error {
	cmd := Command{
		Execute: "device_del",
		Arguments: map[string]interface{}{
			"id": deviceID,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("device_del", cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}

// QueryStatus returns the current VM status
func (q *Client) QueryStatus() (map[string]interface{}, error) {
	cmd := Command{
		Execute: "query-status",
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	logging.LogCommand("query-status", nil)
	if _, err := q.conn.Write(data); err != nil {
		return nil, err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return nil, err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return nil, fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	status, ok := resp.Return.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return status, nil
}

// SendKey sends a key press to the VM
func (q *Client) SendKey(key string) error {
	// Map common key names to QEMU key codes
	keyMap := map[string]string{
		"enter":     "ret",
		"return":    "ret",
		"backspace": "backspace",
		"tab":       "tab",
		"space":     "spc",
		"esc":       "esc",
		"delete":    "delete",
		":":         "shift_r-semicolon", // Colon is shift+semicolon
		";":         "semicolon",
		"!":         "shift_r-1",
		"@":         "shift_r-2",
		"#":         "shift_r-3",
		"$":         "shift_r-4",
		"%":         "shift_r-5",
		"^":         "shift_r-6",
		"&":         "shift_r-7",
		"*":         "shift_r-8",
		"(":         "shift_r-9",
		")":         "shift_r-0",
		"_":         "shift_r-minus",
		"+":         "shift_r-equal",
		"{":         "shift_r-bracketleft",
		"}":         "shift_r-bracketright",
		"|":         "shift_r-backslash",
		"\"":        "shift_r-apostrophe",
		"<":         "shift_r-comma",
		">":         "shift_r-dot",
		"?":         "shift_r-slash",
		"~":         "shift_r-grave_accent",
	}

	// Check if the key is in our map
	qemuKey, ok := keyMap[strings.ToLower(key)]
	if !ok {
		// If not in the map, handle special cases
		if len(key) == 1 {
			// Single character keys
			r := []rune(key)[0]

			// Handle uppercase letters by sending shift+lowercase
			if unicode.IsUpper(r) {
				// First press shift
				shiftCmd := Command{
					Execute: "send-key",
					Arguments: map[string]interface{}{
						"keys": []map[string]string{
							{"type": "qcode", "data": "shift"},
						},
					},
				}

				shiftData, err := json.Marshal(shiftCmd)
				if err != nil {
					return err
				}

				logging.LogCommand("send-key", shiftCmd.Arguments)
				if _, err := q.conn.Write(shiftData); err != nil {
					return err
				}

				var shiftResp Response
				if err := q.readJSON(&shiftResp); err != nil {
					return err
				}
				logging.LogResponse(shiftResp)

				if shiftResp.Error != nil {
					return fmt.Errorf("QMP error: %s: %s", shiftResp.Error.Class, shiftResp.Error.Desc)
				}

				// Then send the lowercase letter
				qemuKey = strings.ToLower(key)
			} else {
				// For lowercase and other characters, use as-is
				qemuKey = key
			}
		} else if strings.HasPrefix(key, "ctrl-") {
			// Handle ctrl key combinations
			qemuKey = key
		} else if strings.HasPrefix(key, "shift-") {
			// Handle shift key combinations
			qemuKey = "shift_r-" + key[6:]
		} else {
			// For multi-character keys not in our map, use as-is
			qemuKey = key
		}
	}

	// For keys that contain a hyphen (like shift_r-semicolon), we need to send multiple keys
	if strings.Contains(qemuKey, "-") {
		parts := strings.Split(qemuKey, "-")
		if len(parts) == 2 {
			// First press the modifier key
			modifierCmd := Command{
				Execute: "send-key",
				Arguments: map[string]interface{}{
					"keys": []map[string]string{
						{"type": "qcode", "data": parts[0]},
					},
				},
			}

			modifierData, err := json.Marshal(modifierCmd)
			if err != nil {
				return err
			}

			logging.LogCommand("send-key", modifierCmd.Arguments)
			if _, err := q.conn.Write(modifierData); err != nil {
				return err
			}

			var modifierResp Response
			if err := q.readJSON(&modifierResp); err != nil {
				return err
			}
			logging.LogResponse(modifierResp)

			if modifierResp.Error != nil {
				return fmt.Errorf("QMP error: %s: %s", modifierResp.Error.Class, modifierResp.Error.Desc)
			}

			// Then send the key
			qemuKey = parts[1]
		}
	}

	cmd := Command{
		Execute: "send-key",
		Arguments: map[string]interface{}{
			"keys": []map[string]string{
				{"type": "qcode", "data": qemuKey},
			},
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("send-key", cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}

// SendKeys sends multiple key presses to the VM
func (q *Client) SendKeys(keys []string, delay time.Duration) error {
	for _, key := range keys {
		if err := q.SendKey(key); err != nil {
			return err
		}
		time.Sleep(delay)
	}
	return nil
}

// SendString sends a string of text to the VM
func (q *Client) SendString(text string, delay time.Duration) error {
	for _, r := range text {
		key := string(r)
		// Handle special characters
		switch r {
		case '\n':
			key = "ret"
		case '\t':
			key = "tab"
		case ' ':
			key = "spc"
		}

		if err := q.SendKey(key); err != nil {
			return err
		}
		time.Sleep(delay)
	}
	return nil
}

// ScreenDump takes a screenshot and saves it as a PPM file
func (q *Client) ScreenDump(filename string, remoteTempPath string) error {
	// Determine the path to use for the screenshot
	tempPath := ""
	if remoteTempPath != "" {
		// Use the provided remote path
		tempPath = remoteTempPath
		logging.Debug("Using remote temporary path for screenshot", "path", tempPath)
	} else {
		// Create a temporary file for the screenshot
		tempFile, err := os.CreateTemp("", "qmp-screenshot-*.ppm")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %v", err)
		}
		tempPath = tempFile.Name()
		defer os.Remove(tempPath)
		tempFile.Close()
		logging.Debug("Created local temporary file for screenshot", "path", tempPath)
	}

	cmd := Command{
		Execute: "screendump",
		Arguments: map[string]interface{}{
			"filename": tempPath,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("screendump", cmd.Arguments)
	if _, err := q.conn.Write(data); err != nil {
		return err
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return err
	}
	logging.LogResponse(resp)

	if resp.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	// If using a remote path, we can't copy the file locally
	if remoteTempPath != "" {
		logging.Info("Screenshot saved on remote server", "path", remoteTempPath)
		logging.Info("You'll need to manually copy the file from the remote server")
		return nil
	}

	// Copy the temporary file to the destination
	srcFile, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("failed to open temporary file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy screenshot: %v", err)
	}

	return nil
}

// ScreenDumpAndConvert takes a screenshot and converts it to PNG
func (q *Client) ScreenDumpAndConvert(filename string, remoteTempPath string) error {
	// For remote paths, we can't do the conversion locally
	if remoteTempPath != "" {
		logging.Info("When using a remote temporary path, only PPM format is supported")
		logging.Info("You'll need to manually convert the file on the remote server")
		return q.ScreenDump(filename, remoteTempPath)
	}

	// Create a temporary file for the PPM screenshot
	tempFile, err := os.CreateTemp("", "qmp-screenshot-*.ppm")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	tempFile.Close()

	// Take the screenshot in PPM format
	if err := q.ScreenDump(tempPath, ""); err != nil {
		return err
	}

	// Convert PPM to PNG using ImageMagick
	cmd := exec.Command("convert", tempPath, filename)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to convert screenshot to PNG (is ImageMagick installed?): %v", err)
	}

	return nil
}
