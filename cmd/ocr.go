package cmd

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	colorLib "github.com/fatih/color"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/render"
	"github.com/jeeftor/qmp-controller/internal/training"
	"github.com/spf13/cobra"
	_ "github.com/spf13/viper"
)

var (
	// OCR command flags
	resolution       string
	screenWidth      int
	screenHeight     int
	columns          int
	rows             int
	debugMode        bool
	ansiMode         bool
	colorMode        bool
	cropEnabled      bool
	cropRowsStr      string
	cropColsStr      string
	cropStartRow     int
	cropEndRow       int
	cropStartCol     int
	cropEndCol       int
	singleChar       bool
	charRow          int
	charCol          int
	trainingDataPath string
	filterBlankLines bool
	showLineNumbers  bool
	updateTraining   bool
	renderSpecialChars bool
)

// ocrCmd represents the ocr command
var ocrCmd = &cobra.Command{
	Use:   "ocr",
	Short: "Perform OCR on VM console screen",
	Long: `Perform Optical Character Recognition (OCR) on a VM console screen.
This command captures a screenshot, divides it into character cells based on
the specified resolution, and attempts to recognize text characters.

In debug mode, it will output ASCII/ANSI representation of recognized characters.
In ANSI mode, it will output ANSI escape sequences for character bitmaps.`,
}

// ocrVmCmd represents the ocr vm command
var ocrVmCmd = &cobra.Command{
	Use:   "vm [vmid] [training-data] [output-file]",
	Short: "Perform OCR on VM console screen",
	Long: `Perform Optical Character Recognition (OCR) on a VM console screen.
This command captures a screenshot, divides it into character cells based on
the specified resolution, and attempts to recognize text characters.

In debug mode, it will output ASCII/ANSI representation of recognized characters.
In ANSI mode, it will output ANSI escape sequences for character bitmaps.

Examples:
  # Perform OCR with default settings
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50

  # Use debug mode to see character representations
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50 --debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50 --ansi

  # Filter out blank lines from the output
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50 --filter

  # Show line numbers with the output
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50 --line-numbers

  # Interactive training for unrecognized characters
  qmp ocr vm 106 training-data.json text.txt --width 160 --height 50 --update-training`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		vmid := args[0]
		trainingDataPath = args[1]
		var outputFile string
		if len(args) > 2 {
			outputFile = args[2]
		}

		// Parse resolution
		parseResolution()

		// Parse crop parameters
		if cropEnabled {
			parseCropParameters()
		}

		// Validate screen dimensions
		if columns <= 0 || rows <= 0 {
			fmt.Println("Error: Screen width and height must be positive integers")
			os.Exit(1)
		}

		// Create output directory if it doesn't exist
		if outputFile != "" {
			outputDir := filepath.Dir(outputFile)
			if outputDir != "." {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					fmt.Printf("Error creating output directory: %v\n", err)
					os.Exit(1)
				}
			}
		}

		var client *qmp.Client
		if socketPath := GetSocketPath(); socketPath != "" {
			client = qmp.NewWithSocketPath(vmid, socketPath)
		} else {
			client = qmp.New(vmid)
		}

		if err := client.Connect(); err != nil {
			fmt.Printf("Error connecting to VM %s: %v\n", vmid, err)
			os.Exit(1)
		}
		defer client.Close()

		// Create a temporary file for the screenshot
		tmpFile, err := os.CreateTemp("", "qmp-ocr-*.ppm")
		if err != nil {
			fmt.Printf("Error creating temporary file: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Get remote temp path from flag or config
		remotePath := getRemoteTempPath()

		// Take a screenshot
		logging.Debug("Taking screenshot for OCR", "output", tmpFile.Name(), "remoteTempPath", remotePath)
		if err := client.ScreenDump(tmpFile.Name(), remotePath); err != nil {
			fmt.Printf("Error taking screenshot: %v\n", err)
			os.Exit(1)
		}

		// Process the screenshot
		var result *ocr.OCRResult
		var processErr error

		if cropEnabled {
			// Process with cropping
			result, processErr = ocr.ProcessScreenshotWithCropAndTrainingData(
				tmpFile.Name(),
				trainingDataPath,
				columns, rows,
				cropStartRow, cropEndRow,
				cropStartCol, cropEndCol,
				debugMode)
		} else {
			// Process the full image
			result, processErr = ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), trainingDataPath, columns, rows, debugMode)
		}

		if processErr != nil {
			fmt.Fprintf(os.Stderr, "Error processing file for OCR: %v\n", processErr)
			os.Exit(1)
		}

		// Handle interactive training if enabled
		if updateTraining {
			if err := handleInteractiveTraining(result, trainingDataPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error during interactive training: %v\n", err)
				os.Exit(1)
			}

			// Re-run character recognition with updated training data to refresh the Text output
			updatedTrainingData, err := ocr.LoadTrainingData(trainingDataPath)
			if err == nil && updatedTrainingData != nil {
				if err := ocr.RecognizeCharacters(result, updatedTrainingData); err != nil {
					fmt.Printf("Warning: Error re-recognizing characters after training: %v\n", err)
				}
			}
		}

		// Format the output
		var output string
		if debugMode {
			output = formatDebugOutput(result, tmpFile.Name(), colorMode)
		} else if ansiMode {
			output = formatOutput(result, true, colorMode)
		} else if singleChar {
			output = formatSingleCharOutput(result, charRow, charCol)
		} else if renderSpecialChars {
			output = formatTextOutputWithSpecialChars(result, filterBlankLines, showLineNumbers)
		} else if colorMode {
			output = formatColoredTextOutput(result, filterBlankLines, showLineNumbers)
		} else {
			output = formatTextOutput(result, filterBlankLines, showLineNumbers)
		}

		// Save or print the output
		if outputFile != "" {
			// Save to output file
			if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving OCR results: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("OCR results saved to %s\n", outputFile)
		} else {
			// Print to screen
			fmt.Println(output)
		}
	},
}

