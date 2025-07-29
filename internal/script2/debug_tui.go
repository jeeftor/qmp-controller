package script2

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Simple color schemes for the debugger TUI
var (
	primaryColor    = lipgloss.Color("#7C3AED")
	successColor    = lipgloss.Color("#10B981")
	errorColor      = lipgloss.Color("#EF4444")
	warningColor    = lipgloss.Color("#F59E0B")
	infoColor       = lipgloss.Color("#3B82F6")
	mutedColor      = lipgloss.Color("#6B7280")
	textColor       = lipgloss.Color("#F9FAFB")
	backgroundColor = lipgloss.Color("#1F2937")

	headerStyle    = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	sectionStyle   = lipgloss.NewStyle().Foreground(infoColor).Bold(true)
	successStyle   = lipgloss.NewStyle().Foreground(successColor)
	errorStyle     = lipgloss.NewStyle().Foreground(errorColor)
	mutedStyle     = lipgloss.NewStyle().Foreground(mutedColor)
	textStyle      = lipgloss.NewStyle().Foreground(textColor)
	variableStyle  = lipgloss.NewStyle().Foreground(warningColor)
	valueStyle     = lipgloss.NewStyle().Foreground(successColor)
	functionStyle  = lipgloss.NewStyle().Foreground(primaryColor)
	lineNumStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	inputStyle     = lipgloss.NewStyle().Foreground(infoColor)
	footerStyle    = lipgloss.NewStyle().Foreground(mutedColor).Background(backgroundColor)
)

// DebugTUI handles the terminal user interface for debugging
type DebugTUI struct {
	debugger *Debugger
}

// NewDebugTUI creates a new debug TUI
func NewDebugTUI(debugger *Debugger) *DebugTUI {
	return &DebugTUI{
		debugger: debugger,
	}
}

// debugTUIModel represents the TUI model state
type debugTUIModel struct {
	debugger     *Debugger
	viewport     viewport.Model
	textInput    textinput.Model
	keys         debugKeyMap
	lastAction   DebugAction
	inputMode    bool
	currentView  debugView
	width        int
	height       int
	helpVisible  bool
}

// debugView represents different views in the debug TUI
type debugView int

const (
	viewScript debugView = iota
	viewVariables
	viewBreakpoints
	viewHelp
	viewOCR
	viewWatchProgress
)

// debugKeyMap defines keyboard shortcuts for debugging
type debugKeyMap struct {
	Continue    key.Binding
	Step        key.Binding
	StepOver    key.Binding
	Stop        key.Binding
	Screenshot  key.Binding
	Variables   key.Binding
	Breakpoints key.Binding
	OCRView     key.Binding
	WatchView   key.Binding
	Refresh     key.Binding
	Help        key.Binding
	Quit        key.Binding
	Enter       key.Binding
	Escape      key.Binding
	Up          key.Binding
	Down        key.Binding
}

// DefaultDebugKeyMap returns the default key bindings for debugging
func DefaultDebugKeyMap() debugKeyMap {
	return debugKeyMap{
		Continue: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "continue"),
		),
		Step: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "step"),
		),
		StepOver: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "step over"),
		),
		Stop: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "stop"),
		),
		Screenshot: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "screenshot"),
		),
		Variables: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "variables"),
		),
		Breakpoints: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "breakpoints"),
		),
		OCRView: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "OCR view"),
		),
		WatchView: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "watch progress"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh OCR"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "h"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("esc", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
		),
	}
}

// InitialModel returns the initial TUI model
func (t *DebugTUI) InitialModel() tea.Model {
	// Create viewport for script display
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2)

	// Create text input for commands
	ti := textinput.New()
	ti.Placeholder = "Enter debug command..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return &debugTUIModel{
		debugger:    t.debugger,
		viewport:    vp,
		textInput:   ti,
		keys:        DefaultDebugKeyMap(),
		lastAction:  DebugActionContinue,
		inputMode:   false,
		currentView: viewScript,
		helpVisible: false,
	}
}

