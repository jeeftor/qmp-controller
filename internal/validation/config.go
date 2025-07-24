package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/ocr"
)

// ValidationError represents a configuration validation error with context
type ValidationError struct {
	Field   string
	Value   interface{}
	Rule    string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult holds the results of configuration validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
	Warnings []string
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(field string, value interface{}, rule string, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Value:   value,
		Rule:    rule,
		Message: message,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResult) AddWarning(message string) {
	vr.Warnings = append(vr.Warnings, message)
}

// ConfigValidator provides comprehensive configuration validation
type ConfigValidator struct {
	// Add any dependencies needed for validation
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidateOCRConfig performs comprehensive validation of OCR configuration
func (cv *ConfigValidator) ValidateOCRConfig(config *ocr.OCRConfig, vmid string, remoteTempPath string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate screen dimensions
	cv.validateScreenDimensions(config, result)

	// Validate crop parameters against screen dimensions
	cv.validateCropParameters(config, result)

	// Validate training data accessibility
	cv.validateTrainingData(config, result)

	// Validate single character mode
	cv.validateSingleCharMode(config, result)

	// Validate remote connection compatibility
	cv.validateRemoteCompatibility(config, vmid, remoteTempPath, result)

	// Cross-validate configuration combinations
	cv.validateConfigurationCombinations(config, result)

	return result
}

// validateScreenDimensions ensures screen dimensions are valid
func (cv *ConfigValidator) validateScreenDimensions(config *ocr.OCRConfig, result *ValidationResult) {
	if config.Columns <= 0 {
		result.AddError("columns", config.Columns, "positive_integer", "screen width must be a positive integer")
	}

	if config.Rows <= 0 {
		result.AddError("rows", config.Rows, "positive_integer", "screen height must be a positive integer")
	}

	// Check for reasonable limits
	if config.Columns > 1000 {
		result.AddWarning(fmt.Sprintf("very large screen width (%d columns) may impact performance", config.Columns))
	}

	if config.Rows > 1000 {
		result.AddWarning(fmt.Sprintf("very large screen height (%d rows) may impact performance", config.Rows))
	}

	// Log validation
	logging.Debug("Screen dimensions validation",
		"columns", config.Columns,
		"rows", config.Rows,
		"valid", config.Columns > 0 && config.Rows > 0)
}

// validateCropParameters validates crop parameters against screen dimensions
func (cv *ConfigValidator) validateCropParameters(config *ocr.OCRConfig, result *ValidationResult) {
	if !config.CropEnabled {
		return
	}

	// Parse and validate crop rows
	if config.CropRowsStr != "" {
		if err := cv.parseCropRange(config.CropRowsStr, "rows", config.Rows); err != nil {
			result.AddError("crop_rows", config.CropRowsStr, "valid_range", err.Error())
		} else {
			// Parse successful, validate parsed values
			parts := strings.Split(config.CropRowsStr, ":")
			if len(parts) == 2 {
				start, _ := strconv.Atoi(parts[0])
				end, _ := strconv.Atoi(parts[1])

				if start < 0 || end >= config.Rows || start > end {
					result.AddError("crop_rows", config.CropRowsStr, "valid_bounds",
						fmt.Sprintf("crop row range must be 0 <= start <= end < %d", config.Rows))
				}

				config.CropStartRow = start
				config.CropEndRow = end
			}
		}
	}

	// Parse and validate crop columns
	if config.CropColsStr != "" {
		if err := cv.parseCropRange(config.CropColsStr, "columns", config.Columns); err != nil {
			result.AddError("crop_cols", config.CropColsStr, "valid_range", err.Error())
		} else {
			// Parse successful, validate parsed values
			parts := strings.Split(config.CropColsStr, ":")
			if len(parts) == 2 {
				start, _ := strconv.Atoi(parts[0])
				end, _ := strconv.Atoi(parts[1])

				if start < 0 || end >= config.Columns || start > end {
					result.AddError("crop_cols", config.CropColsStr, "valid_bounds",
						fmt.Sprintf("crop column range must be 0 <= start <= end < %d", config.Columns))
				}

				config.CropStartCol = start
				config.CropEndCol = end
			}
		}
	}

	// Validate crop region isn't empty
	if config.CropEnabled {
		cropWidth := config.CropEndCol - config.CropStartCol + 1
		cropHeight := config.CropEndRow - config.CropStartRow + 1

		if cropWidth <= 0 || cropHeight <= 0 {
			result.AddError("crop_region", fmt.Sprintf("%dx%d", cropWidth, cropHeight),
				"non_empty", "crop region must have positive dimensions")
		}

		if cropWidth < 5 || cropHeight < 5 {
			result.AddWarning(fmt.Sprintf("very small crop region (%dx%d) may not contain meaningful text", cropWidth, cropHeight))
		}
	}

	logging.Debug("Crop parameters validation",
		"crop_enabled", config.CropEnabled,
		"crop_rows", config.CropRowsStr,
		"crop_cols", config.CropColsStr,
		"start_row", config.CropStartRow,
		"end_row", config.CropEndRow,
		"start_col", config.CropStartCol,
		"end_col", config.CropEndCol)
}

// parseCropRange parses a crop range string like "5:20"
func (cv *ConfigValidator) parseCropRange(rangeStr, dimension string, maxValue int) error {
	parts := strings.Split(rangeStr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("crop %s must be in format 'start:end' (e.g., '5:20')", dimension)
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid start value in crop %s: %s", dimension, parts[0])
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid end value in crop %s: %s", dimension, parts[1])
	}

	if start < 0 {
		return fmt.Errorf("crop %s start value cannot be negative: %d", dimension, start)
	}

	if end >= maxValue {
		return fmt.Errorf("crop %s end value must be less than %d: %d", dimension, maxValue, end)
	}

	if start > end {
		return fmt.Errorf("crop %s start value cannot be greater than end value: %d > %d", dimension, start, end)
	}

	return nil
}

// validateTrainingData ensures training data file is accessible when needed
func (cv *ConfigValidator) validateTrainingData(config *ocr.OCRConfig, result *ValidationResult) {
	if config.TrainingDataPath == "" {
		// Training data is optional for basic OCR
		return
	}

	// Convert to absolute path for better validation
	absPath, err := filepath.Abs(config.TrainingDataPath)
	if err != nil {
		result.AddWarning(fmt.Sprintf("could not resolve absolute path for training data: %v", err))
	} else {
		config.TrainingDataPath = absPath
	}

	// Check if file exists
	if _, err := os.Stat(config.TrainingDataPath); err != nil {
		if os.IsNotExist(err) {
			if config.UpdateTraining {
				// File will be created during training, this is acceptable
				result.AddWarning(fmt.Sprintf("training data file does not exist, will be created: %s", config.TrainingDataPath))
			} else {
				result.AddError("training_data_path", config.TrainingDataPath, "file_exists",
					"training data file does not exist")
			}
		} else {
			result.AddError("training_data_path", config.TrainingDataPath, "file_accessible",
				fmt.Sprintf("cannot access training data file: %v", err))
		}
	} else {
		// File exists, check if it's readable
		file, err := os.Open(config.TrainingDataPath)
		if err != nil {
			result.AddError("training_data_path", config.TrainingDataPath, "file_readable",
				fmt.Sprintf("cannot read training data file: %v", err))
		} else {
			file.Close()
			logging.Debug("Training data file validation successful",
				"training_data_path", config.TrainingDataPath)
		}
	}

	// Check directory permissions for potential file creation
	if config.UpdateTraining {
		dir := filepath.Dir(config.TrainingDataPath)
		if err := cv.checkDirectoryWritable(dir); err != nil {
			result.AddError("training_data_directory", dir, "directory_writable",
				fmt.Sprintf("cannot write to training data directory: %v", err))
		}
	}
}

// checkDirectoryWritable checks if a directory is writable
func (cv *ConfigValidator) checkDirectoryWritable(dir string) error {
	// Try to create a temporary file in the directory
	tempFile, err := os.CreateTemp(dir, "qmp-validation-*")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	return nil
}

// validateSingleCharMode validates single character mode configuration
func (cv *ConfigValidator) validateSingleCharMode(config *ocr.OCRConfig, result *ValidationResult) {
	if !config.SingleChar {
		return
	}

	if config.CharRow < 0 || config.CharRow >= config.Rows {
		result.AddError("char_row", config.CharRow, "valid_range",
			fmt.Sprintf("character row must be between 0 and %d", config.Rows-1))
	}

	if config.CharCol < 0 || config.CharCol >= config.Columns {
		result.AddError("char_col", config.CharCol, "valid_range",
			fmt.Sprintf("character column must be between 0 and %d", config.Columns-1))
	}

	// Check if single character is within crop region
	if config.CropEnabled {
		if config.CharRow < config.CropStartRow || config.CharRow > config.CropEndRow {
			result.AddError("char_row", config.CharRow, "within_crop",
				fmt.Sprintf("character row must be within crop region (%d:%d)", config.CropStartRow, config.CropEndRow))
		}

		if config.CharCol < config.CropStartCol || config.CharCol > config.CropEndCol {
			result.AddError("char_col", config.CharCol, "within_crop",
				fmt.Sprintf("character column must be within crop region (%d:%d)", config.CropStartCol, config.CropEndCol))
		}
	}

	logging.Debug("Single character mode validation",
		"char_row", config.CharRow,
		"char_col", config.CharCol,
		"within_bounds", config.CharRow >= 0 && config.CharRow < config.Rows &&
						config.CharCol >= 0 && config.CharCol < config.Columns)
}

// validateRemoteCompatibility validates configuration compatibility with remote connections
func (cv *ConfigValidator) validateRemoteCompatibility(config *ocr.OCRConfig, vmid string, remoteTempPath string, result *ValidationResult) {
	isRemote := remoteTempPath != ""

	if !isRemote {
		return // No remote connection, no compatibility issues
	}

	// Log remote connection detection
	logging.Debug("Remote connection detected for validation",
		"vmid", vmid,
		"remote_temp_path", remoteTempPath)

	// Interactive training is incompatible with remote connections
	if config.UpdateTraining {
		result.AddError("update_training", config.UpdateTraining, "remote_compatibility",
			"interactive training (--update-training) is not compatible with remote socket connections")
	}

	// Warn about potential performance implications
	if config.CropEnabled {
		result.AddWarning("cropping with remote connections may have additional network overhead")
	}

	if config.Columns*config.Rows > 50000 { // Large screen
		result.AddWarning("large screen dimensions with remote connections may impact performance")
	}
}

// validateConfigurationCombinations validates cross-configuration dependencies
func (cv *ConfigValidator) validateConfigurationCombinations(config *ocr.OCRConfig, result *ValidationResult) {
	// Single character mode with other modes
	if config.SingleChar {
		if config.CropEnabled {
			result.AddWarning("single character mode with cropping enabled - ensure character is within crop region")
		}

		if config.FilterBlankLines {
			result.AddWarning("single character mode with blank line filtering may not be meaningful")
		}

		if config.ShowLineNumbers {
			result.AddWarning("single character mode with line numbers may not be meaningful")
		}
	}

	// Training modes
	if config.UpdateTraining && config.TrainingDataPath == "" {
		result.AddError("training_configuration", "update_training+no_path", "required_dependency",
			"interactive training requires training data path to be specified")
	}

	// Output modes
	if config.AnsiMode && config.ColorMode {
		result.AddWarning("both ANSI mode and color mode enabled - color mode will be used for ANSI output")
	}

	// Performance considerations
	if config.RenderSpecialChars && config.Columns*config.Rows > 10000 {
		result.AddWarning("special character rendering with large screens may impact performance")
	}
}

// ValidateSocketPath validates socket path configuration for remote connections
func (cv *ConfigValidator) ValidateSocketPath(socketPath string, remoteTempPath string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if socketPath == "" && remoteTempPath == "" {
		// Local connection, no specific validation needed
		return result
	}

	if socketPath != "" {
		// Validate socket path format and accessibility
		if !strings.HasPrefix(socketPath, "/") && !strings.Contains(socketPath, ":") {
			result.AddError("socket_path", socketPath, "valid_format",
				"socket path must be either an absolute file path or host:port format")
		}

		// If it looks like a file path, check if directory exists
		if strings.HasPrefix(socketPath, "/") {
			dir := filepath.Dir(socketPath)
			if _, err := os.Stat(dir); err != nil {
				result.AddWarning(fmt.Sprintf("socket directory may not exist: %s", dir))
			}
		}
	}

	if remoteTempPath != "" {
		// Validate remote temp path format
		if !strings.HasPrefix(remoteTempPath, "/") {
			result.AddError("remote_temp_path", remoteTempPath, "absolute_path",
				"remote temp path must be an absolute path")
		}

		// Warn about permissions
		result.AddWarning("ensure remote temp path is writable by the user running QEMU")
	}

	logging.Debug("Socket path validation",
		"socket_path", socketPath,
		"remote_temp_path", remoteTempPath,
		"is_remote", socketPath != "" || remoteTempPath != "")

	return result
}

// FormatValidationErrors formats validation errors for user display
func FormatValidationErrors(result *ValidationResult) string {
	if result.Valid {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Configuration validation failed:\n")

	for _, err := range result.Errors {
		sb.WriteString(fmt.Sprintf("  • %s\n", err.Error()))
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, warning := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", warning))
		}
	}

	return sb.String()
}
