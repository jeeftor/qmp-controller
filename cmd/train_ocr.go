package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jeeftor/qmp/internal/ocr"
	"github.com/spf13/cobra"
)

var (
	trainScreenWidth  int
	trainScreenHeight int
	trainResolution   string
	trainRows         int
	trainColumns      int
	trainCharacters   string
	interactiveMode   bool
	updateExisting    bool
)

// trainOcrCmd represents the train-ocr command
var trainOcrCmd = &cobra.Command{
	Use:   "train-ocr [input-file] [output-file]",
	Short: "Train the OCR system with a known character set",
	Long: `Train the Optical Character Recognition (OCR) system with a known character set.
This command processes an image file containing known characters and creates training data
that can be used by the OCR command for better character recognition.

You can specify the resolution in several ways:
  --width and --height: Set the number of columns and rows
  --res: Set the resolution in the format "columns x rows" (e.g., "80x25")
  -c and -r: Short form for columns and rows

Training modes:
  --interactive: Enable interactive training mode, which prompts for each unrecognized character
  --update: Update existing training data if the output file exists (works with both modes)

Examples:
  # Train OCR with explicit width and height
  qmp train-ocr training-image.ppm training-data.json --width 80 --height 25

  # Use the resolution format
  qmp train-ocr training-image.ppm training-data.json --res 80x25

  # Use short form flags
  qmp train-ocr training-image.ppm training-data.json -c 80 -r 25

  # Specify custom training characters
  qmp train-ocr training-image.ppm training-data.json --res 80x25 --train-chars "AaBbCcDd0123456789"

  # Use interactive training mode
  qmp train-ocr screenshot.ppm training-data.json --res 80x25 --interactive

  # Update existing training data with interactive mode
  qmp train-ocr screenshot.ppm training-data.json --res 80x25 --interactive --update`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		outputFile := args[1]

		// Parse resolution if provided
		if trainResolution != "" {
			fmt.Sscanf(trainResolution, "%dx%d", &trainColumns, &trainRows)
			if trainColumns <= 0 || trainRows <= 0 {
				fmt.Printf("Error parsing resolution '%s': columns and rows must be positive integers\n", trainResolution)
				os.Exit(1)
			}
		}

		// Use width/height if resolution wasn't provided or didn't parse correctly
		if trainColumns <= 0 {
			trainColumns = trainScreenWidth
		}
		if trainRows <= 0 {
			trainRows = trainScreenHeight
		}

		// Validate screen dimensions
		if trainColumns <= 0 || trainRows <= 0 {
			fmt.Println("Error: Screen columns and rows must be positive integers")
			os.Exit(1)
		}

		// Check if input file exists
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			fmt.Printf("Error: Input file %s does not exist\n", inputFile)
			os.Exit(1)
		}

		// Create output directory if it doesn't exist
		outputDir := filepath.Dir(outputFile)
		if outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				fmt.Printf("Error creating output directory: %v\n", err)
				os.Exit(1)
			}
		}

		// Load existing training data if update flag is set and file exists
		var trainingData *ocr.TrainingData
		var err error

		if updateExisting && fileExists(outputFile) {
			trainingData, err = ocr.LoadTrainingData(outputFile)
			if err != nil {
				fmt.Printf("Error loading existing training data: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Loaded existing training data from %s\n", outputFile)
		} else {
			// Create new training data
			trainingData = &ocr.TrainingData{
				BitmapMap: make(map[string]string),
			}
		}

		if interactiveMode {
			// Interactive training mode
			fmt.Println("Starting interactive OCR training mode...")

			// Process the file to extract character bitmaps
			result, err := ocr.ProcessScreenshot(inputFile, trainColumns, trainRows, false)
			if err != nil {
				fmt.Printf("Error processing file for OCR training: %v\n", err)
				os.Exit(1)
			}

			// Try to recognize characters using existing training data
			if len(trainingData.BitmapMap) > 0 {
				err = ocr.RecognizeCharacters(result, trainingData)
				if err != nil {
					fmt.Printf("Warning: Error recognizing characters: %v\n", err)
				}
			}

			// Interactive training loop
			fmt.Println("\nInteractive OCR Training Mode")
			fmt.Println("============================")
			fmt.Println("For each unrecognized character, enter the correct character.")
			fmt.Println("Press Enter to skip a character.")
			fmt.Println("Type 'quit' or 'exit' to end the training session.")
			fmt.Println("Training data will be saved after each character.")
			fmt.Println("Press Ctrl+C at any time to exit (data will be saved).")
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)
			modified := false

			// Track bitmaps we've seen in this session to avoid duplicate prompts
			seenBitmaps := make(map[string]string)

			// Pre-populate seenBitmaps with existing training data
			for hexBitmap, char := range trainingData.BitmapMap {
				seenBitmaps[hexBitmap] = char
			}

			// Set up signal handling for Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\nReceived interrupt signal. Saving training data before exiting...")
				if modified {
					if err := ocr.SaveTrainingData(trainingData, outputFile); err != nil {
						fmt.Printf("Error saving training data: %v\n", err)
					} else {
						fmt.Printf("Training data saved to %s\n", outputFile)
					}
				} else {
					fmt.Println("No changes were made to training data.")
				}
				os.Exit(0)
			}()

			// Process each character bitmap
			for row := 0; row < result.Height; row++ {
				for col := 0; col < result.Width; col++ {
					idx := row*result.Width + col
					if idx >= len(result.CharBitmaps) {
						continue
					}

					bitmap := result.CharBitmaps[idx]

					// Skip already recognized characters
					if bitmap.Char != "" {
						continue
					}

					// Generate hex key for this bitmap
					hexKey := ocr.FormatBitmapAsHex(&bitmap)

					// Check if we've already seen this bitmap pattern in this session
					if existingChar, found := seenBitmaps[hexKey]; found {
						fmt.Printf("\nSkipping character at position (%d,%d) - matches previously trained character '%s'\n",
							row, col, existingChar)

						// Set the character based on the previously trained one
						bitmap.Char = existingChar
						result.CharBitmaps[idx].Char = existingChar
						continue
					}

					// Display the character bitmap
					fmt.Printf("\nUnrecognized character at position (%d,%d):\n", row, col)
					fmt.Printf("Hex bitmap: %s\n\n", ocr.FormatBitmapAsHex(&bitmap))

					// Print ANSI representation
					for bitmapY := 0; bitmapY < bitmap.Height; bitmapY++ {
						for bitmapX := 0; bitmapX < bitmap.Width; bitmapX++ {
							if bitmapY < len(bitmap.Data) && bitmapX < len(bitmap.Data[bitmapY]) {
								if bitmap.Data[bitmapY][bitmapX] {
									// Character pixel - use ANSI escape sequence for white background (double width)
									fmt.Print("\033[47m  ")
								} else {
									// Empty space - use ANSI escape sequence for black background (double width)
									fmt.Print("\033[40m  ")
								}
							}
						}
						// Reset color at end of line
						fmt.Print("\033[0m\n")
					}
					fmt.Print("\033[0m\n")

					// Prompt for character
					fmt.Print("Enter character (or press Enter to skip): ")
					input, err := reader.ReadString('\n')
					if err != nil {
						fmt.Printf("Error reading input: %v\n", err)
						continue
					}

					// TrimSpace will remove leading/trailing spaces, but we need to check if the input was just a space
					// before trimming
					wasJustSpace := false
					if len(input) == 2 && input[0] == ' ' && input[1] == '\n' {
						wasJustSpace = true
					}

					input = strings.TrimSpace(input)
					if input == "quit" || input == "exit" {
						fmt.Println("Training session ended by user.")
						break
					}

					if input == "" && !wasJustSpace {
						fmt.Println("Skipped.")
						continue
					}

					// Special handling for space character
					var char string
					if wasJustSpace || input == "space" {
						char = " "
						fmt.Println("Using space character.")
					} else {
						// Use only the first character if multiple were entered
						char = string([]rune(input)[0])
					}

					// Add to training data
					bitmap.Char = char

					// Store in BitmapMap (hex string to character)
					hexKey = ocr.FormatBitmapAsHex(&bitmap)
					trainingData.BitmapMap[hexKey] = char

					// Add to seen bitmaps to avoid duplicate prompts
					seenBitmaps[hexKey] = char

					modified = true

					fmt.Printf("Added character '%s' to training data.\n", char)

					// Save training data after each character
					if err := ocr.SaveTrainingData(trainingData, outputFile); err != nil {
						fmt.Printf("Error saving training data: %v\n", err)
					} else {
						fmt.Printf("Training data saved to %s\n", outputFile)
					}
				}
			}

			if !modified {
				fmt.Println("\nNo changes made to training data.")
				return
			}
		} else {
			// Batch training mode with known characters
			fmt.Println("Starting batch OCR training mode...")

			// Extract training data
			extractedData, err := ocr.ExtractTrainingData(inputFile, trainColumns, trainRows, trainCharacters)
			if err != nil {
				fmt.Printf("Error extracting training data: %v\n", err)
				os.Exit(1)
			}

			// If updating, merge the extracted data with existing data
			if updateExisting && len(trainingData.BitmapMap) > 0 {
				for hexBitmap, char := range extractedData.BitmapMap {
					trainingData.BitmapMap[hexBitmap] = char
				}
				fmt.Printf("Updated training data with %d new character patterns\n", len(extractedData.BitmapMap))
			} else {
				// Use the newly extracted data
				trainingData = extractedData
			}
		}

		// Save training data to the specified output file
		if err := ocr.SaveTrainingData(trainingData, outputFile); err != nil {
			fmt.Printf("Error saving training data: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Training data saved to %s\n", outputFile)
		fmt.Printf("Total characters in training data: %d\n", len(trainingData.BitmapMap))
	},
}

func init() {
	rootCmd.AddCommand(trainOcrCmd)

	// Add resolution flags
	trainOcrCmd.Flags().IntVarP(&trainScreenWidth, "width", "c", 80, "Number of columns in the terminal")
	trainOcrCmd.Flags().IntVarP(&trainScreenHeight, "height", "r", 25, "Number of rows in the terminal")
	trainOcrCmd.Flags().StringVar(&trainResolution, "res", "", "Resolution in format 'columns x rows' (e.g., '80x25')")

	// Add training flags
	trainOcrCmd.Flags().StringVar(&trainCharacters, "train-chars", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "Characters to train (for batch mode)")
	trainOcrCmd.Flags().BoolVar(&interactiveMode, "interactive", false, "Enable interactive training mode")
	trainOcrCmd.Flags().BoolVar(&updateExisting, "update", false, "Update existing training data if the file exists")
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