// Init initializes the TUI model
func (m *debugTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles TUI updates
func (m *debugTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		m.updateViewportContent()

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInputMode(msg)
		}
		return m.handleNormalMode(msg)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Update text input
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleNormalMode handles key presses in normal mode
func (m *debugTUIModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Continue):
		m.lastAction = DebugActionContinue
		return m, tea.Quit

	case key.Matches(msg, m.keys.Step):
		m.lastAction = DebugActionStep
		return m, tea.Quit

	case key.Matches(msg, m.keys.StepOver):
		m.lastAction = DebugActionStepOver
		return m, tea.Quit

	case key.Matches(msg, m.keys.Stop):
		m.lastAction = DebugActionStop
		return m, tea.Quit

	case key.Matches(msg, m.keys.Screenshot):
		m.lastAction = DebugActionScreenshot
		return m, tea.Quit

	case key.Matches(msg, m.keys.Variables):
		m.currentView = viewVariables
		m.updateViewportContent()

	case key.Matches(msg, m.keys.Breakpoints):
		m.currentView = viewBreakpoints
		m.updateViewportContent()

	case key.Matches(msg, m.keys.OCRView):
		m.currentView = viewOCR
		m.updateViewportContent()

	case key.Matches(msg, m.keys.WatchView):
		m.currentView = viewWatchProgress
		m.updateViewportContent()

	case key.Matches(msg, m.keys.Refresh):
		// Refresh OCR view with new screenshot
		if m.currentView == viewOCR {
			err := m.debugger.RefreshOCR()
			if err != nil {
				// Could show error message in the view
			}
			m.updateViewportContent()
		}

	case key.Matches(msg, m.keys.Help):
		m.helpVisible = !m.helpVisible
		m.updateViewportContent()

	case key.Matches(msg, m.keys.Quit):
		m.lastAction = DebugActionStop
		return m, tea.Quit

	case msg.String() == "1":
		m.currentView = viewScript
		m.updateViewportContent()

	case msg.String() == "2":
		m.currentView = viewVariables
		m.updateViewportContent()

	case msg.String() == "3":
		m.currentView = viewBreakpoints
		m.updateViewportContent()

	case msg.String() == "4":
		m.currentView = viewOCR
		m.updateViewportContent()

	case msg.String() == "5":
		m.currentView = viewWatchProgress
		m.updateViewportContent()

	case msg.String() == ":":
		m.inputMode = true
		m.textInput.Focus()
		m.textInput.SetValue("")
	}

	return m, nil
}

// handleInputMode handles key presses in input mode
func (m *debugTUIModel) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		command := m.textInput.Value()
		m.processCommand(command)
		m.inputMode = false
		m.textInput.Blur()
		return m, nil

	case tea.KeyEsc:
		m.inputMode = false
		m.textInput.Blur()
		m.textInput.SetValue("")
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// processCommand processes a debug command entered via text input
func (m *debugTUIModel) processCommand(command string) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "break", "b":
		if len(parts) > 1 {
			if lineNum, err := strconv.Atoi(parts[1]); err == nil {
				m.debugger.state.Breakpoints[lineNum] = !m.debugger.state.Breakpoints[lineNum]
				m.updateViewportContent()
			}
		}
	case "goto", "g":
		if len(parts) > 1 {
			if lineNum, err := strconv.Atoi(parts[1]); err == nil {
				m.highlightLine(lineNum)
			}
		}
	}
}

// highlightLine highlights a specific line in the script view
func (m *debugTUIModel) highlightLine(lineNum int) {
	if m.currentView == viewScript {
		// Calculate viewport position to show the line
		linesPerPage := m.viewport.Height - 2
		targetOffset := lineNum - linesPerPage/2
		if targetOffset < 0 {
			targetOffset = 0
		}
		m.viewport.SetYOffset(targetOffset)
	}
}

