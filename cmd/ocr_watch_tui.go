package cmd

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/styles"
)

// OCRWatchTUIModel represents the Bubble Tea model for OCR watch mode
type OCRWatchTUIModel struct {
	client       *qmp.Client
	vmid         string
	config       *ocr.OCRConfig
	outputFile   string

	// Watch state
	baselineText     []string
	currentText      []string
	currentResult    *ocr.OCRResult  // Store full OCR result for color mode
	changes          map[int]map[int]bool
	lastUserAction   time.Time
	stateVersion     int

	// TUI state
	width            int
	height           int
	quitting         bool
	refreshInterval  time.Duration
	lastRefresh      time.Time

	// Status
	totalRefreshes   int
	changesDetected  int
}

// OCR Watch TUI message types
type OCRWatchTickMsg time.Time
type OCRWatchRefreshMsg struct {
	result  *OCRCaptureResult
}
type UserInputMsg struct{}

// NewOCRWatchTUIModel creates a new OCR watch TUI model
func NewOCRWatchTUIModel(client *qmp.Client, vmid string, config *ocr.OCRConfig, outputFile string, interval int) OCRWatchTUIModel {
	return OCRWatchTUIModel{
		client:          client,
		vmid:            vmid,
		config:          config,
		outputFile:      outputFile,
		refreshInterval: time.Duration(interval) * time.Second,
		stateVersion:    1,
		lastUserAction:  time.Now(),
		changes:         make(map[int]map[int]bool),
	}
}

// OCR Watch TUI styles
var (
	watchTitleStyle = styles.TitleStyle
	watchStatusStyle = styles.SuccessStyle
	watchErrorStyle = styles.ErrorStyle
	ocrContentStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(styles.Primary)).
		Padding(1, 2)
	changedCharStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(styles.Warning)).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)
	lineNumberStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(styles.TextMuted)).
		Width(4).
		Align(lipgloss.Right)
	controlsStyle = styles.MutedStyle
)

// Init initializes the OCR watch TUI model
func (m OCRWatchTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.tickCmd(),
		m.performInitialOCR(),
	)
}

// tickCmd returns a command that sends tick messages
func (m OCRWatchTUIModel) tickCmd() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return OCRWatchTickMsg(t)
	})
}

// performInitialOCR performs the initial OCR capture
func (m OCRWatchTUIModel) performInitialOCR() tea.Cmd {
	return func() tea.Msg {
		result, _ := m.captureOCR()
		return OCRWatchRefreshMsg{
			result: result,
		}
	}
}

// performOCRRefresh performs an OCR refresh
func (m OCRWatchTUIModel) performOCRRefresh() tea.Cmd {
	return func() tea.Msg {
		result, _ := m.captureOCR()
		return OCRWatchRefreshMsg{
			result: result,
		}
	}
}

// OCRCaptureResult holds both text and bitmap data for color mode support
type OCRCaptureResult struct {
	Text    []string
	Result  *ocr.OCRResult
	Error   error
}

