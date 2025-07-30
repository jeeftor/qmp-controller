package script2

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/ocr"
)

// EnhancedDebugTUI extends the original DebugTUI with advanced OCR features
type EnhancedDebugTUI struct {
	debugger  *Debugger
	lastModel tea.Model
}

// NewEnhancedDebugTUI creates a new enhanced debug TUI
func NewEnhancedDebugTUI(debugger *Debugger) *EnhancedDebugTUI {
	return &EnhancedDebugTUI{
		debugger: debugger,
	}
}

// GetLastAction returns the last debug action performed by the enhanced TUI
func (t *EnhancedDebugTUI) GetLastAction() DebugAction {
	if model, ok := t.lastModel.(*enhancedDebugTUIModel); ok {
		return model.lastAction
	}
	return DebugActionContinue
}

// Run executes the enhanced debug TUI
func (t *EnhancedDebugTUI) Run() (DebugAction, error) {
	model := t.InitialModel()
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return DebugActionStop, err
	}

	t.lastModel = finalModel
	return t.GetLastAction(), nil
}

// enhancedDebugTUIModel extends the original model with OCR state
type enhancedDebugTUIModel struct {
	debugger       *Debugger
	viewport       viewport.Model
	textInput      textinput.Model
	keys           enhancedDebugKeyMap
	lastAction     DebugAction
	inputMode      bool
	currentView    enhancedDebugView
	width          int
	height         int
	helpVisible    bool

	// Enhanced OCR state
	ocrState       EnhancedOCRState
	searchMode     bool
	searchInput    textinput.Model
}

// enhancedDebugView extends the original views with enhanced OCR modes
type enhancedDebugView int

const (
	enhancedViewScript enhancedDebugView = iota
	enhancedViewVariables
	enhancedViewBreakpoints
	enhancedViewHelp
	enhancedViewOCR           // Basic OCR view
	enhancedViewOCRFull       // Full-screen OCR viewer
	enhancedViewOCRSearch     // OCR search view
	enhancedViewWatchProgress
	enhancedViewPerformance   // Performance monitoring
)

// EnhancedOCRState manages advanced OCR features in the debug TUI
type EnhancedOCRState struct {
	CurrentOCR      *ocr.OCRResult
	PreviousOCR     *ocr.OCRResult
	LastUpdate      time.Time
	RefreshRate     time.Duration
	SearchQuery     string
	SearchMatches   []ocr.SearchResult
	HighlightRow    int
	HighlightCol    int
	AutoRefresh     bool
	ShowDiff        bool
	ShowGrid        bool
	ShowPerformance bool
	PerformanceLog  []PerformanceMetric
}

// PerformanceMetric tracks OCR and debug performance
type PerformanceMetric struct {
	Timestamp    time.Time
	Operation    string
	Duration     time.Duration
	Success      bool
	Details      map[string]interface{}
}

// enhancedDebugKeyMap extends the original key bindings
type enhancedDebugKeyMap struct {
	debugKeyMap                     // Embed original bindings

	// Enhanced OCR controls
	OCRFullView    key.Binding     // Full-screen OCR view
	OCRSearch      key.Binding     // Search in OCR
	OCRToggleGrid  key.Binding     // Toggle grid overlay
	OCRToggleDiff  key.Binding     // Toggle diff highlighting
	OCRToggleAuto  key.Binding     // Toggle auto-refresh
	OCRExport      key.Binding     // Export OCR data

	// Navigation in OCR view
	NavUp          key.Binding
	NavDown        key.Binding
	NavLeft        key.Binding
	NavRight       key.Binding

	// Performance monitoring
	PerfView       key.Binding     // Performance view
	PerfClear      key.Binding     // Clear performance log
}

// EnhancedDebugKeyMap returns enhanced key bindings
func EnhancedDebugKeyMap() enhancedDebugKeyMap {
	return enhancedDebugKeyMap{
		debugKeyMap: DefaultDebugKeyMap(),

		// Enhanced OCR controls
		OCRFullView:    key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "full OCR view")),
		OCRSearch:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search OCR")),
		OCRToggleGrid:  key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "toggle grid")),
		OCRToggleDiff:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle diff")),
		OCRToggleAuto:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "auto-refresh")),
		OCRExport:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export OCR")),

		// Navigation
		NavUp:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		NavDown:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		NavLeft:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "move left")),
		NavRight:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "move right")),

		// Performance
		PerfView:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "performance")),
		PerfClear:      key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "clear perf")),
	}
}