// ocrFileCmd represents the ocr file command
var ocrFileCmd = &cobra.Command{
	Use:   "file [input-file] [training-data] [output-file]",
	Short: "Perform OCR on an image file",
	Long: `Perform Optical Character Recognition (OCR) on an image file.
This command processes an existing image file, divides it into character cells based on
the specified resolution, and attempts to recognize text characters.

You can specify the resolution in several ways:
  --width and --height: Set the number of columns and rows
  --res: Set the resolution in the format "columns x rows" (e.g., "160x50")
  -c and -r: Short form for columns and rows

You can also crop the image to process only a portion:
  --crop: Enable cropping mode
  --crop-rows: Specify start and end rows (e.g., "5:20" for rows 5 through 20)
  --crop-cols: Specify start and end columns (e.g., "10:50" for columns 10 through 50)

Output options:
  --debug: Output ASCII/ANSI representation of recognized characters
  --ansi: Output ANSI escape sequences for character bitmaps
  --filter: Filter out blank lines from the text output
  --line-numbers, -l: Show 0-based line numbers with colored output

Examples:
  # Perform OCR with explicit width and height
  qmp ocr file screenshot.ppm training-data.json text.txt --width 160 --height 50

  # Use the resolution format
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50

  # Use short form flags
  qmp ocr file screenshot.ppm training-data.json text.txt -c 160 -r 50

  # Use debug mode to see character representations
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --ansi

  # Crop the image to process only rows 5-20 and columns 10-50
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --crop --crop-rows 5:20 --crop-cols 10:50

  # Print results to screen instead of saving to a file
  qmp ocr file screenshot.ppm training-data.json --res 160x50

  # Filter out blank lines from the output
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --filter

  # Show line numbers with the output
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --line-numbers

  # Interactive training for unrecognized characters
  qmp ocr file screenshot.ppm training-data.json text.txt --res 160x50 --update-training`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse resolution
		parseResolution()

		// Parse crop parameters
		if cropEnabled {
			parseCropParameters()
		}

		// Get input file
		inputFile := args[0]
		trainingDataPath = args[1]

		// Get output file if provided
		var outputFile string
		if len(args) > 2 {
			outputFile = args[2]

			// Create output directory if it doesn't exist
			outputDir := filepath.Dir(outputFile)
			if outputDir != "." {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
					os.Exit(1)
				}
			}
		}

		// If single character mode is enabled, extract just that character
		if singleChar {
			bitmap, err := ocr.ExtractSingleCharacter(inputFile, columns, rows, charRow, charCol)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error extracting character: %v\n", err)
				os.Exit(1)
			}

			// Create a minimal OCR result with just this character
			result := &ocr.OCRResult{
				Width:       1,
				Height:      1,
				Text:        []string{"?"},
				CharBitmaps: []ocr.CharacterBitmap{*bitmap},
			}

			// Load training data and recognize characters
			trainingData, err := ocr.LoadTrainingData(trainingDataPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN No training data found, using basic recognition error=%v\n", err)
			} else {
				// Recognize characters using training data
				if err := ocr.RecognizeCharacters(result, trainingData); err != nil {
					fmt.Fprintf(os.Stderr, "Error recognizing characters: %v\n", err)
				}

				// For single-char mode, we need to update the Char field directly
				// since RecognizeCharacters only updates the Text field
				if len(result.CharBitmaps) > 0 && len(result.Text) > 0 && len(result.Text[0]) > 0 {
					result.CharBitmaps[0].Char = string(result.Text[0][0])
				}
			}

			// Format the output
			output := formatSingleCharOutput(result, 0, 0)

			// Save or print the output
			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving character extraction: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Character extraction saved to %s\n", outputFile)
			} else {
				fmt.Println(output)
			}
			return
		}

		// Process the image
		var result *ocr.OCRResult
		var processErr error

		if cropEnabled {
			// Process with cropping
			result, processErr = ocr.ProcessScreenshotWithCropAndTrainingData(
				inputFile,
				trainingDataPath,
				columns, rows,
				cropStartRow, cropEndRow,
				cropStartCol, cropEndCol,
				debugMode)
		} else {
			// Process the full image
			result, processErr = ocr.ProcessScreenshotWithTrainingData(inputFile, trainingDataPath, columns, rows, debugMode)
		}

		if processErr != nil {
			fmt.Fprintf(os.Stderr, "Error processing file for OCR: %v\n", processErr)
			os.Exit(1)
		}

		// Handle interactive training if enabled
		if updateTraining {
			if err := handleInteractiveTraining(result, trainingDataPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error during interactive training: %v\n", err)
				os.Exit(1)
			}

			// Re-run character recognition with updated training data to refresh the Text output
			updatedTrainingData, err := ocr.LoadTrainingData(trainingDataPath)
			if err == nil && updatedTrainingData != nil {
				if err := ocr.RecognizeCharacters(result, updatedTrainingData); err != nil {
					fmt.Printf("Warning: Error re-recognizing characters after training: %v\n", err)
				}
			}
		}

		// Format the output
		var output string
		if debugMode {
			output = formatDebugOutput(result, inputFile, colorMode)
		} else if ansiMode {
			output = formatOutput(result, true, colorMode)
		} else if renderSpecialChars {
			output = formatTextOutputWithSpecialChars(result, filterBlankLines, showLineNumbers)
		} else if colorMode {
			output = formatColoredTextOutput(result, filterBlankLines, showLineNumbers)
		} else {
			output = formatTextOutput(result, filterBlankLines, showLineNumbers)
		}

		// Save or print the output
		if outputFile != "" {
			// Save to output file
			if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving OCR results: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("OCR results saved to %s\n", outputFile)
		} else {
			// Print to screen
			fmt.Println(output)
		}
	},
}

