package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeeftor/qmp-controller/internal/args"
	"github.com/jeeftor/qmp-controller/internal/constants"
	"github.com/jeeftor/qmp-controller/internal/filesystem"
	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
	"github.com/jeeftor/qmp-controller/internal/render"
	"github.com/jeeftor/qmp-controller/internal/training"
	"github.com/jeeftor/qmp-controller/internal/utils"
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
The output file is optional. If not provided, it will use the default training data file
~/.qmp_training_data.json, or check the QMP_OUTPUT_FILE environment variable.

SAFETY: If the default training data file already exists, you must use the --update flag
to modify it, preventing accidental overwriting of existing training data.

The resolution can be set using --res (e.g., '160x50') or individually with --columns
and --rows. If --res is provided, it takes precedence over --columns and --rows.

Examples:
  # Use default training data file (only works if file doesn't exist yet)
  qmp train-ocr vm 106

  # Update existing default training data file
  qmp train-ocr vm 106 --update

  # Explicit output file
  qmp train-ocr vm 106 training-data.json

  # Using environment variables
  export QMP_VM_ID=106
  export QMP_OUTPUT_FILE=training-data.json
  qmp train-ocr vm

  # Train OCR with custom resolution
  qmp train-ocr vm 106 training-data.json --res 160x50

  # Update existing training data (uses default file if none specified)
  qmp train-ocr vm 106 --update`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		// Parse arguments using simple argument parser for training
		argParser := args.NewSimpleArgumentParser("train-ocr vm")
		parsedArgs := args.ParseWithHandler(cmdArgs, argParser)

		// Extract VM ID
		vmid := parsedArgs.VMID

		// For training output file, check remaining args, environment, or use default
		var outputFile string
		usingDefault := false
		if len(parsedArgs.RemainingArgs) > 0 {
			outputFile = parsedArgs.RemainingArgs[0]
		} else {
			// Try environment variable QMP_OUTPUT_FILE
			outputFile = os.Getenv("QMP_OUTPUT_FILE")
			if outputFile == "" {
				// Use default training data file
				var err error
				outputFile, err = getDefaultTrainingDataPath()
				if err != nil {
					utils.ValidationError(fmt.Errorf("could not determine default training data file path: %v", err))
				}
				usingDefault = true
			}
		}

		// Print message about using default training data file
		if usingDefault {
			logging.UserInfof("📝 USING DEFAULT TRAINING DATA FILE: %s", outputFile)
			logging.UserInfof("   You can specify a custom file: qmp train-ocr vm %s <custom-file>", vmid)
			logging.UserInfo("   Or set environment variable: export QMP_OUTPUT_FILE=<custom-file>")

			// Safety check: if default file exists and --update not specified, require explicit update flag
			if filesystem.CheckFileExists(outputFile) == nil && !updateExisting {
				logging.UserError("⚠️  ERROR: Default training data file already exists!")
				logging.UserErrorf("   File: %s", outputFile)
				logging.UserErrorf("   To update existing training data, use: qmp train-ocr vm %s --update", vmid)
				logging.UserErrorf("   To use a new file, specify: qmp train-ocr vm %s <new-file>", vmid)
				utils.ValidationError(fmt.Errorf("existing default training data file requires --update flag to modify"))
			}
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

The output file is optional. If not provided, the default training data file
~/.qmp_training_data.json will be used.

SAFETY: If the default training data file already exists, you must use the --update flag
to modify it, preventing accidental overwriting of existing training data.

The resolution can be set using --res (e.g., '160x50') or individually with --columns
and --rows. If --res is provided, it takes precedence over --columns and --rows.

Examples:
  # Train OCR from image file (only works if default file doesn't exist yet)
  qmp train-ocr file training-image.ppm

  # Update existing default training data file
  qmp train-ocr file training-image.ppm --update

  # Train OCR from image file with custom output file
  qmp train-ocr file training-image.ppm training-data.json

  # Train OCR from file with custom resolution
  qmp train-ocr file training-image.ppm training-data.json --res 160x50

  # Train OCR with individual column and row settings
  qmp train-ocr file training-image.ppm training-data.json --columns 160 --rows 50

  # Update existing training data
  qmp train-ocr file training-image.ppm training-data.json --update`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, cmdArgs []string) {
		inputFile := cmdArgs[0]

		// Handle optional output file with default
		var outputFile string
		usingDefault := false
		if len(cmdArgs) > 1 {
			outputFile = cmdArgs[1]
		} else {
			// Use default training data file
			var err error
			outputFile, err = getDefaultTrainingDataPath()
			if err != nil {
				utils.ValidationError(fmt.Errorf("could not determine default training data file path: %v", err))
			}
			usingDefault = true
		}

		// Print message about using default training data file
		if usingDefault {
			logging.UserInfof("📝 USING DEFAULT TRAINING DATA FILE: %s", outputFile)
			logging.UserInfof("   You can specify a custom file: qmp train-ocr file %s <custom-file>", inputFile)

			// Safety check: if default file exists and --update not specified, require explicit update flag
			if filesystem.CheckFileExists(outputFile) == nil && !updateExisting {
				logging.UserError("⚠️  ERROR: Default training data file already exists!")
				logging.UserErrorf("   File: %s", outputFile)
				logging.UserErrorf("   To update existing training data, use: qmp train-ocr file %s --update", inputFile)
				logging.UserErrorf("   To use a new file, specify: qmp train-ocr file %s <new-file>", inputFile)
				utils.ValidationError(fmt.Errorf("existing default training data file requires --update flag to modify"))
			}
		}

		runTrainingFlow(inputFile, outputFile, false)
	},
}