// InitialModel returns the enhanced initial model
func (t *EnhancedDebugTUI) InitialModel() tea.Model {
	// Create text input for search
	searchInput := textinput.New()
	searchInput.Placeholder = "Search OCR text..."
	searchInput.CharLimit = 100

	return &enhancedDebugTUIModel{
		debugger:    t.debugger,
		viewport:    viewport.New(80, 24),
		textInput:   textinput.New(),
		searchInput: searchInput,
		keys:        EnhancedDebugKeyMap(),
		currentView: enhancedViewScript,
		ocrState: EnhancedOCRState{
			RefreshRate:     2 * time.Second,
			AutoRefresh:     false, // Start with manual refresh in debug mode
			ShowGrid:       true,
			ShowDiff:       true,
			ShowPerformance: false,
			PerformanceLog: make([]PerformanceMetric, 0, 100),
		},
	}
}

// Init initializes the enhanced model
func (m *enhancedDebugTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles enhanced TUI updates
func (m *enhancedDebugTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 // Leave space for header/footer

	case tea.KeyMsg:
		if m.searchMode {
			return m.handleSearchMode(msg)
		} else if m.inputMode {
			return m.handleInputMode(msg)
		} else {
			return m.handleEnhancedNormalMode(msg)
		}

	case enhancedOCRRefreshMsg:
		m.recordPerformance("ocr_refresh", msg.duration, msg.error == nil, map[string]interface{}{
			"characters": len(fmt.Sprintf("%v", msg.result)),
		})

		m.ocrState.PreviousOCR = m.ocrState.CurrentOCR
		m.ocrState.CurrentOCR = msg.result
		m.ocrState.LastUpdate = time.Now()

		// Update search matches if we have a query
		if m.ocrState.SearchQuery != "" {
			m.updateSearchMatches()
		}

		m.updateViewportContent()
	}

	// Update text inputs
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.searchInput, cmd = m.searchInput.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// Enhanced message types
type enhancedOCRRefreshMsg struct {
	result   *ocr.OCRResult
	error    error
	duration time.Duration
}

type enhancedExportMsg struct {
	filename string
	success  bool
	error    error
}

// handleEnhancedNormalMode handles key presses with enhanced OCR features
func (m *enhancedDebugTUIModel) handleEnhancedNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	// Original debug controls
	case key.Matches(msg, m.keys.Continue):
		m.lastAction = DebugActionContinue
		return m, tea.Quit
	case key.Matches(msg, m.keys.Step):
		m.lastAction = DebugActionStep
		return m, tea.Quit
	case key.Matches(msg, m.keys.Stop):
		m.lastAction = DebugActionStop
		return m, tea.Quit
	case key.Matches(msg, m.keys.Quit):
		m.lastAction = DebugActionStop
		return m, tea.Quit

	// Enhanced OCR controls
	case key.Matches(msg, m.keys.OCRFullView):
		m.currentView = enhancedViewOCRFull
		m.updateViewportContent()
		return m, m.refreshOCR()

	case key.Matches(msg, m.keys.OCRSearch):
		if m.currentView == enhancedViewOCRFull || m.currentView == enhancedViewOCR {
			m.searchMode = true
			m.searchInput.Focus()
			return m, nil
		}

	case key.Matches(msg, m.keys.OCRToggleGrid):
		m.ocrState.ShowGrid = !m.ocrState.ShowGrid
		m.updateViewportContent()

	case key.Matches(msg, m.keys.OCRToggleDiff):
		m.ocrState.ShowDiff = !m.ocrState.ShowDiff
		m.updateViewportContent()

	case key.Matches(msg, m.keys.OCRToggleAuto):
		m.ocrState.AutoRefresh = !m.ocrState.AutoRefresh
		if m.ocrState.AutoRefresh {
			return m, m.startAutoRefresh()
		}

	case key.Matches(msg, m.keys.OCRExport):
		return m, m.exportOCR()

	case key.Matches(msg, m.keys.PerfView):
		m.currentView = enhancedViewPerformance
		m.updateViewportContent()

	case key.Matches(msg, m.keys.PerfClear):
		m.ocrState.PerformanceLog = m.ocrState.PerformanceLog[:0]
		m.updateViewportContent()

	// Navigation in OCR views
	case key.Matches(msg, m.keys.NavUp):
		if m.currentView == enhancedViewOCRFull {
			if m.ocrState.HighlightRow > 0 {
				m.ocrState.HighlightRow--
			}
			m.updateViewportContent()
		}
	case key.Matches(msg, m.keys.NavDown):
		if m.currentView == enhancedViewOCRFull && m.ocrState.CurrentOCR != nil {
			if m.ocrState.HighlightRow < len(m.ocrState.CurrentOCR.Text)-1 {
				m.ocrState.HighlightRow++
			}
			m.updateViewportContent()
		}
	case key.Matches(msg, m.keys.NavLeft):
		if m.currentView == enhancedViewOCRFull {
			if m.ocrState.HighlightCol > 0 {
				m.ocrState.HighlightCol--
			}
			m.updateViewportContent()
		}
	case key.Matches(msg, m.keys.NavRight):
		if m.currentView == enhancedViewOCRFull && m.ocrState.CurrentOCR != nil {
			maxWidth := 0
			for _, line := range m.ocrState.CurrentOCR.Text {
				if len(line) > maxWidth {
					maxWidth = len(line)
				}
			}
			if m.ocrState.HighlightCol < maxWidth-1 {
				m.ocrState.HighlightCol++
			}
			m.updateViewportContent()
		}

	// View switching
	case key.Matches(msg, m.keys.Variables):
		m.currentView = enhancedViewVariables
		m.updateViewportContent()
	case key.Matches(msg, m.keys.Breakpoints):
		m.currentView = enhancedViewBreakpoints
		m.updateViewportContent()
	case key.Matches(msg, m.keys.OCRView):
		m.currentView = enhancedViewOCR
		m.updateViewportContent()

	// Refresh
	case key.Matches(msg, m.keys.Refresh):
		return m, m.refreshOCR()

	// Number key shortcuts
	case msg.String() == "1":
		m.currentView = enhancedViewScript
		m.updateViewportContent()
	case msg.String() == "2":
		m.currentView = enhancedViewVariables
		m.updateViewportContent()
	case msg.String() == "3":
		m.currentView = enhancedViewBreakpoints
		m.updateViewportContent()
	case msg.String() == "4":
		m.currentView = enhancedViewOCR
		m.updateViewportContent()
	case msg.String() == "5":
		m.currentView = enhancedViewWatchProgress
		m.updateViewportContent()
	case msg.String() == "6":
		m.currentView = enhancedViewOCRFull
		m.updateViewportContent()
		return m, m.refreshOCR()
	case msg.String() == "7":
		m.currentView = enhancedViewPerformance
		m.updateViewportContent()

	case msg.String() == ":":
		m.inputMode = true
		m.textInput.Focus()
		m.textInput.SetValue("")
	}

	return m, nil
}

