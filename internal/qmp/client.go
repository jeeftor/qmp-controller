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

	"github.com/jeeftor/qmp-controller/internal/logging"
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
	logging.Debug("Raw JSON sent", "json", string(data))
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

// mapSpecialKey maps key names to QEMU key codes using authoritative QEMU keymap data
// Handles both aliases (enter -> ret) and shifted characters (! -> shift+1)
func (q *Client) mapSpecialKey(key string) []string {
	// Handle common aliases first
	aliasMap := map[string]string{
		"enter":     "ret",
		"return":    "ret",
		"space":     "spc",
		" ":         "spc",  // Actual space character
		"escape":    "esc",
		"backspace": "backspace",
		"tab":       "tab",
		"delete":    "delete",
		"-":         "minus",
		"=":         "equal",
		"[":         "bracket_left",
		"]":         "bracket_right",
		"\\":        "backslash",
		"'":         "apostrophe",
		",":         "comma",
		".":         "dot",
		"/":         "slash",
		"`":         "grave_accent",
		";":         "semicolon",
	}

	// Check for direct alias
	if alias, ok := aliasMap[strings.ToLower(key)]; ok {
		return []string{alias}
	}

	// Handle shifted punctuation characters
	shiftedChars := map[string][]string{
		":":  {"shift", "semicolon"}, // Colon is shift+semicolon
		"!":  {"shift", "1"},
		"@":  {"shift", "2"},
		"#":  {"shift", "3"},
		"$":  {"shift", "4"},
		"%":  {"shift", "5"},
		"^":  {"shift", "6"},
		"&":  {"shift", "7"},
		"*":  {"shift", "8"},
		"(":  {"shift", "9"},
		")":  {"shift", "0"},
		"_":  {"shift", "minus"},
		"+":  {"shift", "equal"},
		"{":  {"shift", "bracket_left"},
		"}":  {"shift", "bracket_right"},
		"|":  {"shift", "backslash"},
		"\"": {"shift", "apostrophe"},
		"<":  {"shift", "comma"},
		">":  {"shift", "dot"},
		"?":  {"shift", "slash"},
		"~":  {"shift", "grave_accent"},
	}

	// Check for shifted character
	if keyList, ok := shiftedChars[key]; ok {
		return keyList
	}

	// Check if the key exists directly in the QEMU keymap
	if q.isValidQKeyCode(key) {
		return []string{key}
	}

	// No mapping found
	return nil
}

