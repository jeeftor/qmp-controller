package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/utils"
)

// EnsureDirectory creates a directory and all necessary parent directories
func EnsureDirectory(path string) error {
	if path == "." || path == "" {
		return nil // Current directory always exists
	}

	return os.MkdirAll(path, 0755)
}

// EnsureDirectoryForFile creates the parent directory for a given file path
func EnsureDirectoryForFile(filePath string) error {
	dir := filepath.Dir(filePath)
	return EnsureDirectory(dir)
}

// EnsureDirectoryWithLogging creates directory with structured logging
func EnsureDirectoryWithLogging(path string, context string) error {
	if path == "." || path == "" {
		return nil
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		logging.Error("Failed to create directory",
			"path", path,
			"context", context,
			"error", err)
		return err
	}

	logging.Debug("Created directory", "path", path, "context", context)
	return nil
}

// CheckFileExists verifies that a file exists and is readable
func CheckFileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file '%s' does not exist", path)
		}
		return fmt.Errorf("cannot access file '%s': %w", path, err)
	}
	return nil
}

// CheckFileExistsWithError verifies file exists and exits on error
func CheckFileExistsWithError(path string, context string) {
	if err := CheckFileExists(path); err != nil {
		utils.FileSystemError(context, path, err)
	}
}

// CreateTempFile creates a temporary file with the given prefix
func CreateTempFile(prefix string) (*os.File, error) {
	return os.CreateTemp("", prefix+"*.tmp")
}

// CreateTempFileInDir creates a temporary file in a specific directory
func CreateTempFileInDir(dir, prefix string) (*os.File, error) {
	if err := EnsureDirectory(dir); err != nil {
		return nil, fmt.Errorf("failed to create temp directory '%s': %w", dir, err)
	}

	return os.CreateTemp(dir, prefix+"*.tmp")
}

// WriteFileWithDirectory writes content to a file, creating directories as needed
func WriteFileWithDirectory(filePath string, content []byte, perm os.FileMode) error {
	if err := EnsureDirectoryForFile(filePath); err != nil {
		return fmt.Errorf("failed to create directory for file '%s': %w", filePath, err)
	}

	return os.WriteFile(filePath, content, perm)
}

// CopyFile copies a file from src to dst, creating directories as needed
func CopyFile(src, dst string) error {
	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file '%s': %w", src, err)
	}

	// Ensure destination directory
	if err := EnsureDirectoryForFile(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write destination file
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file '%s': %w", dst, err)
	}

	return nil
}

// SafeRemove removes a file/directory, ignoring "not exist" errors
func SafeRemove(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SafeRemoveAll removes a directory tree, ignoring "not exist" errors
func SafeRemoveAll(path string) error {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetFileExtension returns the lowercase file extension without the dot
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	// Remove the leading dot and convert to lowercase
	return ext[1:]
}

// IsImageFile checks if a file path represents an image file
func IsImageFile(path string) bool {
	ext := GetFileExtension(path)
	switch ext {
	case "png", "jpg", "jpeg", "gif", "bmp", "tiff", "webp", "ppm":
		return true
	default:
		return false
	}
}

// IsJSONFile checks if a file path represents a JSON file
func IsJSONFile(path string) bool {
	return GetFileExtension(path) == "json"
}

// IsScriptFile checks if a file path represents a script file
func IsScriptFile(path string) bool {
	ext := GetFileExtension(path)
	switch ext {
	case "sc2", "script2", "qmp2", "script", "sh", "bash":
		return true
	default:
		return false
	}
}

// GetAbsolutePath converts a path to absolute form, handling ~ expansion
func GetAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path provided")
	}

	// Handle ~ expansion
	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		if len(path) == 1 {
			path = homeDir
		} else {
			path = filepath.Join(homeDir, path[1:])
		}
	}

	return filepath.Abs(path)
}

