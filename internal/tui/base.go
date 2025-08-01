package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// CommonTUIState holds state common to all TUI models
type CommonTUIState struct {
	VMID      string
	Client    *qmp.Client
	Width     int
	Height    int
	Quitting  bool
	StartTime time.Time
}

// BaseTUIModel provides common functionality for all TUI models
type BaseTUIModel struct {
	State *CommonTUIState
}

// NewBaseTUIModel creates a new base TUI model
func NewBaseTUIModel(vmid string, client *qmp.Client) *BaseTUIModel {
	return &BaseTUIModel{
		State: &CommonTUIState{
			VMID:      vmid,
			Client:    client,
			Width:     80,
			Height:    24,
			Quitting:  false,
			StartTime: time.Now(),
		},
	}
}

// HandleWindowResize handles window resize messages consistently
func (b *BaseTUIModel) HandleWindowResize(msg tea.WindowSizeMsg) {
	b.State.Width = msg.Width
	b.State.Height = msg.Height
}

// HandleQuitKeys handles common quit key combinations
func (b *BaseTUIModel) HandleQuitKeys(keyString string) bool {
	switch keyString {
	case "ctrl+c", "q", "esc":
		b.State.Quitting = true
		return true
	}
	return false
}

// IsQuitting returns true if the TUI is in quitting state
func (b *BaseTUIModel) IsQuitting() bool {
	return b.State.Quitting
}

// GetUptime returns the time elapsed since the TUI started
func (b *BaseTUIModel) GetUptime() time.Duration {
	return time.Since(b.State.StartTime)
}

// GetDimensions returns the current width and height
func (b *BaseTUIModel) GetDimensions() (int, int) {
	return b.State.Width, b.State.Height
}

// CommonInit provides standard initialization commands
func (b *BaseTUIModel) CommonInit() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		b.TickCmd(),
	)
}

// TickCmd returns a command that sends tick messages every second
func (b *BaseTUIModel) TickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Common message types
type TickMsg time.Time
type QuitMsg struct{}

// ViewportConfig holds common viewport configuration
type ViewportConfig struct {
	Width   int
	Height  int
	Padding int
	Border  bool
}

// GetViewportConfig returns a standard viewport configuration based on screen size
func (b *BaseTUIModel) GetViewportConfig() ViewportConfig {
	padding := 4
	borderHeight := 2
	availableHeight := b.State.Height - borderHeight - padding

	return ViewportConfig{
		Width:   b.State.Width - padding,
		Height:  availableHeight,
		Padding: padding,
		Border:  true,
	}
}

// StatusInfo holds common status information
type StatusInfo struct {
	Title       string
	VMID        string
	Status      string
	Uptime      time.Duration
	LastAction  string
	ActionTime  time.Time
}

// GetCommonStatus returns common status information
func (b *BaseTUIModel) GetCommonStatus(title, status, lastAction string, actionTime time.Time) StatusInfo {
	return StatusInfo{
		Title:      title,
		VMID:       b.State.VMID,
		Status:     status,
		Uptime:     b.GetUptime(),
		LastAction: lastAction,
		ActionTime: actionTime,
	}
}

// LogEntry represents a log entry with timestamp and content
type LogEntry struct {
	Timestamp time.Time
	Content   string
	Level     LogLevel
}

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LogLevelInfo LogLevel = iota
	LogLevelWarn
	LogLevelError
	LogLevelSuccess
	LogLevelDebug
)

// LogManager handles log entries with automatic pruning
type LogManager struct {
	entries []LogEntry
	maxSize int
}

// NewLogManager creates a new log manager with the specified maximum size
func NewLogManager(maxSize int) *LogManager {
	return &LogManager{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a new log entry, pruning old entries if necessary
func (lm *LogManager) Add(content string, level LogLevel) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Content:   content,
		Level:     level,
	}

	lm.entries = append(lm.entries, entry)

	// Prune old entries if we exceed max size
	if len(lm.entries) > lm.maxSize {
		lm.entries = lm.entries[1:]
	}
}

// GetEntries returns all log entries
func (lm *LogManager) GetEntries() []LogEntry {
	return lm.entries
}

// GetRecentEntries returns the most recent N entries
func (lm *LogManager) GetRecentEntries(n int) []LogEntry {
	if n >= len(lm.entries) {
		return lm.entries
	}
	return lm.entries[len(lm.entries)-n:]
}

// Clear removes all log entries
func (lm *LogManager) Clear() {
	lm.entries = lm.entries[:0]
}

// ProgressTracker tracks progress for long-running operations
type ProgressTracker struct {
	Total       int
	Current     int
	StartTime   time.Time
	LastUpdate  time.Time
	Description string
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(total int, description string) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		Total:       total,
		Current:     0,
		StartTime:   now,
		LastUpdate:  now,
		Description: description,
	}
}

// Update updates the progress tracker
func (pt *ProgressTracker) Update(current int) {
	pt.Current = current
	pt.LastUpdate = time.Now()
}

// Increment increments the current progress by 1
func (pt *ProgressTracker) Increment() {
	pt.Update(pt.Current + 1)
}

// GetProgress returns the progress as a float between 0.0 and 1.0
func (pt *ProgressTracker) GetProgress() float64 {
	if pt.Total <= 0 {
		return 0.0
	}
	progress := float64(pt.Current) / float64(pt.Total)
	if progress > 1.0 {
		progress = 1.0
	}
	return progress
}

// IsComplete returns true if the progress is complete
func (pt *ProgressTracker) IsComplete() bool {
	return pt.Current >= pt.Total
}

// GetElapsedTime returns the time elapsed since the tracker started
func (pt *ProgressTracker) GetElapsedTime() time.Duration {
	return time.Since(pt.StartTime)
}

// GetETA estimates the time remaining based on current progress
func (pt *ProgressTracker) GetETA() time.Duration {
	if pt.Current <= 0 {
		return 0
	}

	elapsed := pt.GetElapsedTime()
	progress := pt.GetProgress()

	if progress <= 0 {
		return 0
	}

	totalEstimated := time.Duration(float64(elapsed) / progress)
	remaining := totalEstimated - elapsed

	if remaining < 0 {
		remaining = 0
	}

	return remaining
}
