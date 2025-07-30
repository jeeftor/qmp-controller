package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/styles"
)

// ViewMode represents different TUI display modes
type ViewMode int

const (
	ViewModeScript ViewMode = iota // Standard script execution view
	ViewModeOCR                    // Full-screen OCR console view
	ViewModeSplit                  // Split view: script + OCR
)

// OCRDisplayState manages the enhanced OCR visualization
type OCRDisplayState struct {
	CurrentOCR    *ocr.OCRResult
	PreviousOCR   *ocr.OCRResult
	LastUpdate    time.Time
	RefreshRate   time.Duration
	SearchQuery   string
	SearchMatches []ocr.SearchResult
	HighlightRow  int
	HighlightCol  int
	AutoRefresh   bool
	ShowDiff      bool
	ShowGrid      bool
}

// Use the existing SearchResult type from ocr package
// No need to redefine SearchMatch

// EnhancedScriptTUIModel extends the original model with OCR enhancements
type EnhancedScriptTUIModel struct {
	ScriptTUIModel                    // Embed original model
	viewMode       ViewMode           // Current display mode
	ocrState       OCRDisplayState    // Enhanced OCR state
	keyHelp        []string           // Context-sensitive key help
	statusBar      string             // Enhanced status information
}

// Enhanced message types for OCR features
type ocrRefreshMsg struct {
	result *ocr.OCRResult
	error  error
}

type ocrSearchMsg struct {
	query   string
	matches []ocr.SearchResult
}

type viewModeChangeMsg struct {
	mode ViewMode
}

type screenshotTakenMsg struct {
	filename string
	error    error
}

// NewEnhancedScriptTUIModel creates an enhanced script TUI with OCR features
func NewEnhancedScriptTUIModel(vmid string, client *qmp.Client, scriptFile string, lines []ScriptLine) EnhancedScriptTUIModel {
	baseModel := NewScriptTUIModel(vmid, client, scriptFile, lines)

	return EnhancedScriptTUIModel{
		ScriptTUIModel: baseModel,
		viewMode:       ViewModeScript,
		ocrState: OCRDisplayState{
			RefreshRate: 2 * time.Second, // Default 2-second refresh
			AutoRefresh: true,
			ShowGrid:    false,
			ShowDiff:    true,
		},
		keyHelp: []string{
			"[Space] Pause/Resume",
			"[S] Skip Line",
			"[O] OCR View",
			"[R] Refresh OCR",
			"[/] Search OCR",
			"[C] Screenshot",
			"[Q] Quit",
		},
	}
}

// Init initializes the enhanced TUI model
func (m EnhancedScriptTUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.ScriptTUIModel.Init(),
		m.startOCRRefresh(),
	)
}

// Update handles enhanced TUI messages
func (m EnhancedScriptTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "o", "O":
			// Toggle OCR view mode
			return m.toggleViewMode()

		case "r", "R":
			// Manual OCR refresh
			return m, m.refreshOCR()

		case "/":
			// Start OCR search (would need input handling)
			return m.startOCRSearch()

		case "c", "C":
			// Take screenshot
			return m, m.takeScreenshot()

		case "g", "G":
			// Toggle OCR grid overlay
			return m.toggleOCRGrid()

		case "d", "D":
			// Toggle diff highlighting
			return m.toggleOCRDiff()

		case "a", "A":
			// Toggle auto-refresh
			return m.toggleAutoRefresh()

		case "up", "down", "left", "right":
			if m.viewMode == ViewModeOCR {
				// Navigate OCR grid
				return m.navigateOCRGrid(msg.String())
			}
		}

	case ocrRefreshMsg:
		m.ocrState.PreviousOCR = m.ocrState.CurrentOCR
		m.ocrState.CurrentOCR = msg.result
		m.ocrState.LastUpdate = time.Now()

		// Update search matches if we have a query
		if m.ocrState.SearchQuery != "" {
			m.ocrState.SearchMatches = m.findOCRMatches(m.ocrState.SearchQuery)
		}

		// Schedule next refresh if auto-refresh is enabled
		if m.ocrState.AutoRefresh {
			cmd = tea.Tick(m.ocrState.RefreshRate, func(t time.Time) tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("refresh_internal")}
			})
			cmds = append(cmds, cmd)
		}

	case ocrSearchMsg:
		m.ocrState.SearchQuery = msg.query
		m.ocrState.SearchMatches = msg.matches

	case viewModeChangeMsg:
		m.viewMode = msg.mode
		m.updateKeyHelp()

	case screenshotTakenMsg:
		if msg.error != nil {
			m.statusBar = fmt.Sprintf("Screenshot failed: %v", msg.error)
		} else {
			m.statusBar = fmt.Sprintf("Screenshot saved: %s", msg.filename)
		}
	}

	// Handle base model updates
	baseModel, baseCmd := m.ScriptTUIModel.Update(msg)
	m.ScriptTUIModel = baseModel.(ScriptTUIModel)
	cmds = append(cmds, baseCmd)

	return m, tea.Batch(cmds...)
}

