package cmd

import (
	"bufio"
	"fmt"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/params"
	"github.com/jeeftor/qmp-controller/internal/render"
	"github.com/jeeftor/qmp-controller/internal/training"
	"github.com/spf13/cobra"
)

var (
	trainColumns int
	trainRows    int
	//trainResolution string
	updateExisting bool
)

// trainOcrCmd represents the train-ocr command
var trainOcrCmd = &cobra.Command{
	Use:   "train-ocr",
	Short: "Train the OCR system with a known character set",
	Long: `Train the Optical Character Recognition (OCR) system with a known character set.
This command processes an image file or takes a screenshot from a VM containing known
characters and creates training data that can be used by the OCR command for better
character recognition.

Interactive training mode prompts for each unrecognized character to help you build
accurate training data for character recognition.`,
}

// trainOcrVmCmd represents the train-ocr vm command
var trainOcrVmCmd = &cobra.Command{
	Use:   "vm [vmid] [output-file]",
	Short: "Train OCR by taking a screenshot from a VM",
	Long: `Train OCR by taking a screenshot from a VM and processing it interactively.
This command captures a screenshot from the specified VM and runs interactive
training mode to help you build training data.

The VM ID can be provided as an argument or set via the QMP_VM_ID environment variable.
The output file can be provided as an argument or set via the QMP_OUTPUT_FILE environment variable.

The resolution can be set using --res (e.g., '160x50') or individually with --columns
and --rows. If --res is provided, it takes precedence over --columns and --rows.

Examples:
  # Explicit arguments
  qmp train-ocr vm 106 training-data.json

  # Using environment variables
  export QMP_VM_ID=106
  export QMP_OUTPUT_FILE=training-data.json
  qmp train-ocr vm

  # Train OCR with custom resolution
  qmp train-ocr vm 106 training-data.json --res 160x50

  # Update existing training data
  qmp train-ocr vm 106 training-data.json --update`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve parameters using parameter resolver
		resolver := params.NewParameterResolver()

		// Resolve VM ID
		vmidInfo, err := resolver.ResolveVMIDWithInfo(args, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		vmid := vmidInfo.Value

		// Resolve output file
		var outputFile string
		if len(args) >= 2 {
			outputFile = args[1]
		} else if len(args) == 1 && vmidInfo.Source == "argument" {
			// Only VM ID provided as argument, check for output file in env
			outputFileInfo := resolver.ResolveOutputFileWithInfo([]string{}, -1)
			if outputFileInfo.Value == "" {
				fmt.Fprintf(os.Stderr, "Error: Output file is required: provide as argument or set QMP_OUTPUT_FILE environment variable\n")
				os.Exit(1)
			}
			outputFile = outputFileInfo.Value
		} else {
			// No arguments, both from env vars
			outputFileInfo := resolver.ResolveOutputFileWithInfo([]string{}, -1)
			if outputFileInfo.Value == "" {
				fmt.Fprintf(os.Stderr, "Error: Output file is required: provide as argument or set QMP_OUTPUT_FILE environment variable\n")
				os.Exit(1)
			}
			outputFile = outputFileInfo.Value
		}

		runTrainingFlow(vmid, outputFile, true)
	},
}

// trainOcrFileCmd represents the train-ocr file command
var trainOcrFileCmd = &cobra.Command{
	Use:   "file [input-image] [output-file]",
	Short: "Train OCR from an existing image file",
	Long: `Train OCR from an existing image file using interactive mode.
This command processes the specified image file and runs interactive
training mode to help you build training data.

The resolution can be set using --res (e.g., '160x50') or individually with --columns
and --rows. If --res is provided, it takes precedence over --columns and --rows.

Examples:
  # Train OCR from image file with default resolution
  qmp train-ocr file training-image.ppm training-data.json

  # Train OCR from file with custom resolution
  qmp train-ocr file training-image.ppm training-data.json --res 160x50

  # Train OCR with individual column and row settings
  qmp train-ocr file training-image.ppm training-data.json --columns 160 --rows 50

  # Update existing training data
  qmp train-ocr file training-image.ppm training-data.json --update`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		outputFile := args[1]

		runTrainingFlow(inputFile, outputFile, false)
	},
}

