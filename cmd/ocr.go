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
	"github.com/jeeftor/qmp-controller/internal/filesystem"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/utils"
	"github.com/jeeftor/qmp-controller/internal/render"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"github.com/jeeftor/qmp-controller/internal/training"
	"github.com/jeeftor/qmp-controller/internal/validation"
	"github.com/spf13/cobra"
	_ "github.com/spf13/viper"
)

// Argument name constants
const (
	ArgVMID           = "vmid"
	ArgTrainingData   = "training-data"
	ArgInputImageFile = "input-image-file"
	ArgOutputTextFile = "output-text-file"
	ArgString         = "search-string"
	ArgPattern        = "regex-pattern"
)

// Common description constants
const (
	SearchDescription = `Search for a %s in OCR text results (scanned bottom-up).
Returns all matches by default, or first match only with the --first flag.
Exit codes: 0 = found, 1 = not found, 2 = OCR error`
	RegexDescription = `Search for a regular expression %s in OCR text results (scanned bottom-up).
Returns all matches by default, or first match only with the --first flag.
Exit codes: 0 = found, 1 = not found, 2 = OCR error, 3 = invalid regex`
	SubcommandDescription = `Use subcommands 'vm' or 'file' to specify the input source.`
)

// syncConfigFromFlags populates the OCR config from command flags
// Enhanced to respect priority: CLI flags > environment variables > defaults
func syncConfigFromFlags() {
	// Only override with CLI flag values if they differ from defaults
	// This preserves the environment variable values loaded in NewOCRConfig()

	// Screen dimensions - only override if explicitly set via flags
	if columns != qmp.DEFAULT_WIDTH {
		ocrConfig.Columns = columns
	}
	if rows != qmp.DEFAULT_HEIGHT {
		ocrConfig.Rows = rows
	}

	// Boolean flags - these will be false by default, so we need to check if flag was set
	// For now, we'll assume any true value means the flag was explicitly set
	if ansiMode {
		ocrConfig.AnsiMode = ansiMode
	}
	if colorMode {
		ocrConfig.ColorMode = colorMode
	}
	if filterBlankLines {
		ocrConfig.FilterBlankLines = filterBlankLines
	}
	if showLineNumbers {
		ocrConfig.ShowLineNumbers = showLineNumbers
	}
	if updateTraining {
		ocrConfig.UpdateTraining = updateTraining
	}
	if renderSpecialChars {
		ocrConfig.RenderSpecialChars = renderSpecialChars
	}
	if singleChar {
		ocrConfig.SingleChar = singleChar
	}

	// Numeric flags - only override if non-zero (assuming 0 is default)
	if charRow != 0 {
		ocrConfig.CharRow = charRow
	}
	if charCol != 0 {
		ocrConfig.CharCol = charCol
	}
	if cropStartRow != 0 {
		ocrConfig.CropStartRow = cropStartRow
	}
	if cropEndRow != 0 {
		ocrConfig.CropEndRow = cropEndRow
	}
	if cropStartCol != 0 {
		ocrConfig.CropStartCol = cropStartCol
	}
	if cropEndCol != 0 {
		ocrConfig.CropEndCol = cropEndCol
	}

	// String flags - only override if non-empty
	if cropRowsStr != "" {
		ocrConfig.CropRowsStr = cropRowsStr
		ocrConfig.CropEnabled = true
	}
	if cropColsStr != "" {
		ocrConfig.CropColsStr = cropColsStr
		ocrConfig.CropEnabled = true
	}

	// Training data path - enhanced logic to respect priority
	if trainingDataPath != "" {
		ocrConfig.TrainingDataPath = trainingDataPath
	} else if ocrConfig.TrainingDataPath == "" {
		// If no CLI flag and no env var, use default
		ocrConfig.TrainingDataPath = qmp.GetDefaultTrainingDataPath()
	}

	// Auto-enable cropping if crop parameters are provided
	if ocrConfig.CropRowsStr != "" || ocrConfig.CropColsStr != "" {
		ocrConfig.CropEnabled = true
	}
}

var (
	// Global OCR configuration instance
	ocrConfig = ocr.NewOCRConfig()

	// Command-line flags
	columns            int
	rows               int
	ansiMode           bool
	colorMode          bool
	cropRowsStr        string
	cropColsStr        string
	cropStartRow       int
	cropEndRow         int
	cropStartCol       int
	cropEndCol         int
	singleChar         bool
	charRow            int
	charCol            int
	trainingDataPath   string
	filterBlankLines   bool
	showLineNumbers    bool
	updateTraining     bool
	renderSpecialChars bool
	watchMode          bool
	watchInterval      int
)

// ocrCmd represents the ocr command
var ocrCmd = &cobra.Command{
	Use:   "ocr",
	Short: "Perform OCR on VM console screen or image file",
	Long: `Perform Optical Character Recognition (OCR) on a VM console screen or image file.
This command captures a screenshot or processes an image file, divides it into character cells based on
the specified columns and rows, and attempts to recognize text characters.

In debug mode, it will output ASCII/ANSI representation of recognized characters.
In ANSI mode, it will output ANSI escape sequences for character bitmaps.

Use subcommands 'vm' or 'file' to specify the input source.`,
}

