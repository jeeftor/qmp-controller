package params

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestResolveVMIDWithInfo(t *testing.T) {
	// Save original environment
	originalVMID := os.Getenv("QMP_VM_ID")
	defer func() {
		if originalVMID != "" {
			os.Setenv("QMP_VM_ID", originalVMID)
		} else {
			os.Unsetenv("QMP_VM_ID")
		}
	}()

	tests := []struct {
		name           string
		args           []string
		position       int
		envValue       string
		viperValue     string
		expectedValue  string
		expectedSource string
		expectError    bool
	}{
		{
			name:           "VM ID from argument",
			args:           []string{"123", "other"},
			position:       0,
			envValue:       "",
			viperValue:     "",
			expectedValue:  "123",
			expectedSource: "argument",
			expectError:    false,
		},
		{
			name:           "VM ID from environment when no argument",
			args:           []string{},
			position:       0,
			envValue:       "456",
			viperValue:     "",
			expectedValue:  "456",
			expectedSource: "environment",
			expectError:    false,
		},
		{
			name:           "VM ID from viper when no argument or env",
			args:           []string{},
			position:       0,
			envValue:       "",
			viperValue:     "789",
			expectedValue:  "789",
			expectedSource: "config",
			expectError:    false,
		},
		{
			name:           "Argument takes precedence over environment",
			args:           []string{"123"},
			position:       0,
			envValue:       "456",
			viperValue:     "789",
			expectedValue:  "123",
			expectedSource: "argument",
			expectError:    false,
		},
		{
			name:           "Environment takes precedence over config",
			args:           []string{},
			position:       0,
			envValue:       "456",
			viperValue:     "789",
			expectedValue:  "456",
			expectedSource: "environment",
			expectError:    false,
		},
		{
			name:           "Error when no VM ID available",
			args:           []string{},
			position:       0,
			envValue:       "",
			viperValue:     "",
			expectedValue:  "",
			expectedSource: "",
			expectError:    true,
		},
		{
			name:           "Error when position out of bounds",
			args:           []string{"123"},
			position:       5,
			envValue:       "",
			viperValue:     "",
			expectedValue:  "",
			expectedSource: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("QMP_VM_ID", tt.envValue)
			} else {
				os.Unsetenv("QMP_VM_ID")
			}

			// Set up viper
			if tt.viperValue != "" {
				viper.Set("vm_id", tt.viperValue)
			} else {
				viper.Set("vm_id", "")
			}

			resolver := NewParameterResolver()
			info, err := resolver.ResolveVMIDWithInfo(tt.args, tt.position)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if info.Value != tt.expectedValue {
				t.Errorf("Expected value '%s', got '%s'", tt.expectedValue, info.Value)
			}
			if info.Source != tt.expectedSource {
				t.Errorf("Expected source '%s', got '%s'", tt.expectedSource, info.Source)
			}
		})
	}
}

func TestResolveOutputFileWithInfo(t *testing.T) {
	// Save original environment
	originalOutputFile := os.Getenv("QMP_OUTPUT_FILE")
	defer func() {
		if originalOutputFile != "" {
			os.Setenv("QMP_OUTPUT_FILE", originalOutputFile)
		} else {
			os.Unsetenv("QMP_OUTPUT_FILE")
		}
	}()

	tests := []struct {
		name           string
		args           []string
		position       int
		envValue       string
		viperValue     string
		expectedValue  string
		expectedSource string
	}{
		{
			name:           "Output file from argument",
			args:           []string{"input", "output.txt"},
			position:       1,
			envValue:       "",
			viperValue:     "",
			expectedValue:  "output.txt",
			expectedSource: "argument",
		},
		{
			name:           "Output file from environment when no argument",
			args:           []string{"input"},
			position:       1,
			envValue:       "env-output.txt",
			viperValue:     "",
			expectedValue:  "env-output.txt",
			expectedSource: "environment",
		},
		{
			name:           "Output file from viper when no argument or env",
			args:           []string{"input"},
			position:       1,
			envValue:       "",
			viperValue:     "config-output.txt",
			expectedValue:  "config-output.txt",
			expectedSource: "config",
		},
		{
			name:           "Empty when nothing available",
			args:           []string{"input"},
			position:       1,
			envValue:       "",
			viperValue:     "",
			expectedValue:  "",
			expectedSource: "default",
		},
		{
			name:           "Argument takes precedence",
			args:           []string{"input", "arg-output.txt"},
			position:       1,
			envValue:       "env-output.txt",
			viperValue:     "config-output.txt",
			expectedValue:  "arg-output.txt",
			expectedSource: "argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("QMP_OUTPUT_FILE", tt.envValue)
			} else {
				os.Unsetenv("QMP_OUTPUT_FILE")
			}

			// Set up viper
			if tt.viperValue != "" {
				viper.Set("output_file", tt.viperValue)
			} else {
				viper.Set("output_file", "")
			}

			resolver := NewParameterResolver()
			info := resolver.ResolveOutputFileWithInfo(tt.args, tt.position)

			if info.Value != tt.expectedValue {
				t.Errorf("Expected value '%s', got '%s'", tt.expectedValue, info.Value)
			}
			if info.Source != tt.expectedSource {
				t.Errorf("Expected source '%s', got '%s'", tt.expectedSource, info.Source)
			}
		})
	}
}

