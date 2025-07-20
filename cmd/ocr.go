package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/qmp/internal/logging"
	"github.com/jeeftor/qmp/internal/ocr"
	"github.com/jeeftor/qmp/internal/qmp"
	"github.com/spf13/cobra"
	_ "github.com/spf13/viper"
)

var (
	// OCR command flags
	resolution     string
	screenWidth    int
	screenHeight   int
	columns        int
	rows           int
	debugMode      bool
	ansiMode       bool
	colorMode      bool
	cropEnabled    bool
	cropRowsStr    string
	cropColsStr    string
	cropStartRow   int
	cropEndRow     int
	cropStartCol   int
	cropEndCol     int
	singleChar     bool
	charRow        int
	charCol        int
	trainingDataPath string
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
  qmp ocr vm 106 training-data.json text.txt --width 80 --height 25

  # Use debug mode to see character representations
  qmp ocr vm 106 training-data.json text.txt --width 80 --height 25 --debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr vm 106 training-data.json text.txt --width 80 --height 25 --ansi`,
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

		// Format the output
		var output string
		if debugMode {
			output = formatDebugOutput(result, tmpFile.Name())
		} else if ansiMode {
			output = formatAnsiOutput(result)
		} else if singleChar {
			output = formatSingleCharOutput(result, charRow, charCol)
		} else {
			output = formatTextOutput(result)
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
  --res: Set the resolution in the format "columns x rows" (e.g., "80x25")
  -c and -r: Short form for columns and rows

You can also crop the image to process only a portion:
  --crop: Enable cropping mode
  --crop-rows: Specify start and end rows (e.g., "5:20" for rows 5 through 20)
  --crop-cols: Specify start and end columns (e.g., "10:50" for columns 10 through 50)

Output options:
  --debug: Output ASCII/ANSI representation of recognized characters
  --ansi: Output ANSI escape sequences for character bitmaps

Examples:
  # Perform OCR with explicit width and height
  qmp ocr file screenshot.ppm training-data.json text.txt --width 80 --height 25

  # Use the resolution format
  qmp ocr file screenshot.ppm training-data.json text.txt --res 80x25

  # Use short form flags
  qmp ocr file screenshot.ppm training-data.json text.txt -c 80 -r 25

  # Use debug mode to see character representations
  qmp ocr file screenshot.ppm training-data.json text.txt --res 80x25 --debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr file screenshot.ppm training-data.json text.txt --res 80x25 --ansi

  # Crop the image to process only rows 5-20 and columns 10-50
  qmp ocr file screenshot.ppm training-data.json text.txt --res 80x25 --crop --crop-rows 5:20 --crop-cols 10:50

  # Print results to screen instead of saving to a file
  qmp ocr file screenshot.ppm training-data.json --res 80x25`,
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

		// Format the output
		var output string
		if debugMode {
			output = formatDebugOutput(result, inputFile)
		} else if ansiMode {
			output = formatAnsiOutput(result)
		} else {
			output = formatTextOutput(result)
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
			fmt.Println("Error: Resolution must be in the format 'columns x rows' (e.g., '80x25')")
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

func formatTextOutput(result *ocr.OCRResult) string {
	var sb strings.Builder

	for _, line := range result.Text {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatDebugOutput(result *ocr.OCRResult, screenshotPath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("OCR Debug Output for %s\n", screenshotPath))
	sb.WriteString(fmt.Sprintf("Resolution: %dx%d characters\n\n", result.Width, result.Height))

	// Add recognized text
	sb.WriteString("Recognized Text:\n")
	sb.WriteString("--------------\n")
	for _, line := range result.Text {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatAnsiOutput(result *ocr.OCRResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("OCR Result (%dx%d) with ANSI visualization:\n\n", result.Width, result.Height))

	// Print recognized text
	for _, line := range result.Text {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\nCharacter Bitmaps:\n\n")

	// Process each character bitmap
	for i, bitmap := range result.CharBitmaps {
		row := i / result.Width
		col := i % result.Width

		// Print character information
		if bitmap.Char != "" {
			sb.WriteString(fmt.Sprintf("\nCharacter '%s' at position (%d,%d):\n", bitmap.Char, row, col))
		} else {
			sb.WriteString(fmt.Sprintf("\nUNRECOGNIZED CHARACTER at position (%d,%d):\n", row, col))
			// Add hex representation of the bitmap
			sb.WriteString(formatBitmapAsHex(&bitmap))
			sb.WriteString("\n")
		}

		// Print the bitmap using ANSI escape sequences
		for bitmapY := 0; bitmapY < bitmap.Height; bitmapY++ {
			for bitmapX := 0; bitmapX < bitmap.Width; bitmapX++ {
				if bitmapY < len(bitmap.Data) && bitmapX < len(bitmap.Data[bitmapY]) {
					if bitmap.Data[bitmapY][bitmapX] {
						if colorMode && bitmapY < len(bitmap.Colors) && bitmapX < len(bitmap.Colors[bitmapY]) && bitmap.Colors[bitmapY][bitmapX] != nil {
							// Use original color if color mode is enabled
							r, g, b, _ := bitmap.Colors[bitmapY][bitmapX].RGBA()
							// Convert 16-bit color to 8-bit
							r8 := uint8(r >> 8)
							g8 := uint8(g >> 8)
							b8 := uint8(b >> 8)
							// Use true color ANSI escape sequence
							sb.WriteString(fmt.Sprintf("\033[48;2;%d;%d;%dm  ", r8, g8, b8))
						} else {
							// Character pixel - use ANSI escape sequence for white background (double width)
							sb.WriteString("\033[47m  ")
						}
					} else {
						// Empty space - use ANSI escape sequence for black background (double width)
						sb.WriteString("\033[40m  ")
					}
				}
			}
			// Reset color at end of line
			sb.WriteString("\033[0m\n")
		}
	}

	return sb.String()
}

func formatSingleCharOutput(result *ocr.OCRResult, row, col int) string {
	var sb strings.Builder

	// Check if the requested character is within bounds
	if row < 0 || row >= result.Height || col < 0 || col >= result.Width {
		return fmt.Sprintf("Error: Character position (%d,%d) is out of bounds for a %dx%d grid",
			row, col, result.Height, result.Width)
	}

	charIndex := row*result.Width + col
	if charIndex >= len(result.CharBitmaps) {
		return fmt.Sprintf("Error: No bitmap available for character at position (%d,%d)", row, col)
	}

	bitmap := result.CharBitmaps[charIndex]

	// If ansiMode is not enabled, only print the hex representation
	if !ansiMode {
		if bitmap.Char != "" {
			return fmt.Sprintf("Character '%s' at position (%d,%d):\n%s",
				bitmap.Char, row, col, formatBitmapAsHex(&bitmap))
		} else {
			return fmt.Sprintf("UNRECOGNIZED CHARACTER at position (%d,%d):\n%s",
				row, col, formatBitmapAsHex(&bitmap))
		}
	}

	// Print character information with ANSI visualization
	if bitmap.Char != "" {
		sb.WriteString(fmt.Sprintf("Character '%s' at position (%d,%d):\n\n", bitmap.Char, row, col))
	} else {
		sb.WriteString(fmt.Sprintf("UNRECOGNIZED CHARACTER at position (%d,%d):\n", row, col))
		// Add hex representation of the bitmap
		sb.WriteString(formatBitmapAsHex(&bitmap))
		sb.WriteString("\n\n")
	}

	// Print the bitmap using ANSI escape sequences
	for bitmapY := 0; bitmapY < bitmap.Height; bitmapY++ {
		for bitmapX := 0; bitmapX < bitmap.Width; bitmapX++ {
			if bitmapY < len(bitmap.Data) && bitmapX < len(bitmap.Data[bitmapY]) {
				if bitmap.Data[bitmapY][bitmapX] {
					if colorMode && bitmapY < len(bitmap.Colors) && bitmapX < len(bitmap.Colors[bitmapY]) && bitmap.Colors[bitmapY][bitmapX] != nil {
						// Use original color if color mode is enabled
						r, g, b, _ := bitmap.Colors[bitmapY][bitmapX].RGBA()
						// Convert 16-bit color to 8-bit
						r8 := uint8(r >> 8)
						g8 := uint8(g >> 8)
						b8 := uint8(b >> 8)
						// Use true color ANSI escape sequence
						sb.WriteString(fmt.Sprintf("\033[48;2;%d;%d;%dm  ", r8, g8, b8))
					} else {
						// Character pixel - use ANSI escape sequence for white background (double width)
						sb.WriteString("\033[47m  ")
					}
				} else {
					// Empty space - use ANSI escape sequence for black background (double width)
					sb.WriteString("\033[40m  ")
				}
			}
		}
		// Reset color at end of line
		sb.WriteString("\033[0m\n")
	}

	return sb.String()
}

// formatBitmapAsHex converts a bitmap to a hex representation
func formatBitmapAsHex(bitmap *ocr.CharacterBitmap) string {
	var hexString strings.Builder
	hexString.WriteString("Hex bitmap: 0x")

	// Process each row and combine into a single hex string
	for y := 0; y < bitmap.Height; y++ {
		var rowValue uint32

		// Convert row to binary
		for x := 0; x < bitmap.Width; x++ {
			if x < len(bitmap.Data[y]) && bitmap.Data[y][x] {
				// Set bit if pixel is on
				rowValue |= 1 << (bitmap.Width - 1 - x)
			}
		}

		// Format as hex without 0x prefix
		hexString.WriteString(fmt.Sprintf("%0*X", (bitmap.Width+3)/4, rowValue))
	}

	return hexString.String()
}

func init() {
	// Add OCR command to root command
	rootCmd.AddCommand(ocrCmd)

	// Add subcommands to OCR command
	ocrCmd.AddCommand(ocrVmCmd)
	ocrCmd.AddCommand(ocrFileCmd)

	// Add flags to VM command
	ocrVmCmd.Flags().IntVar(&screenWidth, "width", 80, "Screen width in characters")
	ocrVmCmd.Flags().IntVar(&screenHeight, "height", 25, "Screen height in characters")
	ocrVmCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug output")
	ocrVmCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrVmCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")

	// Add flags to File command
	ocrFileCmd.Flags().IntVar(&screenWidth, "width", 80, "Screen width in characters")
	ocrFileCmd.Flags().IntVar(&screenHeight, "height", 25, "Screen height in characters")
	ocrFileCmd.Flags().IntVarP(&columns, "columns", "c", 0, "Number of columns (overrides width)")
	ocrFileCmd.Flags().IntVarP(&rows, "rows", "r", 0, "Number of rows (overrides height)")
	ocrFileCmd.Flags().StringVar(&resolution, "res", "", "Resolution in format 'columns x rows' (e.g., '80x25')")
	ocrFileCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug output")
	ocrFileCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrFileCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")
	ocrFileCmd.Flags().BoolVar(&singleChar, "single-char", false, "Extract a single character")
	ocrFileCmd.Flags().IntVar(&charRow, "char-row", 0, "Row of the character to extract")
	ocrFileCmd.Flags().IntVar(&charCol, "char-col", 0, "Column of the character to extract")

	// Add crop flags
	ocrFileCmd.Flags().BoolVar(&cropEnabled, "crop", false, "Enable cropping mode")
	ocrFileCmd.Flags().StringVar(&cropRowsStr, "crop-rows", "", "Crop rows range in format 'start:end' (e.g., '5:20')")
	ocrFileCmd.Flags().StringVar(&cropColsStr, "crop-cols", "", "Crop columns range in format 'start:end' (e.g., '10:50')")
}
