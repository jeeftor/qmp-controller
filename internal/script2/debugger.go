package script2

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/logging"
)

// Simple debug styling
var (
	debugHeaderColor = lipgloss.Color("#7C3AED")
	debugSectionColor = lipgloss.Color("#3B82F6")
	debugVariableColor = lipgloss.Color("#F59E0B")
	debugValueColor = lipgloss.Color("#10B981")

	debugHeaderStyle = lipgloss.NewStyle().Foreground(debugHeaderColor).Bold(true)
	debugSectionStyle = lipgloss.NewStyle().Foreground(debugSectionColor).Bold(true)
	debugVariableStyle = lipgloss.NewStyle().Foreground(debugVariableColor)
	debugValueStyle = lipgloss.NewStyle().Foreground(debugValueColor)
)

// DebugMode represents the current debugging mode
type DebugMode int

const (
	DebugModeNone DebugMode = iota // No debugging (normal execution)
	DebugModeStep                  // Step through each line
	DebugModeBreakpoints          // Only stop at breakpoints
	DebugModeInteractive          // Interactive TUI debugging
)

// DebugAction represents actions the user can take during debugging
type DebugAction int

const (
	DebugActionContinue DebugAction = iota // Continue execution
	DebugActionStep                        // Execute next line
	DebugActionStepOver                    // Step over function calls
	DebugActionStop                        // Stop execution
	DebugActionInspect                     // Inspect variables/state
	DebugActionScreenshot                  // Take debug screenshot
	DebugActionToggleBreakpoint           // Toggle breakpoint on current line
	DebugActionListBreakpoints            // List all breakpoints
)

// DebugState holds the current debugging state
type DebugState struct {
	Mode           DebugMode
	Breakpoints    map[int]bool      // Line numbers with breakpoints
	CurrentLine    int               // Current execution line
	Variables      map[string]string // Current variable state
	CallStack      []string          // Function call stack
	StepMode       bool              // True if stepping through execution
	LastAction     DebugAction       // Last action taken
	DebugMessages  []string          // Debug messages/output
	ExecutionPaused bool             // True if execution is paused
}

// Debugger handles script debugging functionality
type Debugger struct {
	state     *DebugState
	script    *Script
	executor  *Executor
	logger    *logging.ContextualLogger
	tui       *DebugTUI
	enabled   bool
}

// NewDebugger creates a new debugger instance
func NewDebugger(script *Script, executor *Executor) *Debugger {
	return &Debugger{
		state: &DebugState{
			Mode:        DebugModeNone,
			Breakpoints: make(map[int]bool),
			Variables:   make(map[string]string),
			CallStack:   make([]string, 0),
		},
		script:   script,
		executor: executor,
		logger:   logging.NewContextualLogger(executor.context.VMID, "script2_debugger"),
		enabled:  false,
	}
}

// Enable enables debugging with the specified mode
func (d *Debugger) Enable(mode DebugMode) {
	d.enabled = true
	d.state.Mode = mode
	d.logger.Info("Debugger enabled", "mode", d.debugModeString(mode))

	if mode == DebugModeInteractive {
		d.tui = NewDebugTUI(d)
	}
}

// Disable disables debugging
func (d *Debugger) Disable() {
	d.enabled = false
	d.state.Mode = DebugModeNone
	d.logger.Info("Debugger disabled")
}

// IsEnabled returns true if debugging is enabled
func (d *Debugger) IsEnabled() bool {
	return d.enabled
}

// ShouldBreak returns true if execution should break at the current line
func (d *Debugger) ShouldBreak(lineNumber int, line ParsedLine) bool {
	if !d.enabled {
		return false
	}

	d.state.CurrentLine = lineNumber

	// Always break on explicit <BREAK> directives
	if line.Type == DirectiveLine && strings.Contains(strings.ToUpper(line.Content), "<BREAK>") {
		d.logger.Info("Break directive encountered", "line", lineNumber)
		return true
	}

	// Break on breakpoints
	if d.state.Breakpoints[lineNumber] {
		d.logger.Info("Breakpoint hit", "line", lineNumber)
		return true
	}

	// Break in step mode
	if d.state.StepMode {
		return true
	}

	return false
}