func parseResolution() {
	// Parse resolution
	if resolution != "" {
		parts := strings.Split(resolution, "x")
		if len(parts) != 2 {
			fmt.Println("Error: Resolution must be in the format 'columns x rows' (e.g., '160x50')")
			os.Exit(1)
		}

		//var err error
		fmt.Sscanf(parts[0], "%d", &columns)
		fmt.Sscanf(parts[1], "%d", &rows)

		if columns <= 0 || rows <= 0 {
			fmt.Printf("Error parsing resolution '%s': columns and rows must be positive integers\n", resolution)
			os.Exit(1)
		}
	}

	// Use width/height if resolution wasn't provided or didn't parse correctly
	if columns <= 0 {
		columns = screenWidth
	}
	if rows <= 0 {
		rows = screenHeight
	}

	// Validate screen dimensions
	if columns <= 0 || rows <= 0 {
		fmt.Println("Error: Screen columns and rows must be positive integers")
		os.Exit(1)
	}
}

func parseCropParameters() {
	// Parse crop rows
	if cropRowsStr != "" {
		parts := strings.Split(cropRowsStr, ":")
		if len(parts) != 2 {
			fmt.Println("Error: Crop rows must be in the format 'start:end' (e.g., '5:20')")
			os.Exit(1)
		}

		fmt.Sscanf(parts[0], "%d", &cropStartRow)
		fmt.Sscanf(parts[1], "%d", &cropEndRow)

		if cropStartRow < 0 || cropEndRow < cropStartRow || cropEndRow >= rows {
			fmt.Printf("Error: Invalid crop row range %d:%d (must be 0 <= start <= end < %d)\n",
				cropStartRow, cropEndRow, rows)
			os.Exit(1)
		}
	} else {
		cropStartRow = 0
		cropEndRow = rows - 1
	}

	// Parse crop columns
	if cropColsStr != "" {
		parts := strings.Split(cropColsStr, ":")
		if len(parts) != 2 {
			fmt.Println("Error: Crop columns must be in the format 'start:end' (e.g., '10:50')")
			os.Exit(1)
		}

		fmt.Sscanf(parts[0], "%d", &cropStartCol)
		fmt.Sscanf(parts[1], "%d", &cropEndCol)

		if cropStartCol < 0 || cropEndCol < cropStartCol || cropEndCol >= columns {
			fmt.Printf("Error: Invalid crop column range %d:%d (must be 0 <= start <= end < %d)\n",
				cropStartCol, cropEndCol, columns)
			os.Exit(1)
		}
	} else {
		cropStartCol = 0
		cropEndCol = columns - 1
	}

	logging.Debug("Crop enabled",
		"rows", fmt.Sprintf("%d:%d", cropStartRow, cropEndRow),
		"columns", fmt.Sprintf("%d:%d", cropStartCol, cropEndCol))
}

