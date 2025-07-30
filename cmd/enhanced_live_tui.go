package cmd

import (
	"context"
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

// EnhancedLiveTUIModel represents the enhanced Bubble Tea model for live mode with OCR
type EnhancedLiveTUIModel struct {
	vmid         string
	client       *qmp.Client
	history      []string
	startTime    time.Time
	width        int
	height       int
	quitting     bool

	// OCR state
	ocrResult        []string
	lastOCRUpdate    time.Time
	ocrError         error
	trainingDataPath string
	ocrWidth         int
	ocrHeight        int
	autoRefresh      bool
	refreshRate      time.Duration
	ocrRetryCount    int
	maxOCRRetries    int

	// UI state
	showOCR          bool
	lastTypedKey     string
	keyTimestamp     time.Time
}

// Enhanced message types
type EnhancedTickMsg time.Time
type OCRRefreshMsg struct {
	result []string
	error  error
	retryAttempt int
}
type KeySendResult struct {
	key     string
	success bool
	error   error
	timestamp string
}
type OCRRetryMsg struct {
	retryDelay time.Duration
}

// NewEnhancedLiveTUIModel creates a new enhanced Bubble Tea model for live mode with OCR
func NewEnhancedLiveTUIModel(vmid string, client *qmp.Client, trainingDataPath string, width, height int) EnhancedLiveTUIModel {
	return EnhancedLiveTUIModel{
		vmid:             vmid,
		client:           client,
		history:          make([]string, 0),
		startTime:        time.Now(),
		quitting:         false,
		trainingDataPath: trainingDataPath,
		ocrWidth:         width,
		ocrHeight:        height,
		autoRefresh:      true,
		refreshRate:      2 * time.Second,
		showOCR:          true,
		ocrRetryCount:    0,
		maxOCRRetries:    3,
	}
}

// Enhanced styles for the OCR live TUI (additional styles beyond the shared ones in live_tui.go)
var (
	ocrPanelStyle     = styles.BoxStyle.BorderForeground(lipgloss.Color(styles.Primary)).Padding(0, 1)
	typingPanelStyle  = styles.BoxStyle.BorderForeground(lipgloss.Color(styles.Success)).Padding(0, 1)
	ocrLineStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Text))
	ocrErrorStyle     = styles.ErrorStyle
	lastKeyStyle      = styles.BoldStyle.Foreground(lipgloss.Color(styles.Warning))
	autoRefreshStyle  = styles.MutedStyle
)

// Init initializes the enhanced model
func (m EnhancedLiveTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.tickCmd(),
		m.refreshOCR(), // Initial OCR refresh
	)
}

// tickCmd returns a command that sends a tick message every second
func (m EnhancedLiveTUIModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return EnhancedTickMsg(t)
	})
}

// refreshOCR performs OCR on current screen with robust error handling and retry logic
func (m EnhancedLiveTUIModel) refreshOCR() tea.Cmd {
	return m.refreshOCRWithRetry(m.ocrRetryCount)
}

// refreshOCRWithRetry performs OCR with specific retry attempt tracking
func (m EnhancedLiveTUIModel) refreshOCRWithRetry(retryAttempt int) tea.Cmd {
	return func() tea.Msg {
		if m.trainingDataPath == "" {
			return OCRRefreshMsg{
				result: []string{"OCR disabled - no training data provided"},
				error:  fmt.Errorf("no training data"),
				retryAttempt: retryAttempt,
			}
		}

		// Take screenshot using centralized helper with error recovery
		tmpFile, err := TakeTemporaryScreenshot(m.client, "qmp-live-ocr")
		if err != nil {
			// Provide more detailed error information for debugging
			errorMsg := fmt.Sprintf("screenshot failed: %v", err)

			// Check for specific QMP/JSON errors
			if strings.Contains(err.Error(), "invalid character") {
				errorMsg = "QMP connection error - invalid response format"
			} else if strings.Contains(err.Error(), "connection") {
				errorMsg = "QMP connection lost. Check VM status"
			}

			return OCRRefreshMsg{
				result: []string{"Error: " + errorMsg, "", "Auto-retrying..."},
				error:  fmt.Errorf(errorMsg),
				retryAttempt: retryAttempt,
			}
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Process with OCR using the provided training data
		result, err := ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), m.trainingDataPath, m.ocrWidth, m.ocrHeight)
		if err != nil {
			return OCRRefreshMsg{
				result: []string{"OCR processing failed: " + err.Error(), "", "Auto-retrying..."},
				error:  fmt.Errorf("OCR failed: %v", err),
				retryAttempt: retryAttempt,
			}
		}

		return OCRRefreshMsg{
			result: result.Text,
			error:  nil,
			retryAttempt: retryAttempt,
		}
	}
}

