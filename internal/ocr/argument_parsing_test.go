package ocr

import (
	"testing"
)

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FileType
	}{
		// VM ID detection
		{
			name:     "Simple VM ID",
			input:    "123",
			expected: FileTypeVMID,
		},
		{
			name:     "VM ID with leading zeros",
			input:    "001",
			expected: FileTypeVMID,
		},
		{
			name:     "Large VM ID",
			input:    "999999",
			expected: FileTypeVMID,
		},

		// Training data detection
		{
			name:     "Training data JSON file",
			input:    "training.json",
			expected: FileTypeTrainingData,
		},
		{
			name:     "Training data with path",
			input:    "/path/to/training.json",
			expected: FileTypeTrainingData,
		},
		{
			name:     "Custom training data name",
			input:    "my-ocr-data.json",
			expected: FileTypeTrainingData,
		},

		// Image file detection
		{
			name:     "PPM image file",
			input:    "screenshot.ppm",
			expected: FileTypeImage,
		},
		{
			name:     "PNG image file",
			input:    "screenshot.png",
			expected: FileTypeImage,
		},
		{
			name:     "JPG image file",
			input:    "photo.jpg",
			expected: FileTypeImage,
		},
		{
			name:     "JPEG image file",
			input:    "image.jpeg",
			expected: FileTypeImage,
		},

		// Output file detection
		{
			name:     "Text output file",
			input:    "output.txt",
			expected: FileTypeOutput,
		},
		{
			name:     "Output file with path",
			input:    "/tmp/ocr-result.txt",
			expected: FileTypeOutput,
		},

		// Unknown/ambiguous cases
		{
			name:     "Non-numeric string",
			input:    "hello",
			expected: FileTypeUnknown,
		},
		{
			name:     "Mixed alphanumeric",
			input:    "abc123",
			expected: FileTypeUnknown,
		},
		{
			name:     "File without clear extension",
			input:    "somefile",
			expected: FileTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.input)
			if result != tt.expected {
				t.Errorf("DetectFileType(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectVMID   bool
		expectedVMID string
		expectedTD   string
		expectedImg  string
		expectedOut  string
		expectErrors bool
	}{
		{
			name:         "VM command with all arguments",
			args:         []string{"123", "training.json", "output.txt"},
			expectVMID:   true,
			expectedVMID: "123",
			expectedTD:   "training.json",
			expectedImg:  "",
			expectedOut:  "output.txt",
			expectErrors: false,
		},
		{
			name:         "File command with image and training data",
			args:         []string{"screenshot.ppm", "training.json", "output.txt"},
			expectVMID:   false,
			expectedVMID: "",
			expectedTD:   "training.json",
			expectedImg:  "screenshot.ppm",
			expectedOut:  "output.txt",
			expectErrors: false,
		},
		{
			name:         "Flexible order - VM ID last",
			args:         []string{"training.json", "output.txt", "123"},
			expectVMID:   true,
			expectedVMID: "123",
			expectedTD:   "training.json",
			expectedImg:  "",
			expectedOut:  "output.txt",
			expectErrors: false,
		},
		{
			name:         "Flexible order - training data last",
			args:         []string{"123", "output.txt", "training.json"},
			expectVMID:   true,
			expectedVMID: "123",
			expectedTD:   "training.json",
			expectedImg:  "",
			expectedOut:  "output.txt",
			expectErrors: false,
		},
		{
			name:         "VM ID only",
			args:         []string{"123"},
			expectVMID:   true,
			expectedVMID: "123",
			expectedTD:   "",
			expectedImg:  "",
			expectedOut:  "",
			expectErrors: false,
		},
		{
			name:         "Image file only",
			args:         []string{"screenshot.ppm"},
			expectVMID:   false,
			expectedVMID: "",
			expectedTD:   "",
			expectedImg:  "screenshot.ppm",
			expectedOut:  "",
			expectErrors: false,
		},
		{
			name:         "Multiple VM IDs should error",
			args:         []string{"123", "456", "training.json"},
			expectVMID:   true,
			expectedVMID: "",
			expectedTD:   "",
			expectedImg:  "",
			expectedOut:  "",
			expectErrors: true,
		},
		{
			name:         "Multiple training data files should error",
			args:         []string{"123", "training1.json", "training2.json"},
			expectVMID:   true,
			expectedVMID: "",
			expectedTD:   "",
			expectedImg:  "",
			expectedOut:  "",
			expectErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := ParseArguments(tt.args, tt.expectVMID)

			if tt.expectErrors {
				if len(parser.Errors) == 0 {
					t.Errorf("Expected parsing errors for args %v, but got none", tt.args)
				}
				return
			}

			if len(parser.Errors) > 0 {
				t.Errorf("Unexpected parsing errors for args %v: %v", tt.args, parser.Errors)
				return
			}

			if parser.VMID != tt.expectedVMID {
				t.Errorf("Expected VMID '%s', got '%s'", tt.expectedVMID, parser.VMID)
			}
			if parser.TrainingData != tt.expectedTD {
				t.Errorf("Expected TrainingData '%s', got '%s'", tt.expectedTD, parser.TrainingData)
			}
			if parser.ImageFile != tt.expectedImg {
				t.Errorf("Expected ImageFile '%s', got '%s'", tt.expectedImg, parser.ImageFile)
			}
			if parser.OutputFile != tt.expectedOut {
				t.Errorf("Expected OutputFile '%s', got '%s'", tt.expectedOut, parser.OutputFile)
			}
		})
	}
}