// handleSearchMode handles search input
func (m *enhancedDebugTUIModel) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		query := m.searchInput.Value()
		m.ocrState.SearchQuery = query
		m.updateSearchMatches()
		m.searchMode = false
		m.searchInput.Blur()
		m.updateViewportContent()

	case tea.KeyEsc:
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// Helper methods for enhanced functionality

func (m *enhancedDebugTUIModel) refreshOCR() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		err := m.debugger.RefreshOCR()
		duration := time.Since(start)

		state := m.debugger.GetState()

		// Convert string slice to OCR result for compatibility
		result := &ocr.OCRResult{
			Width:  120, // Estimate
			Height: len(state.LastOCRResult),
			Text:   state.LastOCRResult,
		}

		return enhancedOCRRefreshMsg{
			result:   result,
			error:    err,
			duration: duration,
		}
	}
}

func (m *enhancedDebugTUIModel) updateSearchMatches() {
	if m.ocrState.CurrentOCR == nil || m.ocrState.SearchQuery == "" {
		m.ocrState.SearchMatches = nil
		return
	}

	searchConfig := ocr.SearchConfig{
		IgnoreCase:  false,
		FirstOnly:   false,
		Quiet:       true,
		LineNumbers: false,
	}

	searchResults := ocr.FindString(m.ocrState.CurrentOCR, m.ocrState.SearchQuery, searchConfig)
	m.ocrState.SearchMatches = searchResults.Matches
}