// View renders the enhanced TUI based on current view mode
func (m EnhancedScriptTUIModel) View() string {
	switch m.viewMode {
	case ViewModeOCR:
		return m.renderOCRView()
	case ViewModeSplit:
		return m.renderSplitView()
	default:
		return m.renderEnhancedScriptView()
	}
}

// renderOCRView renders the full-screen OCR console view
func (m EnhancedScriptTUIModel) renderOCRView() string {
	var s strings.Builder

	// Title bar
	title := styles.BoldStyle.Render(fmt.Sprintf(" OCR Console View - VM %s ", m.vmid))
	s.WriteString(title + "\n\n")

	if m.ocrState.CurrentOCR == nil {
		s.WriteString("Loading OCR data...\n")
		return s.String()
	}

	// OCR grid display
	s.WriteString(m.renderOCRGrid())

	// Status and search info
	s.WriteString("\n")
	s.WriteString(m.renderOCRStatus())

	// Key help
	s.WriteString("\n")
	s.WriteString(styles.MutedStyle.Render(strings.Join(m.keyHelp, " • ")))

	return s.String()
}

// renderOCRGrid renders the OCR text with enhancements
func (m EnhancedScriptTUIModel) renderOCRGrid() string {
	if m.ocrState.CurrentOCR == nil {
		return "No OCR data available"
	}

	var s strings.Builder

	// Grid header with column numbers (if enabled)
	if m.ocrState.ShowGrid {
		s.WriteString("   ") // Line number space
		maxWidth := 0
		for _, line := range m.ocrState.CurrentOCR.Text {
			if len(line) > maxWidth {
				maxWidth = len(line)
			}
		}
		for col := 0; col < maxWidth && col < 120; col += 10 {
			s.WriteString(fmt.Sprintf("%-10d", col))
		}
		s.WriteString("\n")
	}

	// Render each line
	for row, line := range m.ocrState.CurrentOCR.Text {
		// Row number (if grid enabled)
		if m.ocrState.ShowGrid {
			s.WriteString(fmt.Sprintf("%2d ", row))
		}

		// Render characters in this line with highlighting
		for col, char := range line {
			style := m.getCharStyle(row, col, char)
			s.WriteString(style.Render(string(char)))
		}

		// Handle cursor at end of line
		if row == m.ocrState.HighlightRow && m.ocrState.HighlightCol >= len(line) {
			style := m.getCharStyle(row, len(line), ' ')
			s.WriteString(style.Render(" "))
		}

		s.WriteString("\n")
	}

	return s.String()
}

// getOCRCharAt gets the character at a specific position with bounds checking
func (m EnhancedScriptTUIModel) getOCRCharAt(row, col int) rune {
	if m.ocrState.CurrentOCR == nil {
		return ' '
	}

	if row >= 0 && row < len(m.ocrState.CurrentOCR.Text) {
		line := m.ocrState.CurrentOCR.Text[row]
		if col >= 0 && col < len(line) {
			return rune(line[col])
		}
	}
	return ' '
}

