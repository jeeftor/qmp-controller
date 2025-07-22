package render

import (
	"fmt"
	"io"
	"strings"

	fatihcolor "github.com/fatih/color"

	"github.com/jeeftor/qmp-controller/internal/ocr"
)

// RenderBitmap renders a character bitmap to a string builder using fatih/color
// for consistent color output across the application
func RenderBitmap(bitmap *ocr.CharacterBitmap, writer io.Writer, useColor bool) {
	for bitmapY := 0; bitmapY < bitmap.Height; bitmapY++ {
		for bitmapX := 0; bitmapX < bitmap.Width; bitmapX++ {
			if bitmapY < len(bitmap.Data) && bitmapX < len(bitmap.Data[bitmapY]) {
				// In OCR, bitmap.Data[y][x] is true for BLACK pixels (character foreground)
				// and false for WHITE/BACKGROUND pixels
				if bitmap.Data[bitmapY][bitmapX] {
					// This is a foreground pixel (part of the character)
					fatihcolor.New(fatihcolor.BgBlack).Fprint(writer, "  ")
				} else {
					// This is a background pixel
					if useColor && bitmapY < len(bitmap.Colors) && bitmapX < len(bitmap.Colors[bitmapY]) && bitmap.Colors[bitmapY][bitmapX] != nil {
						// Use original background color if color mode is enabled
						r, g, b, _ := bitmap.Colors[bitmapY][bitmapX].RGBA()
						// Convert 16-bit color to 8-bit
						r8 := uint8(r >> 8)
						g8 := uint8(g >> 8)
						b8 := uint8(b >> 8)

						// Use fatih/color for colored background
						bgColor := fatihcolor.BgRGB(int(r8), int(g8), int(b8))
						bgColor.Fprint(writer, "  ")
					} else {
						// Default background - gray
						fatihcolor.New(fatihcolor.BgHiBlack).Fprint(writer, "  ")
					}
				}
			}
		}
		// Reset color at end of line
		fmt.Fprintln(writer)
	}
}

// FormatBitmapAsHex formats a bitmap as a hex string
// This is a wrapper around the ocr package function to maintain API compatibility
func FormatBitmapAsHex(bitmap *ocr.CharacterBitmap) string {
	return ocr.FormatBitmapAsHex(bitmap)
}

// FormatBitmapOutput formats a complete bitmap output including the hex representation
// and optional ANSI visualization
func FormatBitmapOutput(bitmap *ocr.CharacterBitmap, useAnsi bool, useColor bool) string {
	var sb strings.Builder

	// Add hex representation of the bitmap
	sb.WriteString(fmt.Sprintf("Hex bitmap: %s\n\n", FormatBitmapAsHex(bitmap)))

	// If ANSI mode is enabled, add the bitmap visualization
	if useAnsi {
		RenderBitmap(bitmap, &sb, useColor)
	}

	return sb.String()
}