func (m *enhancedDebugTUIModel) exportOCR() tea.Cmd {
	return func() tea.Msg {
		filename := fmt.Sprintf("debug-ocr-%d.txt", time.Now().Unix())

		if m.ocrState.CurrentOCR == nil {
			return enhancedExportMsg{
				filename: filename,
				success:  false,
				error:    fmt.Errorf("no OCR data to export"),
			}
		}

		content := strings.Join(m.ocrState.CurrentOCR.Text, "\n")
		err := os.WriteFile(filename, []byte(content), 0644)

		return enhancedExportMsg{
			filename: filename,
			success:  err == nil,
			error:    err,
		}
	}
}

func (m *enhancedDebugTUIModel) startAutoRefresh() tea.Cmd {
	return tea.Tick(m.ocrState.RefreshRate, func(t time.Time) tea.Msg {
		return "auto_refresh"
	})
}

func (m *enhancedDebugTUIModel) recordPerformance(operation string, duration time.Duration, success bool, details map[string]interface{}) {
	metric := PerformanceMetric{
		Timestamp: time.Now(),
		Operation: operation,
		Duration:  duration,
		Success:   success,
		Details:   details,
	}

	m.ocrState.PerformanceLog = append(m.ocrState.PerformanceLog, metric)

	// Keep only last 100 entries
	if len(m.ocrState.PerformanceLog) > 100 {
		m.ocrState.PerformanceLog = m.ocrState.PerformanceLog[1:]
	}
}

// This would include the enhanced rendering methods
// For now, I'll add placeholders that would use our existing OCR rendering logic

func (m *enhancedDebugTUIModel) updateViewportContent() {
	switch m.currentView {
	case enhancedViewScript:
		m.viewport.SetContent(m.renderEnhancedScriptView())
	case enhancedViewVariables:
		m.viewport.SetContent(m.renderEnhancedVariablesView())
	case enhancedViewBreakpoints:
		m.viewport.SetContent(m.renderEnhancedBreakpointsView())
	case enhancedViewOCR:
		m.viewport.SetContent(m.renderEnhancedOCRView())
	case enhancedViewOCRFull:
		m.viewport.SetContent(m.renderFullScreenOCRView())
	case enhancedViewOCRSearch:
		m.viewport.SetContent(m.renderOCRSearchView())
	case enhancedViewWatchProgress:
		m.viewport.SetContent(m.renderEnhancedWatchProgressView())
	case enhancedViewPerformance:
		m.viewport.SetContent(m.renderPerformanceView())
	case enhancedViewHelp:
		m.viewport.SetContent(m.renderEnhancedHelpView())
	}
}

// Enhanced rendering methods that integrate with existing debug TUI functionality
func (m *enhancedDebugTUIModel) renderEnhancedScriptView() string {
	var content strings.Builder
	state := m.debugger.GetState()
	script := m.debugger.GetScript()

	content.WriteString(headerStyle.Render("📜 ENHANCED SCRIPT VIEW") + "\n\n")

	// OCR status integration
	if m.ocrState.CurrentOCR != nil {
		content.WriteString(fmt.Sprintf("🔍 OCR Status: %dx%d grid, updated %s\n",
			m.ocrState.CurrentOCR.Width,
			m.ocrState.CurrentOCR.Height,
			m.ocrState.LastUpdate.Format("15:04:05")))
		if m.ocrState.SearchQuery != "" {
			content.WriteString(fmt.Sprintf("🔍 Search: '%s' (%d matches)\n", m.ocrState.SearchQuery, len(m.ocrState.SearchMatches)))
		}
		content.WriteString("\n")
	}

	// Enhanced script rendering with current line highlighting
	for i, line := range script.Lines {
		lineNumber := i + 1
		prefix := fmt.Sprintf("%3d", lineNumber)

		// Style the line based on execution state
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

		// Format the line with enhanced information
		formattedLine := fmt.Sprintf("%s %s │ %s", marker, prefix, line.Content)
		content.WriteString(lineStyle.Render(formattedLine) + "\n")
	}

	return content.String()
}

