package params

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/viper"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// ParameterResolver handles resolution of common parameters from multiple sources
type ParameterResolver struct {
	// Sources in priority order: CLI args > flags > env vars > config > defaults
}

// NewParameterResolver creates a new parameter resolver
func NewParameterResolver() *ParameterResolver {
	return &ParameterResolver{}
}

// ResolveVMID resolves VM ID from arguments, flags, environment, or config
// Priority: explicit argument > QMP_VM_ID env var > config > error
func (r *ParameterResolver) ResolveVMID(args []string, argIndex int) (string, error) {
	// 1. Explicit argument (highest priority)
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		vmid := args[argIndex]
		// Validate it's numeric
		if _, err := strconv.Atoi(vmid); err != nil {
			return "", fmt.Errorf("invalid VM ID '%s': must be numeric", vmid)
		}
		return vmid, nil
	}

	// 2. Environment variable
	if viper.IsSet("vm_id") {
		vmid := viper.GetString("vm_id")
		if vmid != "" {
			// Validate it's numeric
			if _, err := strconv.Atoi(vmid); err != nil {
				return "", fmt.Errorf("invalid QMP_VM_ID '%s': must be numeric", vmid)
			}
			return vmid, nil
		}
	}

	// 3. No VM ID available
	return "", fmt.Errorf("VM ID is required: provide as argument or set QMP_VM_ID environment variable")
}

// ResolveTrainingData resolves training data path from arguments, environment, or default
// Priority: explicit argument > QMP_TRAINING_DATA env var > default path
func (r *ParameterResolver) ResolveTrainingData(args []string, argIndex int) string {
	// 1. Explicit argument (highest priority)
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return args[argIndex]
	}

	// 2. Environment variable
	if viper.IsSet("training_data") {
		trainingData := viper.GetString("training_data")
		if trainingData != "" {
			return trainingData
		}
	}

	// 3. Default path
	return qmp.GetDefaultTrainingDataPath()
}

// ResolveImageFile resolves image file path from arguments or environment
// Priority: explicit argument > QMP_IMAGE_FILE env var > empty string
func (r *ParameterResolver) ResolveImageFile(args []string, argIndex int) string {
	// 1. Explicit argument (highest priority)
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return args[argIndex]
	}

	// 2. Environment variable
	if viper.IsSet("image_file") {
		imageFile := viper.GetString("image_file")
		if imageFile != "" {
			return imageFile
		}
	}

	// 3. No image file specified
	return ""
}

// ResolveOutputFile resolves output file path from arguments or environment
// Priority: explicit argument > QMP_OUTPUT_FILE env var > empty string
func (r *ParameterResolver) ResolveOutputFile(args []string, argIndex int) string {
	// 1. Explicit argument (highest priority)
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return args[argIndex]
	}

	// 2. Environment variable
	if viper.IsSet("output_file") {
		outputFile := viper.GetString("output_file")
		if outputFile != "" {
			return outputFile
		}
	}

	// 3. No output file specified
	return ""
}

// ResolveScreenDimensions resolves screen dimensions from flags or environment
// Priority: CLI flags > QMP_COLUMNS/QMP_ROWS env vars > defaults
func (r *ParameterResolver) ResolveScreenDimensions(flagColumns, flagRows int) (int, int) {
	columns := flagColumns
	rows := flagRows

	// Use environment variables if flags are at default values
	if columns == qmp.DEFAULT_WIDTH && viper.IsSet("columns") {
		columns = viper.GetInt("columns")
	}
	if rows == qmp.DEFAULT_HEIGHT && viper.IsSet("rows") {
		rows = viper.GetInt("rows")
	}

	return columns, rows
}

// ParameterInfo provides information about where a parameter came from
type ParameterInfo struct {
	Value  string
	Source string // "argument", "environment", "config", "default"
}

// ResolveVMIDWithInfo returns VM ID and source information for debugging
func (r *ParameterResolver) ResolveVMIDWithInfo(args []string, argIndex int) (ParameterInfo, error) {
	// 1. Explicit argument
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		vmid := args[argIndex]
		if _, err := strconv.Atoi(vmid); err != nil {
			return ParameterInfo{}, fmt.Errorf("invalid VM ID '%s': must be numeric", vmid)
		}
		return ParameterInfo{Value: vmid, Source: "argument"}, nil
	}

	// 2. Environment variable
	if viper.IsSet("vm_id") {
		vmid := viper.GetString("vm_id")
		if vmid != "" {
			if _, err := strconv.Atoi(vmid); err != nil {
				return ParameterInfo{}, fmt.Errorf("invalid QMP_VM_ID '%s': must be numeric", vmid)
			}
			return ParameterInfo{Value: vmid, Source: "environment"}, nil
		}
	}

	return ParameterInfo{}, fmt.Errorf("VM ID is required: provide as argument or set QMP_VM_ID environment variable")
}