// captureOCR captures OCR from the VM and returns both text and full result for color support
func (m OCRWatchTUIModel) captureOCR() (*OCRCaptureResult, error) {
	// Take screenshot
	tmpFile, err := TakeTemporaryScreenshot(m.client, "qmp-ocr-watch-tui")
	if err != nil {
		return &OCRCaptureResult{Error: fmt.Errorf("screenshot failed: %v", err)}, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Process OCR
	var result *ocr.OCRResult
	if m.config.CropEnabled {
		result, err = ocr.ProcessScreenshotWithCropAndTrainingData(
			tmpFile.Name(),
			m.config.TrainingDataPath,
			m.config.Columns,
			m.config.Rows,
			m.config.CropStartRow,
			m.config.CropEndRow,
			m.config.CropStartCol,
			m.config.CropEndCol,
		)
	} else {
		result, err = ocr.ProcessScreenshotWithTrainingData(
			tmpFile.Name(),
			m.config.TrainingDataPath,
			m.config.Columns,
			m.config.Rows,
		)
	}

	if err != nil {
		return &OCRCaptureResult{Error: fmt.Errorf("OCR processing failed: %v", err)}, err
	}

	return &OCRCaptureResult{Text: result.Text, Result: result}, nil
}

// Update handles messages and updates the model
func (m OCRWatchTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "r":
			// Manual refresh
			return m, m.performOCRRefresh()
		default:
			// Any other key simulates user input
			return m, func() tea.Msg { return UserInputMsg{} }
		}

	case UserInputMsg:
		// User input detected - advance state
		m.lastUserAction = time.Now()
		if m.stateVersion == 1 {
			m.stateVersion = 2
		} else {
			// Move current to baseline, advance state
			if len(m.currentText) > 0 {
				m.baselineText = make([]string, len(m.currentText))
				copy(m.baselineText, m.currentText)
			}
			m.stateVersion++
		}
		return m, nil

	case OCRWatchTickMsg:
		// Regular refresh
		return m, tea.Batch(
			m.tickCmd(),
			m.performOCRRefresh(),
		)

	case OCRWatchRefreshMsg:
		m.totalRefreshes++
		m.lastRefresh = time.Now()

		if msg.result.Error != nil {
			return m, nil
		}

		// Store the full OCR result for color mode support
		m.currentResult = msg.result.Result

		// Handle the OCR result based on state
		if m.baselineText == nil {
			// Initial capture
			m.baselineText = make([]string, len(msg.result.Text))
			copy(m.baselineText, msg.result.Text)
			m.currentText = make([]string, len(msg.result.Text))
			copy(m.currentText, msg.result.Text)
			m.changes = make(map[int]map[int]bool)
		} else {
			// Regular capture - detect changes
			var baselineForComparison []string

			// Determine what to compare against based on state logic
			if m.stateVersion == 2 && time.Since(m.lastUserAction) > m.refreshInterval {
				// State 2 - compare against original baseline (State 1)
				baselineForComparison = m.baselineText
			} else if len(m.currentText) > 0 {
				// State 3+ - compare against current stable state
				baselineForComparison = m.currentText
			} else {
				baselineForComparison = m.baselineText
			}

			// Detect changes
			newChanges := m.detectTextChanges(baselineForComparison, msg.result.Text)
			if len(newChanges) > 0 {
				m.changes = newChanges
				m.changesDetected += len(newChanges)
			}

			// Update current text
			m.currentText = make([]string, len(msg.result.Text))
			copy(m.currentText, msg.result.Text)
		}

		return m, nil

	default:
		return m, nil
	}
}

// detectTextChanges compares two text arrays with special handling for console scrolling
// In a console, new text appears at the bottom and old text scrolls off the top
func (m OCRWatchTUIModel) detectTextChanges(baseline []string, current []string) map[int]map[int]bool {
	changes := make(map[int]map[int]bool)

	// Handle console scrolling by comparing bottom-up first
	if m.isConsoleScrolling(baseline, current) {
		// Focus on the bottom part of the screen where new content appears
		return m.detectScrollingChanges(baseline, current)
	}

	// Standard line-by-line comparison for non-scrolling scenarios
	maxLines := len(baseline)
	if len(current) > maxLines {
		maxLines = len(current)
	}

	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		var baselineLine, currentLine string

		if lineIdx < len(baseline) {
			baselineLine = baseline[lineIdx]
		}
		if lineIdx < len(current) {
			currentLine = current[lineIdx]
		}

		if baselineLine != currentLine {
			if changes[lineIdx] == nil {
				changes[lineIdx] = make(map[int]bool)
			}

			// Mark character-level changes
			maxChars := len(baselineLine)
			if len(currentLine) > maxChars {
				maxChars = len(currentLine)
			}

			for charIdx := 0; charIdx < maxChars; charIdx++ {
				var baselineChar, currentChar rune

				if charIdx < len(baselineLine) {
					baselineChar = rune(baselineLine[charIdx])
				}
				if charIdx < len(currentLine) {
					currentChar = rune(currentLine[charIdx])
				}

				if baselineChar != currentChar {
					changes[lineIdx][charIdx] = true
				}
			}
		}
	}

	return changes
}