// ocrVmCmd represents the ocr vm command
var ocrVmCmd = &cobra.Command{
	Use:   fmt.Sprintf("vm [%s] [%s] [%s]", ArgVMID, ArgTrainingData, ArgOutputTextFile),
	Short: "Perform OCR on VM console screen",
	Long: fmt.Sprintf(`Perform Optical Character Recognition (OCR) on a VM console screen.
This command captures a screenshot from the specified %s, divides it into character cells based on
the specified columns and rows, and attempts to recognize text characters. The %s is used to improve character recognition,
and the results can be saved to an optional %s.

✨ FLEXIBLE ARGUMENT ORDER: Arguments can be provided in any order - the system will auto-detect file types!

Examples:
  # Standard order
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50

  # Flexible order - same result!
  qmp ocr vm training.json 106 text.txt --columns 160 --rows 50
  qmp ocr vm text.txt training.json 106 --columns 160 --rows 50

  # Use environment variables
  export QMP_TRAINING_DATA=training.json
  export QMP_COLUMNS=160
  export QMP_ROWS=50
  qmp ocr vm 106 text.txt  # training data and dimensions from env vars

  # Use debug mode to see character representations
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50 --log-level debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50 --ansi

  # Filter out blank lines from the output
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50 --filter

  # Show line numbers with the output
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50 --line-numbers

  # Interactive training for unrecognized characters
  qmp ocr vm 106 training.json text.txt --columns 160 --rows 50 --update-training

  # Watch mode - continuously monitor screen changes with TUI interface
  qmp ocr vm 106 training.json --watch --watch-interval 2

  # Watch mode with filtering and line numbers
  qmp ocr vm 106 training.json --watch --filter --line-numbers`, ArgVMID, ArgTrainingData, ArgOutputTextFile),
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Use flexible argument parsing
		parser := ocr.ParseArguments(args, true) // expectVMID = true

		// Validate parsed arguments
		if validationErrors := parser.Validate(true, false, false); len(validationErrors) > 0 {
			fmt.Fprintf(os.Stderr, "Argument parsing errors:\n")
			for _, err := range validationErrors {
				fmt.Fprintf(os.Stderr, "  • %s\n", err)
			}
			fmt.Fprintf(os.Stderr, "\n💡 Auto-detected argument types:\n")
			for i, arg := range args {
				fileType := ocr.DetectFileType(arg)
				fmt.Fprintf(os.Stderr, "  [%d] %s -> %s\n", i+1, arg, fileType.String())
			}
			os.Exit(1)
		}

		// Extract parsed values
		vmid := parser.VMID
		if parser.TrainingData != "" {
			ocrConfig.TrainingDataPath = parser.TrainingData
		}
		outputTextFile := parser.OutputFile

		// Log the initial training data path
		logging.Info("Initial training data path", "path", ocrConfig.TrainingDataPath)

		// Convert training data path to absolute path if it's not empty
		if ocrConfig.TrainingDataPath != "" {
			absPath, err := filepath.Abs(ocrConfig.TrainingDataPath)
			if err != nil {
				logging.Warn("Failed to convert training data path to absolute path",
					"path", ocrConfig.TrainingDataPath,
					"error", err)
			} else {
				ocrConfig.TrainingDataPath = absPath
				logging.Info("Using absolute training data path", "path", ocrConfig.TrainingDataPath)
			}
		}

		// Sync flags to config structure
		syncConfigFromFlags()

		// Comprehensive configuration validation
		validator := validation.NewConfigValidator()
		remoteTempPath := getRemoteTempPath()
		validationResult := validator.ValidateOCRConfig(ocrConfig, vmid, remoteTempPath)

		if !validationResult.Valid {
			logging.Error("OCR configuration validation failed",
				"vmid", vmid,
				"validation_errors", len(validationResult.Errors),
				"validation_warnings", len(validationResult.Warnings))

			// Display detailed validation errors to user
			fmt.Print(validation.FormatValidationErrors(validationResult))
			os.Exit(1)
		}

		// Log validation warnings if any
		if len(validationResult.Warnings) > 0 {
			for _, warning := range validationResult.Warnings {
				logging.Warn("Configuration validation warning", "warning", warning)
			}
		}

		// Fallback to basic validation for any edge cases
		if err := ocrConfig.ValidateAndParse(); err != nil {
			utils.FatalErrorWithCode(err, "Basic OCR configuration validation failed", utils.ExitCodeValidation)
		}

		// Log the training data path after validation
		logging.Info("Final training data path", "path", ocrConfig.TrainingDataPath)

		// Validate screen dimensions
		if ocrConfig.Columns <= 0 || ocrConfig.Rows <= 0 {
			utils.ConfigurationError("Screen dimensions:", "Columns and rows must be positive integers")
		}

		// Create output directory if it doesn't exist
		if outputTextFile != "" {
			if err := filesystem.EnsureDirectoryForFile(outputTextFile); err != nil {
				utils.FileSystemError("create output directory", outputTextFile, err)
			}
		}

		client, err := ConnectToVM(vmid)
		if err != nil {
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()

		// Take a temporary screenshot using centralized helper
		tmpFile, err := TakeTemporaryScreenshot(client, "qmp-ocr")
		if err != nil {
			utils.FatalErrorWithCode(err, "Failed to take screenshot", utils.ExitCodeGeneral)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Process the screenshot with structured logging
		var result *ocr.OCRResult
		var processErr error

		processErr = logging.LogOperation("ocr_process", vmid, func() error {
			if ocrConfig.CropEnabled {
				// Process with cropping
				result, processErr = ocr.ProcessScreenshotWithCropAndTrainingData(
					tmpFile.Name(),
					ocrConfig.TrainingDataPath,
					ocrConfig.Columns, ocrConfig.Rows,
					ocrConfig.CropStartRow, ocrConfig.CropEndRow,
					ocrConfig.CropStartCol, ocrConfig.CropEndCol)
			} else {
				// Process the full image
				result, processErr = ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), ocrConfig.TrainingDataPath, ocrConfig.Columns, ocrConfig.Rows)
			}
			return processErr
		})

		if processErr != nil {
			utils.ProcessingError("processing file for OCR", processErr)
		}

		// Log OCR results
		charactersFound := 0
		if result != nil {
			for _, row := range result.Text {
				charactersFound += len(strings.TrimSpace(row))
			}
		}
		logging.LogOCR(vmid, tmpFile.Name(), charactersFound, 0, true)

		// Handle interactive training if enabled
		if ocrConfig.UpdateTraining {
			if err := handleInteractiveTraining(result, ocrConfig.TrainingDataPath); err != nil {
				utils.FatalError(err, "interactive training failed")
			}

			// Re-run character recognition with updated training data to refresh the Text output
			updatedTrainingData, err := ocr.LoadTrainingData(ocrConfig.TrainingDataPath)
			if err == nil && updatedTrainingData != nil {
				if err := ocr.RecognizeCharacters(result, updatedTrainingData); err != nil {
					logging.Warn("Failed to re-recognize characters after training", "error", err)
				}
			}
		}

		// Format the output
		var output string
		if ocrConfig.AnsiMode {
			output = formatOutput(result, true, ocrConfig.ColorMode)
		} else if ocrConfig.RenderSpecialChars {
			output = formatTextOutputWithSpecialChars(result, ocrConfig.FilterBlankLines, ocrConfig.ShowLineNumbers)
		} else if ocrConfig.ColorMode {
			output = formatColoredTextOutput(result, ocrConfig.FilterBlankLines, ocrConfig.ShowLineNumbers)
		} else {
			output = formatTextOutput(result, ocrConfig.FilterBlankLines, ocrConfig.ShowLineNumbers)
		}

		// Save or print the output
		if outputTextFile != "" {
			// Save to output file
			if err := os.WriteFile(outputTextFile, []byte(output), 0644); err != nil {
				utils.FileSystemError("save OCR results to", outputTextFile, err)
			}
			logging.Info("OCR results saved successfully", "output_file", outputTextFile)
		} else {
			// Check if watch mode is enabled
			if watchMode {
				// Start watch mode TUI with change detection
				runOCRWatchTUI(client, vmid, ocrConfig, outputTextFile)
			} else {
				// Print to screen (regular one-time OCR)
				fmt.Println(output)
			}
		}
	},
}