// getCharStyle determines the styling for a character based on various factors
func (m EnhancedScriptTUIModel) getCharStyle(row, col int, char rune) lipgloss.Style {
	baseStyle := lipgloss.NewStyle()

	// Highlight current cursor position
	if row == m.ocrState.HighlightRow && col == m.ocrState.HighlightCol {
		baseStyle = baseStyle.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	}

	// Highlight search matches
	for _, match := range m.ocrState.SearchMatches {
		if row == match.LineNumber && col >= match.StartCol && col < match.EndCol {
			baseStyle = baseStyle.Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
		}
	}

	// Show differences from previous OCR (if enabled)
	if m.ocrState.ShowDiff && m.ocrState.PreviousOCR != nil {
		prevChar := m.getPreviousOCRCharAt(row, col)
		if char != prevChar {
			baseStyle = baseStyle.Foreground(lipgloss.Color("2")) // Green for changes
		}
	}

	// Handle special characters
	if char == ' ' {
		return baseStyle // Keep background for highlighting
	}

	return baseStyle
}

// getPreviousOCRCharAt gets character from previous OCR result
func (m EnhancedScriptTUIModel) getPreviousOCRCharAt(row, col int) rune {
	if m.ocrState.PreviousOCR == nil {
		return ' '
	}

	if row >= 0 && row < len(m.ocrState.PreviousOCR.Text) {
		line := m.ocrState.PreviousOCR.Text[row]
		if col >= 0 && col < len(line) {
			return rune(line[col])
		}
	}
	return ' '
}

// renderOCRStatus renders status information for OCR view
func (m EnhancedScriptTUIModel) renderOCRStatus() string {
	var status strings.Builder

	if m.ocrState.CurrentOCR != nil {
		status.WriteString(fmt.Sprintf("Grid: %dx%d • ",
			m.ocrState.CurrentOCR.Width, m.ocrState.CurrentOCR.Height))
	}

	status.WriteString(fmt.Sprintf("Updated: %s • ",
		m.ocrState.LastUpdate.Format("15:04:05")))

	if m.ocrState.AutoRefresh {
		status.WriteString("Auto-refresh: ON • ")
	} else {
		status.WriteString("Auto-refresh: OFF • ")
	}

	if m.ocrState.SearchQuery != "" {
		status.WriteString(fmt.Sprintf("Search: '%s' (%d matches) • ",
			m.ocrState.SearchQuery, len(m.ocrState.SearchMatches)))
	}

	if m.statusBar != "" {
		status.WriteString(m.statusBar)
	}

	return styles.MutedStyle.Render(status.String())
}

// Helper methods for TUI functionality

func (m EnhancedScriptTUIModel) toggleViewMode() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeScript:
		m.viewMode = ViewModeOCR
	case ViewModeOCR:
		m.viewMode = ViewModeSplit
	case ViewModeSplit:
		m.viewMode = ViewModeScript
	}

	m.updateKeyHelp()

	return m, func() tea.Msg {
		return viewModeChangeMsg{mode: m.viewMode}
	}
}