// autoRefreshCmd returns a command for automatic OCR refresh
func (m EnhancedLiveTUIModel) autoRefreshCmd() tea.Cmd {
	if !m.autoRefresh {
		return nil
	}
	return tea.Tick(m.refreshRate, func(t time.Time) tea.Msg {
		return "auto_refresh_ocr"
	})
}

// Update handles messages and updates the enhanced model
func (m EnhancedLiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+^", "ctrl+6":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+r":
			// Manual OCR refresh - reset retry counter and re-enable auto-refresh
			m.ocrRetryCount = 0
			m.autoRefresh = true
			return m, m.refreshOCR()
		case "ctrl+a":
			// Toggle auto-refresh
			m.autoRefresh = !m.autoRefresh
			if m.autoRefresh {
				return m, m.autoRefreshCmd()
			}
			return m, nil
		case "ctrl+o":
			// Toggle OCR display
			m.showOCR = !m.showOCR
			return m, nil
		case "ctrl+c":
			// Handle Ctrl+C gracefully - send to VM but don't quit TUI
			return m, m.sendKeyAsync("ctrl+c")
		default:
			// Send the key to the VM asynchronously to prevent blocking
			key := msg.String()

			// Track last typed key for display immediately
			m.lastTypedKey = key
			m.keyTimestamp = time.Now()

			// Send key asynchronously
			return m, m.sendKeyAsync(key)
		}

	case EnhancedTickMsg:
		// Continue ticking and handle auto-refresh
		var cmds []tea.Cmd
		cmds = append(cmds, m.tickCmd())
		if m.autoRefresh {
			cmds = append(cmds, m.autoRefreshCmd())
		}
		return m, tea.Batch(cmds...)

	case OCRRefreshMsg:
		if msg.error != nil {
			// Handle OCR errors with retry logic
			if msg.retryAttempt < m.maxOCRRetries {
				// Still have retries left - increment count and retry after delay
				m.ocrRetryCount = msg.retryAttempt + 1

				// Show retry status in OCR panel
				retryMsg := fmt.Sprintf("OCR Error (attempt %d/%d) - retrying in 2s...", m.ocrRetryCount, m.maxOCRRetries)
				m.ocrResult = []string{retryMsg, "", "Error: " + msg.error.Error()}
				m.ocrError = nil // Don't show error state during retries

				// Schedule retry after 2 second delay
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return OCRRetryMsg{retryDelay: 2 * time.Second}
				})
			} else {
				// Max retries reached - show final error
				m.ocrRetryCount = 0 // Reset for next manual retry
				m.ocrResult = msg.result
				m.ocrError = msg.error
				m.lastOCRUpdate = time.Now()

				// Disable auto-refresh after max retries to prevent spam
				m.autoRefresh = false
			}
		} else {
			// Success - reset retry count and update normally
			m.ocrRetryCount = 0
			m.ocrResult = msg.result
			m.ocrError = msg.error
			m.lastOCRUpdate = time.Now()
		}

		return m, nil

	case KeySendResult:
		// Handle the result of an asynchronous key send operation
		result := msg

		// Add to history with the result
		if result.success {
			m.history = append(m.history, fmt.Sprintf("[%s] ✓ %s", result.timestamp, result.key))
		} else {
			// Provide more informative error messages
			errorStr := result.error.Error()
			if strings.Contains(errorStr, "invalid character") {
				errorStr = "QMP connection error"
			} else if strings.Contains(errorStr, "timeout") {
				errorStr = "operation timed out"
			} else if len(errorStr) > 30 {
				errorStr = errorStr[:30] + "..."
			}
			m.history = append(m.history, fmt.Sprintf("[%s] ✗ %s (%s)", result.timestamp, result.key, errorStr))
		}

		// Keep only last 5 entries since we only show 2 max
		if len(m.history) > 5 {
			m.history = m.history[1:]
		}

		// If this was a significant key and we don't have OCR errors, refresh OCR after a short delay
		var cmd tea.Cmd
		if m.shouldRefreshAfterKey(result.key) && m.ocrError == nil && m.autoRefresh && result.success {
			cmd = tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return "delayed_refresh_ocr"
			})
		}

		return m, cmd

	case OCRRetryMsg:
		// Handle OCR retry after delay
		return m, m.refreshOCRWithRetry(m.ocrRetryCount)

	case string:
		if msg == "auto_refresh_ocr" || msg == "delayed_refresh_ocr" {
			var cmds []tea.Cmd
			cmds = append(cmds, m.refreshOCR())
			if msg == "auto_refresh_ocr" && m.autoRefresh {
				cmds = append(cmds, m.autoRefreshCmd())
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil

	default:
		return m, nil
	}
}