func TestResolveTrainingData(t *testing.T) {
	// Save original environment
	originalTrainingData := os.Getenv("QMP_TRAINING_DATA")
	defer func() {
		if originalTrainingData != "" {
			os.Setenv("QMP_TRAINING_DATA", originalTrainingData)
		} else {
			os.Unsetenv("QMP_TRAINING_DATA")
		}
	}()

	tests := []struct {
		name          string
		args          []string
		position      int
		envValue      string
		viperValue    string
		expectedValue string
	}{
		{
			name:          "Training data from argument",
			args:          []string{"123", "training.json"},
			position:      1,
			envValue:      "",
			viperValue:    "",
			expectedValue: "training.json",
		},
		{
			name:          "Training data from environment",
			args:          []string{"123"},
			position:      1,
			envValue:      "env-training.json",
			viperValue:    "",
			expectedValue: "env-training.json",
		},
		{
			name:          "Training data from viper",
			args:          []string{"123"},
			position:      1,
			envValue:      "",
			viperValue:    "config-training.json",
			expectedValue: "config-training.json",
		},
		{
			name:          "Empty when nothing available",
			args:          []string{"123"},
			position:      1,
			envValue:      "",
			viperValue:    "",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("QMP_TRAINING_DATA", tt.envValue)
			} else {
				os.Unsetenv("QMP_TRAINING_DATA")
			}

			// Set up viper
			if tt.viperValue != "" {
				viper.Set("training_data", tt.viperValue)
			} else {
				viper.Set("training_data", "")
			}

			resolver := NewParameterResolver()
			result := resolver.ResolveTrainingData(tt.args, tt.position)

			if result != tt.expectedValue {
				t.Errorf("Expected value '%s', got '%s'", tt.expectedValue, result)
			}
		})
	}
}

func TestResolveImageFile(t *testing.T) {
	// Save original environment
	originalImageFile := os.Getenv("QMP_IMAGE_FILE")
	defer func() {
		if originalImageFile != "" {
			os.Setenv("QMP_IMAGE_FILE", originalImageFile)
		} else {
			os.Unsetenv("QMP_IMAGE_FILE")
		}
	}()

	tests := []struct {
		name          string
		args          []string
		position      int
		envValue      string
		viperValue    string
		expectedValue string
	}{
		{
			name:          "Image file from argument",
			args:          []string{"screenshot.ppm", "output.txt"},
			position:      0,
			envValue:      "",
			viperValue:    "",
			expectedValue: "screenshot.ppm",
		},
		{
			name:          "Image file from environment",
			args:          []string{"output.txt"},
			position:      0,
			envValue:      "env-screenshot.ppm",
			viperValue:    "",
			expectedValue: "env-screenshot.ppm",
		},
		{
			name:          "Image file from viper",
			args:          []string{"output.txt"},
			position:      0,
			envValue:      "",
			viperValue:    "config-screenshot.ppm",
			expectedValue: "config-screenshot.ppm",
		},
		{
			name:          "Empty when nothing available",
			args:          []string{"output.txt"},
			position:      0,
			envValue:      "",
			viperValue:    "",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("QMP_IMAGE_FILE", tt.envValue)
			} else {
				os.Unsetenv("QMP_IMAGE_FILE")
			}

			// Set up viper
			if tt.viperValue != "" {
				viper.Set("image_file", tt.viperValue)
			} else {
				viper.Set("image_file", "")
			}

			resolver := NewParameterResolver()
			result := resolver.ResolveImageFile(tt.args, tt.position)

			if result != tt.expectedValue {
				t.Errorf("Expected value '%s', got '%s'", tt.expectedValue, result)
			}
		})
	}
}