func (m *enhancedDebugTUIModel) renderFullScreenOCRView() string {
	var content strings.Builder

	content.WriteString(headerStyle.Render("🔍 FULL-SCREEN OCR VIEWER") + "\n\n")

	if m.ocrState.CurrentOCR == nil {
		content.WriteString(mutedStyle.Render("No OCR data available. Press 'r' to refresh.") + "\n")
		return content.String()
	}

	// Navigation information
	content.WriteString(fmt.Sprintf("📍 Position: %s | Size: %dx%d\n",
		valueStyle.Render(fmt.Sprintf("%d,%d", m.ocrState.HighlightRow, m.ocrState.HighlightCol)),
		m.ocrState.CurrentOCR.Width, m.ocrState.CurrentOCR.Height))

	if m.ocrState.SearchQuery != "" {
		content.WriteString(fmt.Sprintf("🔍 Search: %s (%s matches)\n",
			variableStyle.Render(fmt.Sprintf("'%s'", m.ocrState.SearchQuery)),
			valueStyle.Render(fmt.Sprintf("%d", len(m.ocrState.SearchMatches)))))
	}

	content.WriteString("\n")

	// Render OCR text with enhanced highlighting
	for i, line := range m.ocrState.CurrentOCR.Text {
		lineNum := fmt.Sprintf("%2d", i+1)

		// Highlight current row
		lineStyle := textStyle.Copy()
		if i == m.ocrState.HighlightRow {
			lineStyle = lineStyle.Background(lipgloss.Color("#2d3748"))
		}

		// Process line for search highlighting
		displayLine := line
		if m.ocrState.SearchQuery != "" {
			// Highlight search matches (simplified highlighting)
			if strings.Contains(strings.ToLower(line), strings.ToLower(m.ocrState.SearchQuery)) {
				displayLine = strings.ReplaceAll(line, m.ocrState.SearchQuery,
					successStyle.Background(lipgloss.Color("#fef08a")).Foreground(lipgloss.Color("#000")).Render(m.ocrState.SearchQuery))
			}
		}

		// Add grid overlay if enabled
		gridMarker := "│"
		if m.ocrState.ShowGrid {
			gridMarker = mutedStyle.Render("│")
		}

		content.WriteString(fmt.Sprintf("%s %s %s\n",
			lineNumStyle.Render(lineNum),
			gridMarker,
			lineStyle.Render(displayLine)))
	}

	// Show controls
	content.WriteString("\n" + mutedStyle.Render("Controls: ↑↓←→/hjkl=navigate, /=search, r=refresh, g=grid, d=diff, e=export"))

	return content.String()
}

func (m *enhancedDebugTUIModel) renderPerformanceView() string {
	content := "⚡ PERFORMANCE MONITORING\n\n"

	if len(m.ocrState.PerformanceLog) == 0 {
		content += "No performance data collected yet.\n"
		return content
	}

	// Show recent performance metrics
	for i := len(m.ocrState.PerformanceLog) - 1; i >= 0 && i >= len(m.ocrState.PerformanceLog)-10; i-- {
		metric := m.ocrState.PerformanceLog[i]
		status := "✅"
		if !metric.Success {
			status = "❌"
		}
		content += fmt.Sprintf("%s %s %s (%v)\n",
			status,
			metric.Timestamp.Format("15:04:05"),
			metric.Operation,
			metric.Duration)
	}

	return content
}

func (m *enhancedDebugTUIModel) renderEnhancedVariablesView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("🔢 ENHANCED VARIABLES") + "\n\n")

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

	// Enhanced: Show OCR variable context if available
	if m.ocrState.CurrentOCR != nil {
		content.WriteString("\n" + sectionStyle.Render("👁️ OCR CONTEXT") + "\n")
		content.WriteString(fmt.Sprintf("Screen size: %dx%d\n", m.ocrState.CurrentOCR.Width, m.ocrState.CurrentOCR.Height))
		content.WriteString(fmt.Sprintf("Last update: %s\n", m.ocrState.LastUpdate.Format("15:04:05")))
		if m.ocrState.SearchQuery != "" {
			content.WriteString(fmt.Sprintf("Active search: '%s' (%d matches)\n",
				m.ocrState.SearchQuery, len(m.ocrState.SearchMatches)))
		}
	}

	return content.String()
}

