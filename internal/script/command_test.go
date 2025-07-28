package script

import (
	"testing"
	"time"
)

func TestParseScriptCommand(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		lineNumber  int
		expectedCmd string
		expectError bool
	}{
		{
			name:        "Simple text line becomes TypeCommand",
			line:        "echo hello world",
			lineNumber:  1,
			expectedCmd: "TYPE echo hello world",
			expectError: false,
		},
		{
			name:        "Sleep command parsing",
			line:        "<# Sleep 5s",
			lineNumber:  2,
			expectedCmd: "SLEEP 5s",
			expectError: false,
		},
		{
			name:        "Console command parsing",
			line:        "<# Console 1",
			lineNumber:  3,
			expectedCmd: "CONSOLE 1",
			expectError: false,
		},
		{
			name:        "Key command parsing",
			line:        "<# Key enter",
			lineNumber:  4,
			expectedCmd: "KEY enter",
			expectError: false,
		},
		{
			name:        "Complex key command",
			line:        "<# Key ctrl-alt-f1",
			lineNumber:  5,
			expectedCmd: "KEY ctrl-alt-f1",
			expectError: false,
		},
		{
			name:        "WATCH command parsing",
			line:        "<# WATCH \"login:\" TIMEOUT 30s",
			lineNumber:  6,
			expectedCmd: "WATCH \"login:\" TIMEOUT 30s",
			expectError: false,
		},
		{
			name:        "Invalid sleep duration",
			line:        "<# Sleep invalid",
			lineNumber:  7,
			expectedCmd: "",
			expectError: true,
		},
		{
			name:        "Invalid console number",
			line:        "<# Console abc",
			lineNumber:  8,
			expectedCmd: "",
			expectError: true,
		},
		{
			name:        "Unknown control command",
			line:        "<# INVALID command",
			lineNumber:  9,
			expectedCmd: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseScriptCommand(tt.line, tt.lineNumber)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for line '%s', but got none", tt.line)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for line '%s': %v", tt.line, err)
				return
			}

			if cmd == nil {
				t.Errorf("Expected command for line '%s', but got nil", tt.line)
				return
			}

			if cmd.String() != tt.expectedCmd {
				t.Errorf("Expected command string '%s', got '%s'", tt.expectedCmd, cmd.String())
			}
		})
	}
}

func TestTypeCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		expectError bool
	}{
		{
			name:        "Valid text command",
			text:        "echo hello",
			expectError: false,
		},
		{
			name:        "Empty text should be valid (might be intentional)",
			text:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &TypeCommand{Text: tt.text}
			err := cmd.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for text '%s', but got none", tt.text)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for text '%s': %v", tt.text, err)
			}
		})
	}
}

func TestSleepCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		duration    time.Duration
		expectError bool
	}{
		{
			name:        "Valid duration",
			duration:    5 * time.Second,
			expectError: false,
		},
		{
			name:        "Negative duration should fail",
			duration:    -1 * time.Second,
			expectError: true,
		},
		{
			name:        "Zero duration should fail",
			duration:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &SleepCommand{Duration: tt.duration}
			err := cmd.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for duration %v, but got none", tt.duration)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for duration %v: %v", tt.duration, err)
			}
		})
	}
}

func TestConsoleCommandValidation(t *testing.T) {
	tests := []struct {
		name          string
		consoleNumber int
		expectError   bool
	}{
		{
			name:          "Valid console 1",
			consoleNumber: 1,
			expectError:   false,
		},
		{
			name:          "Valid console 6",
			consoleNumber: 6,
			expectError:   false,
		},
		{
			name:          "Invalid console 0",
			consoleNumber: 0,
			expectError:   true,
		},
		{
			name:          "Invalid console 7",
			consoleNumber: 7,
			expectError:   true,
		},
		{
			name:          "Invalid negative console",
			consoleNumber: -1,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &ConsoleCommand{ConsoleNumber: tt.consoleNumber}
			err := cmd.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for console %d, but got none", tt.consoleNumber)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for console %d: %v", tt.consoleNumber, err)
			}
		})
	}
}

func TestWatchCommandValidation(t *testing.T) {
	tests := []struct {
		name         string
		searchString string
		timeout      time.Duration
		pollInterval time.Duration
		expectError  bool
	}{
		{
			name:         "Valid WATCH command",
			searchString: "login:",
			timeout:      30 * time.Second,
			pollInterval: 1 * time.Second,
			expectError:  false,
		},
		{
			name:         "Empty search string should fail",
			searchString: "",
			timeout:      30 * time.Second,
			pollInterval: 1 * time.Second,
			expectError:  true,
		},
		{
			name:         "Zero timeout should fail",
			searchString: "login:",
			timeout:      0,
			pollInterval: 1 * time.Second,
			expectError:  true,
		},
		{
			name:         "Zero poll interval should fail",
			searchString: "login:",
			timeout:      30 * time.Second,
			pollInterval: 0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &WatchCommand{
				SearchString: tt.searchString,
				Timeout:      tt.timeout,
				PollInterval: tt.pollInterval,
			}
			err := cmd.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for WATCH command, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for WATCH command: %v", err)
			}
		})
	}
}

func TestParseWatchCommand(t *testing.T) {
	tests := []struct {
		name        string
		parts       []string
		expectError bool
		expectedCmd *WatchCommand
	}{
		{
			name:        "Simple quoted string",
			parts:       []string{"WATCH", "\"login:\"", "TIMEOUT", "30s"},
			expectError: false,
			expectedCmd: &WatchCommand{
				SearchString: "login:",
				Timeout:      30 * time.Second,
				PollInterval: 1 * time.Second, // default
			},
		},
		{
			name:        "With custom poll interval",
			parts:       []string{"WATCH", "\"ready\"", "TIMEOUT", "10s", "POLL", "500ms"},
			expectError: false,
			expectedCmd: &WatchCommand{
				SearchString: "ready",
				Timeout:      10 * time.Second,
				PollInterval: 500 * time.Millisecond,
			},
		},
		{
			name:        "Missing search string",
			parts:       []string{"WATCH"},
			expectError: true,
			expectedCmd: nil,
		},
		{
			name:        "Invalid timeout format",
			parts:       []string{"WATCH", "\"test\"", "TIMEOUT", "invalid"},
			expectError: true,
			expectedCmd: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := parseWatchCommand(tt.parts)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for parts %v, but got none", tt.parts)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for parts %v: %v", tt.parts, err)
				return
			}

			watchCmd, ok := cmd.(*WatchCommand)
			if !ok {
				t.Errorf("Expected WatchCommand, got %T", cmd)
				return
			}

			if watchCmd.SearchString != tt.expectedCmd.SearchString {
				t.Errorf("Expected search string '%s', got '%s'", tt.expectedCmd.SearchString, watchCmd.SearchString)
			}
			if watchCmd.Timeout != tt.expectedCmd.Timeout {
				t.Errorf("Expected timeout %v, got %v", tt.expectedCmd.Timeout, watchCmd.Timeout)
			}
			if watchCmd.PollInterval != tt.expectedCmd.PollInterval {
				t.Errorf("Expected poll interval %v, got %v", tt.expectedCmd.PollInterval, watchCmd.PollInterval)
			}
		})
	}
}