// updateViewportContent updates the viewport content based on current view
func (m *debugTUIModel) updateViewportContent() {
	switch m.currentView {
	case viewScript:
		m.viewport.SetContent(m.renderScriptView())
	case viewVariables:
		m.viewport.SetContent(m.renderVariablesView())
	case viewBreakpoints:
		m.viewport.SetContent(m.renderBreakpointsView())
	case viewOCR:
		m.viewport.SetContent(m.renderOCRView())
	case viewWatchProgress:
		m.viewport.SetContent(m.renderWatchProgressView())
	case viewHelp:
		m.viewport.SetContent(m.renderHelpView())
	}
}

// renderScriptView renders the script view with current line highlighted
func (m *debugTUIModel) renderScriptView() string {
	var content strings.Builder
	state := m.debugger.GetState()
	script := m.debugger.GetScript()

	content.WriteString(headerStyle.Render("📜 SCRIPT VIEW") + "\n\n")

	for i, line := range script.Lines {
		lineNumber := i + 1
		prefix := fmt.Sprintf("%3d", lineNumber)

		// Style the line based on state
		var lineStyle lipgloss.Style
		var marker string

		if lineNumber == state.CurrentLine {
			// Current execution line
			lineStyle = successStyle.Copy().Background(lipgloss.Color("#2d3748"))
			marker = "►"
		} else if state.Breakpoints[lineNumber] {
			// Breakpoint line
			lineStyle = errorStyle.Copy()
			marker = "●"
		} else {
			// Normal line
			lineStyle = textStyle.Copy()
			marker = " "
		}

		// Format the line
		formattedLine := fmt.Sprintf("%s %s │ %s", marker, prefix, line.Content)
		content.WriteString(lineStyle.Render(formattedLine) + "\n")
	}

	return content.String()
}

// renderVariablesView renders the variables view
func (m *debugTUIModel) renderVariablesView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("📋 VARIABLES") + "\n\n")

	if len(state.Variables) == 0 {
		content.WriteString(mutedStyle.Render("No variables defined") + "\n")
	} else {
		for name, value := range state.Variables {
			content.WriteString(fmt.Sprintf("%s = %s\n",
				variableStyle.Render(name),
				valueStyle.Render(value)))
		}
	}

	// Show call stack if available
	if len(state.CallStack) > 0 {
		content.WriteString("\n" + sectionStyle.Render("📞 CALL STACK") + "\n")
		for i, frame := range state.CallStack {
			content.WriteString(fmt.Sprintf("%s%s\n",
				strings.Repeat("  ", i),
				functionStyle.Render(frame)))
		}
	}

	return content.String()
}

// renderBreakpointsView renders the breakpoints view
func (m *debugTUIModel) renderBreakpointsView() string {
	var content strings.Builder
	state := m.debugger.GetState()
	script := m.debugger.GetScript()

	content.WriteString(headerStyle.Render("🔴 BREAKPOINTS") + "\n\n")

	if len(state.Breakpoints) == 0 {
		content.WriteString(mutedStyle.Render("No breakpoints set") + "\n")
	} else {
		for lineNumber := range state.Breakpoints {
			if lineNumber <= len(script.Lines) {
				line := script.Lines[lineNumber-1]
				content.WriteString(fmt.Sprintf("Line %s: %s\n",
					lineNumStyle.Render(fmt.Sprintf("%d", lineNumber)),
					textStyle.Render(line.Content)))
			}
		}
	}

	content.WriteString("\n" + mutedStyle.Render("Tip: Use ':break <line>' to toggle breakpoints") + "\n")

	return content.String()
}