// ResolveTrainingDataWithInfo returns training data path and source information
func (r *ParameterResolver) ResolveTrainingDataWithInfo(args []string, argIndex int) ParameterInfo {
	// 1. Explicit argument
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return ParameterInfo{Value: args[argIndex], Source: "argument"}
	}

	// 2. Environment variable
	if viper.IsSet("training_data") {
		trainingData := viper.GetString("training_data")
		if trainingData != "" {
			return ParameterInfo{Value: trainingData, Source: "environment"}
		}
	}

	// 3. Default path
	return ParameterInfo{Value: qmp.GetDefaultTrainingDataPath(), Source: "default"}
}

// ResolveOutputFileWithInfo returns output file path and source information
func (r *ParameterResolver) ResolveOutputFileWithInfo(args []string, argIndex int) ParameterInfo {
	// 1. Explicit argument
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return ParameterInfo{Value: args[argIndex], Source: "argument"}
	}

	// 2. Environment variable
	if viper.IsSet("output_file") {
		outputFile := viper.GetString("output_file")
		if outputFile != "" {
			return ParameterInfo{Value: outputFile, Source: "environment"}
		}
	}

	// 3. No output file specified
	return ParameterInfo{Value: "", Source: "none"}
}

// ResolveImageFileWithInfo returns image file path and source information
func (r *ParameterResolver) ResolveImageFileWithInfo(args []string, argIndex int) ParameterInfo {
	// 1. Explicit argument
	if argIndex >= 0 && argIndex < len(args) && args[argIndex] != "" {
		return ParameterInfo{Value: args[argIndex], Source: "argument"}
	}

	// 2. Environment variable
	if viper.IsSet("image_file") {
		imageFile := viper.GetString("image_file")
		if imageFile != "" {
			return ParameterInfo{Value: imageFile, Source: "environment"}
		}
	}

	// 3. No image file specified
	return ParameterInfo{Value: "", Source: "none"}
}

// ResolveKeyDelay resolves keyboard input delay from environment or returns default
// Priority: QMP_KEY_DELAY env var > default (50ms)
func (r *ParameterResolver) ResolveKeyDelay() time.Duration {
	if viper.IsSet("key_delay") {
		delayStr := viper.GetString("key_delay")
		if delayStr != "" {
			if duration, err := time.ParseDuration(delayStr); err == nil {
				return duration
			}
		}
	}
	return 50 * time.Millisecond
}

// ResolveScriptDelay resolves script execution delay from environment or returns default
// Priority: QMP_SCRIPT_DELAY env var > default (50ms)
func (r *ParameterResolver) ResolveScriptDelay() time.Duration {
	if viper.IsSet("script_delay") {
		delayStr := viper.GetString("script_delay")
		if delayStr != "" {
			if duration, err := time.ParseDuration(delayStr); err == nil {
				return duration
			}
		}
	}
	return 50 * time.Millisecond
}

// ResolveScreenshotFormat resolves screenshot format from environment or returns default
// Priority: QMP_SCREENSHOT_FORMAT env var > default ("png")
func (r *ParameterResolver) ResolveScreenshotFormat() string {
	if viper.IsSet("screenshot_format") {
		format := viper.GetString("screenshot_format")
		if format != "" {
			return format
		}
	}
	return "png"
}

// ResolveCommentChar resolves script comment character from environment or returns default
// Priority: QMP_COMMENT_CHAR env var > default ("#")
func (r *ParameterResolver) ResolveCommentChar() string {
	if viper.IsSet("comment_char") {
		commentChar := viper.GetString("comment_char")
		if commentChar != "" {
			return commentChar
		}
	}
	return "#"
}

// ResolveControlChar resolves script control command prefix from environment or returns default
// Priority: QMP_CONTROL_CHAR env var > default ("<#")
func (r *ParameterResolver) ResolveControlChar() string {
	if viper.IsSet("control_char") {
		controlChar := viper.GetString("control_char")
		if controlChar != "" {
			return controlChar
		}
	}
	return "<#"
}
