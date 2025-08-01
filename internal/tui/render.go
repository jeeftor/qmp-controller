package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/styles"
)

// Common TUI styles using the centralized styles package
var (
	TitleStyle     = styles.TitleStyle
	StatusStyle    = styles.SuccessStyle
	ErrorStyle     = styles.ErrorStyle
	WarningStyle   = styles.WarningStyle
	InfoStyle      = styles.InfoStyle
	MutedStyle     = styles.MutedStyle
	BoxStyle       = styles.BoxStyle
	BoldStyle      = styles.BoldStyle
)

// Additional TUI-specific styles
var (
	HighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(styles.Highlight)).
				Bold(true)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(styles.Success)).
				Background(lipgloss.Color(styles.TextMuted))

	ProgressBackgroundStyle = lipgloss.NewStyle().
					Background(lipgloss.Color(styles.BackgroundCard))

	LogInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.Text))

	LogWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.Warning))

	LogErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.Error))

	LogSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(styles.Success))

	LogDebugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.TextMuted))

	UptimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.TextMuted))
)

// TUIRenderer provides common rendering functions for TUI models
type TUIRenderer struct {
	width  int
	height int
}

// NewTUIRenderer creates a new TUI renderer
func NewTUIRenderer(width, height int) *TUIRenderer {
	return &TUIRenderer{
		width:  width,
		height: height,
	}
}

// UpdateDimensions updates the renderer dimensions
func (r *TUIRenderer) UpdateDimensions(width, height int) {
	r.width = width
	r.height = height
}

// RenderTitle renders a consistent title bar
func (r *TUIRenderer) RenderTitle(title string) string {
	if r.width <= 0 {
		return TitleStyle.Render(title)
	}

	// Center the title
	padding := (r.width - len(title)) / 2
	if padding < 0 {
		padding = 0
	}

	centeredTitle := strings.Repeat(" ", padding) + title
	return TitleStyle.Width(r.width).Render(centeredTitle)
}

// RenderStatus renders a status line with common information
func (r *TUIRenderer) RenderStatus(info StatusInfo) string {
	var parts []string

	// VM ID
	if info.VMID != "" {
		parts = append(parts, fmt.Sprintf("VM: %s", HighlightStyle.Render(info.VMID)))
	}

	// Status
	if info.Status != "" {
		parts = append(parts, fmt.Sprintf("Status: %s", StatusStyle.Render(info.Status)))
	}

	// Uptime
	uptime := r.formatDuration(info.Uptime)
	parts = append(parts, fmt.Sprintf("Uptime: %s", UptimeStyle.Render(uptime)))

	// Last action
	if info.LastAction != "" && !info.ActionTime.IsZero() {
		timeSince := time.Since(info.ActionTime)
		parts = append(parts, fmt.Sprintf("Last: %s (%s ago)",
			info.LastAction, r.formatDuration(timeSince)))
	}

	return strings.Join(parts, " | ")
}

// RenderProgressBar renders a progress bar with percentage
func (r *TUIRenderer) RenderProgressBar(progress float64, width int, label string) string {
	if width <= 0 {
		width = 40
	}

	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := fmt.Sprintf("%.1f%%", progress*100)

	progressBar := ProgressBarStyle.Render(bar)

	if label != "" {
		return fmt.Sprintf("%s %s %s", label, progressBar, percentage)
	}
	return fmt.Sprintf("%s %s", progressBar, percentage)
}

// RenderProgressWithETA renders a progress bar with ETA information
func (r *TUIRenderer) RenderProgressWithETA(tracker *ProgressTracker, width int) string {
	progress := tracker.GetProgress()

	var parts []string

	// Description
	if tracker.Description != "" {
		parts = append(parts, BoldStyle.Render(tracker.Description))
	}

	// Progress bar
	progressBar := r.RenderProgressBar(progress, width, "")
	parts = append(parts, progressBar)

	// Progress info
	info := fmt.Sprintf("%d/%d", tracker.Current, tracker.Total)
	parts = append(parts, info)

	// ETA
	if !tracker.IsComplete() && tracker.Current > 0 {
		eta := tracker.GetETA()
		if eta > 0 {
			parts = append(parts, fmt.Sprintf("ETA: %s", r.formatDuration(eta)))
		}
	}

	// Elapsed time
	elapsed := tracker.GetElapsedTime()
	parts = append(parts, fmt.Sprintf("Elapsed: %s", r.formatDuration(elapsed)))

	return strings.Join(parts, " | ")
}

