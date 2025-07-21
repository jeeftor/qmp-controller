package render

import (
	"fmt"
	"io"
	"strings"

	fatihcolor "github.com/fatih/color"

	"github.com/jeeftor/qmp/internal/ocr"
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
// This is a unified implementation to ensure consistent hex output
func FormatBitmapAsHex(bitmap *ocr.CharacterBitmap) string {
	var hexStr strings.Builder
	hexStr.WriteString("0x")

	for y := 0; y < bitmap.Height; y++ {
		var rowValue uint32

		// Convert row to binary
		for x := 0; x < bitmap.Width; x++ {
			if y < len(bitmap.Data) && x < len(bitmap.Data[y]) && bitmap.Data[y][x] {
				// Set bit if pixel is on
				rowValue |= 1 << (bitmap.Width - 1 - x)
			}
		}

		// Format as hex without 0x prefix
		hexStr.WriteString(fmt.Sprintf("%0*X", (bitmap.Width+3)/4, rowValue))
	}

	return hexStr.String()
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