// shouldRefreshAfterKey determines if OCR should refresh after typing a key
func (m EnhancedLiveTUIModel) shouldRefreshAfterKey(key string) bool {
	significantKeys := map[string]bool{
		"ret":       true, // Enter
		"enter":     true,
		"space":     true,
		"tab":       true,
		"backspace": true,
		"delete":    true,
		"up":        true,
		"down":      true,
		"left":      true,
		"right":     true,
	}

	// Refresh after significant keys or if it's been a while since last refresh
	return significantKeys[key] || time.Since(m.lastOCRUpdate) > 5*time.Second
}

// sendKeyAsync sends a key to the VM asynchronously with timeout protection
func (m EnhancedLiveTUIModel) sendKeyAsync(key string) tea.Cmd {
	return func() tea.Msg {
		timestamp := time.Now().Format("15:04:05")

		// Create a context with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Channel to receive the result
		resultChan := make(chan KeySendResult, 1)

		// Send the key in a goroutine
		go func() {
			defer close(resultChan)
			err := m.client.SendKey(key)

			select {
			case resultChan <- KeySendResult{
				key:       key,
				success:   err == nil,
				error:     err,
				timestamp: timestamp,
			}:
			case <-ctx.Done():
				// Context cancelled, don't send anything
			}
		}()

		// Wait for result or timeout
		select {
		case result := <-resultChan:
			return result
		case <-ctx.Done():
			return KeySendResult{
				key:       key,
				success:   false,
				error:     fmt.Errorf("operation timed out after 5 seconds"),
				timestamp: timestamp,
			}
		}
	}
}

