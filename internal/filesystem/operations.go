package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

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
