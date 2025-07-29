package constants

import "fmt"

// Default screen dimensions used throughout the application
const (
	// Console screen dimensions
	DefaultScreenWidth  = 160
	DefaultScreenHeight = 50

	// Common screen sizes
	SmallScreenWidth   = 80
	SmallScreenHeight  = 25

	LargeScreenWidth   = 240
	LargeScreenHeight  = 60

	// Maximum reasonable screen sizes
	MaxScreenWidth     = 1000
	MaxScreenHeight    = 1000

	// Minimum screen sizes
	MinScreenWidth     = 20
	MinScreenHeight    = 10
)

// GetDefaultScreenDimensions returns the default screen dimensions
func GetDefaultScreenDimensions() (int, int) {
	return DefaultScreenWidth, DefaultScreenHeight
}

// ValidateScreenDimensions validates that screen dimensions are within reasonable bounds
func ValidateScreenDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("screen dimensions must be positive integers")
	}

	if width < MinScreenWidth || height < MinScreenHeight {
		return fmt.Errorf("screen dimensions too small: minimum %dx%d", MinScreenWidth, MinScreenHeight)
	}

	if width > MaxScreenWidth || height > MaxScreenHeight {
		return fmt.Errorf("screen dimensions too large: maximum %dx%d", MaxScreenWidth, MaxScreenHeight)
	}

	return nil
}