func (m *enhancedDebugTUIModel) renderEnhancedBreakpointsView() string {
	var content strings.Builder
	state := m.debugger.GetState()
	script := m.debugger.GetScript()

	content.WriteString(headerStyle.Render("🔴 ENHANCED BREAKPOINTS") + "\n\n")

	if len(state.Breakpoints) == 0 {
		content.WriteString(mutedStyle.Render("No breakpoints set") + "\n")
	} else {
		for lineNumber := range state.Breakpoints {
			if lineNumber <= len(script.Lines) {
				line := script.Lines[lineNumber-1]

				// Enhanced: Show if breakpoint is at current line
				status := "●"
				if lineNumber == state.CurrentLine {
					status = successStyle.Render("►●") // Current + breakpoint
				} else {
					status = errorStyle.Render("●")
				}

				content.WriteString(fmt.Sprintf("%s Line %s: %s\n",
					status,
					lineNumStyle.Render(fmt.Sprintf("%d", lineNumber)),
					textStyle.Render(line.Content)))
			}
		}
	}

	content.WriteString("\n" + mutedStyle.Render("Tip: Use debugging commands to manage breakpoints") + "\n")

	// Enhanced: Show OCR-based conditional breakpoints (future feature)
	if m.ocrState.CurrentOCR != nil {
		content.WriteString("\n" + sectionStyle.Render("👁️ OCR BREAKPOINT CONTEXT") + "\n")
		content.WriteString(mutedStyle.Render("Future: OCR-based conditional breakpoints will appear here") + "\n")
	}

	return content.String()
}

func (m *enhancedDebugTUIModel) renderEnhancedOCRView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("👁️ ENHANCED OCR PREVIEW") + "\n\n")

	// Check if we have current OCR data
	if m.ocrState.CurrentOCR == nil || len(m.ocrState.CurrentOCR.Text) == 0 {
		// Fallback to debugger state OCR if available
		if len(state.LastOCRResult) == 0 {
			content.WriteString(mutedStyle.Render("No OCR data available") + "\n")
			content.WriteString(mutedStyle.Render("Press 'r' to refresh with current screen") + "\n")
			return content.String()
		}

		// Use debugger state OCR data
		for i, line := range state.LastOCRResult {
			lineNum := fmt.Sprintf("%2d", i+1)
			content.WriteString(fmt.Sprintf("%s │ %s\n",
				lineNumStyle.Render(lineNum),
				textStyle.Render(line)))
		}
	} else {
		// Use enhanced OCR data with search highlighting
		for i, line := range m.ocrState.CurrentOCR.Text {
			lineNum := fmt.Sprintf("%2d", i+1)

			// Apply search highlighting if active
			displayLine := line
			if m.ocrState.SearchQuery != "" && strings.Contains(strings.ToLower(line), strings.ToLower(m.ocrState.SearchQuery)) {
				displayLine = strings.ReplaceAll(line, m.ocrState.SearchQuery,
					successStyle.Background(lipgloss.Color("#fef08a")).Foreground(lipgloss.Color("#000")).Render(m.ocrState.SearchQuery))
			}

			content.WriteString(fmt.Sprintf("%s │ %s\n",
				lineNumStyle.Render(lineNum),
				textStyle.Render(displayLine)))
		}
	}

	// Show search results if available
	if m.ocrState.SearchQuery != "" {
		content.WriteString("\n" + sectionStyle.Render("🔍 SEARCH RESULTS") + "\n")
		content.WriteString(fmt.Sprintf("Query: %s\n", variableStyle.Render(fmt.Sprintf("'%s'", m.ocrState.SearchQuery))))
		content.WriteString(fmt.Sprintf("Matches: %s\n", valueStyle.Render(fmt.Sprintf("%d", len(m.ocrState.SearchMatches)))))
	}

	// Show last search from debugger state if available
	if state.LastSearchTerm != "" {
		content.WriteString("\n" + sectionStyle.Render("🔍 DEBUGGER SEARCH") + "\n")
		content.WriteString(fmt.Sprintf("Term: %s\n", variableStyle.Render(state.LastSearchTerm)))
		content.WriteString(fmt.Sprintf("Found: %s\n",
			func() string {
				if state.LastSearchFound {
					return successStyle.Render("YES")
				}
				return errorStyle.Render("NO")
			}()))
	}

	content.WriteString("\n" + mutedStyle.Render("Press 'O' for full-screen OCR, 'r' to refresh, '/' to search") + "\n")

	return content.String()
}