// StatWithLogging gets file statistics and logs file size and metadata
func StatWithLogging(filePath string, operation string) (os.FileInfo, error) {
	start := time.Now()
	stat, err := os.Stat(filePath)

	if err != nil {
		logging.Error("File stat operation failed",
			"file_path", filePath,
			"operation", operation,
			"error", err,
			"duration", time.Since(start))
		return nil, err
	}

	logging.Debug("File stat operation completed",
		"file_path", filePath,
		"operation", operation,
		"size_bytes", stat.Size(),
		"mod_time", stat.ModTime(),
		"is_dir", stat.IsDir(),
		"duration", time.Since(start))

	return stat, nil
}

// StatWithMetrics gets file statistics and logs metrics for performance monitoring
func StatWithMetrics(filePath string, operation string) (os.FileInfo, error) {
	start := time.Now()
	stat, err := os.Stat(filePath)
	duration := time.Since(start)

	if err != nil {
		logging.LogPerformance("file_stat", "", map[string]interface{}{
			"file_path": filePath,
			"operation": operation,
			"success":   false,
			"duration":  duration,
			"error":     err.Error(),
		})
		return nil, err
	}

	logging.LogPerformance("file_stat", "", map[string]interface{}{
		"file_path":  filePath,
		"operation":  operation,
		"success":    true,
		"size_bytes": stat.Size(),
		"duration":   duration,
	})

	return stat, nil
}

// ValidateOutputFile validates that an output file path is valid and writable
func ValidateOutputFile(outputFile string, paramName string, envVar string) error {
	if outputFile == "" {
		if envVar != "" {
			return fmt.Errorf("%s is required: provide as argument or set %s environment variable", paramName, envVar)
		}
		return fmt.Errorf("%s is required", paramName)
	}

	// Ensure the directory exists
	if err := EnsureDirectoryForFile(outputFile); err != nil {
		return fmt.Errorf("cannot create directory for %s '%s': %w", paramName, outputFile, err)
	}

	// Check if file exists and is writable, or if it can be created
	if _, err := os.Stat(outputFile); err == nil {
		// File exists, check if it's writable
		if file, err := os.OpenFile(outputFile, os.O_WRONLY, 0); err != nil {
			return fmt.Errorf("%s '%s' exists but is not writable: %w", paramName, outputFile, err)
		} else {
			file.Close()
		}
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("cannot access %s '%s': %w", paramName, outputFile, err)
	}
	// If file doesn't exist, that's okay - we'll create it

	return nil
}

// ValidateOutputFileWithExit validates output file and exits on error
func ValidateOutputFileWithExit(outputFile string, paramName string, envVar string) {
	if err := ValidateOutputFile(outputFile, paramName, envVar); err != nil {
		utils.ValidationError(err)
	}
}

// ValidateInputFile validates that an input file exists and is readable
func ValidateInputFile(inputFile string, paramName string, envVar string) error {
	if inputFile == "" {
		if envVar != "" {
			return fmt.Errorf("%s is required: provide as argument or set %s environment variable", paramName, envVar)
		}
		return fmt.Errorf("%s is required", paramName)
	}

	// Check if file exists and is readable
	if _, err := os.Stat(inputFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s '%s' does not exist", paramName, inputFile)
		}
		return fmt.Errorf("cannot access %s '%s': %w", paramName, inputFile, err)
	}

	// Check if file is readable
	if file, err := os.Open(inputFile); err != nil {
		return fmt.Errorf("%s '%s' is not readable: %w", paramName, inputFile, err)
	} else {
		file.Close()
	}

	return nil
}

// ValidateInputFileWithExit validates input file and exits on error
func ValidateInputFileWithExit(inputFile string, paramName string, envVar string) {
	if err := ValidateInputFile(inputFile, paramName, envVar); err != nil {
		utils.ValidationError(err)
	}
}

// GetFileSizeWithLogging gets file size and logs the operation
func GetFileSizeWithLogging(filePath string, operation string) (int64, error) {
	stat, err := StatWithLogging(filePath, operation)
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}
