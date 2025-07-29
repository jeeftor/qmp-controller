package ui

import (
	"fmt"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/styles"
)

// Icons for consistent UI messaging
const (
	SuccessIcon  = "✅"
	ErrorIcon    = "❌"
	InfoIcon     = "ℹ️"
	WarningIcon  = "⚠️"
	ProgressIcon = "⏳"
	ResultIcon   = "📊"
	HeaderIcon   = "🔸"
)

// Helper functions for styling specific types of content
func Success(text string) string {
	return styles.SuccessStyle.Render(text)
}

func Error(text string) string {
	return styles.ErrorStyle.Render(text)
}

func Info(text string) string {
	return styles.InfoStyle.Render(text)
}

func Warning(text string) string {
	return styles.WarningStyle.Render(text)
}

func Bold(text string) string {
	return styles.BoldStyle.Render(text)
}

func Muted(text string) string {
	return styles.MutedStyle.Render(text)
}

func Key(text string) string {
	return styles.KeyStyle.Render(text)
}

func Value(text string) string {
	return styles.ValueStyle.Render(text)
}

func Code(text string) string {
	return styles.CodeStyle.Render(text)
}

// StatusMessage formats a status message with consistent styling
func StatusMessage(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", SuccessIcon, message)
}

// ErrorMessage formats an error message with consistent styling
func ErrorMessage(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", ErrorIcon, Error(message))
}

// InfoMessage formats an informational message with consistent styling
func InfoMessage(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", InfoIcon, message)
}

// VMOperationSuccess formats a successful VM operation message
func VMOperationSuccess(vmid, operation, details string) {
	fmt.Printf("%s %s %s for VM %s\n",
		SuccessIcon,
		Success(strings.Title(operation)),
		details,
		Bold(vmid))
}

// VMOperationInfo formats an informational VM operation message
func VMOperationInfo(vmid, operation, details string) {
	fmt.Printf("%s %s %s for VM %s\n",
		InfoIcon,
		Info(strings.Title(operation)),
		details,
		Bold(vmid))
}

// USBDeviceMessage formats USB device operation messages
func USBDeviceMessage(vmid, operation, deviceType, deviceID string) {
	switch operation {
	case "added":
		fmt.Printf("%s Added USB %s with ID %s to VM %s\n",
			SuccessIcon,
			Bold(deviceType),
			Code(deviceID),
			Bold(vmid))
	case "removed":
		fmt.Printf("%s Removed device %s from VM %s\n",
			SuccessIcon,
			Code(deviceID),
			Bold(vmid))
	case "listed":
		fmt.Printf("%s USB devices for VM %s:\n",
			InfoIcon,
			Bold(vmid))
	}
}

// KeyboardOperationMessage formats keyboard operation messages
func KeyboardOperationMessage(vmid, operation, details string) {
	fmt.Printf("%s %s %s for VM %s\n",
		SuccessIcon,
		Success(strings.Title(operation)),
		details,
		Bold(vmid))
}

// ScriptMessage formats script execution messages
func ScriptMessage(vmid, scriptFile string) {
	fmt.Printf("🖥️  Target VM: %s\n", Bold(vmid))
	fmt.Printf("📜 Script: %s\n", Code(scriptFile))
}

// EnvironmentVariableExample formats environment variable usage examples
func EnvironmentVariableExample(varName, example string) {
	fmt.Printf("export %s=%s\n",
		Key(varName),
		Value(example))
}

// CommandExample formats command usage examples
func CommandExample(command, description string) {
	fmt.Printf("%-40s # %s\n",
		Code(command),
		Muted(description))
}

// SectionHeader formats a section header with styling
func SectionHeader(title string) {
	fmt.Printf("\n%s %s\n", HeaderIcon, Bold(title))
}

// BulletPoint formats a bullet point with consistent styling
func BulletPoint(text string) {
	fmt.Printf("• %s\n", text)
}

// ValidationErrorMsg formats validation error messages
func ValidationErrorMsg(field, message string) {
	fmt.Printf("%s Validation error for %s: %s\n",
		ErrorIcon,
		Bold(field),
		Error(message))
}

// ProgressMessage formats progress messages
func ProgressMessage(operation, details string) {
	fmt.Printf("%s %s: %s\n",
		ProgressIcon,
		Bold(operation),
		details)
}

// ResultSummary formats a results summary
func ResultSummary(operation string, results map[string]interface{}) {
	fmt.Printf("\n%s %s Summary:\n", ResultIcon, Bold(strings.Title(operation)))
	for key, value := range results {
		fmt.Printf("  %s: %v\n", Key(key), value)
	}
}