func (m *enhancedDebugTUIModel) renderOCRSearchView() string {
	var content strings.Builder

	content.WriteString(headerStyle.Render("🔍 OCR SEARCH RESULTS") + "\n\n")

	if m.ocrState.SearchQuery == "" {
		content.WriteString(mutedStyle.Render("No active search. Press '/' to start searching.") + "\n")
		return content.String()
	}

	content.WriteString(fmt.Sprintf("Query: %s\n", variableStyle.Render(fmt.Sprintf("'%s'", m.ocrState.SearchQuery))))
	content.WriteString(fmt.Sprintf("Matches: %s\n\n", valueStyle.Render(fmt.Sprintf("%d", len(m.ocrState.SearchMatches)))))

	if len(m.ocrState.SearchMatches) == 0 {
		content.WriteString(errorStyle.Render("No matches found") + "\n")
		return content.String()
	}

	// Display search results with context
	for i, match := range m.ocrState.SearchMatches {
		if i >= 10 { // Limit display to first 10 matches
			content.WriteString(mutedStyle.Render(fmt.Sprintf("... and %d more matches", len(m.ocrState.SearchMatches)-10)) + "\n")
			break
		}

		// Show match with line context
		content.WriteString(fmt.Sprintf("%s Match %d at line %d:\n",
			successStyle.Render("●"),
			i+1,
			match.LineNumber))

		// Highlight the matched text
		highlightedLine := strings.ReplaceAll(match.Line, m.ocrState.SearchQuery,
			successStyle.Background(lipgloss.Color("#fef08a")).Foreground(lipgloss.Color("#000")).Render(m.ocrState.SearchQuery))

		content.WriteString(fmt.Sprintf("   %s\n\n", textStyle.Render(highlightedLine)))
	}

	content.WriteString(mutedStyle.Render("Press 'O' for full OCR view with navigation") + "\n")

	return content.String()
}