// renderHelpView renders the help view
func (m *debugTUIModel) renderHelpView() string {
	var content strings.Builder

	content.WriteString(headerStyle.Render("❓ DEBUG HELP") + "\n\n")

	content.WriteString(sectionStyle.Render("Execution Controls:") + "\n")
	content.WriteString("  c       - Continue execution\n")
	content.WriteString("  s       - Step to next line\n")
	content.WriteString("  n       - Step over function calls\n")
	content.WriteString("  q       - Stop script execution\n")
	content.WriteString("  p       - Take screenshot\n\n")

	content.WriteString(sectionStyle.Render("Views:") + "\n")
	content.WriteString("  1       - Script view\n")
	content.WriteString("  2       - Variables view\n")
	content.WriteString("  3       - Breakpoints view\n")
	content.WriteString("  4       - OCR preview view\n")
	content.WriteString("  5       - Watch progress view\n")
	content.WriteString("  v       - Toggle variables view\n")
	content.WriteString("  b       - Toggle breakpoints view\n")
	content.WriteString("  o       - Toggle OCR view\n")
	content.WriteString("  w       - Toggle watch progress view\n")
	content.WriteString("  r       - Refresh OCR (when in OCR view)\n\n")

	content.WriteString(sectionStyle.Render("Commands:") + "\n")
	content.WriteString("  :       - Enter command mode\n")
	content.WriteString("  :break <line> - Toggle breakpoint\n")
	content.WriteString("  :goto <line>  - Go to line\n\n")

	content.WriteString(sectionStyle.Render("Navigation:") + "\n")
	content.WriteString("  ↑/↓ or k/j - Scroll up/down\n")
	content.WriteString("  Esc     - Exit current mode\n")

	return content.String()
}

// View renders the TUI
func (m *debugTUIModel) View() string {
	var content strings.Builder
	state := m.debugger.GetState()

	// Header
	title := fmt.Sprintf("🐛 Script2 Debugger - Line %d/%d",
		state.CurrentLine, len(m.debugger.GetScript().Lines))
	header := headerStyle.Copy().
		Width(m.width).
		Align(lipgloss.Center).
		Render(title)
	content.WriteString(header + "\n")

	// Status bar
	status := fmt.Sprintf("Mode: %s | View: %s",
		m.debugger.debugModeString(state.Mode),
		m.currentViewString())
	statusBar := lipgloss.NewStyle().
		Foreground(infoColor).
		Width(m.width).
		Align(lipgloss.Left).
		Render(status)
	content.WriteString(statusBar + "\n")

	// Main viewport
	content.WriteString(m.viewport.View() + "\n")

	// Command input (if in input mode)
	if m.inputMode {
		inputView := "Command: " + m.textInput.View()
		content.WriteString(inputStyle.Render(inputView) + "\n")
	}

	// Footer with key hints
	footer := "c:continue s:step n:next q:quit v:vars b:breaks o:ocr w:watch r:refresh ?:help :cmd"
	footerBar := footerStyle.Copy().
		Width(m.width).
		Align(lipgloss.Center).
		Render(footer)
	content.WriteString(footerBar)

	return content.String()
}

// renderOCRView renders the OCR preview view
func (m *debugTUIModel) renderOCRView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("👁️  OCR PREVIEW") + "\n\n")

	// Check if we have current OCR data
	if len(state.LastOCRResult) == 0 {
		content.WriteString(mutedStyle.Render("No OCR data available") + "\n")
		content.WriteString(mutedStyle.Render("Press 'r' to refresh with current screen") + "\n")
		return content.String()
	}

	// Display OCR text with line numbers
	for i, line := range state.LastOCRResult {
		lineNum := fmt.Sprintf("%2d", i+1)
		content.WriteString(fmt.Sprintf("%s │ %s\n",
			lineNumStyle.Render(lineNum),
			textStyle.Render(line)))
	}

	// Show search highlights if there's an active search
	if state.LastSearchTerm != "" {
		content.WriteString("\n" + sectionStyle.Render("🔍 LAST SEARCH") + "\n")
		content.WriteString(fmt.Sprintf("Term: %s\n", variableStyle.Render(state.LastSearchTerm)))
		content.WriteString(fmt.Sprintf("Found: %s\n",
			func() string {
				if state.LastSearchFound {
					return successStyle.Render("YES")
				}
				return errorStyle.Render("NO")
			}()))
	}

	content.WriteString("\n" + mutedStyle.Render("Press 'r' to refresh OCR") + "\n")

	return content.String()
}

