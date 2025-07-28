package script

import (
	"testing"
	"time"
)

// Test the core command pattern functionality without complex dependencies
func TestScriptCommandInterface(t *testing.T) {
	tests := []struct {
		name string
		cmd  ScriptCommand
	}{
		{
			name: "TypeCommand implements interface",
			cmd:  &TypeCommand{Text: "hello"},
		},
		{
			name: "KeyCommand implements interface",
			cmd:  &KeyCommand{Key: "enter"},
		},
		{
			name: "SleepCommand implements interface",
			cmd:  &SleepCommand{Duration: 1 * time.Second},
		},
		{
			name: "ConsoleCommand implements interface",
			cmd:  &ConsoleCommand{ConsoleNumber: 1},
		},
		{
			name: "WatchCommand implements interface",
			cmd:  &WatchCommand{SearchString: "test", Timeout: 30 * time.Second, PollInterval: 1 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that all commands implement the interface methods
			if tt.cmd.String() == "" {
				t.Errorf("String() should not return empty string")
			}

			// Test validation - should not panic
			err := tt.cmd.Validate()
			// We don't test the specific validation logic here, just that it doesn't panic
			_ = err
		})
	}
}

func TestTypeCommandBasics(t *testing.T) {
	cmd := &TypeCommand{Text: "hello world"}

	if cmd.String() != "TYPE hello world" {
		t.Errorf("Expected 'TYPE hello world', got '%s'", cmd.String())
	}

	if err := cmd.Validate(); err != nil {
		t.Errorf("Valid TypeCommand should not fail validation: %v", err)
	}
}

func TestSleepCommandBasics(t *testing.T) {
	cmd := &SleepCommand{Duration: 5 * time.Second}

	if cmd.String() != "SLEEP 5s" {
		t.Errorf("Expected 'SLEEP 5s', got '%s'", cmd.String())
	}

	if err := cmd.Validate(); err != nil {
		t.Errorf("Valid SleepCommand should not fail validation: %v", err)
	}
}

func TestConsoleCommandBasics(t *testing.T) {
	cmd := &ConsoleCommand{ConsoleNumber: 3}

	if cmd.String() != "CONSOLE 3" {
		t.Errorf("Expected 'CONSOLE 3', got '%s'", cmd.String())
	}

	if err := cmd.Validate(); err != nil {
		t.Errorf("Valid ConsoleCommand should not fail validation: %v", err)
	}
}

func TestWatchCommandBasics(t *testing.T) {
	cmd := &WatchCommand{
		SearchString: "login:",
		Timeout:      30 * time.Second,
		PollInterval: 1 * time.Second,
	}

	expected := "WATCH \"login:\" TIMEOUT 30s"
	if cmd.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, cmd.String())
	}

	if err := cmd.Validate(); err != nil {
		t.Errorf("Valid WatchCommand should not fail validation: %v", err)
	}
}

func TestParseSimpleCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Regular text becomes TypeCommand",
			input:    "echo hello",
			expected: "TYPE echo hello",
		},
		{
			name:     "Sleep command",
			input:    "<# Sleep 2s",
			expected: "SLEEP 2s",
		},
		{
			name:     "Console command",
			input:    "<# Console 2",
			expected: "CONSOLE 2",
		},
		{
			name:     "Key command",
			input:    "<# Key enter",
			expected: "KEY enter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseScriptCommand(tt.input, 1)
			if err != nil {
				t.Fatalf("Unexpected parse error: %v", err)
			}

			if cmd.String() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, cmd.String())
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		cmd  ScriptCommand
	}{
		{
			name: "Empty TypeCommand should fail",
			cmd:  &TypeCommand{Text: ""},
		},
		{
			name: "Negative SleepCommand should fail",
			cmd:  &SleepCommand{Duration: -1 * time.Second},
		},
		{
			name: "Invalid ConsoleCommand should fail",
			cmd:  &ConsoleCommand{ConsoleNumber: 0},
		},
		{
			name: "Empty WatchCommand should fail",
			cmd:  &WatchCommand{SearchString: "", Timeout: 30 * time.Second, PollInterval: 1 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if err == nil {
				t.Errorf("Expected validation error for invalid command, but got none")
			}
		})
	}
}