// isConsoleScrolling detects if the text change represents console scrolling
// This happens when the bottom lines of baseline match the top lines of current
func (m OCRWatchTUIModel) isConsoleScrolling(baseline []string, current []string) bool {
	if len(baseline) == 0 || len(current) == 0 {
		return false
	}

	// Check if bottom half of baseline matches top half of current
	// This indicates text has scrolled up
	minLen := len(baseline)
	if len(current) < minLen {
		minLen = len(current)
	}

	if minLen < 2 {
		return false
	}

	// Compare bottom quarter of baseline with top quarter of current
	compareLines := minLen / 4
	if compareLines < 1 {
		compareLines = 1
	}

	matchingLines := 0
	for i := 0; i < compareLines; i++ {
		baselineIdx := len(baseline) - compareLines + i
		currentIdx := i

		if baselineIdx >= 0 && baselineIdx < len(baseline) && currentIdx < len(current) {
			if baseline[baselineIdx] == current[currentIdx] {
				matchingLines++
			}
		}
	}

	// If most lines match, it's likely scrolling
	return matchingLines >= compareLines/2
}

// detectScrollingChanges focuses on changes at the bottom of the screen for scrolling scenarios
func (m OCRWatchTUIModel) detectScrollingChanges(baseline []string, current []string) map[int]map[int]bool {
	changes := make(map[int]map[int]bool)

	// In scrolling scenarios, focus on the bottom 25% of the screen where new content appears
	focusLines := len(current) / 4
	if focusLines < 3 {
		focusLines = len(current) / 2 // At least half the screen
	}
	if focusLines < 1 {
		focusLines = len(current) // Fallback to all lines
	}

	// Compare the bottom part of current with corresponding part of baseline
	startLine := len(current) - focusLines
	if startLine < 0 {
		startLine = 0
	}

	for lineIdx := startLine; lineIdx < len(current); lineIdx++ {
		var baselineLine string
		currentLine := current[lineIdx]

		// Try to find corresponding line in baseline
		if lineIdx < len(baseline) {
			baselineLine = baseline[lineIdx]
		}

		if baselineLine != currentLine {
			if changes[lineIdx] == nil {
				changes[lineIdx] = make(map[int]bool)
			}

			// Mark all characters in changed lines for scrolling scenarios
			for charIdx, char := range currentLine {
				var baselineChar rune
				if charIdx < len(baselineLine) {
					baselineChar = rune(baselineLine[charIdx])
				}

				if baselineChar != char {
					changes[lineIdx][charIdx] = true
				}
			}

			// Handle case where current line is longer than baseline
			if len(currentLine) > len(baselineLine) {
				for charIdx := len(baselineLine); charIdx < len(currentLine); charIdx++ {
					changes[lineIdx][charIdx] = true
				}
			}
		}
	}

	return changes
}

// View renders the OCR watch TUI
func (m OCRWatchTUIModel) View() string {
	if m.quitting {
		return "OCR Watch Mode ended.\n"
	}

	var sections []string

	// Title
	title := watchTitleStyle.Render(fmt.Sprintf(" OCR Watch Mode - VM %s ", m.vmid))
	sections = append(sections, title)

	// Status line
	status := fmt.Sprintf("State: %d | Refreshes: %d | Changes: %d | Last: %s | Interval: %v",
		m.stateVersion,
		m.totalRefreshes,
		m.changesDetected,
		m.lastRefresh.Format("15:04:05"),
		m.refreshInterval,
	)
	sections = append(sections, watchStatusStyle.Render(status))
	sections = append(sections, "")

	// OCR content
	if len(m.currentText) > 0 {
		contentLines := m.renderOCRContent()
		content := ocrContentStyle.Width(m.width - 4).Render(strings.Join(contentLines, "\n"))
		sections = append(sections, content)
	} else {
		sections = append(sections, ocrContentStyle.Render("Waiting for initial OCR capture..."))
	}

	// Controls
	sections = append(sections, "")
	controls := controlsStyle.Render("Controls: r (manual refresh) | any key (simulate user input) | q/Ctrl+C (quit)")
	sections = append(sections, controls)

	return strings.Join(sections, "\n")
}