// ocrFileCmd represents the ocr file command
var ocrFileCmd = &cobra.Command{
	Use:   fmt.Sprintf("file [%s] [%s] [%s]", ArgTrainingData, ArgInputImageFile, ArgOutputTextFile),
	Short: "Perform OCR on an image file",
	Long: fmt.Sprintf(`Perform Optical Character Recognition (OCR) on an image file.
This command processes an existing %s, divides it into character cells based on
the specified columns and rows, and attempts to recognize text characters. The %s is used to improve character recognition,
and the results can be saved to an optional %s.

You can specify the resolution with:
  --columns and --rows: Set the number of columns and rows
  -c and -r: Short form for columns and rows

You can also crop the image to process only a portion:
  --crop-rows: Specify start and end rows (e.g., "5:20" for rows 5 through 20)
  --crop-cols: Specify start and end columns (e.g., "10:50" for columns 10 through 50)

Output options:
  --log-level debug: Output ASCII/ANSI representation of recognized characters
  --ansi: Output ANSI escape sequences for character bitmaps
  --filter: Filter out blank lines from the text output
  --line-numbers, -n: Show 0-based line numbers with colored output

Examples:
  # Perform OCR with explicit columns and rows
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50

  # Use short form flags
  qmp ocr file training.json screenshot.ppm text.txt -c 160 -r 50

  # Use debug mode to see character representations
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --log-level debug

  # Use ANSI mode to see character bitmaps with ANSI escape sequences
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --ansi

  # Crop the image to process only rows 5-20 and columns 10-50
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --crop-rows 5:20 --crop-cols 10:50

  # Print results to screen instead of saving to a file
  qmp ocr file training.json screenshot.ppm --columns 160 --rows 50

  # Filter out blank lines from the output
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --filter

  # Show line numbers with the output
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --line-numbers

  # Interactive training for unrecognized characters
  qmp ocr file training.json screenshot.ppm text.txt --columns 160 --rows 50 --update-training`, ArgInputImageFile, ArgTrainingData, ArgOutputTextFile),
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Use flexible argument parsing
		parser := ocr.ParseArguments(args, false) // expectVMID = false

		// Validate parsed arguments
		validationErrors := parser.Validate(false, false, false)

		// For file command, we need at least an image file
		if parser.ImageFile == "" {
			validationErrors = append(validationErrors, "Image file is required")
		}

		if len(validationErrors) > 0 {
			fmt.Fprintf(os.Stderr, "Argument parsing errors:\n")
			for _, err := range validationErrors {
				fmt.Fprintf(os.Stderr, "  • %s\n", err)
			}
			fmt.Fprintf(os.Stderr, "\n💡 Auto-detected argument types:\n")
			for i, arg := range args {
				fileType := ocr.DetectFileType(arg)
				fmt.Fprintf(os.Stderr, "  [%d] %s -> %s\n", i+1, arg, fileType.String())
			}
			os.Exit(1)
		}

		// Extract parsed values
		inputImageFile := parser.ImageFile
		if parser.TrainingData != "" {
			ocrConfig.TrainingDataPath = parser.TrainingData
		}

		// Sync flags to config structure for validation
		syncConfigFromFlags()

		// Comprehensive configuration validation
		validator := validation.NewConfigValidator()
		remoteTempPath := getRemoteTempPath()
		validationResult := validator.ValidateOCRConfig(ocrConfig, "", remoteTempPath) // No vmid for file mode

		if !validationResult.Valid {
			logging.Error("OCR file configuration validation failed",
				"input_file", inputImageFile,
				"training_data", trainingDataPath,
				"validation_errors", len(validationResult.Errors),
				"validation_warnings", len(validationResult.Warnings))

			// Display detailed validation errors to user
			fmt.Fprint(os.Stderr, validation.FormatValidationErrors(validationResult))
			os.Exit(1)
		}

		// Log validation warnings if any
		if len(validationResult.Warnings) > 0 {
			for _, warning := range validationResult.Warnings {
				logging.Warn("Configuration validation warning", "warning", warning)
			}
		}
		// Handle output file from flexible parsing
		outputTextFile := parser.OutputFile
		if outputTextFile != "" {
			// Create output directory if it doesn't exist
			if err := filesystem.EnsureDirectoryForFile(outputTextFile); err != nil {
				utils.FileSystemError("create output directory", outputTextFile, err)
			}
		}

		// If single character mode is enabled, extract just that character
		if singleChar {
			bitmap, err := ocr.ExtractSingleCharacter(inputImageFile, columns, rows, charRow, charCol)
			if err != nil {
				utils.ProcessingError("extracting character", err)
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
				if len(result.CharBitmaps) > 0 && len(result.Text) > 0 && len(result.Text[0]) > 0 {
					result.CharBitmaps[0].Char = string(result.Text[0][0])
				}
			}

			// Format the output
			output := formatSingleCharOutput(result, 0, 0)

			// Save or print the output
			if outputTextFile != "" {
				if err := os.WriteFile(outputTextFile, []byte(output), 0644); err != nil {
					utils.FileSystemError("write", outputTextFile, err)
				}
				logging.Info("Character extraction saved successfully", "output_file", outputTextFile)
			} else {
				fmt.Println(output)
			}
			return
		}

		// Process the image
		var result *ocr.OCRResult
		var processErr error

		if cropRowsStr != "" || cropColsStr != "" {
			// Process with cropping
			result, processErr = ocr.ProcessScreenshotWithCropAndTrainingData(
				inputImageFile,
				trainingDataPath,
				columns, rows,
				cropStartRow, cropEndRow,
				cropStartCol, cropEndCol)
		} else {
			// Process the full image
			result, processErr = ocr.ProcessScreenshotWithTrainingData(inputImageFile, trainingDataPath, columns, rows)
		}

		if processErr != nil {
			utils.ProcessingError("processing file for OCR", processErr)
		}

		// Handle interactive training if enabled
		if updateTraining {
			if err := handleInteractiveTraining(result, trainingDataPath); err != nil {
				utils.FatalError(err, "interactive training failed")
			}

			// Re-run character recognition with updated training data to refresh the Text output
			updatedTrainingData, err := ocr.LoadTrainingData(trainingDataPath)
			if err == nil && updatedTrainingData != nil {
				if err := ocr.RecognizeCharacters(result, updatedTrainingData); err != nil {
					logging.Warn("Failed to re-recognize characters after training", "error", err)
				}
			}
		}

		// Format the output
		var output string
		if ansiMode {
			output = formatOutput(result, true, colorMode)
		} else if renderSpecialChars {
			output = formatTextOutputWithSpecialChars(result, filterBlankLines, showLineNumbers)
		} else if colorMode {
			output = formatColoredTextOutput(result, filterBlankLines, showLineNumbers)
		} else {
			output = formatTextOutput(result, filterBlankLines, showLineNumbers)
		}

		// Save or print the output
		if outputTextFile != "" {
			// Save to output file
			if err := os.WriteFile(outputTextFile, []byte(output), 0644); err != nil {
				utils.FileSystemError("save OCR results to", outputTextFile, err)
			}
			logging.Info("OCR results saved successfully", "output_file", outputTextFile)
		} else {
			// Print to screen
			fmt.Println(output)
		}
	},
}