// renderWatchProgressView renders the watch command progress view
func (m *debugTUIModel) renderWatchProgressView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("⏱️  WATCH PROGRESS") + "\n\n")

	// Show current watch operation if active
	if state.CurrentWatchOperation != nil {
		op := state.CurrentWatchOperation
		content.WriteString(sectionStyle.Render("🔄 ACTIVE WATCH") + "\n")
		content.WriteString(fmt.Sprintf("Searching for: %s\n", variableStyle.Render(op.SearchTerm)))
		content.WriteString(fmt.Sprintf("Timeout: %s\n", valueStyle.Render(op.Timeout.String())))
		content.WriteString(fmt.Sprintf("Poll interval: %s\n", valueStyle.Render(op.PollInterval.String())))
		content.WriteString(fmt.Sprintf("Attempts: %s\n", valueStyle.Render(fmt.Sprintf("%d", op.Attempts))))

		elapsed := op.ElapsedTime()
		remaining := op.Timeout - elapsed
		if remaining < 0 {
			remaining = 0
		}

		content.WriteString(fmt.Sprintf("Elapsed: %s\n", valueStyle.Render(elapsed.Truncate(time.Millisecond).String())))
		content.WriteString(fmt.Sprintf("Remaining: %s\n", valueStyle.Render(remaining.Truncate(time.Millisecond).String())))

		// Progress bar
		progress := float64(elapsed) / float64(op.Timeout)
		if progress > 1.0 {
			progress = 1.0
		}
		progressBar := m.renderProgressBar(progress, 40)
		content.WriteString(fmt.Sprintf("Progress: %s\n", progressBar))

		// Show incremental text updates if available
		if len(op.IncrementalText) > 0 {
			content.WriteString("\n" + sectionStyle.Render("📝 NEW CONSOLE TEXT") + "\n")
			for _, line := range op.IncrementalText {
				content.WriteString(textStyle.Render(line) + "\n")
			}
		}
	} else {
		content.WriteString(mutedStyle.Render("No active watch operation") + "\n")
	}

	// Show watch history
	if len(state.WatchHistory) > 0 {
		content.WriteString("\n" + sectionStyle.Render("📜 WATCH HISTORY") + "\n")
		for i, entry := range state.WatchHistory {
			if i >= 10 { // Limit to last 10 entries
				break
			}
			status := errorStyle.Render("TIMEOUT")
			if entry.Found {
				status = successStyle.Render("FOUND")
			}
			content.WriteString(fmt.Sprintf("%s: %s (%s, %d attempts)\n",
				status,
				variableStyle.Render(entry.SearchTerm),
				valueStyle.Render(entry.Duration.Truncate(time.Millisecond).String()),
				entry.Attempts))
		}
	}

	return content.String()
}

// renderProgressBar renders a simple progress bar
func (m *debugTUIModel) renderProgressBar(progress float64, width int) string {
	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := fmt.Sprintf("%.1f%%", progress*100)

	return fmt.Sprintf("[%s] %s",
		successStyle.Render(bar),
		mutedStyle.Render(percentage))
}

// currentViewString returns the current view as a string
func (m *debugTUIModel) currentViewString() string {
	switch m.currentView {
	case viewScript:
		return "Script"
	case viewVariables:
		return "Variables"
	case viewBreakpoints:
		return "Breakpoints"
	case viewOCR:
		return "OCR Preview"
	case viewWatchProgress:
		return "Watch Progress"
	case viewHelp:
		return "Help"
	default:
		return "Unknown"
	}
}