func formatTextOutputWithSpecialChars(result *ocr.OCRResult, filterBlanks bool, lineNumbers bool) string {
	var sb strings.Builder

	// Create color functions for special characters
	lineNumColor := colorLib.New(colorLib.FgCyan, colorLib.Bold)
	unrecognizedColor := colorLib.New(colorLib.FgHiRed, colorLib.Bold, colorLib.BgWhite)
	spaceColor := colorLib.New(colorLib.FgYellow, colorLib.Bold)
	tabColor := colorLib.New(colorLib.FgMagenta, colorLib.Bold)
	newlineColor := colorLib.New(colorLib.FgGreen, colorLib.Bold)

	lineCounter := 0
	for _, line := range result.Text {
		// Filter out blank lines if requested
		if filterBlanks && strings.TrimSpace(line) == "" {
			continue
		}

		if lineNumbers {
			// Add colored line number prefix
			linePrefix := lineNumColor.Sprintf("%02d: ", lineCounter)
			sb.WriteString(linePrefix)
		}

		// Process each character to render special chars visually
		for _, char := range line {
			switch char {
			case ' ':
				// Render spaces as colored dots
				sb.WriteString(spaceColor.Sprint("·"))
			case '\t':
				// Render tabs as colored arrows
				sb.WriteString(tabColor.Sprint("→"))
			case '\r':
				// Render carriage returns as colored "CR"
				sb.WriteString(newlineColor.Sprint("⏎"))
			case '\n':
				// Render newlines as colored "LF"
				sb.WriteString(newlineColor.Sprint("↩"))
			case '?':
				// Color unrecognized characters
				sb.WriteString(unrecognizedColor.Sprint(string(char)))
			default:
				// Regular characters unchanged
				sb.WriteString(string(char))
			}
		}
		// Add actual newline at end of each line
		sb.WriteString("\n")
		lineCounter++
	}

	return sb.String()
}

func formatTextOutput(result *ocr.OCRResult, filterBlanks bool, lineNumbers bool) string {
	var sb strings.Builder

	// Create a color function for line numbers
	lineNumColor := colorLib.New(colorLib.FgCyan, colorLib.Bold)
	// Create a color function for unrecognized characters
	unrecognizedColor := colorLib.New(colorLib.FgHiRed, colorLib.Bold, colorLib.BgWhite)

	lineCounter := 0
	for _, line := range result.Text {
		// Filter out blank lines if requested
		if filterBlanks && strings.TrimSpace(line) == "" {
			continue
		}

		if lineNumbers {
			// Add colored line number prefix
			linePrefix := lineNumColor.Sprintf("%02d: ", lineCounter)
			sb.WriteString(linePrefix)
		}

		// Color unrecognized characters
		for _, char := range line {
			if char == '?' {
				sb.WriteString(unrecognizedColor.Sprint(string(char)))
			} else {
				sb.WriteString(string(char))
			}
		}
		sb.WriteString("\n")
		lineCounter++
	}

	return sb.String()
}