// getDefaultTrainingDataPath returns the default path for training data file
func getDefaultTrainingDataPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %v", err)
	}
	return filepath.Join(homeDir, ".qmp_training_data.json"), nil
}

func runTrainingFlow(input, outputFile string, isVM bool) {
	// Set default dimensions if not provided
	if trainColumns <= 0 {
		trainColumns = constants.DefaultScreenWidth
	}
	if trainRows <= 0 {
		trainRows = constants.DefaultScreenHeight
	}

	// Validate dimensions using centralized validation
	if err := constants.ValidateScreenDimensions(trainColumns, trainRows); err != nil {
		utils.ValidationError(err)
	}

	// Determine the screenshot file path
	var inputFile string
	var tempFile *os.File

	if isVM {
		// VM mode - take screenshot
		vmid := input
		client, err := ConnectToVM(vmid)
		if err != nil {
			utils.ConnectionError(vmid, err)
		}
		defer client.Close()

		// Take temporary screenshot
		tempFile, err = TakeTemporaryScreenshot(client, "qmp-train-ocr")
		if err != nil {
			utils.FatalError(err, "taking screenshot for training")
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		inputFile = tempFile.Name()
		logging.Info("Screenshot captured for training", "vmid", vmid, "file", inputFile)
	} else {
		// File mode - check if input file exists
		inputFile = input
		filesystem.CheckFileExistsWithError(inputFile, "validating input file")
	}

	// Create output directory if it doesn't exist
	if err := filesystem.EnsureDirectoryForFile(outputFile); err != nil {
		utils.FileSystemError("create training data directory", outputFile, err)
	}

	// Load existing training data if update flag is set and file exists
	var trainingData *ocr.TrainingData
	var err error

	if updateExisting && (filesystem.CheckFileExists(outputFile) == nil) {
		trainingData, err = ocr.LoadTrainingData(outputFile)
		if err != nil {
			utils.FatalError(err, "loading existing training data")
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
		utils.FatalError(err, "processing file for OCR training")
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
		utils.FatalError(err, "saving training data")
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
		cmd.Flags().IntVarP(&trainColumns, "columns", "c", constants.DefaultScreenWidth, "Number of columns in the terminal")
		cmd.Flags().IntVarP(&trainRows, "rows", "r", constants.DefaultScreenHeight, "Number of rows in the terminal")
		cmd.Flags().BoolVar(&updateExisting, "update", false, "Update existing training data (required when default file exists)")
	}
}