// View renders the enhanced TUI with OCR display
func (m EnhancedLiveTUIModel) View() string {
	if m.quitting {
		return exitStyle.Render("Exited enhanced live keyboard mode\n")
	}

	var s strings.Builder

	// Calculate available space
	availableHeight := m.height - 6 // Reserve space for header, status, and controls
	typingPanelHeight := 5 // Fixed small size for typing panel (1-3 lines of content + borders)
	ocrHeight := availableHeight - typingPanelHeight // Give most space to OCR

	// Title with OCR status
	ocrStatus := "OFF"
	if m.showOCR {
		if m.ocrError != nil {
			ocrStatus = "ERROR"
		} else {
			ocrStatus = "ON"
		}
	}

	title := titleStyle.Render(fmt.Sprintf(" Enhanced Live Mode - VM %s | OCR: %s ", m.vmid, ocrStatus))
	s.WriteString(title + "\n")

	// Status line with controls
	uptime := time.Since(m.startTime).Truncate(time.Second)
	autoStatus := ""
	if m.autoRefresh {
		autoStatus = autoRefreshStyle.Render(" [AUTO-REFRESH ON]")
	}

	lastUpdate := ""
	if !m.lastOCRUpdate.IsZero() {
		lastUpdate = fmt.Sprintf(" | Last OCR: %s", m.lastOCRUpdate.Format("15:04:05"))
	}

	statusLine := statusStyle.Render(fmt.Sprintf("Uptime: %v%s%s", uptime, lastUpdate, autoStatus))
	s.WriteString(statusLine + "\n\n")

	if m.showOCR {
		// OCR Panel
		s.WriteString("🖥️  Screen Content (OCR):\n")

		if m.ocrError != nil {
			// Display error with helpful instructions
			var errorLines []string
			errorLines = append(errorLines, ocrErrorStyle.Render("⚠️  OCR Error Occurred"))
			errorLines = append(errorLines, "")

			// Show OCR result if available (might contain error message with instructions)
			if len(m.ocrResult) > 0 {
				for _, line := range m.ocrResult {
					errorLines = append(errorLines, autoRefreshStyle.Render(line))
				}
			} else {
				errorLines = append(errorLines, autoRefreshStyle.Render(fmt.Sprintf("Error: %v", m.ocrError)))
			}

			errorLines = append(errorLines, "")
			errorLines = append(errorLines, styles.MutedStyle.Render("💡 Recovery options:"))
			errorLines = append(errorLines, styles.MutedStyle.Render("   • Press Ctrl+R to retry OCR (resets retry counter)"))
			errorLines = append(errorLines, styles.MutedStyle.Render("   • Press Ctrl+O to disable OCR"))
			errorLines = append(errorLines, styles.MutedStyle.Render("   • Press Ctrl+A to toggle auto-refresh"))
			if m.ocrRetryCount > 0 {
				errorLines = append(errorLines, "")
				errorLines = append(errorLines, styles.MutedStyle.Render(fmt.Sprintf("Auto-retry: %d/%d attempts made", m.ocrRetryCount, m.maxOCRRetries)))
			}

			errorContent := strings.Join(errorLines, "\n")
			ocrPanel := ocrPanelStyle.Width(m.width - 4).Height(ocrHeight - 2).Render(errorContent)
			s.WriteString(ocrPanel + "\n")
		} else if len(m.ocrResult) == 0 {
			loadingContent := autoRefreshStyle.Render("Loading OCR data...")
			ocrPanel := ocrPanelStyle.Width(m.width - 4).Height(ocrHeight - 2).Render(loadingContent)
			s.WriteString(ocrPanel + "\n")
		} else {
			// Display OCR results - show bottom lines when text overflows
			var ocrLines []string
			maxLines := ocrHeight - 4 // Account for border and padding

			// Show the bottom/most recent OCR lines that fit (no word wrap)
			start := 0
			if len(m.ocrResult) > maxLines {
				start = len(m.ocrResult) - maxLines
			}

			for i := start; i < len(m.ocrResult); i++ {
				line := m.ocrResult[i]
				// No word wrapping - just truncate cleanly if too long
				maxWidth := m.width - 12 // Account for borders, padding, and line numbers
				if len(line) > maxWidth {
					line = line[:maxWidth]
				}

				lineNum := fmt.Sprintf("%2d", i+1)
				ocrLines = append(ocrLines, fmt.Sprintf("%s │ %s",
					styles.MutedStyle.Render(lineNum),
					ocrLineStyle.Render(line)))
			}

			ocrContent := strings.Join(ocrLines, "\n")
			ocrPanel := ocrPanelStyle.Width(m.width - 4).Height(ocrHeight - 2).Render(ocrContent)
			s.WriteString(ocrPanel + "\n")
		}
	} else {
		// Show placeholder when OCR is disabled
		placeholder := autoRefreshStyle.Render("OCR display disabled. Press Ctrl+O to enable.")
		ocrPanel := ocrPanelStyle.Width(m.width - 4).Height(ocrHeight - 2).Render(placeholder)
		s.WriteString(ocrPanel + "\n")
	}

	// Compact Typing Panel (just 1-3 lines max)
	s.WriteString("⌨️  Live Typing:\n")

	var typingLines []string

	// Show just the most recent key if recent
	if m.lastTypedKey != "" && time.Since(m.keyTimestamp) < 3*time.Second {
		lastKeyInfo := fmt.Sprintf("Last key: %s", lastKeyStyle.Render(m.lastTypedKey))
		typingLines = append(typingLines, lastKeyInfo)
	} else if len(m.history) == 0 {
		typingLines = append(typingLines, "No keys sent yet... Start typing!")
	} else {
		// Show only the most recent 2 history entries
		start := len(m.history) - 2
		if start < 0 {
			start = 0
		}

		for i := start; i < len(m.history); i++ {
			entry := m.history[i]
			// Truncate long entries to fit in compact space
			maxEntryWidth := m.width - 12
			if len(entry) > maxEntryWidth {
				entry = entry[:maxEntryWidth-3] + "..."
			}

			if i == len(m.history)-1 {
				// Most recent entry
				typingLines = append(typingLines, recentKeyStyle.Render("→ "+entry))
			} else {
				// Previous entry
				typingLines = append(typingLines, oldKeyStyle.Render("  "+entry))
			}
		}
	}

	// Ensure we have at least 1 line but no more than 3
	if len(typingLines) == 0 {
		typingLines = append(typingLines, "Ready for input...")
	}
	for len(typingLines) < 3 {
		typingLines = append(typingLines, "") // Add padding
	}

	typingContent := strings.Join(typingLines, "\n")
	typingPanel := typingPanelStyle.Width(m.width - 4).Height(typingPanelHeight - 2).Render(typingContent)
	s.WriteString(typingPanel + "\n")

	// Control instructions
	controls := styles.MutedStyle.Render("Controls: Ctrl+^ (exit) | Ctrl+R (refresh OCR) | Ctrl+A (toggle auto-refresh) | Ctrl+O (toggle OCR display)")
	s.WriteString(controls)

	return s.String()
}
