package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"github.com/spf13/viper"
)

// ScriptState represents the execution state of a script line
type ScriptState int

const (
	StatePending ScriptState = iota
	StateExecuting
	StateCompleted
	StateFailed
	StateWatching
)

// ScriptLine represents a single line in the script
type ScriptLine struct {
	Number    int
	Content   string
	State     ScriptState
	Error     string
	StartTime time.Time
	Duration  time.Duration
}

// WatchState represents the state of a WATCH command
type WatchState struct {
	Active       bool
	SearchString string
	Timeout      time.Duration
	StartTime    time.Time
	PollCount    int
	LastOCR      string
	Found        bool
}

// ScriptTUIModel represents the Bubble Tea model for script execution
type ScriptTUIModel struct {
	vmid        string
	client      *qmp.Client
	scriptFile  string
	lines       []ScriptLine
	currentLine int
	executing   bool
	startTime   time.Time
	watchState  WatchState
	width       int
	height      int
	logs        []string
	maxLogs     int
	quitting    bool
	paused      bool

	// Script execution configuration
	commentPrefix string
	controlPrefix string
	scriptDelay   time.Duration
	trainingData  string
	columns       int
	rows          int
	pollInterval  time.Duration
}

// Message types
type scriptStartMsg struct{}
type scriptLineCompleteMsg struct {
	lineNum  int
	success  bool
	error    string
	duration time.Duration
}
type watchUpdateMsg struct {
	pollCount int
	ocrText   string
	found     bool
}
type watchCompleteMsg struct {
	found bool
	error string
}
type logMsg string

// NewScriptTUIModel creates a new script execution TUI model
func NewScriptTUIModel(vmid string, client *qmp.Client, scriptFile string, lines []ScriptLine) ScriptTUIModel {
	return ScriptTUIModel{
		vmid:          vmid,
		client:        client,
		scriptFile:    scriptFile,
		lines:         lines,
		currentLine:   0,
		executing:     false,
		startTime:     time.Now(),
		logs:          make([]string, 0),
		maxLogs:       20,
		quitting:      false,
		paused:        false,
		commentPrefix: getCommentChar(),
		controlPrefix: getControlChar(),
		scriptDelay:   getScriptDelay(),
		trainingData:  viper.GetString("watch-training-data"),
		columns:       viper.GetInt("columns"),
		rows:          viper.GetInt("rows"),
		pollInterval:  viper.GetDuration("watch-poll-interval"),
	}
}

// Use centralized styles from styles package
var (
	scriptTitleStyle   = styles.TitleStyle
	scriptBoxStyle     = styles.BoxStyle.BorderForeground(lipgloss.Color(styles.TextMuted)).Padding(1, 2)
	watchBoxStyle      = styles.BoxStyle.BorderForeground(lipgloss.Color(styles.Warning)).Padding(1, 2)
	logsBoxStyle       = styles.BoxStyle.BorderForeground(lipgloss.Color("#555555")).Padding(0, 1)
	pendingStyle       = styles.MutedStyle
	executingStyle     = styles.WarningStyle.Bold(true)
	completedStyle     = styles.SuccessStyle
	failedStyle        = styles.ErrorStyle
	watchingStyle      = styles.WarningStyle.Bold(true)
	currentLineStyle   = styles.BoldStyle.Background(lipgloss.Color(styles.BackgroundCard))
	scriptStatusStyle  = styles.InfoStyle
	scriptErrorStyle   = styles.ErrorStyle
	scriptSuccessStyle = styles.SuccessStyle
)

// Init initializes the model
func (m ScriptTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.addLog("Script execution initialized"),
		func() tea.Msg { return scriptStartMsg{} },
	)
}

