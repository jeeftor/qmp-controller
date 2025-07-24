package script

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// ScriptCommand represents a single command that can be executed in a script
type ScriptCommand interface {
	Execute(client *qmp.Client, context *ScriptContext) error
	Validate() error
	String() string
}

// ScriptContext holds the execution context for script commands
type ScriptContext struct {
	VMId             string
	ScriptFile       string
	LineNumber       int
	TrainingDataPath string
	OCRColumns       int
	OCRRows          int
	DebugMode        bool
	Delay            time.Duration
}

// TypeCommand represents a text typing command
type TypeCommand struct {
	Text string
}

func (c *TypeCommand) Execute(client *qmp.Client, ctx *ScriptContext) error {
	logging.Info("Executing TYPE command", "vmid", ctx.VMId, "line", ctx.LineNumber, "text", c.Text)
	return logging.LogOperation("script_type", ctx.VMId, func() error {
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "executing", nil)
		err := client.SendString(c.Text, ctx.Delay)
		if err != nil {
			logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "failed", err)
			return err
		}
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "completed", nil)
		return nil
	})
}

func (c *TypeCommand) Validate() error {
	if c.Text == "" {
		return fmt.Errorf("TYPE command cannot have empty text")
	}
	return nil
}

func (c *TypeCommand) String() string {
	return fmt.Sprintf("TYPE %s", c.Text)
}

// KeyCommand represents a key press command
type KeyCommand struct {
	Key string
}

func (c *KeyCommand) Execute(client *qmp.Client, ctx *ScriptContext) error {
	logging.Info("Executing KEY command", "vmid", ctx.VMId, "line", ctx.LineNumber, "key", c.Key)
	return logging.LogOperation("script_key", ctx.VMId, func() error {
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "executing", nil)
		err := client.SendKey(c.Key)
		if err != nil {
			logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "failed", err)
			return err
		}
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "completed", nil)
		return nil
	})
}

func (c *KeyCommand) Validate() error {
	if c.Key == "" {
		return fmt.Errorf("KEY command cannot have empty key")
	}
	return nil
}

func (c *KeyCommand) String() string {
	return fmt.Sprintf("KEY %s", c.Key)
}

// SleepCommand represents a sleep/wait command
type SleepCommand struct {
	Duration time.Duration
}

func (c *SleepCommand) Execute(client *qmp.Client, ctx *ScriptContext) error {
	logging.Info("Executing SLEEP command", "vmid", ctx.VMId, "line", ctx.LineNumber, "duration", c.Duration)
	return logging.LogOperation("script_sleep", ctx.VMId, func() error {
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "executing", nil)
		time.Sleep(c.Duration)
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "completed", nil)
		return nil
	})
}

func (c *SleepCommand) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("SLEEP duration must be positive, got %v", c.Duration)
	}
	return nil
}

func (c *SleepCommand) String() string {
	return fmt.Sprintf("SLEEP %v", c.Duration)
}

// WatchCommand represents a WATCH command that waits for text to appear
type WatchCommand struct {
	SearchString string
	Timeout      time.Duration
	PollInterval time.Duration
}

func (c *WatchCommand) Execute(client *qmp.Client, ctx *ScriptContext) error {
	logging.Info("Executing WATCH command", "vmid", ctx.VMId, "line", ctx.LineNumber, "search", c.SearchString, "timeout", c.Timeout, "poll", c.PollInterval)
	return logging.LogOperation("script_watch", ctx.VMId, func() error {
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "executing", nil)
		start := time.Now()
		attempts := 0
		for time.Since(start) < c.Timeout {
			attempts++
			found, err := performWatchCheck(client, c.SearchString, ctx.TrainingDataPath, ctx.OCRColumns, ctx.OCRRows)
			if err != nil {
				logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "failed", err)
				return fmt.Errorf("WATCH check failed: %v", err)
			}
			if found {
				duration := time.Since(start)
				logging.LogWatch(ctx.VMId, c.SearchString, true, attempts, duration)
				logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "completed", nil)
				return nil
			}
			time.Sleep(c.PollInterval)
		}
		// Timeout reached
		duration := time.Since(start)
		logging.LogWatch(ctx.VMId, c.SearchString, false, attempts, duration)
		err := fmt.Errorf("WATCH timeout after %v waiting for '%s'", c.Timeout, c.SearchString)
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "timeout", err)
		return err
	})
}

func (c *WatchCommand) Validate() error {
	if c.SearchString == "" {
		return fmt.Errorf("WATCH command cannot have empty search string")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("WATCH timeout must be positive, got %v", c.Timeout)
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("WATCH poll interval must be positive, got %v", c.PollInterval)
	}
	return nil
}

func (c *WatchCommand) String() string {
	return fmt.Sprintf("WATCH \"%s\" TIMEOUT %v", c.SearchString, c.Timeout)
}

// ConsoleCommand represents a console switching command
type ConsoleCommand struct {
	ConsoleNumber int
}