func (m EnhancedScriptTUIModel) startOCRRefresh() tea.Cmd {
	return func() tea.Msg {
		// Take screenshot and process OCR using existing infrastructure
		if m.client == nil {
			return ocrRefreshMsg{result: nil, error: fmt.Errorf("no QMP client available")}
		}

		// Take temporary screenshot
		tmpFile, err := TakeTemporaryScreenshot(m.client, "qmp-tui-ocr")
		if err != nil {
			return ocrRefreshMsg{result: nil, error: err}
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Process with OCR using configured dimensions
		trainingDataPath := m.trainingData
		if trainingDataPath == "" {
			trainingDataPath = scriptOCRConfig.TrainingDataPath
		}

		result, err := ocr.ProcessScreenshotWithTrainingData(
			tmpFile.Name(),
			trainingDataPath,
			m.columns,
			m.rows,
		)

		return ocrRefreshMsg{result: result, error: err}
	}
}

func (m EnhancedScriptTUIModel) refreshOCR() tea.Cmd {
	// Manual refresh uses the same logic as auto-refresh
	return m.startOCRRefresh()
}

func (m EnhancedScriptTUIModel) findOCRMatches(query string) []ocr.SearchResult {
	if m.ocrState.CurrentOCR == nil {
		return nil
	}

	// Use the existing OCR search functionality
	searchConfig := ocr.SearchConfig{
		IgnoreCase:  false,
		FirstOnly:   false,
		Quiet:       true,
		LineNumbers: false,
	}

	searchResults := ocr.FindString(m.ocrState.CurrentOCR, query, searchConfig)
	return searchResults.Matches
}

func (m EnhancedScriptTUIModel) updateKeyHelp() {
	switch m.viewMode {
	case ViewModeOCR:
		m.keyHelp = []string{
			"[O] Script View",
			"[R] Refresh",
			"[/] Search",
			"[G] Grid Toggle",
			"[D] Diff Toggle",
			"[A] Auto-refresh",
			"[C] Screenshot",
			"[Q] Quit",
		}
	case ViewModeSplit:
		m.keyHelp = []string{
			"[O] Script View",
			"[R] Refresh OCR",
			"[Space] Pause/Resume",
			"[S] Skip",
			"[Q] Quit",
		}
	default:
		m.keyHelp = []string{
			"[Space] Pause/Resume",
			"[S] Skip Line",
			"[O] OCR View",
			"[R] Refresh OCR",
			"[C] Screenshot",
			"[Q] Quit",
		}
	}
}

// Placeholder implementations for additional features
func (m EnhancedScriptTUIModel) startOCRSearch() (tea.Model, tea.Cmd) {
	// This would open a search input dialog
	return m, nil
}

func (m EnhancedScriptTUIModel) takeScreenshot() tea.Cmd {
	return func() tea.Msg {
		filename := fmt.Sprintf("qmp-screenshot-%d.png", time.Now().Unix())

		if m.client == nil {
			return screenshotTakenMsg{filename: filename, error: fmt.Errorf("no QMP client available")}
		}

		// Use the centralized screenshot helper
		err := TakeScreenshot(m.client, filename, "png")
		return screenshotTakenMsg{filename: filename, error: err}
	}
}

func (m EnhancedScriptTUIModel) toggleOCRGrid() (tea.Model, tea.Cmd) {
	m.ocrState.ShowGrid = !m.ocrState.ShowGrid
	return m, nil
}

func (m EnhancedScriptTUIModel) toggleOCRDiff() (tea.Model, tea.Cmd) {
	m.ocrState.ShowDiff = !m.ocrState.ShowDiff
	return m, nil
}

func (m EnhancedScriptTUIModel) toggleAutoRefresh() (tea.Model, tea.Cmd) {
	m.ocrState.AutoRefresh = !m.ocrState.AutoRefresh
	if m.ocrState.AutoRefresh {
		return m, m.startOCRRefresh()
	}
	return m, nil
}

func (m EnhancedScriptTUIModel) navigateOCRGrid(direction string) (tea.Model, tea.Cmd) {
	if m.ocrState.CurrentOCR == nil {
		return m, nil
	}

	maxWidth := 0
	for _, line := range m.ocrState.CurrentOCR.Text {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	switch direction {
	case "up":
		if m.ocrState.HighlightRow > 0 {
			m.ocrState.HighlightRow--
		}
	case "down":
		if m.ocrState.HighlightRow < len(m.ocrState.CurrentOCR.Text)-1 {
			m.ocrState.HighlightRow++
		}
	case "left":
		if m.ocrState.HighlightCol > 0 {
			m.ocrState.HighlightCol--
		}
	case "right":
		if m.ocrState.HighlightCol < maxWidth-1 {
			m.ocrState.HighlightCol++
		}
	}

	return m, nil
}

func (m EnhancedScriptTUIModel) renderEnhancedScriptView() string {
	// Enhance the original script view with OCR preview
	originalView := m.ScriptTUIModel.View()

	// Add OCR status to the original view
	if m.ocrState.CurrentOCR != nil {
		ocrStatus := fmt.Sprintf("\nOCR Status: %dx%d grid, updated %s",
			m.ocrState.CurrentOCR.Width,
			m.ocrState.CurrentOCR.Height,
			m.ocrState.LastUpdate.Format("15:04:05"))
		originalView += styles.MutedStyle.Render(ocrStatus)
	}

	return originalView
}

func (m EnhancedScriptTUIModel) renderSplitView() string {
	// Split view implementation would go here
	// For now, return the script view
	return m.renderEnhancedScriptView()
}