// SendKey sends a key press to the VM
func (q *Client) SendKey(key string) error {
	var keys []string

	// Use the new mapSpecialKey function that leverages QEMU keymap data
	if keyList := q.mapSpecialKey(key); len(keyList) > 0 {
		keys = keyList
	} else if len([]rune(key)) == 1 {
		// Single character keys (check rune length, not byte length)
		r := []rune(key)[0]

		// Handle uppercase letters by sending shift + lowercase letter
		if unicode.IsUpper(r) {
			keys = []string{"shift", strings.ToLower(key)}
		} else if r > 127 {
			logging.Debug("SendKey detected Unicode character", "char", string(r), "code", int(r))
			// For Unicode characters, check if QEMU supports them natively first
			// If not, fall back to Alt code conversion
			if q.isValidQKeyCode(key) {
				logging.Debug("Unicode character has native QEMU support", "char", string(r), "key", key)
				keys = []string{key}
			} else {
				// Try Alt code conversion as fallback
				altKeys := q.convertToAltCode(r)
				if len(altKeys) > 0 {
					logging.Debug("Converting Unicode character to Alt code", "char", string(r), "code", int(r), "altKeys", altKeys)
					keys = altKeys
				} else {
					// No Alt code mapping available, skip this character
					logging.Debug("No Alt code or QEMU mapping found, skipping character", "char", string(r), "code", int(r))
					return nil
				}
			}
		} else if q.isValidQKeyCode(key) {
			// ASCII character that maps to a valid QKeyCode
			logging.Debug("Using valid QKeyCode", "key", key)
			keys = []string{key}
		} else {
			// ASCII character that's not a valid QKeyCode, skip it
			logging.Debug("Invalid QKeyCode, skipping character", "char", string(r), "key", key)
			return nil
		}
	} else if strings.Contains(key, "-") || strings.Contains(key, "+") {
		// Handle complex key combinations (ctrl-alt-del, ctrl+shift+a, etc.)
		keys = parseKeyCombo(key)
	} else if strings.HasPrefix(key, "0x") {
		// Already a Unicode hex code, use as-is
		keys = []string{key}
	} else {
		// For multi-character keys not in our map, use as-is
		keys = []string{key}
	}

	// Build the keys array for the QMP command
	var qmpKeys []map[string]interface{}
	for _, k := range keys {
		qmpType := "qcode"
		var qmpData interface{}

		// For all keys, use qcode type with plain string
		// QEMU supports Unicode in U#### format as qcode
		qmpData = k

		qmpKeys = append(qmpKeys, map[string]interface{}{
			"type": qmpType,
			"data": qmpData,
		})
	}

	cmd := Command{
		Execute: "send-key",
		Arguments: map[string]interface{}{
			"keys": qmpKeys,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("send-key", cmd.Arguments)
	logging.Debug("Raw JSON sent", "json", string(data))
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

// isModifierKey checks if a key is a modifier key
func isModifierKey(key string) bool {
	modifiers := map[string]bool{
		"shift":   true,
		"shift_r": true,
		"alt":     true,
		"alt_r":   true,
		"ctrl":    true,
		"ctrl_r":  true,
		"meta":    true,
		"meta_r":  true,
	}
	return modifiers[key]
}

// sendKeyEvent sends a key event (press or release) to the VM
func (q *Client) sendKeyEvent(key string, down bool) error {
	qmpType := "qcode"
	var qmpData interface{}

	// For all keys, use qcode type with plain string
	// QEMU supports Unicode in U#### format as qcode
	qmpData = key

	cmd := Command{
		Execute: "send-key",
		Arguments: map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"type": qmpType,
					"data": qmpData,
					"down": down,
				},
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

// parseKeyCombo parses complex key combinations like ctrl-alt-del, ctrl+shift+a, etc.
func parseKeyCombo(combo string) []string {
	// Normalize separators to use '-' consistently
	normalized := strings.ReplaceAll(combo, "+", "-")

	// Split on '-' to get individual components
	parts := strings.Split(normalized, "-")

	// Map common modifier aliases
	modifierMap := map[string]string{
		"ctrl":    "ctrl",
		"control": "ctrl",
		"alt":     "alt",
		"shift":   "shift",
		"cmd":     "cmd",
		"super":   "cmd",
		"meta":    "meta",
	}

	var keys []string

	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))

		// Check if it's a known modifier
		if modifier, ok := modifierMap[part]; ok {
			keys = append(keys, modifier)
		} else {
			// Map some common key aliases
			switch part {
			case "del", "delete":
				keys = append(keys, "delete")
			case "enter", "return":
				keys = append(keys, "ret")
			case "space":
				keys = append(keys, "spc")
			case "escape", "esc":
				keys = append(keys, "esc")
			case "backspace", "bksp":
				keys = append(keys, "backspace")
			case "tab":
				keys = append(keys, "tab")
			default:
				// For other keys (letters, numbers, function keys), use as-is
				keys = append(keys, part)
			}
		}
	}

	return keys
}

// QKeyInfo represents QEMU key mapping data
type QKeyInfo struct {
	EvdevCode int    // Linux evdev code
	QKeyCode  string // QEMU key code name
	ScanCode  int    // Hardware scan code
}

// getQEMUKeyMappings returns the authoritative QEMU key mappings from EN-US keymap
// https://github.com/qemu/qemu/blob/master/pc-bios/keymaps/en-us
func getQEMUKeyMappings() map[string]QKeyInfo {
	return map[string]QKeyInfo{
		// From your QEMU EN-US keymap data
		"esc":              {1, "esc", 0x1},
		"1":                {2, "1", 0x2},
		"2":                {3, "2", 0x3},
		"3":                {4, "3", 0x4},
		"4":                {5, "4", 0x5},
		"5":                {6, "5", 0x6},
		"6":                {7, "6", 0x7},
		"7":                {8, "7", 0x8},
		"8":                {9, "8", 0x9},
		"9":                {10, "9", 0xa},
		"0":                {11, "0", 0xb},
		"minus":            {12, "minus", 0xc},
		"equal":            {13, "equal", 0xd},
		"backspace":        {14, "backspace", 0xe},
		"tab":              {15, "tab", 0xf},
		"q":                {16, "q", 0x10},
		"w":                {17, "w", 0x11},
		"e":                {18, "e", 0x12},
		"r":                {19, "r", 0x13},
		"t":                {20, "t", 0x14},
		"y":                {21, "y", 0x15},
		"u":                {22, "u", 0x16},
		"i":                {23, "i", 0x17},
		"o":                {24, "o", 0x18},
		"p":                {25, "p", 0x19},
		"bracket_left":     {26, "bracket_left", 0x1a},
		"bracket_right":    {27, "bracket_right", 0x1b},
		"ret":              {28, "ret", 0x1c},
		"ctrl":             {29, "ctrl", 0x1d},
		"a":                {30, "a", 0x1e},
		"s":                {31, "s", 0x1f},
		"d":                {32, "d", 0x20},
		"f":                {33, "f", 0x21},
		"g":                {34, "g", 0x22},
		"h":                {35, "h", 0x23},
		"j":                {36, "j", 0x24},
		"k":                {37, "k", 0x25},
		"l":                {38, "l", 0x26},
		"semicolon":        {39, "semicolon", 0x27},
		"apostrophe":       {40, "apostrophe", 0x28},
		"grave_accent":     {41, "grave_accent", 0x29},
		"shift":            {42, "shift", 0x2a},
		"backslash":        {43, "backslash", 0x2b},
		"z":                {44, "z", 0x2c},
		"x":                {45, "x", 0x2d},
		"c":                {46, "c", 0x2e},
		"v":                {47, "v", 0x2f},
		"b":                {48, "b", 0x30},
		"n":                {49, "n", 0x31},
		"m":                {50, "m", 0x32},
		"comma":            {51, "comma", 0x33},
		"dot":              {52, "dot", 0x34},
		"slash":            {53, "slash", 0x35},
		"shift_r":          {54, "shift_r", 0x36},
		"kp_multiply":      {55, "kp_multiply", 0x37},
		"alt":              {56, "alt", 0x38},
		"spc":              {57, "spc", 0x39},
		"caps_lock":        {58, "caps_lock", 0x3a},
		"f1":               {59, "f1", 0x3b},
		"f2":               {60, "f2", 0x3c},
		"f3":               {61, "f3", 0x3d},
		"f4":               {62, "f4", 0x3e},
		"f5":               {63, "f5", 0x3f},
		"f6":               {64, "f6", 0x40},
		"f7":               {65, "f7", 0x41},
		"f8":               {66, "f8", 0x42},
		"f9":               {67, "f9", 0x43},
		"f10":              {68, "f10", 0x44},
		"num_lock":         {69, "num_lock", 0x45},
		"scroll_lock":      {70, "scroll_lock", 0x46},
		"kp_7":             {71, "kp_7", 0x47},
		"kp_8":             {72, "kp_8", 0x48},
		"kp_9":             {73, "kp_9", 0x49},
		"kp_subtract":      {74, "kp_subtract", 0x4a},
		"kp_4":             {75, "kp_4", 0x4b},
		"kp_5":             {76, "kp_5", 0x4c},
		"kp_6":             {77, "kp_6", 0x4d},
		"kp_add":           {78, "kp_add", 0x4e},
		"kp_1":             {79, "kp_1", 0x4f},
		"kp_2":             {80, "kp_2", 0x50},
		"kp_3":             {81, "kp_3", 0x51},
		"kp_0":             {82, "kp_0", 0x52},
		"kp_decimal":       {83, "kp_decimal", 0x53},
		"less":             {86, "less", 0x56},
		"f11":              {87, "f11", 0x57},
		"f12":              {88, "f12", 0x58},
		"ro":               {89, "ro", 0x73},
		"hiragana":         {91, "hiragana", 0x77},
		"henkan":           {92, "henkan", 0x79},
		"katakanahiragana": {93, "katakanahiragana", 0x70},
		"muhenkan":         {94, "muhenkan", 0x7b},
		"kp_enter":         {96, "kp_enter", 0x9c},
		"ctrl_r":           {97, "ctrl_r", 0x9d},
		"kp_divide":        {98, "kp_divide", 0xb5},
		"sysrq":            {99, "sysrq", 0x54},
		"alt_r":            {100, "alt_r", 0xb8},
		"lf":               {101, "lf", 0x5b},
		"home":             {102, "home", 0xc7},
		"up":               {103, "up", 0xc8},
		"pgup":             {104, "pgup", 0xc9},
		"left":             {105, "left", 0xcb},
		"right":            {106, "right", 0xcd},
		"end":              {107, "end", 0xcf},
		"down":             {108, "down", 0xd0},
		"pgdn":             {109, "pgdn", 0xd1},
		"insert":           {110, "insert", 0xd2},
		"delete":           {111, "delete", 0xd3},
		"audiomute":        {113, "audiomute", 0xa0},
		"volumedown":       {114, "volumedown", 0xae},
		"volumeup":         {115, "volumeup", 0xb0},
		"power":            {116, "power", 0xde},
		"kp_equals":        {117, "kp_equals", 0x59},
		"pause":            {119, "pause", 0xc6},
		"kp_comma":         {121, "kp_comma", 0x7e},
		"yen":              {124, "yen", 0x7d},
		"meta_l":           {125, "meta_l", 0xdb},
		"meta_r":           {126, "meta_r", 0xdc},
		"compose":          {127, "compose", 0xdd},
		"stop":             {128, "stop", 0xe8},
		"again":            {129, "again", 0x85},
		"props":            {130, "props", 0x86},
		"undo":             {131, "undo", 0x87},
		"front":            {132, "front", 0x8c},
		"copy":             {133, "copy", 0xf8},
		"open":             {134, "open", 0x64},
		"paste":            {135, "paste", 0x65},
		"find":             {136, "find", 0xc1},
		"cut":              {137, "cut", 0xbc},
		"help":             {138, "help", 0xf5},
		"menu":             {139, "menu", 0x9e},
		"calculator":       {140, "calculator", 0xa1},
		"sleep":            {142, "sleep", 0xdf},
		"wake":             {143, "wake", 0xe3},
		"mail":             {155, "mail", 0xec},
		"ac_bookmarks":     {156, "ac_bookmarks", 0xe6},
		"computer":         {157, "computer", 0xeb},
		"ac_back":          {158, "ac_back", 0xea},
		"ac_forward":       {159, "ac_forward", 0xe9},
		"audionext":        {163, "audionext", 0x99},
		"audioplay":        {164, "audioplay", 0xa2},
		"audioprev":        {165, "audioprev", 0x90},
		"audiostop":        {166, "audiostop", 0xa4},
		"ac_home":          {172, "ac_home", 0xb2},
		"ac_refresh":       {173, "ac_refresh", 0xe7},
		"mediaselect":      {226, "mediaselect", 0xed},
	}
}

// isValidQKeyCode checks if a key maps to a valid QEMU QKeyCode
func (q *Client) isValidQKeyCode(key string) bool {
	keyMappings := getQEMUKeyMappings()
	_, exists := keyMappings[key]
	return exists
}

// getQKeyInfo returns the QEMU key mapping info for a key
func (q *Client) getQKeyInfo(key string) (QKeyInfo, bool) {
	keyMappings := getQEMUKeyMappings()
	info, exists := keyMappings[key]
	return info, exists
}

// convertToAltCode converts a Unicode character to Alt code key sequence
func (q *Client) convertToAltCode(r rune) []string {
	// Comprehensive Alt code mappings from IBM PC / Windows Alt codes
	// Only includes characters that can actually be entered via Alt+number
	altCodeMap := map[rune]int{
		// Special symbols and faces (0-31)
		'☺': 1,   '☻': 2,   '♥': 3,   '♦': 4,   '♣': 5,   '♠': 6,   '•': 7,
		'◘': 8,   '○': 9,   '◙': 10,  '♂': 11,  '♀': 12,  '♪': 13,  '♫': 14,  '☼': 15,
		'►': 16,  '◄': 17,  '↕': 18,  '‼': 19,  '¶': 20,  '§': 21,  '▬': 22,  '↨': 23,
		'↑': 24,  '↓': 25,  '→': 26,  '←': 27,  '∟': 28,  '↔': 29,  '▲': 30,  '▼': 31,

		// ASCII control characters (127)
		'⌂': 127,

		// Latin-1 Supplement characters (128-175)
		'Ç': 128, 'ü': 129, 'é': 130, 'â': 131, 'ä': 132, 'à': 133, 'å': 134, 'ç': 135,
		'ê': 136, 'ë': 137, 'è': 138, 'ï': 139, 'î': 140, 'ì': 141, 'Ä': 142, 'Å': 143,
		'É': 144, 'æ': 145, 'Æ': 146, 'ô': 147, 'ö': 148, 'ò': 149, 'û': 150, 'ù': 151,
		'ÿ': 152, 'Ö': 153, 'Ü': 154, '¢': 155, '£': 156, '¥': 157, '₧': 158, 'ƒ': 159,
		'á': 160, 'í': 161, 'ó': 162, 'ú': 163, 'ñ': 164, 'Ñ': 165, 'ª': 166, 'º': 167,
		'¿': 168, '⌐': 169, '¬': 170, '½': 171, '¼': 172, '¡': 173, '«': 174, '»': 175,

		// Box drawing and block elements (176-223)
		'░': 176, '▒': 177, '▓': 178, '│': 179, '┤': 180, 'Á': 181, 'Â': 182, 'À': 183,
		'©': 184, '╣': 185, '║': 186, '╗': 187, '╝': 188, '¤': 189, '®': 190, '┐': 191,
		'└': 192, '┴': 193, '┬': 194, '├': 195, '─': 196, '┼': 197, 'ã': 198, 'Ã': 199,
		'╚': 200, '╔': 201, '╩': 202, '╦': 203, '╠': 204, '═': 205, '╬': 206, '₫': 207,
		'ð': 208, 'Ð': 209, 'Ê': 210, 'Ë': 211, 'È': 212, 'ı': 213, 'Í': 214, 'Î': 215,
		'Ï': 216, '┘': 217, '┌': 218, '█': 219, '▄': 220, '¦': 221, 'Ì': 222, '▀': 223,

		// Greek letters and mathematical symbols (224-255)
		'α': 224, 'ß': 225, 'Γ': 226, 'π': 227, 'Σ': 228, 'σ': 229, 'µ': 230, 'τ': 231,
		'Φ': 232, 'Θ': 233, 'Ω': 234, 'δ': 235, '∞': 236, 'φ': 237, 'ε': 238, '∩': 239,
		'≡': 240, '±': 241, '≥': 242, '≤': 243, '⌠': 244, '⌡': 245, '÷': 246, '≈': 247,
		'°': 248, '∙': 249, '·': 250, '√': 251, 'ⁿ': 252, '²': 253, '■': 254, ' ': 255,

		// Additional box drawing characters (using available codes)
		'╒': 213, '╓': 214, '╕': 184, '╖': 183, '╘': 212, '╙': 211, '╛': 190, '╜': 189,
		'╞': 198, '╟': 199, '╢': 185, '╤': 209, '╥': 210, '╧': 207, '╨': 208, '╪': 216,
		'╫': 215,
	}

	// Check if we have an Alt code mapping for this character
	if altCode, exists := altCodeMap[r]; exists {
		// Convert the Alt code number to individual keypad digits
		return q.altCodeToKeys(altCode)
	}

	// No mapping found
	return nil
}

// altCodeToKeys converts an Alt code number to a sequence of keypad key presses
func (q *Client) altCodeToKeys(altCode int) []string {
	keys := []string{"alt"} // Start with Alt key

	// Convert number to string and add each digit as keypad key
	codeStr := fmt.Sprintf("%d", altCode)
	for _, digit := range codeStr {
		keys = append(keys, fmt.Sprintf("kp_%c", digit))
	}

	return keys
}

// TestConvertToAltCode is a public wrapper for testing the Alt code conversion
func (q *Client) TestConvertToAltCode(r rune) []string {
	return q.convertToAltCode(r)
}

// SendString sends a string of text to the VM
func (q *Client) SendString(text string, delay time.Duration) error {
	for _, r := range text {
		logging.Debug("SendString processing character", "char", string(r), "code", int(r))
		key := string(r)

		logging.Debug("SendString calling SendKey", "key", key)
		if err := q.SendKey(key); err != nil {
			return fmt.Errorf("failed to send character '%s' (code %d): %w", string(r), int(r), err)
		}
		time.Sleep(delay)
	}
	return nil
}

// SendRawJSON sends a raw JSON command directly to the QMP socket
func (q *Client) SendRawJSON(jsonStr string) error {
	// Validate that it's valid JSON by unmarshaling and re-marshaling
	var cmd interface{}
	if err := json.Unmarshal([]byte(jsonStr), &cmd); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Re-marshal to ensure clean formatting
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	logging.Debug("Sending raw JSON command", "json", string(data))
	if _, err := q.conn.Write(data); err != nil {
		return fmt.Errorf("failed to send raw JSON: %w", err)
	}

	var resp Response
	if err := q.readJSON(&resp); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
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

// SendKeyCombo sends multiple keys simultaneously as a key combination
func (q *Client) SendKeyCombo(keys []string) error {
	// Build the keys array for the QMP command, validating each key
	var qmpKeys []map[string]interface{}
	for _, k := range keys {
		// Use mapSpecialKey to validate and map the key
		mappedKeys := q.mapSpecialKey(k)
		if len(mappedKeys) == 0 {
			// If no mapping found, check if it's a valid qcode directly
			if !q.isValidQKeyCode(k) {
				return fmt.Errorf("invalid key for combo: %s", k)
			}
			mappedKeys = []string{k}
		}

		// For key combos, we only expect single keys, not sequences
		if len(mappedKeys) > 1 {
			return fmt.Errorf("key combo cannot contain key sequences: %s maps to %v", k, mappedKeys)
		}

		qmpKeys = append(qmpKeys, map[string]interface{}{
			"type": "qcode",
			"data": mappedKeys[0],
		})
	}

	cmd := Command{
		Execute: "send-key",
		Arguments: map[string]interface{}{
			"keys": qmpKeys,
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	logging.LogCommand("send-key", cmd.Arguments)
	logging.Debug("Raw JSON sent", "json", string(data))
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