// RenderLogEntries renders log entries with appropriate styling
func (r *TUIRenderer) RenderLogEntries(entries []LogEntry, maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 10
	}

	// Get recent entries
	recentEntries := entries
	if len(entries) > maxLines {
		recentEntries = entries[len(entries)-maxLines:]
	}

	var lines []string
	for _, entry := range recentEntries {
		timestamp := entry.Timestamp.Format("15:04:05")
		content := entry.Content

		var style lipgloss.Style
		var levelIndicator string

		switch entry.Level {
		case LogLevelInfo:
			style = LogInfoStyle
			levelIndicator = "ℹ"
		case LogLevelWarn:
			style = LogWarnStyle
			levelIndicator = "⚠"
		case LogLevelError:
			style = LogErrorStyle
			levelIndicator = "✗"
		case LogLevelSuccess:
			style = LogSuccessStyle
			levelIndicator = "✓"
		case LogLevelDebug:
			style = LogDebugStyle
			levelIndicator = "🐛"
		default:
			style = LogInfoStyle
			levelIndicator = "•"
		}

		line := fmt.Sprintf("%s %s %s",
			MutedStyle.Render(timestamp),
			style.Render(levelIndicator),
			style.Render(content))

		lines = append(lines, line)
	}

	return lines
}

// RenderBox renders content in a box with optional title
func (r *TUIRenderer) RenderBox(content string, title string, width int) string {
	boxStyle := BoxStyle

	if width > 0 {
		boxStyle = boxStyle.Width(width)
	}

	if title != "" {
		boxStyle = boxStyle.BorderTop(true).BorderTopForeground(lipgloss.Color(styles.Primary))
		// Add title to the border
		content = BoldStyle.Render(title) + "\n\n" + content
	}

	return boxStyle.Render(content)
}

// RenderKeyHelp renders help text for keyboard shortcuts
func (r *TUIRenderer) RenderKeyHelp(shortcuts map[string]string) string {
	var helpLines []string

	for key, description := range shortcuts {
		helpLine := fmt.Sprintf("%s %s",
			HighlightStyle.Render(key),
			description)
		helpLines = append(helpLines, helpLine)
	}

	return r.RenderBox(strings.Join(helpLines, "\n"), "Keyboard Shortcuts", 0)
}

// RenderFooter renders a consistent footer
func (r *TUIRenderer) RenderFooter(leftContent, rightContent string) string {
	if r.width <= 0 {
		return fmt.Sprintf("%s | %s", leftContent, rightContent)
	}

	// Calculate spacing
	totalContent := len(leftContent) + len(rightContent)
	spacing := r.width - totalContent
	if spacing < 3 {
		spacing = 3
	}

	footer := leftContent + strings.Repeat(" ", spacing) + rightContent
	return MutedStyle.Width(r.width).Render(footer)
}

// formatDuration formats a duration in a human-readable way
func (r *TUIRenderer) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int((d % time.Hour).Minutes())
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}

// RenderList renders a list of items with optional numbering
func (r *TUIRenderer) RenderList(items []string, numbered bool, selectedIndex int) []string {
	var lines []string

	for i, item := range items {
		var line string

		if numbered {
			line = fmt.Sprintf("%s. %s",
				MutedStyle.Render(fmt.Sprintf("%d", i+1)),
				item)
		} else {
			line = fmt.Sprintf("• %s", item)
		}

		// Highlight selected item
		if selectedIndex >= 0 && i == selectedIndex {
			line = HighlightStyle.Render(line)
		}

		lines = append(lines, line)
	}

	return lines
}

// RenderTable renders tabular data with headers
func (r *TUIRenderer) RenderTable(headers []string, rows [][]string, maxWidth int) string {
	if len(headers) == 0 || len(rows) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Apply max width constraint
	if maxWidth > 0 {
		totalWidth := 0
		for _, width := range colWidths {
			totalWidth += width
		}
		if totalWidth > maxWidth {
			// Proportionally reduce column widths
			factor := float64(maxWidth) / float64(totalWidth)
			for i := range colWidths {
				colWidths[i] = int(float64(colWidths[i]) * factor)
				if colWidths[i] < 3 {
					colWidths[i] = 3
				}
			}
		}
	}

	var lines []string

	// Render header
	var headerParts []string
	for i, header := range headers {
		headerParts = append(headerParts, BoldStyle.Width(colWidths[i]).Render(header))
	}
	lines = append(lines, strings.Join(headerParts, " | "))

	// Render separator
	var sepParts []string
	for _, width := range colWidths {
		sepParts = append(sepParts, strings.Repeat("-", width))
	}
	lines = append(lines, strings.Join(sepParts, "-+-"))

	// Render rows
	for _, row := range rows {
		var rowParts []string
		for i, cell := range row {
			if i < len(colWidths) {
				// Truncate cell if too long
				if len(cell) > colWidths[i] {
					cell = cell[:colWidths[i]-3] + "..."
				}
				rowParts = append(rowParts, fmt.Sprintf("%-*s", colWidths[i], cell))
			}
		}
		lines = append(lines, strings.Join(rowParts, " | "))
	}

	return strings.Join(lines, "\n")
}
