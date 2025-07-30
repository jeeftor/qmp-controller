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

// OCRWatchTUIModel represents the Bubble Tea model for OCR watch mode
type OCRWatchTUIModel struct {
	client       *qmp.Client
	vmid         string
	config       *ocr.OCRConfig
	outputFile   string

	// Watch state
	baselineText     []string
	currentText      []string
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
	text    []string
	error   error
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
		text, err := m.captureOCR()
		return OCRWatchRefreshMsg{
			text:  text,
			error: err,
		}
	}
}

// performOCRRefresh performs an OCR refresh
func (m OCRWatchTUIModel) performOCRRefresh() tea.Cmd {
	return func() tea.Msg {
		text, err := m.captureOCR()
		return OCRWatchRefreshMsg{
			text:  text,
			error: err,
		}
	}
}

// captureOCR captures OCR from the VM
func (m OCRWatchTUIModel) captureOCR() ([]string, error) {
	// Take screenshot
	tmpFile, err := TakeTemporaryScreenshot(m.client, "qmp-ocr-watch-tui")
	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %v", err)
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
		return nil, fmt.Errorf("OCR processing failed: %v", err)
	}

	return result.Text, nil
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

		if msg.error != nil {
			return m, nil
		}

		// Handle the OCR result based on state
		if m.baselineText == nil {
			// Initial capture
			m.baselineText = make([]string, len(msg.text))
			copy(m.baselineText, msg.text)
			m.currentText = make([]string, len(msg.text))
			copy(m.currentText, msg.text)
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
			newChanges := m.detectTextChanges(baselineForComparison, msg.text)
			if len(newChanges) > 0 {
				m.changes = newChanges
				m.changesDetected += len(newChanges)
			}

			// Update current text
			m.currentText = make([]string, len(msg.text))
			copy(m.currentText, msg.text)
		}

		return m, nil

	default:
		return m, nil
	}
}

// detectTextChanges compares two text arrays and returns positions of changes
func (m OCRWatchTUIModel) detectTextChanges(baseline []string, current []string) map[int]map[int]bool {
	changes := make(map[int]map[int]bool)

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

// renderOCRContent renders the OCR content with change highlighting
func (m OCRWatchTUIModel) renderOCRContent() []string {
	var lines []string

	text := m.currentText
	if m.config.FilterBlankLines {
		text = m.removeBlankLines(text)
	}

	for lineIdx, line := range text {
		var renderedLine strings.Builder

		// Add line number if enabled
		if m.config.ShowLineNumbers {
			lineNum := lineNumberStyle.Render(fmt.Sprintf("%3d", lineIdx))
			renderedLine.WriteString(lineNum)
			renderedLine.WriteString("│ ")
		}

		// Render line with change highlighting
		if m.changes != nil && m.changes[lineIdx] != nil {
			// Line has changes - highlight individual characters
			for charIdx, char := range line {
				if m.changes[lineIdx][charIdx] {
					// Highlight changed character with background color
					renderedLine.WriteString(changedCharStyle.Render(string(char)))
				} else {
					renderedLine.WriteString(string(char))
				}
			}
		} else {
			// No changes in this line
			renderedLine.WriteString(line)
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
