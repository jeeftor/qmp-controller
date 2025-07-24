package training

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/render"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"golang.org/x/term"
	"os"
)

// CharacterBatch represents a group of characters to be displayed together
type CharacterBatch struct {
	Bitmaps   []ocr.CharacterBitmap
	Positions []struct{ Row, Col int }
	HexKeys   []string
}

// GetTerminalDimensions detects the current terminal size
func GetTerminalDimensions() (width, height int, err error) {
	// Try to get terminal size from file descriptor 0 (stdin)
	width, height, err = term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		// Fallback to reasonable defaults if detection fails
		return 80, 24, nil
	}
	return width, height, nil
}

// CalculateOptimalCharCount determines how many characters can fit horizontally
func CalculateOptimalCharCount(termWidth int, bitmapWidth int) int {
	// Constants for UI spacing
	const (
		marginSpace     = 10 // Left/right margins
		charPadding     = 4  // Padding between characters
		charLabelSpace  = 8  // Space for "[1]", "[2]" labels
		minCharSpace    = 20 // Minimum space needed per character
	)

	// Calculate space needed per character
	charSpace := bitmapWidth + charPadding + charLabelSpace
	if charSpace < minCharSpace {
		charSpace = minCharSpace
	}

	// Calculate available width
	availableWidth := termWidth - marginSpace
	if availableWidth < charSpace {
		return 1 // Always show at least 1 character
	}

	// Calculate max characters that fit
	maxChars := availableWidth / charSpace

	// Enforce reasonable limits
	if maxChars < 1 {
		return 1
	}
	if maxChars > 5 {
		return 5 // Don't overwhelm the user
	}

	return maxChars
}

// GetDefaultBitmapWidth estimates bitmap display width
func GetDefaultBitmapWidth() int {
	// Most OCR character bitmaps are around 8-12 columns wide when rendered
	return 12
}

// CreateCharacterBatches groups unrecognized characters into display batches
func CreateCharacterBatches(result *ocr.OCRResult, termWidth int) []CharacterBatch {
	// Find all unrecognized characters
	var uniqueBitmaps []ocr.CharacterBitmap
	var positions []struct{ Row, Col int }
	var hexKeys []string
	seenHexKeys := make(map[string]bool)

	for i, bitmap := range result.CharBitmaps {
		// Check for unrecognized characters: either empty string (no training data) or "?" (unrecognized with training data)
		if bitmap.Char == "" || bitmap.Char == "?" {
			hexKey := render.FormatBitmapAsHex(&bitmap)
			if !seenHexKeys[hexKey] {
				uniqueBitmaps = append(uniqueBitmaps, bitmap)
				positions = append(positions, struct{ Row, Col int }{
					Row: i / result.Width,
					Col: i % result.Width,
				})
				hexKeys = append(hexKeys, hexKey)
				seenHexKeys[hexKey] = true
			}
		}
	}

	// Calculate how many characters per batch
	bitmapWidth := GetDefaultBitmapWidth()
	charsPerBatch := CalculateOptimalCharCount(termWidth, bitmapWidth)

	// Group into batches
	var batches []CharacterBatch
	for i := 0; i < len(uniqueBitmaps); i += charsPerBatch {
		end := i + charsPerBatch
		if end > len(uniqueBitmaps) {
			end = len(uniqueBitmaps)
		}

		batch := CharacterBatch{
			Bitmaps:   uniqueBitmaps[i:end],
			Positions: positions[i:end],
			HexKeys:   hexKeys[i:end],
		}
		batches = append(batches, batch)
	}

	return batches
}

// Use centralized styles from styles package
var (
	TitleStyle     = styles.TitleStyle
	CharBoxStyle   = styles.BoxStyle
	CharLabelStyle = styles.SuccessStyle
	PositionStyle  = styles.MutedStyle
	HexStyle       = styles.DebugStyle
	PromptStyle    = styles.InfoStyle
	InputStyle     = styles.BoldStyle
	SkipStyle      = styles.WarningStyle.Italic(true)
)