func (c *ConsoleCommand) Execute(client *qmp.Client, ctx *ScriptContext) error {
	logging.Info("Executing CONSOLE command", "vmid", ctx.VMId, "line", ctx.LineNumber, "console", c.ConsoleNumber)
	return logging.LogOperation("script_console", ctx.VMId, func() error {
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "executing", nil)
		// Build the F-key name (f1, f2, etc.)
		fKey := fmt.Sprintf("f%d", c.ConsoleNumber)
		// Send Ctrl+Alt+F[1-6] combination
		err := client.SendKeyCombo([]string{"ctrl", "alt", fKey})
		if err != nil {
			logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "failed", err)
			return err
		}
		logging.LogScript(ctx.VMId, ctx.ScriptFile, ctx.LineNumber, c.String(), "completed", nil)
		return nil
	})
}

func (c *ConsoleCommand) Validate() error {
	if c.ConsoleNumber < 1 || c.ConsoleNumber > 6 {
		return fmt.Errorf("console number must be between 1 and 6, got %d", c.ConsoleNumber)
	}
	return nil
}

func (c *ConsoleCommand) String() string {
	return fmt.Sprintf("CONSOLE %d", c.ConsoleNumber)
}

// ParseScriptCommand parses a script line and returns the appropriate command
func ParseScriptCommand(line string, lineNumber int) (ScriptCommand, error) {
	line = strings.TrimSpace(line)

	// Handle control commands (starting with <# )
	if strings.HasPrefix(line, "<#") {
		// Remove optional trailing > and any trailing whitespace
		content := strings.TrimSpace(strings.TrimSuffix(line[2:], ">"))
		parts := strings.Fields(content)

		if len(parts) == 0 {
			return nil, fmt.Errorf("empty control command")
		}

		switch strings.ToLower(parts[0]) {
		case "sleep":
			if len(parts) != 2 {
				return nil, fmt.Errorf("SLEEP command requires duration (e.g., <# Sleep 1s >)")
			}
			dur, err := time.ParseDuration(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid duration for SLEEP: %v", err)
			}
			return &SleepCommand{Duration: dur}, nil
		case "console":
			if len(parts) != 2 {
				return nil, fmt.Errorf("CONSOLE command requires console number (e.g., <# Console 1 >)")
			}
			consoleNum, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid console number '%s': %v", parts[1], err)
			}
			return &ConsoleCommand{ConsoleNumber: consoleNum}, nil
		case "watch":
			return parseWatchCommand(parts)
		case "key":
			if len(parts) < 2 {
				return nil, fmt.Errorf("KEY command requires a key name (e.g., <# KEY ENTER >)")
			}
			keyCombo := strings.Join(parts[1:], " ")
			return &KeyCommand{Key: keyCombo}, nil
		default:
			return nil, fmt.Errorf("unknown control command: %s", parts[0])
		}
	}

	// Regular line - treat as TYPE command
	return &TypeCommand{Text: line}, nil
}

// parseWatchCommand parses WATCH command arguments
func parseWatchCommand(parts []string) (ScriptCommand, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("WATCH command requires search string")
	}

	// Find the search string (should be quoted)
	var searchString string
	var remainingParts []string

	if strings.HasPrefix(parts[1], "\"") {
		// Find the closing quote
		quotedParts := []string{parts[1]}
		i := 2
		for i < len(parts) && !strings.HasSuffix(parts[i-1], "\"") {
			quotedParts = append(quotedParts, parts[i])
			i++
		}
		searchString = strings.Join(quotedParts, " ")
		searchString = strings.Trim(searchString, "\"")
		remainingParts = parts[i:]
	} else {
		// Unquoted string - take just the next part
		searchString = parts[1]
		remainingParts = parts[2:]
	}

	// Default values
	timeout := 30 * time.Second
	pollInterval := 1 * time.Second

	// Parse remaining arguments
	for i := 0; i < len(remainingParts); i += 2 {
		if i+1 >= len(remainingParts) {
			return nil, fmt.Errorf("WATCH command argument '%s' missing value", remainingParts[i])
		}

		key := strings.ToUpper(remainingParts[i])
		value := remainingParts[i+1]

		switch key {
		case "TIMEOUT":
			var err error
			timeout, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("invalid timeout duration '%s': %v", value, err)
			}
		case "POLL":
			var err error
			pollInterval, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("invalid poll interval '%s': %v", value, err)
			}
		default:
			return nil, fmt.Errorf("unknown WATCH argument: %s", key)
		}
	}

	return &WatchCommand{
		SearchString: searchString,
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, nil
}

// performWatchCheck is implemented in cmd/script.go - we'll use it through a function pointer
var PerformWatchCheckFunc func(client *qmp.Client, searchString, trainingDataPath string, width, height int) (bool, error)

// performWatchCheck calls the actual implementation
func performWatchCheck(client *qmp.Client, searchString, trainingDataPath string, width, height int) (bool, error) {
	if PerformWatchCheckFunc == nil {
		return false, fmt.Errorf("WATCH check function not initialized")
	}
	return PerformWatchCheckFunc(client, searchString, trainingDataPath, width, height)
}