// formatColoredTextOutput formats OCR results with original colors preserved
func formatColoredTextOutput(result *ocr.OCRResult, filterBlanks bool, lineNumbers bool) string {
	var sb strings.Builder
	// Create a color function for line numbers
	lineNumColor := colorLib.New(colorLib.FgCyan, colorLib.Bold)
	// Create a color function for unrecognized characters
	unrecognizedColor := colorLib.New(colorLib.FgHiRed, colorLib.Bold, colorLib.BgWhite)

	lineCounter := 0
	charIdx := 0

	for y, line := range result.Text {
		// Filter out blank lines if requested
		if filterBlanks && strings.TrimSpace(line) == "" {
			charIdx += len(line)
			continue
		}

		if lineNumbers {
			// Add colored line number prefix
			linePrefix := lineNumColor.Sprintf("%02d: ", lineCounter)
			sb.WriteString(linePrefix)
		}

		// Process each character with its original color
		for x, char := range line {
			if char == '?' {
				sb.WriteString(unrecognizedColor.Sprint(string(char)))
			} else {
				// Calculate the character bitmap index
				bitmapIdx := y*result.Width + x

				if bitmapIdx < len(result.CharBitmaps) {
					bitmap := result.CharBitmaps[bitmapIdx]

					// Find a representative color from the bitmap (first non-background pixel)
					originalColor := getRepresentativeColor(bitmap)

					if originalColor != nil {
						// Create a color function based on the original pixel color
						r, g, b, _ := originalColor.RGBA()
						r8, g8, b8 := r>>8, g>>8, b>>8

						// Convert to ANSI color codes
						colorFunc := getClosestANSIColor(r8, g8, b8)
						sb.WriteString(colorFunc.Sprint(string(char)))
					} else {
						// Fallback to normal color
						sb.WriteString(string(char))
					}
				} else {
					sb.WriteString(string(char))
				}
			}
		}

		sb.WriteString("\n")
		lineCounter++
		charIdx += len(line)
	}

	return sb.String()
}

// getRepresentativeColor finds a representative color from a character bitmap (first text pixel)
func getRepresentativeColor(bitmap ocr.CharacterBitmap) color.Color {
	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			if bitmap.Data[y][x] { // If this pixel is "text" (not background)
				if y < len(bitmap.Colors) && x < len(bitmap.Colors[y]) {
					return bitmap.Colors[y][x]
				}
			}
		}
	}
	return nil
}

// getClosestANSIColor maps RGB values to the closest ANSI color
func getClosestANSIColor(r, g, b uint32) *colorLib.Color {
	// Simple color mapping to common ANSI colors
	if r < 100 && g < 100 && b > 150 {
		// Blue-ish
		return colorLib.New(colorLib.FgBlue)
	} else if r < 100 && g > 150 && b < 100 {
		// Green-ish
		return colorLib.New(colorLib.FgGreen)
	} else if r > 150 && g < 100 && b < 100 {
		// Red-ish
		return colorLib.New(colorLib.FgRed)
	} else if r > 150 && g > 150 && b < 100 {
		// Yellow-ish
		return colorLib.New(colorLib.FgYellow)
	} else if r > 150 && g < 100 && b > 150 {
		// Magenta-ish
		return colorLib.New(colorLib.FgMagenta)
	} else if r < 100 && g > 150 && b > 150 {
		// Cyan-ish
		return colorLib.New(colorLib.FgCyan)
	} else if r < 80 && g < 80 && b < 80 {
		// Dark/Black
		return colorLib.New(colorLib.FgBlack)
	} else if r > 200 && g > 200 && b > 200 {
		// Light/White
		return colorLib.New(colorLib.FgWhite)
	} else {
		// Default to white for anything else
		return colorLib.New(colorLib.FgWhite)
	}
}

