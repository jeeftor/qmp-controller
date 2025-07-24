package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/styles"
)

// LiveTUIModel represents the Bubble Tea model for live mode
type LiveTUIModel struct {
	vmid      string
	client    *qmp.Client
	history   []string
	startTime time.Time
	width     int
	height    int
	quitting  bool
}

// TickMsg is sent periodically to update the uptime
type TickMsg time.Time

// KeySentMsg is sent when a key is successfully sent to the VM
type KeySentMsg struct {
	Key     string
	Success bool
}

// NewLiveTUIModel creates a new Bubble Tea model for live mode
func NewLiveTUIModel(vmid string, client *qmp.Client) LiveTUIModel {
	return LiveTUIModel{
		vmid:      vmid,
		client:    client,
		history:   make([]string, 0),
		startTime: time.Now(),
		quitting:  false,
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
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
	)
}

// tickCmd returns a command that sends a tick message every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages and updates the model
func (m LiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		default:
			// Send the key to the VM
			key := msg.String()
			err := m.client.SendKey(key)
			success := err == nil

			// Add to history
			timestamp := time.Now().Format("15:04:05")
			if success {
				m.history = append(m.history, fmt.Sprintf("[%s] ✓ %s", timestamp, key))
			} else {
				m.history = append(m.history, fmt.Sprintf("[%s] ✗ %s (error: %v)", timestamp, key, err))
			}

			// Keep only last 20 entries
			if len(m.history) > 20 {
				m.history = m.history[1:]
			}

			return m, nil
		}

	case TickMsg:
		// Continue ticking
		return m, tickCmd()

	default:
		return m, nil
	}
}

// View renders the TUI
func (m LiveTUIModel) View() string {
	if m.quitting {
		return exitStyle.Render("Exited live keyboard mode\n")
	}

	var s strings.Builder

	// Title
	title := titleStyle.Render(fmt.Sprintf(" QMP Live Mode - VM %s ", m.vmid))
	s.WriteString(title + "\n\n")

	// Exit instructions
	exitMsg := exitStyle.Render("Press Ctrl+^ (Ctrl+6) to exit")
	s.WriteString(exitMsg + "\n")

	// Status
	uptime := time.Since(m.startTime).Truncate(time.Second)
	status := statusStyle.Render(fmt.Sprintf("Connected for %v", uptime))
	s.WriteString(status + "\n\n")

	// History
	s.WriteString("Command History:\n")

	if len(m.history) == 0 {
		historyContent := "No commands sent yet...\nStart typing to send keys to the VM!"
		historyBox := historyStyle.Width(m.width - 4).Render(historyContent)
		s.WriteString(historyBox)
	} else {
		// Show recent history
		var historyLines []string
		maxLines := m.height - 10 // Reserve space for header and footer
		if maxLines < 5 {
			maxLines = 5
		}

		start := 0
		if len(m.history) > maxLines {
			start = len(m.history) - maxLines
		}

		for i := start; i < len(m.history); i++ {
			entry := m.history[i]
			// Highlight recent entries
			if i >= len(m.history)-3 {
				historyLines = append(historyLines, recentKeyStyle.Render("→ "+entry))
			} else {
				historyLines = append(historyLines, oldKeyStyle.Render("  "+entry))
			}
		}

		historyContent := strings.Join(historyLines, "\n")
		historyBox := historyStyle.Width(m.width - 4).Render(historyContent)
		s.WriteString(historyBox)
	}

	s.WriteString("\n\nWaiting for input... (live mode active)")

	return s.String()
}
