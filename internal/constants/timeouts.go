package constants

import "time"

// Default timeouts and delays used throughout the application
const (
	// Keyboard and input delays
	DefaultKeyDelay      = 50 * time.Millisecond
	DefaultInputDelay    = 100 * time.Millisecond
	KeySequenceDelay     = 25 * time.Millisecond

	// Operation timeouts
	DefaultTimeout       = 30 * time.Second
	DefaultShortTimeout  = 5 * time.Second
	DefaultLongTimeout   = 60 * time.Second
	DefaultScriptTimeout = 300 * time.Second

	// Polling intervals
	DefaultPollInterval  = 1 * time.Second
	FastPollInterval     = 500 * time.Millisecond
	SlowPollInterval     = 2 * time.Second

	// Processing delays
	ProcessingDelay      = 100 * time.Millisecond
	RetryDelay          = 1 * time.Second
	WatchPollInterval   = 500 * time.Millisecond

	// Connection timeouts
	ConnectionTimeout    = 10 * time.Second
	SocketTimeout       = 5 * time.Second

	// Screenshot and OCR timeouts
	ScreenshotTimeout   = 15 * time.Second
	OCRProcessingTimeout = 20 * time.Second
)

// GetKeyDelay returns the configured key delay or default
func GetKeyDelay() time.Duration {
	// TODO: Make this configurable from environment variable QMP_KEY_DELAY
	return DefaultKeyDelay
}

// GetScriptDelay returns the configured script delay or default
func GetScriptDelay() time.Duration {
	// TODO: Make this configurable from environment variable QMP_SCRIPT_DELAY
	return ProcessingDelay
}

// GetTimeout returns a timeout duration based on the operation type
func GetTimeout(operation string) time.Duration {
	switch operation {
	case "connection", "connect":
		return ConnectionTimeout
	case "socket":
		return SocketTimeout
	case "screenshot":
		return ScreenshotTimeout
	case "ocr":
		return OCRProcessingTimeout
	case "script":
		return DefaultScriptTimeout
	case "short":
		return DefaultShortTimeout
	case "long":
		return DefaultLongTimeout
	default:
		return DefaultTimeout
	}
}

// GetPollInterval returns a poll interval based on the operation type
func GetPollInterval(operation string) time.Duration {
	switch operation {
	case "fast", "keyboard", "keys":
		return FastPollInterval
	case "slow", "background":
		return SlowPollInterval
	case "watch":
		return WatchPollInterval
	default:
		return DefaultPollInterval
	}
}
