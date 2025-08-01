package ocr

import (
	"fmt"
	"os"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// CaptureConfig holds configuration for OCR capture operations
type CaptureConfig struct {
	TrainingDataPath string
	Columns          int
	Rows             int
	CropEnabled      bool
	CropStartRow     int
	CropEndRow       int
	CropStartCol     int
	CropEndCol       int
	Prefix           string // Prefix for temporary file names
}

// DefaultCaptureConfig returns a default OCR capture configuration
func DefaultCaptureConfig() *CaptureConfig {
	return &CaptureConfig{
		TrainingDataPath: "/Users/jstein/.qmp_training_data.json", // TODO: Use proper default path detection
		Columns:          160,
		Rows:             50,
		CropEnabled:      false,
		Prefix:           "qmp-ocr-capture",
	}
}

// CaptureResult holds the results of an OCR capture operation
type CaptureResult struct {
	Text   []string
	Result *OCRResult
	Error  error
}

// OCRCapture provides unified OCR screenshot capture functionality
type OCRCapture struct {
	config                *CaptureConfig
	logger                *logging.ContextualLogger
	takeScreenshotFunc    func(client *qmp.Client, prefix string) (*os.File, error)
}

// NewOCRCapture creates a new OCR capture instance
func NewOCRCapture(config *CaptureConfig, takeScreenshotFunc func(*qmp.Client, string) (*os.File, error)) *OCRCapture {
	if config == nil {
		config = DefaultCaptureConfig()
	}
	if takeScreenshotFunc == nil {
		// Default implementation (callers should provide their own)
		takeScreenshotFunc = defaultTakeTemporaryScreenshot
	}

	return &OCRCapture{
		config:             config,
		logger:             logging.NewContextualLogger("", "ocr_capture"),
		takeScreenshotFunc: takeScreenshotFunc,
	}
}

// Capture performs a complete OCR capture operation (screenshot + OCR processing)
func (c *OCRCapture) Capture(client *qmp.Client) *CaptureResult {
	if client == nil {
		return &CaptureResult{Error: fmt.Errorf("QMP client not available for OCR capture")}
	}

	// Take temporary screenshot using provided function
	tempFile, err := c.takeScreenshotFunc(client, c.config.Prefix)
	if err != nil {
		c.logger.Error("Failed to take screenshot for OCR", "error", err)
		return &CaptureResult{Error: fmt.Errorf("screenshot failed: %w", err)}
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	// Process with OCR
	result, err := c.processScreenshot(tempFile.Name())
	if err != nil {
		c.logger.Error("OCR processing failed", "error", err)
		return &CaptureResult{Error: fmt.Errorf("OCR processing failed: %w", err)}
	}

	c.logger.Debug("OCR capture completed successfully",
		"lines", len(result.Text),
		"chars", len(result.CharBitmaps))

	return &CaptureResult{
		Text:   result.Text,
		Result: result,
		Error:  nil,
	}
}

// CaptureWithTimeout performs OCR capture with a timeout
func (c *OCRCapture) CaptureWithTimeout(client *qmp.Client, timeout time.Duration) *CaptureResult {
	done := make(chan *CaptureResult, 1)

	go func() {
		done <- c.Capture(client)
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(timeout):
		return &CaptureResult{Error: fmt.Errorf("OCR capture timed out after %v", timeout)}
	}
}

// MockCapture returns mock OCR data for dry-run/testing scenarios
func (c *OCRCapture) MockCapture() *CaptureResult {
	c.logger.Info("Using mock OCR data for dry-run mode")

	mockResult := &OCRResult{
		Text: []string{
			"[DRY-RUN] Mock console output",
			"user@server:~$ echo 'hello world'",
			"hello world",
			"user@server:~$ ls -la",
			"total 12",
			"drwxr-xr-x 2 user user 4096 Jan 01 12:00 .",
			"drwxr-xr-x 3 user user 4096 Jan 01 12:00 ..",
			"-rw-r--r-- 1 user user  220 Jan 01 12:00 .bashrc",
			"user@server:~$ ",
		},
		Width:        160,
		Height:       50,
		CharBitmaps:  make([]CharacterBitmap, 0), // Empty for mock
	}

	return &CaptureResult{
		Text:   mockResult.Text,
		Result: mockResult,
		Error:  nil,
	}
}

// defaultTakeTemporaryScreenshot provides a fallback implementation
func defaultTakeTemporaryScreenshot(client *qmp.Client, prefix string) (*os.File, error) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", prefix+"-*.ppm")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %w", err)
	}

	// Take screenshot to temporary file
	if err := client.ScreenDump(tmpFile.Name(), ""); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("error taking screenshot: %w", err)
	}

	return tmpFile, nil
}

// processScreenshot processes a screenshot file with OCR
func (c *OCRCapture) processScreenshot(screenshotPath string) (*OCRResult, error) {
	var result *OCRResult
	var err error

	if c.config.CropEnabled {
		c.logger.Debug("Processing OCR with cropping enabled",
			"crop_rows", fmt.Sprintf("%d-%d", c.config.CropStartRow, c.config.CropEndRow),
			"crop_cols", fmt.Sprintf("%d-%d", c.config.CropStartCol, c.config.CropEndCol))

		result, err = ProcessScreenshotWithCropAndTrainingData(
			screenshotPath,
			c.config.TrainingDataPath,
			c.config.Columns,
			c.config.Rows,
			c.config.CropStartRow,
			c.config.CropEndRow,
			c.config.CropStartCol,
			c.config.CropEndCol,
		)
	} else {
		c.logger.Debug("Processing OCR without cropping",
			"columns", c.config.Columns,
			"rows", c.config.Rows)

		result, err = ProcessScreenshotWithTrainingData(
			screenshotPath,
			c.config.TrainingDataPath,
			c.config.Columns,
			c.config.Rows,
		)
	}

	return result, err
}

// UpdateConfig updates the capture configuration
func (c *OCRCapture) UpdateConfig(config *CaptureConfig) {
	if config != nil {
		c.config = config
		c.logger.Debug("OCR capture configuration updated")
	}
}

// GetConfig returns the current capture configuration
func (c *OCRCapture) GetConfig() *CaptureConfig {
	return c.config
}

// SetLogger sets a custom logger for the capture instance
func (c *OCRCapture) SetLogger(logger *logging.ContextualLogger) {
	if logger != nil {
		c.logger = logger
	}
}