// Update handles messages and updates the model
func (m ScriptTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case " ":
			// Toggle pause/resume
			m.paused = !m.paused
			if m.paused {
				return m, m.addLog("⏸ Script paused")
			} else {
				// Resume script execution if we're in the middle of executing
				if m.executing && m.currentLine < len(m.lines) {
					return m, tea.Batch(
						m.addLog("▶ Script resumed"),
						m.executeNextLine(),
					)
				} else {
					return m, m.addLog("▶ Script resumed")
				}
			}
		case "s":
			// Skip current line
			if m.executing && m.currentLine < len(m.lines) {
				return m, m.addLog(fmt.Sprintf("⏭ Skipped line %d", m.currentLine+1))
			}
		}

	case scriptStartMsg:
		if !m.paused {
			m.executing = true
			return m, m.executeNextLine()
		}

	case scriptLineCompleteMsg:
		// Update the completed line
		if msg.lineNum < len(m.lines) {
			m.lines[msg.lineNum].Duration = msg.duration
			if msg.success {
				m.lines[msg.lineNum].State = StateCompleted
			} else {
				m.lines[msg.lineNum].State = StateFailed
				m.lines[msg.lineNum].Error = msg.error
			}
		}

		// Move to next line
		m.currentLine++
		if m.currentLine >= len(m.lines) {
			// Script completed
			m.executing = false
			return m, m.addLog("✅ Script execution completed")
		}

		// Execute next line if not paused
		if !m.paused {
			return m, m.executeNextLine()
		}

	case watchUpdateMsg:
		if m.watchState.Active {
			m.watchState.PollCount = msg.pollCount
			m.watchState.LastOCR = msg.ocrText
			m.watchState.Found = msg.found
		}

	case watchCompleteMsg:
		m.watchState.Active = false
		if msg.found {
			return m, tea.Batch(
				m.addLog(fmt.Sprintf("✅ WATCH found: '%s'", m.watchState.SearchString)),
				func() tea.Msg {
					return scriptLineCompleteMsg{
						lineNum:  m.currentLine,
						success:  true,
						duration: time.Since(m.lines[m.currentLine].StartTime),
					}
				},
			)
		} else {
			return m, tea.Batch(
				m.addLog(fmt.Sprintf("❌ WATCH timeout: '%s'", m.watchState.SearchString)),
				func() tea.Msg {
					return scriptLineCompleteMsg{
						lineNum:  m.currentLine,
						success:  false,
						error:    msg.error,
						duration: time.Since(m.lines[m.currentLine].StartTime),
					}
				},
			)
		}

	case logMsg:
		// Add log message
		timestamp := time.Now().Format("15:04:05")
		logEntry := fmt.Sprintf("[%s] %s", timestamp, string(msg))
		m.logs = append(m.logs, logEntry)

		// Keep only recent logs
		if len(m.logs) > m.maxLogs {
			m.logs = m.logs[1:]
		}
	}

	return m, nil
}

// addLog creates a command to add a log message
func (m ScriptTUIModel) addLog(message string) tea.Cmd {
	return func() tea.Msg {
		return logMsg(message)
	}
}

// executeNextLine executes the next line in the script
func (m ScriptTUIModel) executeNextLine() tea.Cmd {
	if m.currentLine >= len(m.lines) {
		return nil
	}

	line := &m.lines[m.currentLine]
	line.State = StateExecuting
	line.StartTime = time.Now()

	return func() tea.Msg {
		// Simulate script execution - in real implementation, this would
		// execute the actual script line
		time.Sleep(100 * time.Millisecond) // Simulate processing time

		// Check if it's a WATCH command
		if strings.Contains(strings.ToUpper(line.Content), "WATCH") {
			// Start WATCH monitoring
			// This is a simplified version - real implementation would parse the WATCH command
			return scriptLineCompleteMsg{
				lineNum:  m.currentLine,
				success:  true,
				duration: time.Since(line.StartTime),
			}
		}

		// Regular command execution
		return scriptLineCompleteMsg{
			lineNum:  m.currentLine,
			success:  true,
			duration: time.Since(line.StartTime),
		}
	}
}

