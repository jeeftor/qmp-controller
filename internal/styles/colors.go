package styles

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

// Color constants using a consistent palette
const (
	// Primary colors
	Primary     = "#7D56F4"
	PrimaryText = "#FAFAFA"

	// Status colors
	Success = "#04B575"
	Warning = "#FFA500"
	Error   = "#FF6B6B"
	Info    = "#00CED1"

	// Text colors
	Text      = "#FAFAFA"
	TextMuted = "#626262"
	TextBold  = "#90EE90"

	// Background colors
	Background     = "#1E1E1E"
	BackgroundCard = "#444444"

	// Special colors
	Debug     = "#FF8C00"
	Accent    = "#CCCCCC"
	Highlight = "#FFFF00"
)

// Predefined styles for common use cases
var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(PrimaryText)).
			Background(lipgloss.Color(Primary)).
			Padding(0, 1)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Success)).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Error)).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Warning)).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Info)).
			Bold(true)

	// Text styles
	BoldStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextBold)).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextMuted)).
			Italic(true)

	DebugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Debug)).
			Faint(true)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Primary)).
			Padding(1, 1).
			Margin(0, 1)

	// Log level styles
	LogDebugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CED1"))

	LogInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Success))

	LogWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Warning))

	LogErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Error))

	// Environment variable display styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(Primary)).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	SectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(TextBold)).
			Margin(1, 0, 0, 0)

	KeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Info)).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Text))

	DefaultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextMuted)).
			Italic(true)

	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Accent)).
			Bold(true)

	DescriptionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Text))

	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Primary)).
			Bold(true)
)

// ANSI color mapping for backwards compatibility with fatih/color
type ANSIColor struct {
	Foreground lipgloss.Color
	Background lipgloss.Color
}

var ANSIColors = map[string]ANSIColor{
	"black":     {Foreground: "#000000", Background: "#000000"},
	"red":       {Foreground: "#FF0000", Background: "#FF0000"},
	"green":     {Foreground: "#00FF00", Background: "#00FF00"},
	"yellow":    {Foreground: "#FFFF00", Background: "#FFFF00"},
	"blue":      {Foreground: "#0000FF", Background: "#0000FF"},
	"magenta":   {Foreground: "#FF00FF", Background: "#FF00FF"},
	"cyan":      {Foreground: "#00FFFF", Background: "#00FFFF"},
	"white":     {Foreground: "#FFFFFF", Background: "#FFFFFF"},
	"hiblack":   {Foreground: "#808080", Background: "#808080"},
	"hired":     {Foreground: "#FF6B6B", Background: "#FF6B6B"},
	"higreen":   {Foreground: "#90EE90", Background: "#90EE90"},
	"hiyellow":  {Foreground: "#FFFF99", Background: "#FFFF99"},
	"hiblue":    {Foreground: "#87CEEB", Background: "#87CEEB"},
	"himagenta": {Foreground: "#DDA0DD", Background: "#DDA0DD"},
	"hicyan":    {Foreground: "#00CED1", Background: "#00CED1"},
	"hiwhite":   {Foreground: "#FFFFFF", Background: "#FFFFFF"},
}

// PrintStyled prints text with a lipgloss style to the writer
func PrintStyled(w io.Writer, style lipgloss.Style, text string) {
	fmt.Fprint(w, style.Render(text))
}

// PrintStyledln prints text with a lipgloss style and adds a newline
func PrintStyledln(w io.Writer, style lipgloss.Style, text string) {
	fmt.Fprintln(w, style.Render(text))
}

// GetClosestLipglossColor converts RGB values to the closest predefined lipgloss color
func GetClosestLipglossColor(r, g, b uint32) lipgloss.Color {
	// Convert 16-bit to 8-bit values
	r8 := r >> 8
	g8 := g >> 8
	b8 := b >> 8

	// Determine dominant color
	if r8 > g8 && r8 > b8 {
		if r8 > 200 {
			return ANSIColors["hired"].Foreground
		}
		return ANSIColors["red"].Foreground
	} else if g8 > r8 && g8 > b8 {
		if g8 > 200 {
			return ANSIColors["higreen"].Foreground
		}
		return ANSIColors["green"].Foreground
	} else if b8 > r8 && b8 > g8 {
		if b8 > 200 {
			return ANSIColors["hiblue"].Foreground
		}
		return ANSIColors["blue"].Foreground
	} else if r8 > 150 && g8 > 150 {
		return ANSIColors["yellow"].Foreground
	} else if r8 > 150 && b8 > 150 {
		return ANSIColors["magenta"].Foreground
	} else if g8 > 150 && b8 > 150 {
		return ANSIColors["cyan"].Foreground
	} else if r8 < 50 && g8 < 50 && b8 < 50 {
		return ANSIColors["black"].Foreground
	} else if r8 > 200 && g8 > 200 && b8 > 200 {
		return ANSIColors["white"].Foreground
	}

	return ANSIColors["white"].Foreground
}

// CreateBgStyle creates a background style with the given RGB color
func CreateBgStyle(r, g, b uint8) lipgloss.Style {
	color := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	return lipgloss.NewStyle().Background(lipgloss.Color(color))
}

// CreateFgStyle creates a foreground style with the given RGB color
func CreateFgStyle(r, g, b uint8) lipgloss.Style {
	color := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}
