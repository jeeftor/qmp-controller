package script2

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/stretchr/testify/assert"
)

// MockQMPClient is a mock implementation of the QMP client for testing
type MockQMPClient struct {
	SentStrings []string
	SentKeys    []string
}

// SendString records sent strings for testing
func (m *MockQMPClient) SendString(text string, delay time.Duration) error {
	m.SentStrings = append(m.SentStrings, text)
	return nil
}

// SendKey records sent keys for testing
func (m *MockQMPClient) SendKey(key string) error {
	m.SentKeys = append(m.SentKeys, key)
	return nil
}

// ScreenDump is a mock implementation
func (m *MockQMPClient) ScreenDump(filename, format string) error {
	// Create an empty file to simulate screenshot
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

// TestFunctionParameterExpansion tests that function parameters are properly expanded
func TestFunctionParameterExpansion(t *testing.T) {
	// Initialize mock client
	mockClient := &MockQMPClient{
		SentStrings: make([]string, 0),
		SentKeys:    make([]string, 0),
	}

	// Create execution context
	context := &ExecutionContext{
		Client:        mockClient,
		VMID:          "test-vm",
		Variables:     NewVariableExpander(nil, nil, nil),
		CurrentLine:   1,
		StartTime:     time.Now(),
		Timeout:       60 * time.Second,
		DryRun:        false,
		Debug:         true,
		TrainingData:  "",
		FunctionStack: make([]*FunctionCallContext, 0),
	}

	// Create executor
	executor := NewExecutor(context, true)

	// Create a simple script with a function that uses parameters
	script := &Script{
		Lines:     make([]ParsedLine, 0),
		Variables: make(map[string]string),
		Functions: make(map[string]*Function),
	}

	// Create a function with parameters
	echoFunction := &Function{
		Name:      "echo",
		Lines:     make([]ParsedLine, 0),
		Defined:   true,
		LineStart: 1,
		LineEnd:   3,
	}

	// Add a line to the function that uses parameters
	echoLine := ParsedLine{
		Type:         TextLine,
		LineNumber:   2,
		OriginalText: "echo \"[$1] [$2] [$3]\"",
		Content:      "echo \"[$1] [$2] [$3]\"",
		ExpandedText: "echo \"[$1] [$2] [$3]\"", // This would be unchanged at parse time
	}
	echoFunction.Lines = append(echoFunction.Lines, echoLine)

	// Add the function to the script
	script.Functions["echo"] = echoFunction

	// Set the script in the executor
	executor.SetScript(script)

	// Create a function call directive
	directive := &Directive{
		Type:         FunctionCall,
		FunctionName: "echo",
		FunctionArgs: []string{"a", "b", "c"},
	}

	// Create a logger for testing
	logger := logging.NewContextualLogger("test", "test")

	// Execute the function call
	err := executor.executeFunctionCall(directive, logger)

	// Verify no errors
	assert.NoError(t, err, "Function call should not produce errors")

	// Verify the string was sent with parameters expanded
	assert.Len(t, mockClient.SentStrings, 1, "Should have sent one string")
	assert.Equal(t, "echo \"[a] [b] [c]\"", mockClient.SentStrings[0], "Parameters should be expanded")
}

// TestFunctionReturnDirective tests that the return directive exits functions early
func TestFunctionReturnDirective(t *testing.T) {
	// Initialize mock client
	mockClient := &MockQMPClient{
		SentStrings: make([]string, 0),
		SentKeys:    make([]string, 0),
	}

	// Create execution context
	context := &ExecutionContext{
		Client:        mockClient,
		VMID:          "test-vm",
		Variables:     NewVariableExpander(nil, nil, nil),
		CurrentLine:   1,
		StartTime:     time.Now(),
		Timeout:       60 * time.Second,
		DryRun:        false,
		Debug:         true,
		TrainingData:  "",
		FunctionStack: make([]*FunctionCallContext, 0),
	}

	// Create executor
	executor := NewExecutor(context, true)

	// Create a simple script with a function that uses return
	script := &Script{
		Lines:     make([]ParsedLine, 0),
		Variables: make(map[string]string),
		Functions: make(map[string]*Function),
	}

	// Create a function with early return
	testFunction := &Function{
		Name:      "test_return",
		Lines:     make([]ParsedLine, 0),
		Defined:   true,
		LineStart: 1,
		LineEnd:   5,
	}

	// Add lines to the function
	// Line 1: First message
	testFunction.Lines = append(testFunction.Lines, ParsedLine{
		Type:         TextLine,
		LineNumber:   2,
		OriginalText: "first message",
		Content:      "first message",
		ExpandedText: "first message",
	})

	// Line 2: Return directive
	returnDirective := &Directive{
		Type: Return,
	}
	testFunction.Lines = append(testFunction.Lines, ParsedLine{
		Type:         DirectiveLine,
		LineNumber:   3,
		OriginalText: "<return>",
		Content:      "return",
		Directive:    returnDirective,
	})

	// Line 3: Second message (should not be executed)
	testFunction.Lines = append(testFunction.Lines, ParsedLine{
		Type:         TextLine,
		LineNumber:   4,
		OriginalText: "second message",
		Content:      "second message",
		ExpandedText: "second message",
	})

	// Add the function to the script
	script.Functions["test_return"] = testFunction

	// Set the script in the executor
	executor.SetScript(script)

	// Create a function call directive
	directive := &Directive{
		Type:         FunctionCall,
		FunctionName: "test_return",
		FunctionArgs: []string{},
	}

	// Create a logger for testing
	logger := logging.NewContextualLogger("test", "test")

	// Execute the function call
	err := executor.executeFunctionCall(directive, logger)

	// Verify no errors
	assert.NoError(t, err, "Function call should not produce errors")

	// Verify only the first message was sent (before the return)
	assert.Len(t, mockClient.SentStrings, 1, "Should have sent only one string")
	assert.Equal(t, "first message", mockClient.SentStrings[0], "Only the first message should be sent")
	assert.NotContains(t, mockClient.SentStrings, "second message", "Second message should not be sent")
}
