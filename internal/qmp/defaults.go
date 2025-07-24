package qmp

import (
	"os"
	"path/filepath"
)

// TODO find a better place for this stuff here
const DEFAULT_WIDTH = 160
const DEFAULT_HEIGHT = 50
const DEFAULT_TRAINING_DATA_FILENAME = ".qmp_training_data.json"

// GetDefaultTrainingDataPath returns the default path for OCR training data
// in the user's home directory (persistent across reboots)
func GetDefaultTrainingDataPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return DEFAULT_TRAINING_DATA_FILENAME
	}
	return filepath.Join(homeDir, DEFAULT_TRAINING_DATA_FILENAME)
}
