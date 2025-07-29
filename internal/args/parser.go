package args

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/params"
)

// ParsedArguments represents the result of argument parsing
type ParsedArguments struct {
	VMID            string
	ScriptFile      string
	TrainingData    string
	OutputFile      string
	SubCommand      string
	RemainingArgs   []string
	Source          string // For debugging: shows how args were resolved
}

// ArgumentParser interface for different command argument parsing strategies
type ArgumentParser interface {
	Parse(args []string) (*ParsedArguments, error)
	GetExpectedFormats() []string
	GetDescription() string
}

// ScriptArgumentParser handles script command arguments (script, script2)
type ScriptArgumentParser struct {
	resolver *params.ParameterResolver
}

// NewScriptArgumentParser creates a new script argument parser
func NewScriptArgumentParser() *ScriptArgumentParser {
	return &ScriptArgumentParser{
		resolver: params.NewParameterResolver(),
	}
}

func (p *ScriptArgumentParser) Parse(args []string) (*ParsedArguments, error) {
	result := &ParsedArguments{}

	switch len(args) {
	case 3:
		// Traditional format: vmid scriptfile trainingdata
		result.VMID = args[0]
		result.ScriptFile = args[1]
		result.TrainingData = args[2]
		result.Source = "3-arg format (vmid, script, training)"

	case 2:
		// Could be: vmid scriptfile OR scriptfile trainingdata
		vmidInfo, err := p.resolver.ResolveVMIDWithInfo(args, 0)
		if err == nil {
			// First arg is valid VM ID
			result.VMID = vmidInfo.Value
			result.ScriptFile = args[1]
			result.TrainingData = p.resolver.ResolveTrainingData([]string{}, -1)
			result.Source = fmt.Sprintf("2-arg format (vmid from arg, training from %s)", vmidInfo.Source)
		} else {
			// First arg is not VM ID, try environment variable
			vmidInfo, err := p.resolver.ResolveVMIDWithInfo([]string{}, -1)
			if err != nil {
				return nil, fmt.Errorf("VM ID is required: %w", err)
			}
			result.VMID = vmidInfo.Value
			result.ScriptFile = args[0]
			result.TrainingData = args[1]
			result.Source = fmt.Sprintf("2-arg format (vmid from %s, script+training from args)", vmidInfo.Source)
		}

	case 1:
		// Only script file, VM ID and training data from env vars
		vmidInfo, err := p.resolver.ResolveVMIDWithInfo([]string{}, -1)
		if err != nil {
			return nil, fmt.Errorf("VM ID is required: %w", err)
		}
		result.VMID = vmidInfo.Value
		result.ScriptFile = args[0]
		result.TrainingData = p.resolver.ResolveTrainingData([]string{}, -1)
		result.Source = fmt.Sprintf("1-arg format (vmid from %s, training from env/default)", vmidInfo.Source)

	default:
		return nil, fmt.Errorf("invalid number of arguments: expected 1-3, got %d", len(args))
	}

	// Validate script file exists
	if result.ScriptFile == "" {
		return nil, fmt.Errorf("script file is required")
	}

	if _, err := os.Stat(result.ScriptFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("script file '%s' not found", result.ScriptFile)
	}

	return result, nil
}

func (p *ScriptArgumentParser) GetExpectedFormats() []string {
	return []string{
		"script.sc2",
		"vmid script.sc2",
		"vmid script.sc2 training.json",
		"script.sc2 training.json (if QMP_VM_ID set)",
	}
}

func (p *ScriptArgumentParser) GetDescription() string {
	return "Parses script command arguments with flexible VM ID and training data resolution"
}

// USBArgumentParser handles USB command arguments
type USBArgumentParser struct {
	resolver *params.ParameterResolver
}

// NewUSBArgumentParser creates a new USB argument parser
func NewUSBArgumentParser() *USBArgumentParser {
	return &USBArgumentParser{
		resolver: params.NewParameterResolver(),
	}
}