// getRepresentativeColor extracts the representative color from a character bitmap
func (m OCRWatchTUIModel) getRepresentativeColor(bitmap ocr.CharacterBitmap) color.Color {
	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			if bitmap.Data[y][x] { // If this pixel is "text" (not background)
				if y < len(bitmap.Colors) && x < len(bitmap.Colors[y]) {
					return bitmap.Colors[y][x]
				}
			}
		}
	}
	return nil
}

// renderOCRContent renders the OCR content with change highlighting and color mode support
func (m OCRWatchTUIModel) renderOCRContent() []string {
	var lines []string

	text := m.currentText
	if m.config.FilterBlankLines {
		text = m.removeBlankLines(text)
	}

	// Create unrecognized character style for highlighting '?' characters
	unrecognizedStyle := styles.ErrorStyle.Background(lipgloss.Color("#FFFFFF"))

	for lineIdx, line := range text {
		var renderedLine strings.Builder

		// Add line number if enabled
		if m.config.ShowLineNumbers {
			lineNum := lineNumberStyle.Render(fmt.Sprintf("%3d", lineIdx))
			renderedLine.WriteString(lineNum)
			renderedLine.WriteString("│ ")
		}

		// Process each character with color mode and change highlighting
		for charIdx, char := range line {
			var characterStyle lipgloss.Style

			// Start with the base character styling
			if m.config.ColorMode && m.currentResult != nil {
				// Color mode - apply original colors from OCR bitmaps
				if string(char) == ocr.UnknownCharIndicator {
					// Special handling for unrecognized characters
					characterStyle = unrecognizedStyle
				} else {
					// Calculate the character bitmap index (row-major order)
					bitmapIdx := lineIdx*m.currentResult.Width + charIdx

					if bitmapIdx < len(m.currentResult.CharBitmaps) {
						bitmap := m.currentResult.CharBitmaps[bitmapIdx]
						originalColor := m.getRepresentativeColor(bitmap)

						if originalColor != nil {
							// Create a color style based on the original pixel color
							r, g, b, _ := originalColor.RGBA()
							r8, g8, b8 := r>>8, g>>8, b>>8
							characterStyle = styles.CreateFgStyle(uint8(r8), uint8(g8), uint8(b8))
						} else {
							// Fallback to normal color
							characterStyle = lipgloss.NewStyle()
						}
					} else {
						characterStyle = lipgloss.NewStyle()
					}
				}
			} else {
				// No color mode - use default styling
				if string(char) == ocr.UnknownCharIndicator {
					characterStyle = unrecognizedStyle
				} else {
					characterStyle = lipgloss.NewStyle()
				}
			}

			// Apply change highlighting if the character has changed
			if m.changes != nil && m.changes[lineIdx] != nil && m.changes[lineIdx][charIdx] {
				// Character changed - add background highlighting
				// Preserve the foreground color but add background
				characterStyle = characterStyle.Background(lipgloss.Color(styles.Warning))
			}

			// Render the character with the combined styling
			renderedLine.WriteString(characterStyle.Render(string(char)))
		}

		lines = append(lines, renderedLine.String())
	}

	return lines
}

// removeBlankLines removes empty lines from text array
func (m OCRWatchTUIModel) removeBlankLines(lines []string) []string {
	var filtered []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

// runOCRWatchTUI starts the OCR watch TUI mode
func runOCRWatchTUI(client *qmp.Client, vmid string, config *ocr.OCRConfig, outputFile string) {
	model := NewOCRWatchTUIModel(client, vmid, config, outputFile, watchInterval)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running OCR Watch TUI: %v\n", err)
		os.Exit(1)
	}
}