func runTrainingFlow(input, outputFile string, isVM bool) {
	// Set default dimensions if not provided
	if trainColumns <= 0 {
		trainColumns = qmp.DEFAULT_WIDTH
	}
	if trainRows <= 0 {
		trainRows = qmp.DEFAULT_HEIGHT
	}

	// Validate dimensions
	if trainColumns <= 0 || trainRows <= 0 {
		fmt.Println("Error: Columns and rows must be positive integers")
		os.Exit(1)
	}

	// Determine the screenshot file path
	var inputFile string
	var tempFile *os.File

	if isVM {
		// VM mode - take screenshot
		vmid := input
		client, err := ConnectToVM(vmid)
		if err != nil {
			logging.Error("Failed to connect to VM for training", "vmid", vmid, "error", err)
			os.Exit(1)
		}
		defer client.Close()

		// Take temporary screenshot
		tempFile, err = TakeTemporaryScreenshot(client, "qmp-train-ocr")
		if err != nil {
			logging.Error("Failed to take screenshot for training", "vmid", vmid, "error", err)
			os.Exit(1)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		inputFile = tempFile.Name()
		logging.Info("Screenshot captured for training", "vmid", vmid, "file", inputFile)
	} else {
		// File mode - check if input file exists
		inputFile = input
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			logging.Error("Input file does not exist", "input_file", inputFile)
			os.Exit(1)
		}
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			logging.Error("Failed to create training data output directory",
				"output_dir", outputDir,
				"output_file", outputFile,
				"error", err)
			os.Exit(1)
		}
	}

	// Load existing training data if update flag is set and file exists
	var trainingData *ocr.TrainingData
	var err error

	if updateExisting && fileExists(outputFile) {
		trainingData, err = ocr.LoadTrainingData(outputFile)
		if err != nil {
			logging.Error("Failed to load existing training data",
				"training_data_file", outputFile,
				"error", err)
			os.Exit(1)
		}
		logging.Info("Loaded existing training data",
			"training_data_file", outputFile,
			"existing_patterns", len(trainingData.BitmapMap))
	} else {
		// Create new training data
		trainingData = &ocr.TrainingData{
			BitmapMap: make(map[string]string),
		}
	}

	// Interactive training mode
	if isVM {
		fmt.Println("Starting interactive OCR training mode from VM screenshot...")
	} else {
		fmt.Println("Starting interactive OCR training mode from file...")
	}

	// Process the file to extract character bitmaps
	result, err := ocr.ProcessScreenshot(inputFile, trainColumns, trainRows)
	if err != nil {
		logging.Error("Failed to process file for OCR training",
			"input_file", inputFile,
			"columns", trainColumns,
			"rows", trainRows,
			"error", err)
		os.Exit(1)
	}

	// Try to recognize characters using existing training data
	if len(trainingData.BitmapMap) > 0 {
		err = ocr.RecognizeCharacters(result, trainingData)
		if err != nil {
			logging.Warn("Failed to recognize characters with existing training data",
				"existing_patterns", len(trainingData.BitmapMap),
				"error", err)
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

	// Get terminal dimensions for multi-character layout
	termWidth, _, err := training.GetTerminalDimensions()
	if err != nil {
		fmt.Printf("Warning: Could not detect terminal size, using defaults: %v\n", err)
		termWidth = 80
	}

	// Create character batches for multi-character display
	batches := training.CreateCharacterBatches(result, termWidth)

	if len(batches) == 0 {
		fmt.Println("No unrecognized characters found for training.")
		return
	}

	fmt.Printf("Found %d unrecognized character patterns to train.\n",
		func() int {
			total := 0
			for _, batch := range batches {
				total += len(batch.Bitmaps)
			}
			return total
		}())

	// Process each batch
	for batchNum, batch := range batches {
		// Skip batch if all characters already seen
		hasNewChars := false
		for _, hexKey := range batch.HexKeys {
			if _, found := seenBitmaps[hexKey]; !found {
				hasNewChars = true
				break
			}
		}

		if !hasNewChars {
			fmt.Printf("Skipping batch %d/%d - all characters already trained\n",
				batchNum+1, len(batches))
			continue
		}

		// Display the batch
		batchDisplay := training.RenderCharacterBatch(batch, batchNum+1, len(batches))
		fmt.Print(batchDisplay)

		// Process input for this batch
		batchMappings := training.ProcessBatchInput(batch, reader)

		// Apply mappings to training data
		for hexKey, char := range batchMappings {
			trainingData.BitmapMap[hexKey] = char
			seenBitmaps[hexKey] = char
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

	// Save training data to the specified output file
	if err := ocr.SaveTrainingData(trainingData, outputFile); err != nil {
		fmt.Printf("Error saving training data: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Training data saved to %s\n", outputFile)
	fmt.Printf("Total characters in training data: %d\n", len(trainingData.BitmapMap))
}

func init() {
	rootCmd.AddCommand(trainOcrCmd)
	trainOcrCmd.AddCommand(trainOcrVmCmd)
	trainOcrCmd.AddCommand(trainOcrFileCmd)

	// Add resolution flags to both subcommands
	for _, cmd := range []*cobra.Command{trainOcrVmCmd, trainOcrFileCmd} {
		cmd.Flags().IntVarP(&trainColumns, "columns", "c", qmp.DEFAULT_WIDTH, "Number of columns in the terminal")
		cmd.Flags().IntVarP(&trainRows, "rows", "r", qmp.DEFAULT_HEIGHT, "Number of rows in the terminal")
		cmd.Flags().BoolVar(&updateExisting, "update", false, "Update existing training data if the file exists")
	}
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