func formatDebugOutput(result *ocr.OCRResult, screenshotPath string, colorMode bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("OCR Debug Output for %s\n", screenshotPath))
	sb.WriteString(fmt.Sprintf("Resolution: %dx%d characters\n\n", result.Width, result.Height))

	// Add recognized text
	sb.WriteString("Recognized Text:\n")
	sb.WriteString("--------------\n")

	if colorMode {
		// Use colored output for the recognized text
		coloredOutput := formatColoredTextOutput(result, false, false)
		sb.WriteString(coloredOutput)
	} else {
		// Use regular output
		for _, line := range result.Text {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatOutput(result *ocr.OCRResult, ansiMode, colorMode bool) string {
	var sb strings.Builder

	if result == nil {
		return "Error: No OCR result available"
	}

	sb.WriteString(fmt.Sprintf("OCR Result (%dx%d) with ANSI visualization:\n\n", result.Width, result.Height))

	// Process each character bitmap
	for i, bitmap := range result.CharBitmaps {
		// Calculate row and column
		row := i / result.Width
		col := i % result.Width

		if bitmap.Char != "" {
			sb.WriteString(fmt.Sprintf("\nCharacter '%s' at position (%d,%d):\n", bitmap.Char, row, col))
		} else {
			sb.WriteString(fmt.Sprintf("\nUNRECOGNIZED CHARACTER at position (%d,%d):\n", row, col))
		}

		// Add the formatted bitmap output
		if ansiMode {
			sb.WriteString(render.FormatBitmapOutput(&bitmap, true, colorMode))
		} else {
			// If ansiMode is not enabled, only print the hex representation
			sb.WriteString(fmt.Sprintf("Hex bitmap: %s\n", render.FormatBitmapAsHex(&bitmap)))
		}
	}

	return sb.String()
}

func formatSingleCharOutput(result *ocr.OCRResult, row, col int) string {
	var sb strings.Builder

	if result == nil || len(result.CharBitmaps) == 0 {
		return "Error: No OCR result available"
	}

	charIndex := 0 // In single-char mode, we only have one character at index 0
	if charIndex >= len(result.CharBitmaps) {
		return fmt.Sprintf("Error: No bitmap available for character at position (%d,%d)", row, col)
	}

	bitmap := result.CharBitmaps[charIndex]

	// If ansiMode is not enabled, only print the hex representation
	if !ansiMode {
		if bitmap.Char != "" {
			return fmt.Sprintf("Character '%s' at position (%d,%d): %s",
				bitmap.Char, row, col, render.FormatBitmapAsHex(&bitmap))
		} else {
			return fmt.Sprintf("UNRECOGNIZED CHARACTER at position (%d,%d): %s",
				row, col, render.FormatBitmapAsHex(&bitmap))
		}
	}

	// Print character information with ANSI visualization
	if bitmap.Char != "" {
		sb.WriteString(fmt.Sprintf("Character '%s' at position (%d,%d):\n\n", bitmap.Char, row, col))
	} else {
		sb.WriteString(fmt.Sprintf("UNRECOGNIZED CHARACTER at position (%d,%d):\n\n", row, col))
	}

	// Add the formatted bitmap output
	sb.WriteString(render.FormatBitmapOutput(&bitmap, true, colorMode))

	return sb.String()
}

func init() {
	// Add OCR command to root command
	rootCmd.AddCommand(ocrCmd)

	// Add subcommands to OCR command
	ocrCmd.AddCommand(ocrVmCmd)
	ocrCmd.AddCommand(ocrFileCmd)

	// Add flags to VM command
	ocrVmCmd.Flags().IntVar(&screenWidth, "width", 160, "Screen width in characters")
	ocrVmCmd.Flags().IntVar(&screenHeight, "height", 50, "Screen height in characters")
	ocrVmCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug output")
	ocrVmCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrVmCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")
	ocrVmCmd.Flags().BoolVar(&filterBlankLines, "filter", false, "Filter out blank lines from output")
	ocrVmCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "l", false, "Show line numbers (0-based) with colored output")
	ocrVmCmd.Flags().BoolVar(&updateTraining, "update-training", false, "Interactively train unrecognized characters")
	ocrVmCmd.Flags().BoolVar(&renderSpecialChars, "render", false, "Render special characters (spaces, tabs, newlines) visually")

	// Add flags to File command
	ocrFileCmd.Flags().IntVar(&screenWidth, "width", 160, "Screen width in characters")
	ocrFileCmd.Flags().IntVar(&screenHeight, "height", 50, "Screen height in characters")
	ocrFileCmd.Flags().IntVarP(&columns, "columns", "c", 0, "Number of columns (overrides width)")
	ocrFileCmd.Flags().IntVarP(&rows, "rows", "r", 0, "Number of rows (overrides height)")
	ocrFileCmd.Flags().StringVar(&resolution, "res", "", "Resolution in format 'columns x rows' (e.g., '160x50')")
	ocrFileCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug output")
	ocrFileCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrFileCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")
	ocrFileCmd.Flags().BoolVar(&filterBlankLines, "filter", false, "Filter out blank lines from output")
	ocrFileCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "l", false, "Show line numbers (0-based) with colored output")
	ocrFileCmd.Flags().BoolVar(&singleChar, "single-char", false, "Extract a single character")
	ocrFileCmd.Flags().IntVar(&charRow, "char-row", 0, "Row of the character to extract")
	ocrFileCmd.Flags().IntVar(&charCol, "char-col", 0, "Column of the character to extract")

	// Add crop flags
	ocrFileCmd.Flags().BoolVar(&cropEnabled, "crop", false, "Enable cropping mode")
	ocrFileCmd.Flags().StringVar(&cropRowsStr, "crop-rows", "", "Crop rows range in format 'start:end' (e.g., '5:20')")
	ocrFileCmd.Flags().StringVar(&cropColsStr, "crop-cols", "", "Crop columns range in format 'start:end' (e.g., '10:50')")
	ocrFileCmd.Flags().BoolVar(&updateTraining, "update-training", false, "Interactively train unrecognized characters")
	ocrFileCmd.Flags().BoolVar(&renderSpecialChars, "render", false, "Render special characters (spaces, tabs, newlines) visually")
}

