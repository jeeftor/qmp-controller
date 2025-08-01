package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"github.com/jeeftor/qmp-controller/internal/tui"
)

// LiveTUIModel represents the Bubble Tea model for live mode
type LiveTUIModel struct {
	*tui.BaseTUIModel
	history     []string
	keyHandler  *tui.KeyHandler
	renderer    *tui.TUIRenderer
	logManager  *tui.LogManager
}

// KeySentMsg is sent when a key is successfully sent to the VM
type KeySentMsg struct {
	Key     string
	Success bool
}

// NewLiveTUIModel creates a new Bubble Tea model for live mode
func NewLiveTUIModel(vmid string, client *qmp.Client) LiveTUIModel {
	baseTUI := tui.NewBaseTUIModel(vmid, client)

	return LiveTUIModel{
		BaseTUIModel: baseTUI,
		history:      make([]string, 0),
		keyHandler:   tui.NewKeyHandler(),
		renderer:     tui.NewTUIRenderer(80, 24),
		logManager:   tui.NewLogManager(50),
	}
}

// Use centralized styles from styles package
var (
	titleStyle     = styles.TitleStyle
	exitStyle      = styles.ErrorStyle
	statusStyle    = styles.SuccessStyle
	historyStyle   = styles.BoxStyle.BorderForeground(lipgloss.Color(styles.TextMuted)).Padding(1, 2)
	recentKeyStyle = styles.BoldStyle
	oldKeyStyle    = styles.MutedStyle
)

// Init initializes the model
func (m LiveTUIModel) Init() tea.Cmd {
	return m.CommonInit()
}

// Update handles messages and updates the model
func (m LiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.HandleWindowResize(msg)
		m.renderer.UpdateDimensions(msg.Width, msg.Height)
		return m, nil

	case tui.TickMsg:
		return m, m.TickCmd()

	case tea.KeyMsg:
		// Handle common keys first
		if action, handled := m.keyHandler.HandleCommonKeys(msg); handled {
			switch action {
			case tui.KeyActionQuit:
				return m, tea.Quit
			case tui.KeyActionRefresh:
				// Add refresh logic here if needed
				return m, nil
			}
		}

		// Handle live-mode specific keys
		switch msg.String() {
		case "ctrl+^", "ctrl+6":
			return m, tea.Quit
		default:
			// Send the key to the VM
			key := msg.String()
			err := m.State.Client.SendKey(key)
			success := err == nil

			// Add to history using log manager
			if success {
				m.logManager.Add(fmt.Sprintf("✓ %s", key), tui.LogLevelSuccess)
				m.history = append(m.history, fmt.Sprintf("✓ %s", key))
			} else {
				m.logManager.Add(fmt.Sprintf("✗ %s (error: %v)", key, err), tui.LogLevelError)
				m.history = append(m.history, fmt.Sprintf("✗ %s (error: %v)", key, err))
			}

			// Keep only last 20 entries
			if len(m.history) > 20 {
				m.history = m.history[1:]
			}

			return m, nil
		}

	default:
		return m, nil
	}
}

// View renders the TUI
func (m LiveTUIModel) View() string {
	if m.IsQuitting() {
		return exitStyle.Render("Exited live keyboard mode\n")
	}

	var s strings.Builder

	// Title using renderer
	title := m.renderer.RenderTitle(fmt.Sprintf("QMP Live Mode - VM %s", m.State.VMID))
	s.WriteString(title + "\n\n")

	// Exit instructions
	exitMsg := exitStyle.Render("Press Ctrl+^ (Ctrl+6) to exit")
	s.WriteString(exitMsg + "\n")

	// Status using renderer
	statusInfo := m.GetCommonStatus("Live Mode", "Connected", "", time.Time{})
	statusLine := m.renderer.RenderStatus(statusInfo)
	s.WriteString(statusLine + "\n\n")

	// History
	s.WriteString("Command History:\n")

	if len(m.history) == 0 {
		historyContent := "No commands sent yet...\nStart typing to send keys to the VM!"
		width, _ := m.GetDimensions()
		historyBox := historyStyle.Width(width - 4).Render(historyContent)
		s.WriteString(historyBox)
	} else {
		// Show recent history using renderer
		width, height := m.GetDimensions()
		maxLines := height - 10 // Reserve space for header and footer
		if maxLines < 5 {
			maxLines = 5
		}

		recentHistory := m.history
		if len(m.history) > maxLines {
			recentHistory = m.history[len(m.history)-maxLines:]
		}

		historyLines := m.renderer.RenderList(recentHistory, false, -1)
		historyContent := strings.Join(historyLines, "\n")
		historyBox := historyStyle.Width(width - 4).Render(historyContent)
		s.WriteString(historyBox)
	}

	s.WriteString("\n\nWaiting for input... (live mode active)")

	return s.String()
}