func TestFlexibleArgumentParserValidation(t *testing.T) {
	tests := []struct {
		name               string
		parser             *FlexibleArgumentParser
		requireVMID        bool
		requireTrainingData bool
		requireImageFile   bool
		expectErrors       bool
		expectedErrorCount int
	}{
		{
			name: "Valid VM command",
			parser: &FlexibleArgumentParser{
				VMID:         "123",
				TrainingData: "training.json",
				OutputFile:   "output.txt",
			},
			requireVMID:        true,
			requireTrainingData: false,
			requireImageFile:   false,
			expectErrors:       false,
			expectedErrorCount: 0,
		},
		{
			name: "Missing required VM ID",
			parser: &FlexibleArgumentParser{
				TrainingData: "training.json",
				OutputFile:   "output.txt",
			},
			requireVMID:        true,
			requireTrainingData: false,
			requireImageFile:   false,
			expectErrors:       true,
			expectedErrorCount: 1,
		},
		{
			name: "Missing required training data",
			parser: &FlexibleArgumentParser{
				VMID:       "123",
				OutputFile: "output.txt",
			},
			requireVMID:        true,
			requireTrainingData: true,
			requireImageFile:   false,
			expectErrors:       true,
			expectedErrorCount: 1,
		},
		{
			name: "Missing required image file",
			parser: &FlexibleArgumentParser{
				VMID:         "123",
				TrainingData: "training.json",
			},
			requireVMID:        false,
			requireTrainingData: false,
			requireImageFile:   true,
			expectErrors:       true,
			expectedErrorCount: 1,
		},
		{
			name: "Multiple validation errors",
			parser: &FlexibleArgumentParser{
				OutputFile: "output.txt",
			},
			requireVMID:        true,
			requireTrainingData: true,
			requireImageFile:   true,
			expectErrors:       true,
			expectedErrorCount: 3,
		},
		{
			name: "Pre-existing parsing errors",
			parser: &FlexibleArgumentParser{
				VMID:       "123",
				OutputFile: "output.txt",
				Errors:     []string{"Previous parsing error"},
			},
			requireVMID:        true,
			requireTrainingData: false,
			requireImageFile:   false,
			expectErrors:       true,
			expectedErrorCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := tt.parser.Validate(tt.requireVMID, tt.requireTrainingData, tt.requireImageFile)

			if tt.expectErrors && len(errors) == 0 {
				t.Errorf("Expected validation errors, but got none")
			}
			if !tt.expectErrors && len(errors) > 0 {
				t.Errorf("Unexpected validation errors: %v", errors)
			}
			if len(errors) != tt.expectedErrorCount {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrorCount, len(errors), errors)
			}
		})
	}
}