// RenderCharacterBatch creates a professional side-by-side display of multiple characters
func RenderCharacterBatch(batch CharacterBatch, batchNum, totalBatches int) string {
	var result strings.Builder

	// Header with styling
	title := TitleStyle.Render(fmt.Sprintf(" Character Training - Batch %d/%d ", batchNum, totalBatches))
	result.WriteString("\n" + title + "\n\n")

	// If only one character, use vertical layout
	if len(batch.Bitmaps) == 1 {
		bitmap := batch.Bitmaps[0]
		pos := batch.Positions[0]
		hexKey := batch.HexKeys[0]

		// Create character display
		var charContent strings.Builder
		charContent.WriteString(CharLabelStyle.Render(fmt.Sprintf("[1] ")) +
			PositionStyle.Render(fmt.Sprintf("Position (%d,%d)", pos.Row, pos.Col)) + "\n\n")

		// Render the bitmap
		var bitmapOutput strings.Builder
		render.RenderBitmap(&bitmap, &bitmapOutput, false)
		charContent.WriteString(bitmapOutput.String())

		// Put in a styled box
		charBox := CharBoxStyle.Render(charContent.String())
		result.WriteString(charBox + "\n")

		// Display full hex separately
		result.WriteString(HexStyle.Render(fmt.Sprintf("1: %s", hexKey)) + "\n\n")

	} else {
		// Multiple characters - create horizontal layout
		var charBoxes []string

		for i, bitmap := range batch.Bitmaps {
			pos := batch.Positions[i]

			// Create content for this character
			var charContent strings.Builder
			charContent.WriteString(CharLabelStyle.Render(fmt.Sprintf("[%d] ", i+1)) +
				PositionStyle.Render(fmt.Sprintf("(%d,%d)", pos.Row, pos.Col)) + "\n\n")

			// Render the bitmap
			var bitmapOutput strings.Builder
			render.RenderBitmap(&bitmap, &bitmapOutput, false)
			charContent.WriteString(bitmapOutput.String())

			// Create styled box
			charBox := CharBoxStyle.Render(charContent.String())
			charBoxes = append(charBoxes, charBox)
		}

		// Join boxes horizontally
		horizontalLayout := lipgloss.JoinHorizontal(lipgloss.Top, charBoxes...)
		result.WriteString(horizontalLayout + "\n\n")

		// Display all full hex strings separately
		for i, hexKey := range batch.HexKeys {
			result.WriteString(HexStyle.Render(fmt.Sprintf("%d: %s", i+1, hexKey)) + "\n")
		}
	}

	result.WriteString("\n")
	return result.String()
}

// ProcessBatchInput handles interactive input for a batch of characters
func ProcessBatchInput(batch CharacterBatch, reader *bufio.Reader) map[string]string {
	mappings := make(map[string]string)

	// First, try batch input - ask for all characters at once
	if len(batch.HexKeys) > 1 {
		batchPrompt := PromptStyle.Render(fmt.Sprintf("Enter all %d characters at once", len(batch.HexKeys))) +
			fmt.Sprintf(" (e.g., 'abcd' for [1][2][3][4]),\n") +
			"press Enter for individual prompts, or Ctrl+D for individual mode: "
		fmt.Print(batchPrompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			// Ctrl+D sends EOF, which causes an error
			fmt.Println("Ctrl+D pressed - switching to individual character prompts...")
			// Skip to individual prompts
		} else {
			input = strings.TrimRight(input, "\n\r")

			// Check if user wants individual prompts with text command (backup)
			lowerInput := strings.ToLower(input)
			if lowerInput == "skip" || lowerInput == "individual" {
				fmt.Println("Switching to individual character prompts...")
				// Skip to individual prompts
			} else if input != "" {
				// If user provided input, try to map it to characters
				runes := []rune(input)

				// Handle special cases for spaces
				if input == "space" || input == "SPACE" {
					runes = []rune{' '}
				} else if strings.Contains(input, "space") {
					// Replace "space" with actual space character
					input = strings.ReplaceAll(input, "space", " ")
					input = strings.ReplaceAll(input, "SPACE", " ")
					runes = []rune(input)
				}

				// Map characters to hex keys
				mapped := 0
				for i, hexKey := range batch.HexKeys {
					if i < len(runes) {
						char := string(runes[i])
						mappings[hexKey] = char
						mapped++

						pos := batch.Positions[i]
						successMsg := InputStyle.Render(fmt.Sprintf("Mapped [%d] at (%d,%d): '%s'",
							i+1, pos.Row, pos.Col, char))
						fmt.Printf("%s\n", successMsg)
					}
				}

				if mapped > 0 {
					fmt.Printf("\nSuccessfully mapped %d characters!\n\n", mapped)
					return mappings
				}
			}
		}
	}

	// Fallback to individual prompts
	fmt.Println("Switching to individual character prompts...\n")

	for i, hexKey := range batch.HexKeys {
		pos := batch.Positions[i]

		// Styled prompt for this character
		prompt := PromptStyle.Render(fmt.Sprintf("Enter character for [%d] at position (%d,%d)",
			i+1, pos.Row, pos.Col)) +
			" (or press Enter to skip): "
		fmt.Print(prompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		// Handle the input
		input = strings.TrimRight(input, "\n\r")
		if input == "" {
			skipMsg := SkipStyle.Render(fmt.Sprintf("Skipped [%d]", i+1))
			fmt.Printf("%s\n\n", skipMsg)
			continue
		}

		// Handle space character input
		if input == "space" || input == "SPACE" {
			input = " "
		}

		// Take only the first character
		char := string([]rune(input)[0])

		// Add to mappings
		mappings[hexKey] = char
		successMsg := InputStyle.Render(fmt.Sprintf("Added mapping for [%d]: '%s'", i+1, char))
		fmt.Printf("%s\n\n", successMsg)
	}

	return mappings
}