// View renders the TUI
func (m ScriptTUIModel) View() string {
	if m.quitting {
		return scriptSuccessStyle.Render("Script execution terminated\n")
	}

	var s strings.Builder

	// Title
	title := scriptTitleStyle.Render(fmt.Sprintf(" Script Execution - VM %s ", m.vmid))
	s.WriteString(title + "\n\n")

	// Status line
	status := ""
	if m.executing {
		if m.paused {
			status = "⏸ PAUSED"
		} else if m.watchState.Active {
			status = fmt.Sprintf("👁 WATCHING for '%s'", m.watchState.SearchString)
		} else {
			status = "▶ EXECUTING"
		}
	} else {
		status = "✅ COMPLETED"
	}

	uptime := time.Since(m.startTime).Truncate(time.Second)
	statusLine := scriptStatusStyle.Render(fmt.Sprintf("%s | Line %d/%d | Runtime: %v",
		status, m.currentLine+1, len(m.lines), uptime))
	s.WriteString(statusLine + "\n\n")

	// Main content area - split into two columns
	leftWidth := m.width/2 - 2
	rightWidth := m.width - leftWidth - 4

	// Left column: Script execution
	var scriptContent strings.Builder
	scriptContent.WriteString("Script Progress:\n\n")

	// Show script lines with status
	visibleLines := m.height - 15 // Reserve space for other elements
	if visibleLines < 5 {
		visibleLines = 5
	}

	start := 0
	if len(m.lines) > visibleLines {
		// Center the current line in view
		start = m.currentLine - visibleLines/2
		if start < 0 {
			start = 0
		}
		if start > len(m.lines)-visibleLines {
			start = len(m.lines) - visibleLines
		}
	}

	for i := start; i < start+visibleLines && i < len(m.lines); i++ {
		line := m.lines[i]
		var lineStyle lipgloss.Style
		var statusIcon string

		switch line.State {
		case StatePending:
			lineStyle = pendingStyle
			statusIcon = "○"
		case StateExecuting:
			lineStyle = executingStyle
			statusIcon = "⏳"
		case StateCompleted:
			lineStyle = completedStyle
			statusIcon = "✓"
		case StateFailed:
			lineStyle = failedStyle
			statusIcon = "✗"
		case StateWatching:
			lineStyle = watchingStyle
			statusIcon = "👁"
		}

		// Highlight current line
		if i == m.currentLine {
			lineStyle = currentLineStyle.Copy().Inherit(lineStyle)
		}

		lineText := fmt.Sprintf("%s %3d: %s", statusIcon, line.Number, line.Content)
		if line.Duration > 0 {
			lineText += fmt.Sprintf(" (%v)", line.Duration.Truncate(time.Millisecond))
		}
		if line.Error != "" {
			lineText += fmt.Sprintf(" - %s", line.Error)
		}

		// Truncate if too long
		if len(lineText) > leftWidth-4 && leftWidth > 7 {
			lineText = lineText[:leftWidth-7] + "..."
		} else if len(lineText) > leftWidth-4 {
			// If terminal is very narrow, just truncate to available space
			maxLen := leftWidth - 4
			if maxLen > 0 {
				lineText = lineText[:maxLen]
			}
		}

		scriptContent.WriteString(lineStyle.Render(lineText) + "\n")
	}

	scriptBox := scriptBoxStyle.Width(leftWidth).Render(scriptContent.String())

	// Right column: WATCH monitor or logs
	var rightContent strings.Builder
	if m.watchState.Active {
		// Show WATCH monitoring
		rightContent.WriteString(fmt.Sprintf("WATCH Monitor:\n\n"))
		rightContent.WriteString(fmt.Sprintf("Searching for: '%s'\n", m.watchState.SearchString))
		rightContent.WriteString(fmt.Sprintf("Poll count: %d\n", m.watchState.PollCount))

		elapsed := time.Since(m.watchState.StartTime).Truncate(time.Second)
		remaining := m.watchState.Timeout - elapsed
		rightContent.WriteString(fmt.Sprintf("Elapsed: %v\n", elapsed))
		rightContent.WriteString(fmt.Sprintf("Remaining: %v\n", remaining))
		rightContent.WriteString("\nLast OCR Result:\n")
		rightContent.WriteString("┌─────────────────────┐\n")

		// Show OCR preview (truncated)
		ocrLines := strings.Split(m.watchState.LastOCR, "\n")
		maxOCRLines := 8
		for i, line := range ocrLines {
			if i >= maxOCRLines {
				rightContent.WriteString("│ ... (truncated)     │\n")
				break
			}
			if len(line) > 19 {
				line = line[:16] + "..."
			}
			rightContent.WriteString(fmt.Sprintf("│ %-19s │\n", line))
		}
		rightContent.WriteString("└─────────────────────┘\n")

		rightBox := watchBoxStyle.Width(rightWidth).Render(rightContent.String())
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, scriptBox, rightBox))
	} else {
		// Show logs
		rightContent.WriteString("Execution Log:\n\n")
		for _, log := range m.logs {
			if len(log) > rightWidth-4 && rightWidth > 7 {
				log = log[:rightWidth-7] + "..."
			} else if len(log) > rightWidth-4 {
				// If terminal is very narrow, just truncate to available space
				maxLen := rightWidth - 4
				if maxLen > 0 {
					log = log[:maxLen]
				}
			}
			rightContent.WriteString(log + "\n")
		}

		rightBox := logsBoxStyle.Width(rightWidth).Render(rightContent.String())
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, scriptBox, rightBox))
	}

	// Controls
	s.WriteString("\n\n")
	controls := "Controls: [Space] Pause/Resume • [S] Skip Line • [Q] Quit"
	s.WriteString(scriptStatusStyle.Render(controls))

	return s.String()
}

// parseScriptFile reads and parses a script file into ScriptLine objects
func parseScriptFile(filename string) ([]ScriptLine, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening script file: %v", err)
	}
	defer file.Close()

	var lines []ScriptLine
	lineNum := 0

	// Read file line by line
	content := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	// Split into lines and process
	fileLines := strings.Split(string(content), "\n")
	commentPrefix := getCommentChar()

	for _, line := range fileLines {
		lineNum++
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle comments - skip pure comments but include escaped ones
		if processedLine, isComment := processCommentLine(line, commentPrefix); isComment {
			if processedLine == "" {
				continue // Pure comment, skip
			}
			line = processedLine // Escaped comment, use processed version
		}

		// Add line to script
		scriptLine := ScriptLine{
			Number:  lineNum,
			Content: line,
			State:   StatePending,
		}
		lines = append(lines, scriptLine)
	}

	return lines, nil
}