func (p *USBArgumentParser) Parse(args []string) (*ParsedArguments, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("subcommand is required (add, remove, list)")
	}

	result := &ParsedArguments{
		SubCommand: args[0],
		RemainingArgs: args[1:],
	}

	remainingArgs := args[1:]

	// USB list doesn't need VM ID, but if provided in remaining args, try to use it
	if result.SubCommand == "list" {
		if len(remainingArgs) > 0 {
			// Try to resolve VM ID from the argument
			vmidInfo, err := p.resolver.ResolveVMIDWithInfo(remainingArgs, 0)
			if err == nil {
				result.VMID = vmidInfo.Value
				result.Source = fmt.Sprintf("list subcommand (vmid from %s)", vmidInfo.Source)
				// Remove VM ID from remaining args if it was found there
				if len(remainingArgs) > 0 && remainingArgs[0] == result.VMID {
					result.RemainingArgs = remainingArgs[1:]
				}
			} else {
				result.Source = "list subcommand (no VM ID required)"
			}
		} else {
			result.Source = "list subcommand (no VM ID required)"
		}
		return result, nil
	}

	// USB add/remove need VM ID
	vmidInfo, err := p.resolver.ResolveVMIDWithInfo(remainingArgs, 0)
	if err != nil {
		return nil, fmt.Errorf("VM ID is required for %s command: %w", result.SubCommand, err)
	}

	result.VMID = vmidInfo.Value
	result.Source = fmt.Sprintf("%s subcommand (vmid from %s)", result.SubCommand, vmidInfo.Source)

	// Remove VM ID from remaining args if it was found there
	if len(remainingArgs) > 0 && remainingArgs[0] == result.VMID {
		result.RemainingArgs = remainingArgs[1:]
	} else {
		result.RemainingArgs = remainingArgs
	}

	return result, nil
}

func (p *USBArgumentParser) GetExpectedFormats() []string {
	return []string{
		"list",
		"add vmid device_path",
		"remove vmid device_id",
		"add device_path (if QMP_VM_ID set)",
		"remove device_id (if QMP_VM_ID set)",
	}
}

func (p *USBArgumentParser) GetDescription() string {
	return "Parses USB command arguments with flexible VM ID resolution for add/remove operations"
}

// KeyboardArgumentParser handles keyboard command arguments
type KeyboardArgumentParser struct {
	resolver *params.ParameterResolver
}

// NewKeyboardArgumentParser creates a new keyboard argument parser
func NewKeyboardArgumentParser() *KeyboardArgumentParser {
	return &KeyboardArgumentParser{
		resolver: params.NewParameterResolver(),
	}
}

func (p *KeyboardArgumentParser) Parse(args []string) (*ParsedArguments, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("subcommand is required (live, send, type)")
	}

	result := &ParsedArguments{
		SubCommand: args[0],
	}

	remainingArgs := args[1:]

	// All keyboard commands need VM ID
	vmidInfo, err := p.resolver.ResolveVMIDWithInfo(remainingArgs, 0)
	if err != nil {
		return nil, fmt.Errorf("VM ID is required for %s command: %w", result.SubCommand, err)
	}

	result.VMID = vmidInfo.Value
	result.Source = fmt.Sprintf("%s subcommand (vmid from %s)", result.SubCommand, vmidInfo.Source)

	// Remove VM ID from remaining args if it was found there
	if len(remainingArgs) > 0 && remainingArgs[0] == result.VMID {
		result.RemainingArgs = remainingArgs[1:]
	} else {
		result.RemainingArgs = remainingArgs
	}

	return result, nil
}

func (p *KeyboardArgumentParser) GetExpectedFormats() []string {
	return []string{
		"live vmid",
		"send vmid key_sequence",
		"type vmid text_to_type",
		"live (if QMP_VM_ID set)",
		"send key_sequence (if QMP_VM_ID set)",
		"type text_to_type (if QMP_VM_ID set)",
	}
}

func (p *KeyboardArgumentParser) GetDescription() string {
	return "Parses keyboard command arguments with flexible VM ID resolution"
}

// OCRArgumentParser handles OCR command arguments with flexible file type detection
type OCRArgumentParser struct {
	resolver *params.ParameterResolver
}

// NewOCRArgumentParser creates a new OCR argument parser
func NewOCRArgumentParser() *OCRArgumentParser {
	return &OCRArgumentParser{
		resolver: params.NewParameterResolver(),
	}
}