func parseCropParameters() {
	// Parse crop rows
	if cropRowsStr != "" {
		parts := strings.Split(cropRowsStr, ":")
		if len(parts) != 2 {
			utils.CropFormatError("rows", "start:end")
		}

		fmt.Sscanf(parts[0], "%d", &cropStartRow)
		fmt.Sscanf(parts[1], "%d", &cropEndRow)

		if cropStartRow < 0 || cropEndRow < cropStartRow || cropEndRow >= rows {
			utils.RangeValidationError("crop_row", cropStartRow, cropEndRow, rows, "must be 0 <= start <= end < max_rows")
		}
	} else {
		cropStartRow = 0
		cropEndRow = rows - 1
	}

	// Parse crop columns
	if cropColsStr != "" {
		parts := strings.Split(cropColsStr, ":")
		if len(parts) != 2 {
			utils.CropFormatError("columns", "start:end")
		}

		fmt.Sscanf(parts[0], "%d", &cropStartCol)
		fmt.Sscanf(parts[1], "%d", &cropEndCol)

		if cropStartCol < 0 || cropEndCol < cropStartCol || cropEndCol >= columns {
			utils.RangeValidationError("crop_col", cropStartCol, cropEndCol, columns, "must be 0 <= start <= end < max_columns")
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

	// Create lipgloss styles for special characters
	lineNumStyle := styles.InfoStyle
	unrecognizedStyle := styles.ErrorStyle.Background(styles.ANSIColors["white"].Background)
	spaceStyle := styles.WarningStyle
	tabStyle := styles.BoldStyle.Foreground(styles.ANSIColors["magenta"].Foreground)
	newlineStyle := styles.SuccessStyle

	lineCounter := 0
	for _, line := range result.Text {
		// Filter out blank lines if requested
		if filterBlanks && strings.TrimSpace(line) == "" {
			continue
		}

		if lineNumbers {
			// Add colored line number prefix
			linePrefix := lineNumStyle.Render(fmt.Sprintf("%02d: ", lineCounter))
			sb.WriteString(linePrefix)
		}

		// Process each character to render special chars visually
		for _, char := range line {
			switch char {
			case ' ':
				// Render spaces as colored dots
				sb.WriteString(spaceStyle.Render("·"))
			case '\t':
				// Render tabs as colored arrows
				sb.WriteString(tabStyle.Render("→"))
			case '\r':
				// Render carriage returns as colored "CR"
				sb.WriteString(newlineStyle.Render("⏎"))
			case '\n':
				// Render newlines as colored "LF"
				sb.WriteString(newlineStyle.Render("↩"))
			case '?':
				// Color unrecognized characters
				sb.WriteString(unrecognizedStyle.Render(string(char)))
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

	// Create lipgloss styles for line numbers and unrecognized characters
	lineNumStyle := styles.InfoStyle
	unrecognizedStyle := styles.ErrorStyle.Background(styles.ANSIColors["white"].Background)

	lineCounter := 0
	for _, line := range result.Text {
		// Filter out blank lines if requested
		if filterBlanks && strings.TrimSpace(line) == "" {
			continue
		}

		if lineNumbers {
			// Add colored line number prefix
			linePrefix := lineNumStyle.Render(fmt.Sprintf("%02d: ", lineCounter))
			sb.WriteString(linePrefix)
		}

		// Color unrecognized characters
		for _, char := range line {
			if string(char) == ocr.UnknownCharIndicator {
				sb.WriteString(unrecognizedStyle.Render(string(char)))
			} else {
				sb.WriteString(string(char))
			}
		}
		sb.WriteString("\n")
		lineCounter++
	}

	return sb.String()
}

func formatColoredTextOutput(result *ocr.OCRResult, filterBlanks bool, lineNumbers bool) string {
	var sb strings.Builder
	// Create lipgloss styles for line numbers and unrecognized characters
	lineNumStyle := styles.InfoStyle
	unrecognizedStyle := styles.ErrorStyle.Background(styles.ANSIColors["white"].Background)

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
			linePrefix := lineNumStyle.Render(fmt.Sprintf("%02d: ", lineCounter))
			sb.WriteString(linePrefix)
		}

		// Process each character with its original color
		for x, char := range line {
			if string(char) == ocr.UnknownCharIndicator {
				sb.WriteString(unrecognizedStyle.Render(string(char)))
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

						// Convert to lipgloss color
						colorStyle := styles.CreateFgStyle(uint8(r8), uint8(g8), uint8(b8))
						sb.WriteString(colorStyle.Render(string(char)))
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

func getClosestLipglossColor(r, g, b uint32) string {
	// Use the centralized color mapping from styles package
	return string(styles.GetClosestLipglossColor(r, g, b))
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

// Search command variables
var (
	searchFirstOnly  bool
	searchQuiet      bool
	searchIgnoreCase bool
)

// processOCRResult performs OCR processing for both VM and file inputs
func processOCRResult(input string, trainingDataPath string, isVM bool) (*ocr.OCRResult, error) {
	var result *ocr.OCRResult
	var err error

	if isVM {
		// VM mode
		vmid := input
		client, err := ConnectToVM(vmid)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		// Take temporary screenshot using centralized helper
		tmpFile, err := TakeTemporaryScreenshot(client, "qmp-ocr-search")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		result, err = ocr.ProcessScreenshotWithTrainingData(tmpFile.Name(), trainingDataPath, columns, rows)
	} else {
		// File mode
		result, err = ocr.ProcessScreenshotWithTrainingData(input, trainingDataPath, columns, rows)
	}

	return result, err
}

// ocrFindCmd represents the ocr find command for string searching
var ocrFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Search for a string in OCR results",
	Long: fmt.Sprintf("%s.\n\n%s",
		fmt.Sprintf(SearchDescription, ArgString),
		SubcommandDescription),
}

// ocrFindVMCmd represents the ocr find vm command
var ocrFindVMCmd = &cobra.Command{
	Use:   fmt.Sprintf("vm [%s] [%s] [%s]", ArgVMID, ArgTrainingData, ArgString),
	Short: "Search for a string in VM console OCR results",
	Long: fmt.Sprintf("%s from a VM console.\n\nThe VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.\nThe training data can be provided as an argument or set via the QMP_TRAINING_DATA environment variable.\n\nExamples:\n  # Explicit arguments\n  qmp ocr find vm 123 training.json \"Login successful\"\n\n  # Using environment variables\n  export QMP_VM_ID=123\n  export QMP_TRAINING_DATA=training.json\n  qmp ocr find vm \"Login successful\"\n\n  # Search for \"error\" (case-insensitive, first match only)\n  qmp ocr find vm 123 training.json \"error\" --ignore-case --first\n\n  # Search with debug output and line numbers\n  qmp ocr find vm 123 training.json \"root@\" --log-level debug --line-numbers",
		fmt.Sprintf(SearchDescription, ArgString)),
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Use flexible argument parsing similar to other commands
		parser := ocr.ParseArguments(args, true) // expectVMID = true

		// For search commands, we need to extract the search string
		// The search string is the non-VM, non-training-data argument
		var vmid, trainingDataPath, searchString string

		if parser.VMID != "" {
			vmid = parser.VMID
		} else {
			utils.RequiredParameterError("VM ID", "QMP_VM_ID")
		}

		if parser.TrainingData != "" {
			trainingDataPath = parser.TrainingData
		}

		// Find the search string - it's the argument that's not VM ID or training data
		for _, arg := range args {
			fileType := ocr.DetectFileType(arg)
			if fileType != ocr.FileTypeVMID && fileType != ocr.FileTypeTrainingData {
				searchString = arg
				break
			}
		}

		if searchString == "" {
			utils.RequiredParameterError("Search string", "")
		}

		result, err := processOCRResult(vmid, trainingDataPath, true)
		if err != nil {
			utils.FatalErrorWithCode(err, "OCR processing failed", utils.ExitCodeGeneral)
		}

		// Perform search
		config := ocr.SearchConfig{
			IgnoreCase:  searchIgnoreCase,
			FirstOnly:   searchFirstOnly,
			Quiet:       searchQuiet,
			LineNumbers: showLineNumbers,
		}

		searchResults := ocr.FindString(result, searchString, config)

		// Output results and set exit code
		output := ocr.FormatResults(searchResults, config)
		if output != "" {
			fmt.Print(output)
		}

		os.Exit(ocr.GetExitCode(searchResults, nil))
	},
}

// ocrFindFileCmd represents the ocr find file command
var ocrFindFileCmd = &cobra.Command{
	Use:   fmt.Sprintf("file [%s] [%s] [%s]", ArgInputImageFile, ArgTrainingData, ArgString),
	Short: "Search for a string in image file OCR results",
	Long: fmt.Sprintf("%s from an %s.\n\nExamples:\n  # Search for \"Login successful\" in image file\n  qmp ocr find file screenshot.ppm training.json \"Login successful\"\n\n  # Search for \"error\" (case-insensitive, first match only)\n  qmp ocr find file screenshot.ppm training.json \"error\" --ignore-case --first\n\n  # Search with debug output and line numbers\n  qmp ocr find file screenshot.ppm training.json \"root@\" --log-level debug --line-numbers",
		fmt.Sprintf(SearchDescription, ArgString), ArgInputImageFile),
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		inputImageFile := args[0]
		trainingDataPath := args[1]
		searchString := args[2]

		result, err := processOCRResult(inputImageFile, trainingDataPath, false)
		if err != nil {
			utils.FatalErrorWithCode(err, "OCR processing failed", utils.ExitCodeGeneral)
		}

		// Perform search
		config := ocr.SearchConfig{
			IgnoreCase:  searchIgnoreCase,
			FirstOnly:   searchFirstOnly,
			Quiet:       searchQuiet,
			LineNumbers: showLineNumbers,
		}

		searchResults := ocr.FindString(result, searchString, config)

		// Output results and set exit code
		output := ocr.FormatResults(searchResults, config)
		if output != "" {
			fmt.Print(output)
		}

		os.Exit(ocr.GetExitCode(searchResults, nil))
	},
}

// ocrReCmd represents the ocr re command for regex searching
var ocrReCmd = &cobra.Command{
	Use:   "re",
	Short: "Search for a regex pattern in OCR results",
	Long: fmt.Sprintf("%s.\n\n%s",
		fmt.Sprintf(RegexDescription, ArgPattern),
		SubcommandDescription),
}

// ocrReVMCmd represents the ocr re vm command
var ocrReVMCmd = &cobra.Command{
	Use:   fmt.Sprintf("vm [%s] [%s] [%s]", ArgVMID, ArgTrainingData, ArgPattern),
	Short: "Search for a regex pattern in VM console OCR results",
	Long: fmt.Sprintf("%s from a VM console.\n\nThe VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.\nThe training data can be provided as an argument or set via the QMP_TRAINING_DATA environment variable.\n\nExamples:\n  # Explicit arguments\n  qmp ocr re vm 123 training.json \"\\b\\d+\\.\\d+\\.\\d+\\.\\d+\\b\"\n\n  # Using environment variables\n  export QMP_VM_ID=123\n  export QMP_TRAINING_DATA=training.json\n  qmp ocr re vm \"\\b\\d+\\.\\d+\\.\\d+\\.\\d+\\b\"\n\n  # Search for login attempts (with capture groups and debug output)\n  qmp ocr re vm 123 training.json \"Login (successful|failed)\" --log-level debug\n\n  # Search for errors (case-insensitive, first match only)\n  qmp ocr re vm 123 training.json \"[Ee]rror.*connection\" --first",
		fmt.Sprintf(RegexDescription, ArgPattern)),
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		// Use flexible argument parsing similar to other commands
		parser := ocr.ParseArguments(args, true) // expectVMID = true

		// For search commands, we need to extract the regex pattern
		// The pattern is the non-VM, non-training-data argument
		var vmid, trainingDataPath, pattern string

		if parser.VMID != "" {
			vmid = parser.VMID
		} else {
			utils.RequiredParameterError("VM ID", "QMP_VM_ID")
		}

		if parser.TrainingData != "" {
			trainingDataPath = parser.TrainingData
		}

		// Find the pattern - it's the argument that's not VM ID or training data
		for _, arg := range args {
			fileType := ocr.DetectFileType(arg)
			if fileType != ocr.FileTypeVMID && fileType != ocr.FileTypeTrainingData {
				pattern = arg
				break
			}
		}

		if pattern == "" {
			utils.RequiredParameterError("Regex pattern", "")
		}

		result, err := processOCRResult(vmid, trainingDataPath, true)
		if err != nil {
			utils.FatalErrorWithCode(err, "OCR processing failed", utils.ExitCodeGeneral)
		}

		// Perform regex search
		config := ocr.SearchConfig{
			IgnoreCase:  searchIgnoreCase,
			FirstOnly:   searchFirstOnly,
			Quiet:       searchQuiet,
			LineNumbers: showLineNumbers,
		}

		searchResults, regexErr := ocr.FindRegex(result, pattern, config)

		// Output results and set exit code
		exitCode := ocr.GetExitCode(searchResults, regexErr)
		if regexErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", regexErr)
		} else {
			output := ocr.FormatResults(searchResults, config)
			if output != "" {
				fmt.Print(output)
			}
		}

		os.Exit(exitCode)
	},
}

// ocrReFileCmd represents the ocr re file command
var ocrReFileCmd = &cobra.Command{
	Use:   fmt.Sprintf("file [%s] [%s] [%s]", ArgInputImageFile, ArgTrainingData, ArgPattern),
	Short: "Search for a regex pattern in image file OCR results",
	Long: fmt.Sprintf("%s from an %s.\n\nExamples:\n  # Search for IP addresses\n  qmp ocr re file screenshot.ppm training.json \"\\b\\d+\\.\\d+\\.\\d+\\.\\d+\\b\"\n\n  # Search for login attempts (with capture groups and debug output)\n  qmp ocr re file screenshot.ppm training.json \"Login (successful|failed)\" --log-level debug\n\n  # Search for errors (case-insensitive, first match only)\n  qmp ocr re file screenshot.ppm training.json \"[Ee]rror.*connection\" --first",
		fmt.Sprintf(RegexDescription, ArgPattern), ArgInputImageFile),
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		inputImageFile := args[0]
		trainingDataPath := args[1]
		pattern := args[2]

		result, err := processOCRResult(inputImageFile, trainingDataPath, false)
		if err != nil {
			utils.FatalErrorWithCode(err, "OCR processing failed", utils.ExitCodeGeneral)
		}

		// Perform regex search
		config := ocr.SearchConfig{
			IgnoreCase:  searchIgnoreCase,
			FirstOnly:   searchFirstOnly,
			Quiet:       searchQuiet,
			LineNumbers: showLineNumbers,
		}

		searchResults, regexErr := ocr.FindRegex(result, pattern, config)

		// Output results and set exit code
		exitCode := ocr.GetExitCode(searchResults, regexErr)
		if regexErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", regexErr)
		} else {
			output := ocr.FormatResults(searchResults, config)
			if output != "" {
				fmt.Print(output)
			}
		}

		os.Exit(exitCode)
	},
}

func init() {
	// Add OCR command to root command
	rootCmd.AddCommand(ocrCmd)

	// Add subcommands to OCR command
	ocrCmd.AddCommand(ocrVmCmd)
	ocrCmd.AddCommand(ocrFileCmd)
	ocrCmd.AddCommand(ocrFindCmd)
	ocrCmd.AddCommand(ocrReCmd)

	// Add flags to VM command
	ocrVmCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrVmCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrVmCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrVmCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")
	ocrVmCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")
	ocrVmCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers (0-based) with colored output")
	ocrVmCmd.Flags().BoolVarP(&updateTraining, "update-training", "u", false, "Interactively train unrecognized characters")
	ocrVmCmd.Flags().BoolVar(&renderSpecialChars, "render", false, "Render special characters (spaces, tabs, newlines) visually")
	ocrVmCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Watch mode - continuously refresh OCR and highlight changes")
	ocrVmCmd.Flags().IntVar(&watchInterval, "watch-interval", 2, "Watch mode refresh interval in seconds")

	// Add flags to File command
	ocrFileCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrFileCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrFileCmd.Flags().BoolVar(&ansiMode, "ansi", false, "Enable ANSI bitmap output")
	ocrFileCmd.Flags().BoolVar(&colorMode, "color", false, "Enable color output")
	ocrFileCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")
	ocrFileCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers (0-based) with colored output")
	ocrFileCmd.Flags().BoolVar(&singleChar, "single-char", false, "Extract a single character")
	ocrFileCmd.Flags().IntVar(&charRow, "char-row", 0, "Row of the character to extract")
	ocrFileCmd.Flags().IntVar(&charCol, "char-col", 0, "Column of the character to extract")
	ocrFileCmd.Flags().StringVar(&cropRowsStr, "crop-rows", "", "Crop rows range in format 'start:end' (e.g., '5:20')")
	ocrFileCmd.Flags().StringVar(&cropColsStr, "crop-cols", "", "Crop columns range in format 'start:end' (e.g., '10:50')")
	ocrFileCmd.Flags().BoolVarP(&updateTraining, "update-training", "u", false, "Interactively train unrecognized characters")
	ocrFileCmd.Flags().BoolVar(&renderSpecialChars, "render", false, "Render special characters (spaces, tabs, newlines) visually")

	// Add subcommands to Find command
	ocrFindCmd.AddCommand(ocrFindVMCmd)
	ocrFindCmd.AddCommand(ocrFindFileCmd)

	// Add flags for find subcommands
	ocrFindVMCmd.Flags().BoolVar(&searchFirstOnly, "first", false, "Stop at first match")
	ocrFindVMCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false, "No text output, only exit codes")
	ocrFindVMCmd.Flags().BoolVarP(&searchIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	ocrFindVMCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrFindVMCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrFindVMCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers with output")
	ocrFindVMCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")

	ocrFindFileCmd.Flags().BoolVar(&searchFirstOnly, "first", false, "Stop at first match")
	ocrFindFileCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false, "No text output, only exit codes")
	ocrFindFileCmd.Flags().BoolVarP(&searchIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	ocrFindFileCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrFindFileCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrFindFileCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers with output")
	ocrFindFileCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")

	// Add subcommands to Re command
	ocrReCmd.AddCommand(ocrReVMCmd)
	ocrReCmd.AddCommand(ocrReFileCmd)

	// Add flags for re subcommands
	ocrReVMCmd.Flags().BoolVar(&searchFirstOnly, "first", false, "Stop at first match")
	ocrReVMCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false, "No text output, only exit codes")
	ocrReVMCmd.Flags().BoolVarP(&searchIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	ocrReVMCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrReVMCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrReVMCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers with output")
	ocrReVMCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")

	ocrReFileCmd.Flags().BoolVar(&searchFirstOnly, "first", false, "Stop at first match")
	ocrReFileCmd.Flags().BoolVarP(&searchQuiet, "quiet", "q", false, "No text output, only exit codes")
	ocrReFileCmd.Flags().BoolVarP(&searchIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	ocrReFileCmd.Flags().IntVarP(&columns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns")
	ocrReFileCmd.Flags().IntVarP(&rows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows")
	ocrReFileCmd.Flags().BoolVarP(&showLineNumbers, "line-numbers", "n", false, "Show line numbers with output")
	ocrReFileCmd.Flags().BoolVarP(&filterBlankLines, "filter", "f", false, "Filter out blank lines from output")
}

// handleInteractiveTraining processes unrecognized characters using the batch system
func handleInteractiveTraining(result *ocr.OCRResult, trainingDataPath string) error {
	// Get terminal dimensions for multi-character layout
	termWidth, _, err := training.GetTerminalDimensions()
	if err != nil {
		logging.Warn("Could not detect terminal size, using defaults", "error", err, "default_width", 80)
		termWidth = 80
	}

	// Ensure we have a valid training data path
	if trainingDataPath == "" {
		// Use a default path if none was provided
		trainingDataPath = qmp.GetDefaultTrainingDataPath()
		logging.Info("Using default training data location", "path", trainingDataPath)
		logging.UserInfof("📂 Using default training data: %s", trainingDataPath)
	} else {
		// Convert to absolute path
		absPath, err := filepath.Abs(trainingDataPath)
		if err != nil {
			logging.Warn("Failed to convert training data path to absolute path",
				"original_path", ocrConfig.TrainingDataPath,
				"error", err)
		} else {
			trainingDataPath = absPath
		}
	}

	// Create character batches for multi-character display
	batches := training.CreateCharacterBatches(result, termWidth)

	if len(batches) == 0 {
		fmt.Println("No unrecognized characters found!")
		return nil
	}

	logging.Info("Starting interactive training session", "unrecognized_patterns",
		func() int {
			total := 0
			for _, batch := range batches {
				total += len(batch.Bitmaps)
			}
			return total
		}())
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
		logging.Warn("Could not load existing training data, using empty dataset",
			"training_data_path", trainingDataPath,
			"error", err)
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
		logging.Info("Training session interrupted, saving data")
		fmt.Printf("\n\nInterrupted! Saving training data...\n")
		if err := ocr.SaveTrainingData(trainingData, trainingDataPath); err != nil {
			logging.Error("Failed to save training data on interrupt",
				"training_data_path", trainingDataPath,
				"error", err)
			fmt.Printf("Error saving training data: %v\n", err)
		} else {
			logging.Info("Training data saved successfully on interrupt", "training_data_path", trainingDataPath)
			fmt.Printf("Training data saved to: %s\n", trainingDataPath)
		}
		os.Exit(0)
	}()

	// Process each batch
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
				if result.CharBitmaps[i].Char == "" || result.CharBitmaps[i].Char == ocr.UnknownCharIndicator {
					if render.FormatBitmapAsHex(&result.CharBitmaps[i]) == hexKey {
						result.CharBitmaps[i].Char = char
					}
				}
			}

			logging.Debug("Added character to training data", "character", char, "hex_key", hexKey)
			fmt.Printf("Added character '%s' to training data.\n", char)
		}

		// Save training data after each batch
		if len(batchMappings) > 0 {
			if err := ocr.SaveTrainingData(trainingData, trainingDataPath); err != nil {
				logging.Error("Failed to save training data on interrupt",
				"training_data_path", trainingDataPath,
				"error", err)
			fmt.Printf("Error saving training data: %v\n", err)
			} else {
				logging.Debug("Training data saved after batch", "training_data_path", trainingDataPath)
				fmt.Printf("Training data saved to %s\n", trainingDataPath)
			}
		}
	}

	if !modified {
		fmt.Println("\nNo changes made to training data.")
	} else {
		logging.Info("Interactive training completed successfully", "training_data_path", trainingDataPath)
		fmt.Printf("\nTraining complete! Updated training data saved to: %s\n", trainingDataPath)
	}

	return nil
}