// HandleBreak handles execution break and user interaction
func (d *Debugger) HandleBreak(lineNumber int, line ParsedLine) (DebugAction, error) {
	d.state.ExecutionPaused = true
	d.state.CurrentLine = lineNumber

	// Update variable state
	if d.executor != nil {
		d.state.Variables = d.executor.context.Variables.GetAllVariables()
	}

	switch d.state.Mode {
	case DebugModeInteractive:
		return d.handleInteractiveBreak(line)
	case DebugModeStep, DebugModeBreakpoints:
		return d.handleConsoleBreak(line)
	default:
		return DebugActionContinue, nil
	}
}

// handleInteractiveBreak handles breaks in interactive TUI mode
func (d *Debugger) handleInteractiveBreak(line ParsedLine) (DebugAction, error) {
	if d.tui == nil {
		d.tui = NewDebugTUI(d)
	}

	// Run the TUI and wait for user action
	program := tea.NewProgram(d.tui.InitialModel(), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return DebugActionStop, fmt.Errorf("TUI error: %w", err)
	}

	// Extract the action from the final model
	if model, ok := finalModel.(*debugTUIModel); ok {
		return model.lastAction, nil
	}

	return DebugActionContinue, nil
}

// handleConsoleBreak handles breaks in console mode
func (d *Debugger) handleConsoleBreak(line ParsedLine) (DebugAction, error) {
	d.printDebugInfo(line)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n[Debug] Enter command (h for help): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return DebugActionStop, err
		}

		input = strings.TrimSpace(input)
		action, shouldReturn := d.processDebugCommand(input)

		if shouldReturn {
			return action, nil
		}
	}
}

// processDebugCommand processes a debug command and returns the action
func (d *Debugger) processDebugCommand(input string) (DebugAction, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return DebugActionContinue, false
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "c", "continue":
		d.state.StepMode = false
		return DebugActionContinue, true

	case "s", "step":
		d.state.StepMode = true
		return DebugActionStep, true

	case "n", "next", "stepover":
		d.state.StepMode = true
		return DebugActionStepOver, true

	case "q", "quit", "stop":
		return DebugActionStop, true

	case "v", "vars", "variables":
		d.printVariables()
		return DebugActionContinue, false

	case "b", "break", "breakpoint":
		if len(parts) > 1 {
			if lineNum, err := strconv.Atoi(parts[1]); err == nil {
				d.toggleBreakpoint(lineNum)
			} else {
				fmt.Printf("Invalid line number: %s\n", parts[1])
			}
		} else {
			fmt.Printf("Usage: break <line_number>\n")
		}
		return DebugActionContinue, false

	case "l", "list", "breakpoints":
		d.listBreakpoints()
		return DebugActionContinue, false

	case "screenshot", "ss":
		return DebugActionScreenshot, true

	case "i", "inspect":
		d.printInspectionInfo()
		return DebugActionContinue, false

	case "h", "help":
		d.printDebugHelp()
		return DebugActionContinue, false

	default:
		fmt.Printf("Unknown command: %s (type 'h' for help)\n", command)
		return DebugActionContinue, false
	}
}

// printDebugInfo prints current debugging information
func (d *Debugger) printDebugInfo(line ParsedLine) {
	fmt.Printf("\n" + debugHeaderStyle.Render("🐛 SCRIPT DEBUGGER") + "\n")
	fmt.Printf("Line %d: %s\n", d.state.CurrentLine, line.Content)
	fmt.Printf("Type: %s\n", line.Type.String())

	if len(d.state.CallStack) > 0 {
		fmt.Printf("Call Stack: %s\n", strings.Join(d.state.CallStack, " > "))
	}

	fmt.Printf("Variables: %d defined\n", len(d.state.Variables))
	fmt.Printf("Breakpoints: %d active\n", len(d.state.Breakpoints))
}

// printVariables prints all current variables
func (d *Debugger) printVariables() {
	fmt.Printf("\n" + debugSectionStyle.Render("📋 VARIABLES") + "\n")
	if len(d.state.Variables) == 0 {
		fmt.Printf("No variables defined\n")
		return
	}

	for name, value := range d.state.Variables {
		fmt.Printf("  %s = %s\n",
			debugVariableStyle.Render(name),
			debugValueStyle.Render(value))
	}
}

