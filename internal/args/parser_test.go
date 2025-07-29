package args

import (
	"os"
	"testing"
)

func TestScriptArgumentParser(t *testing.T) {
	// Create a temporary script file for testing
	tmpFile, err := os.CreateTemp("", "test-script.sc2")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	parser := NewScriptArgumentParser()

	tests := []struct {
		name     string
		args     []string
		wantVMID string
		wantErr  bool
	}{
		{
			name:     "three args format",
			args:     []string{"123", tmpFile.Name(), "training.json"},
			wantVMID: "123",
			wantErr:  false,
		},
		{
			name:     "two args with numeric VMID",
			args:     []string{"456", tmpFile.Name()},
			wantVMID: "456",
			wantErr:  false,
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "nonexistent script file",
			args:    []string{"123", "nonexistent.sc2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.VMID != tt.wantVMID {
				t.Errorf("Expected VMID %s, got %s", tt.wantVMID, result.VMID)
			}
		})
	}
}

func TestUSBArgumentParser(t *testing.T) {
	parser := NewUSBArgumentParser()

	tests := []struct {
		name        string
		args        []string
		wantSubCmd  string
		wantVMID    string
		wantErr     bool
	}{
		{
			name:       "list command",
			args:       []string{"list"},
			wantSubCmd: "list",
			wantVMID:   "",
			wantErr:    false,
		},
		{
			name:       "add command with VMID",
			args:       []string{"add", "123", "/dev/ttyUSB0"},
			wantSubCmd: "add",
			wantVMID:   "123",
			wantErr:    false,
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.SubCommand != tt.wantSubCmd {
				t.Errorf("Expected subcommand %s, got %s", tt.wantSubCmd, result.SubCommand)
			}

			if result.VMID != tt.wantVMID {
				t.Errorf("Expected VMID %s, got %s", tt.wantVMID, result.VMID)
			}
		})
	}
}

func TestKeyboardArgumentParser(t *testing.T) {
	parser := NewKeyboardArgumentParser()

	tests := []struct {
		name        string
		args        []string
		wantSubCmd  string
		wantVMID    string
		wantErr     bool
	}{
		{
			name:       "live command with VMID",
			args:       []string{"live", "123"},
			wantSubCmd: "live",
			wantVMID:   "123",
			wantErr:    false,
		},
		{
			name:       "send command with VMID and key",
			args:       []string{"send", "456", "enter"},
			wantSubCmd: "send",
			wantVMID:   "456",
			wantErr:    false,
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.SubCommand != tt.wantSubCmd {
				t.Errorf("Expected subcommand %s, got %s", tt.wantSubCmd, result.SubCommand)
			}

			if result.VMID != tt.wantVMID {
				t.Errorf("Expected VMID %s, got %s", tt.wantVMID, result.VMID)
			}
		})
	}
}

func TestSimpleArgumentParser(t *testing.T) {
	parser := NewSimpleArgumentParser("test")

	tests := []struct {
		name     string
		args     []string
		wantVMID string
		wantErr  bool
	}{
		{
			name:     "with VMID",
			args:     []string{"123", "additional", "args"},
			wantVMID: "123",
			wantErr:  false,
		},
		{
			name:    "no VMID available",
			args:    []string{"not-a-vmid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.VMID != tt.wantVMID {
				t.Errorf("Expected VMID %s, got %s", tt.wantVMID, result.VMID)
			}
		})
	}
}

func TestFileTypeDetection(t *testing.T) {
	tests := []struct {
		filename string
		isImage  bool
		isJSON   bool
	}{
		{"test.png", true, false},
		{"test.jpg", true, false},
		{"test.jpeg", true, false},
		{"test.ppm", true, false},
		{"test.json", false, true},
		{"test.txt", false, false},
		{"test.PNG", true, false}, // Test case insensitive
		{"test.JSON", false, true}, // Test case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if isImage := isImageFile(tt.filename); isImage != tt.isImage {
				t.Errorf("isImageFile(%s) = %v, want %v", tt.filename, isImage, tt.isImage)
			}

			if isJSON := isJSONFile(tt.filename); isJSON != tt.isJSON {
				t.Errorf("isJSONFile(%s) = %v, want %v", tt.filename, isJSON, tt.isJSON)
			}
		})
	}
}
