package qmp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDefaultTrainingDataPath(t *testing.T) {
	path := GetDefaultTrainingDataPath()

	// Should not be empty
	if path == "" {
		t.Error("GetDefaultTrainingDataPath() should not return empty string")
	}

	// Should end with the expected filename
	expectedFilename := ".qmp_training_data.json"
	if !strings.HasSuffix(path, expectedFilename) {
		t.Errorf("Expected path to end with '%s', got '%s'", expectedFilename, path)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got '%s'", path)
	}
}

func TestConstants(t *testing.T) {
	// Test that our constants are reasonable
	if DEFAULT_WIDTH <= 0 {
		t.Errorf("DEFAULT_WIDTH should be positive, got %d", DEFAULT_WIDTH)
	}

	if DEFAULT_HEIGHT <= 0 {
		t.Errorf("DEFAULT_HEIGHT should be positive, got %d", DEFAULT_HEIGHT)
	}

	// Test typical screen dimensions
	if DEFAULT_WIDTH < 80 || DEFAULT_WIDTH > 300 {
		t.Errorf("DEFAULT_WIDTH seems unreasonable: %d", DEFAULT_WIDTH)
	}

	if DEFAULT_HEIGHT < 20 || DEFAULT_HEIGHT > 100 {
		t.Errorf("DEFAULT_HEIGHT seems unreasonable: %d", DEFAULT_HEIGHT)
	}
}

func TestDefaultTrainingDataPathExists(t *testing.T) {
	path := GetDefaultTrainingDataPath()

	// The path might not exist (that's OK), but the directory should be accessible
	dir := filepath.Dir(path)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Parent directory of default training data path does not exist: %s", dir)
	}
}