func (m *enhancedDebugTUIModel) renderEnhancedWatchProgressView() string {
	var content strings.Builder
	state := m.debugger.GetState()

	content.WriteString(headerStyle.Render("⏱️ ENHANCED WATCH PROGRESS") + "\n\n")

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

		// Enhanced: Show current OCR state during watch
		if m.ocrState.CurrentOCR != nil {
			content.WriteString("\n" + sectionStyle.Render("👁️ CURRENT SCREEN (OCR)") + "\n")

			// Show last few lines of OCR to see what's being monitored
			startLine := len(m.ocrState.CurrentOCR.Text) - 5
			if startLine < 0 {
				startLine = 0
			}

			for i := startLine; i < len(m.ocrState.CurrentOCR.Text); i++ {
				line := m.ocrState.CurrentOCR.Text[i]

				// Highlight search term if found
				displayLine := line
				if strings.Contains(strings.ToLower(line), strings.ToLower(op.SearchTerm)) {
					displayLine = strings.ReplaceAll(line, op.SearchTerm,
						successStyle.Background(lipgloss.Color("#fef08a")).Foreground(lipgloss.Color("#000")).Render(op.SearchTerm))
				}

				content.WriteString(fmt.Sprintf("  %s\n", textStyle.Render(displayLine)))
			}
		}

		// Show incremental text updates if available
		if len(op.IncrementalText) > 0 {
			content.WriteString("\n" + sectionStyle.Render("📝 NEW CONSOLE TEXT") + "\n")
			for _, line := range op.IncrementalText {
				content.WriteString(textStyle.Render(line) + "\n")
			}
		}
	} else {
		content.WriteString(mutedStyle.Render("No active watch operation") + "\n")

		// Enhanced: Show recent OCR state even when no watch is active
		if m.ocrState.CurrentOCR != nil {
			content.WriteString("\n" + sectionStyle.Render("👁️ CURRENT SCREEN") + "\n")
			content.WriteString(fmt.Sprintf("OCR Size: %dx%d | Last Update: %s\n",
				m.ocrState.CurrentOCR.Width,
				m.ocrState.CurrentOCR.Height,
				m.ocrState.LastUpdate.Format("15:04:05")))
		}
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

func (m *enhancedDebugTUIModel) renderEnhancedHelpView() string {
	content := "📚 ENHANCED DEBUG HELP\n\n"
	content += "Original Controls:\n"
	content += "  c/Enter  - Continue execution\n"
	content += "  s        - Step to next line\n"
	content += "  q        - Quit debugger\n\n"
	content += "Enhanced OCR Controls:\n"
	content += "  O        - Full-screen OCR viewer\n"
	content += "  /        - Search in OCR\n"
	content += "  g        - Toggle grid overlay\n"
	content += "  d        - Toggle diff highlighting\n"
	content += "  a        - Toggle auto-refresh\n"
	content += "  e        - Export OCR to file\n"
	content += "  p        - Performance monitoring\n"
	content += "  r        - Refresh OCR manually\n\n"
	content += "Navigation (in full OCR view):\n"
	content += "  ↑↓←→/hjkl - Navigate character grid\n\n"
	content += "Views:\n"
	content += "  1 - Script   2 - Variables   3 - Breakpoints\n"
	content += "  4 - OCR      5 - Watch       6 - Full OCR\n"
	content += "  7 - Performance\n"
	return content
}

// View renders the enhanced TUI
func (m *enhancedDebugTUIModel) View() string {
	var content strings.Builder

	// Header
	title := fmt.Sprintf("🐛 Enhanced Script2 Debugger - %s", m.currentViewString())
	content.WriteString(headerStyle.Render(title) + "\n")

	// Search bar (if in search mode)
	if m.searchMode {
		content.WriteString("Search: " + m.searchInput.View() + "\n")
	}

	// Main content
	content.WriteString(m.viewport.View())

	// Footer with status and help
	footer := ""
	if m.ocrState.AutoRefresh {
		footer += "🔄 Auto-refresh ON • "
	}
	if m.ocrState.ShowGrid {
		footer += "🗂️ Grid ON • "
	}
	if m.ocrState.ShowDiff {
		footer += "🔄 Diff ON • "
	}
	footer += "Press 'h' for help"

	content.WriteString("\n" + footerStyle.Render(footer))

	return content.String()
}

func (m *enhancedDebugTUIModel) currentViewString() string {
	switch m.currentView {
	case enhancedViewScript:
		return "Script"
	case enhancedViewVariables:
		return "Variables"
	case enhancedViewBreakpoints:
		return "Breakpoints"
	case enhancedViewOCR:
		return "OCR Preview"
	case enhancedViewOCRFull:
		return "Full OCR Viewer"
	case enhancedViewOCRSearch:
		return "OCR Search"
	case enhancedViewWatchProgress:
		return "Watch Progress"
	case enhancedViewPerformance:
		return "Performance"
	case enhancedViewHelp:
		return "Help"
	default:
		return "Unknown"
	}
}

// renderProgressBar renders a progress bar for watch operations
func (m *enhancedDebugTUIModel) renderProgressBar(progress float64, width int) string {
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

// handleInputMode handles command input (enhanced version)
func (m *enhancedDebugTUIModel) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		command := m.textInput.Value()
		m.processEnhancedCommand(command)
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

// processEnhancedCommand processes enhanced debug commands
func (m *enhancedDebugTUIModel) processEnhancedCommand(command string) {
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
	case "search", "find", "s":
		if len(parts) > 1 {
			query := strings.Join(parts[1:], " ")
			m.ocrState.SearchQuery = query
			m.updateSearchMatches()
			m.updateViewportContent()
		}
	case "ocr":
		// Force OCR refresh
		m.refreshOCR()
	case "export":
		// Export current OCR data
		m.exportOCR()
	}
}

// highlightLine highlights a specific line in the script view
func (m *enhancedDebugTUIModel) highlightLine(lineNum int) {
	if m.currentView == enhancedViewScript {
		// Calculate viewport position to show the line
		linesPerPage := m.viewport.Height - 2
		targetOffset := lineNum - linesPerPage/2
		if targetOffset < 0 {
			targetOffset = 0
		}
		m.viewport.SetYOffset(targetOffset)
	}
}