// toggleBreakpoint toggles a breakpoint on the specified line
func (d *Debugger) toggleBreakpoint(lineNumber int) {
	if d.state.Breakpoints[lineNumber] {
		delete(d.state.Breakpoints, lineNumber)
		fmt.Printf("Breakpoint removed from line %d\n", lineNumber)
	} else {
		d.state.Breakpoints[lineNumber] = true
		fmt.Printf("Breakpoint set on line %d\n", lineNumber)
	}
}

// listBreakpoints lists all active breakpoints
func (d *Debugger) listBreakpoints() {
	fmt.Printf("\n" + debugSectionStyle.Render("🔴 BREAKPOINTS") + "\n")
	if len(d.state.Breakpoints) == 0 {
		fmt.Printf("No breakpoints set\n")
		return
	}

	for lineNumber := range d.state.Breakpoints {
		if lineNumber <= len(d.script.Lines) {
			line := d.script.Lines[lineNumber-1]
			fmt.Printf("  Line %d: %s\n", lineNumber, line.Content)
		} else {
			fmt.Printf("  Line %d: (invalid line number)\n", lineNumber)
		}
	}
}

// printInspectionInfo prints detailed inspection information
func (d *Debugger) printInspectionInfo() {
	fmt.Printf("\n" + debugSectionStyle.Render("🔍 INSPECTION") + "\n")
	fmt.Printf("Current Line: %d / %d\n", d.state.CurrentLine, len(d.script.Lines))
	fmt.Printf("Execution Mode: %s\n", d.debugModeString(d.state.Mode))
	fmt.Printf("Step Mode: %t\n", d.state.StepMode)
	fmt.Printf("Execution Time: %s\n", time.Now().Format("15:04:05"))

	if d.executor != nil {
		fmt.Printf("VM ID: %s\n", d.executor.context.VMID)
		fmt.Printf("Dry Run: %t\n", d.executor.context.DryRun)
	}
}

// printDebugHelp prints debugging help
func (d *Debugger) printDebugHelp() {
	fmt.Printf("\n" + debugSectionStyle.Render("❓ DEBUG COMMANDS") + "\n")
	fmt.Printf("  c, continue    - Continue execution\n")
	fmt.Printf("  s, step        - Execute next line (step into)\n")
	fmt.Printf("  n, next        - Execute next line (step over)\n")
	fmt.Printf("  q, quit        - Stop script execution\n")
	fmt.Printf("  v, vars        - Show all variables\n")
	fmt.Printf("  b <line>       - Toggle breakpoint on line\n")
	fmt.Printf("  l, list        - List all breakpoints\n")
	fmt.Printf("  screenshot     - Take debug screenshot\n")
	fmt.Printf("  i, inspect     - Show detailed information\n")
	fmt.Printf("  h, help        - Show this help\n")
}

// debugModeString returns a string representation of the debug mode
func (d *Debugger) debugModeString(mode DebugMode) string {
	switch mode {
	case DebugModeNone:
		return "none"
	case DebugModeStep:
		return "step"
	case DebugModeBreakpoints:
		return "breakpoints"
	case DebugModeInteractive:
		return "interactive"
	default:
		return "unknown"
	}
}

// AddBreakpoint adds a breakpoint at the specified line
func (d *Debugger) AddBreakpoint(lineNumber int) {
	d.state.Breakpoints[lineNumber] = true
	d.logger.Info("Breakpoint added", "line", lineNumber)
}

// RemoveBreakpoint removes a breakpoint from the specified line
func (d *Debugger) RemoveBreakpoint(lineNumber int) {
	delete(d.state.Breakpoints, lineNumber)
	d.logger.Info("Breakpoint removed", "line", lineNumber)
}

// GetState returns the current debug state (for TUI)
func (d *Debugger) GetState() *DebugState {
	return d.state
}

// GetScript returns the script being debugged (for TUI)
func (d *Debugger) GetScript() *Script {
	return d.script
}