func (p *OCRArgumentParser) Parse(args []string) (*ParsedArguments, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("arguments required (vm/file + search text)")
	}

	result := &ParsedArguments{}

	// Use existing flexible argument parsing from internal/ocr/argument_parsing.go patterns
	// This maintains compatibility with the existing OCR command's smart file detection

	// Try to identify VM ID, image file, and search text from arguments
	vmidFound := false
	imageFileFound := false

	for i, arg := range args {
		// Check if this looks like a VM ID (numeric)
		if !vmidFound {
			if vmidInfo, err := p.resolver.ResolveVMIDWithInfo(args, i); err == nil {
				result.VMID = vmidInfo.Value
				result.Source = fmt.Sprintf("vmid from %s at position %d", vmidInfo.Source, i)
				vmidFound = true
				continue
			}
		}

		// Check if this looks like an image file
		if !imageFileFound && isImageFile(arg) {
			result.OutputFile = arg
			imageFileFound = true
			continue
		}

		// Check if this looks like a training data file
		if result.TrainingData == "" && isJSONFile(arg) {
			result.TrainingData = arg
			continue
		}

		// Remaining arguments are search text or other parameters
		result.RemainingArgs = append(result.RemainingArgs, arg)
	}

	// Apply fallbacks for missing values
	if !vmidFound {
		if vmidInfo, err := p.resolver.ResolveVMIDWithInfo([]string{}, -1); err == nil {
			result.VMID = vmidInfo.Value
			result.Source = fmt.Sprintf("vmid from %s (fallback)", vmidInfo.Source)
		} else {
			return nil, fmt.Errorf("VM ID is required: %w", err)
		}
	}

	if result.TrainingData == "" {
		result.TrainingData = p.resolver.ResolveTrainingData([]string{}, -1)
	}

	return result, nil
}

func (p *OCRArgumentParser) GetExpectedFormats() []string {
	return []string{
		"vmid search_text",
		"vmid image.png search_text",
		"vmid training.json search_text",
		"image.png vmid search_text",
		"search_text (if QMP_VM_ID set)",
		"flexible argument order supported",
	}
}

func (p *OCRArgumentParser) GetDescription() string {
	return "Parses OCR command arguments with flexible file type detection and VM ID resolution"
}

// SimpleArgumentParser handles commands that just need VM ID + remaining args
type SimpleArgumentParser struct {
	resolver *params.ParameterResolver
	commandName string
}

// NewSimpleArgumentParser creates a simple argument parser for basic commands
func NewSimpleArgumentParser(commandName string) *SimpleArgumentParser {
	return &SimpleArgumentParser{
		resolver: params.NewParameterResolver(),
		commandName: commandName,
	}
}

func (p *SimpleArgumentParser) Parse(args []string) (*ParsedArguments, error) {
	result := &ParsedArguments{
		RemainingArgs: make([]string, 0),
	}

	// Try to resolve VM ID from arguments or environment
	vmidInfo, err := p.resolver.ResolveVMIDWithInfo(args, 0)
	if err != nil {
		return nil, fmt.Errorf("VM ID is required for %s command: %w", p.commandName, err)
	}

	result.VMID = vmidInfo.Value
	result.Source = fmt.Sprintf("%s command (vmid from %s)", p.commandName, vmidInfo.Source)

	// Remove VM ID from remaining args if it was found there
	if len(args) > 0 && args[0] == result.VMID {
		result.RemainingArgs = args[1:]
	} else {
		result.RemainingArgs = args
	}

	return result, nil
}

func (p *SimpleArgumentParser) GetExpectedFormats() []string {
	return []string{
		fmt.Sprintf("%s vmid [additional_args...]", p.commandName),
		fmt.Sprintf("%s [additional_args...] (if QMP_VM_ID set)", p.commandName),
	}
}

func (p *SimpleArgumentParser) GetDescription() string {
	return fmt.Sprintf("Parses %s command arguments with VM ID resolution", p.commandName)
}

// Utility functions for file type detection
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".ppm"
}

func isJSONFile(filename string) bool {
	return strings.ToLower(filepath.Ext(filename)) == ".json"
}

// HandleParseError provides consistent error handling for argument parsing
func HandleParseError(err error, parser ArgumentParser) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	fmt.Fprintf(os.Stderr, "Expected formats:\n")
	for _, format := range parser.GetExpectedFormats() {
		fmt.Fprintf(os.Stderr, "  %s\n", format)
	}
	fmt.Fprintf(os.Stderr, "\nDescription: %s\n", parser.GetDescription())
	os.Exit(1)
}

// ParseWithHandler combines parsing and error handling in one call
func ParseWithHandler(args []string, parser ArgumentParser) *ParsedArguments {
	result, err := parser.Parse(args)
	if err != nil {
		HandleParseError(err, parser)
	}
	return result
}