// handleInteractiveTraining processes unrecognized characters using the beautiful new batch system
func handleInteractiveTraining(result *ocr.OCRResult, trainingDataPath string) error {
	// Get terminal dimensions for multi-character layout
	termWidth, _, err := training.GetTerminalDimensions()
	if err != nil {
		fmt.Printf("Warning: Could not detect terminal size, using defaults: %v\n", err)
		termWidth = 80
	}

	// Create character batches for multi-character display
	batches := training.CreateCharacterBatches(result, termWidth)

	if len(batches) == 0 {
		fmt.Println("No unrecognized characters found!")
		return nil
	}

	fmt.Printf("Found %d unrecognized character patterns. Starting interactive training...\n",
		func() int {
			total := 0
			for _, batch := range batches {
				total += len(batch.Bitmaps)
			}
			return total
		}())

	// Load existing training data
	trainingData, err := ocr.LoadTrainingData(trainingDataPath)
	if err != nil {
		fmt.Printf("Warning: Could not load existing training data: %v\n", err)
		trainingData = &ocr.TrainingData{
			BitmapMap: make(map[string]string),
		}
	}

	// Set up graceful shutdown
	reader := bufio.NewReader(os.Stdin)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Handle Ctrl+C
	go func() {
		<-c
		fmt.Printf("\n\nInterrupted! Saving training data...\n")
		if err := ocr.SaveTrainingData(trainingData, trainingDataPath); err != nil {
			fmt.Printf("Error saving training data: %v\n", err)
		} else {
			fmt.Printf("Training data saved to: %s\n", trainingDataPath)
		}
		os.Exit(0)
	}()

	// Process each batch with the beautiful new interface
	modified := false
	for batchNum, batch := range batches {
		// Display the batch
		batchDisplay := training.RenderCharacterBatch(batch, batchNum+1, len(batches))
		fmt.Print(batchDisplay)

		// Process input for this batch
		batchMappings := training.ProcessBatchInput(batch, reader)

		// Apply mappings to training data
		for hexKey, char := range batchMappings {
			trainingData.BitmapMap[hexKey] = char
			modified = true

			// Update all matching characters in the result
			for i := range result.CharBitmaps {
				if result.CharBitmaps[i].Char == "" || result.CharBitmaps[i].Char == "?" {
					if render.FormatBitmapAsHex(&result.CharBitmaps[i]) == hexKey {
						result.CharBitmaps[i].Char = char
					}
				}
			}

			fmt.Printf("Added character '%s' to training data.\n", char)
		}

		// Save training data after each batch
		if len(batchMappings) > 0 {
			if err := ocr.SaveTrainingData(trainingData, trainingDataPath); err != nil {
				fmt.Printf("Error saving training data: %v\n", err)
			} else {
				fmt.Printf("Training data saved to %s\n", trainingDataPath)
			}
		}

		// Continue to next batch automatically (user can Ctrl+C to stop)
	}

	if !modified {
		fmt.Println("\nNo changes made to training data.")
	} else {
		fmt.Printf("\nTraining complete! Updated training data saved to: %s\n", trainingDataPath)
	}

	return nil
}
