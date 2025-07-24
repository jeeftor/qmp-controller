package resource

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// ScreenshotOptions holds configuration for screenshot operations
type ScreenshotOptions struct {
	Format         string        // "ppm" or "png"
	Timeout        time.Duration // Operation timeout
	RemoteTempPath string        // Remote temp path for remote connections
	Prefix         string        // Prefix for temporary files
}

// DefaultScreenshotOptions returns default screenshot options
func DefaultScreenshotOptions() *ScreenshotOptions {
	return &ScreenshotOptions{
		Format:  "ppm",
		Timeout: 30 * time.Second,
		Prefix:  "qmp-screenshot",
	}
}

// TakeScreenshotWithContext captures a screenshot with context support and resource management
func (rm *ResourceManager) TakeScreenshotWithContext(ctx context.Context, vmid string, socketPath string, outputFile string, opts *ScreenshotOptions) error {
	if opts == nil {
		opts = DefaultScreenshotOptions()
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Get or create connection
	conn, err := rm.GetOrCreateConnection(timeoutCtx, vmid, socketPath)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer rm.ReleaseConnection(vmid, socketPath)

	// Perform screenshot operation with context
	return rm.takeScreenshotWithTimeout(timeoutCtx, conn.Client, outputFile, opts)
}

// TakeTemporaryScreenshotWithContext creates a temporary screenshot file with context support
func (rm *ResourceManager) TakeTemporaryScreenshotWithContext(ctx context.Context, vmid string, socketPath string, opts *ScreenshotOptions) (*TempFile, error) {
	if opts == nil {
		opts = DefaultScreenshotOptions()
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Create temporary file
	tempFile, err := rm.CreateTempFile(timeoutCtx, opts.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Get or create connection
	conn, err := rm.GetOrCreateConnection(timeoutCtx, vmid, socketPath)
	if err != nil {
		rm.CleanupTempFile(tempFile.Path)
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer rm.ReleaseConnection(vmid, socketPath)

	// Take screenshot to temporary file
	if err := rm.takeScreenshotWithTimeout(timeoutCtx, conn.Client, tempFile.Path, opts); err != nil {
		rm.CleanupTempFile(tempFile.Path)
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return tempFile, nil
}

// takeScreenshotWithTimeout performs the actual screenshot operation with timeout
func (rm *ResourceManager) takeScreenshotWithTimeout(ctx context.Context, client *qmp.Client, outputFile string, opts *ScreenshotOptions) error {
	done := make(chan error, 1)
	start := time.Now()

	go func() {
		var err error
		if opts.Format == "png" {
			logging.Debug("Taking screenshot in PNG format",
				"output", outputFile,
				"remote_temp_path", opts.RemoteTempPath)
			err = client.ScreenDumpAndConvert(outputFile, opts.RemoteTempPath)
		} else {
			logging.Debug("Taking screenshot in PPM format",
				"output", outputFile,
				"remote_temp_path", opts.RemoteTempPath)
			err = client.ScreenDump(outputFile, opts.RemoteTempPath)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("screenshot operation failed: %w", err)
		}

		// Log screenshot details with file size
		if stat, statErr := os.Stat(outputFile); statErr == nil {
			logging.LogScreenshot("", outputFile, opts.Format, stat.Size(), time.Since(start))
		}

		return nil

	case <-ctx.Done():
		// Context cancelled or timed out
		if ctx.Err() == context.DeadlineExceeded {
			logging.Warn("Screenshot operation timed out",
				"output_file", outputFile,
				"timeout", opts.Timeout)
			return fmt.Errorf("screenshot operation timed out after %v", opts.Timeout)
		}

		logging.Debug("Screenshot operation cancelled", "output_file", outputFile)
		return fmt.Errorf("screenshot operation cancelled: %w", ctx.Err())
	}
}

// BatchScreenshot captures multiple screenshots concurrently with resource management
func (rm *ResourceManager) BatchScreenshot(ctx context.Context, requests []ScreenshotRequest) ([]ScreenshotResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	results := make([]ScreenshotResult, len(requests))
	semaphore := make(chan struct{}, 5) // Limit concurrent operations

	// Use a separate context for batch operations
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Process all requests concurrently
	done := make(chan int, len(requests))

	for i, req := range requests {
		go func(index int, request ScreenshotRequest) {
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			result := ScreenshotResult{
				Index:   index,
				Request: request,
			}

			if request.OutputFile != "" {
				// Regular screenshot
				err := rm.TakeScreenshotWithContext(batchCtx, request.VMID, request.SocketPath, request.OutputFile, request.Options)
				result.Error = err
			} else {
				// Temporary screenshot
				tempFile, err := rm.TakeTemporaryScreenshotWithContext(batchCtx, request.VMID, request.SocketPath, request.Options)
				result.TempFile = tempFile
				result.Error = err
			}

			results[index] = result
			done <- index
		}(i, req)
	}

	// Wait for all operations to complete or context to be cancelled
	completed := 0
	for completed < len(requests) {
		select {
		case <-done:
			completed++
		case <-ctx.Done():
			// Cancel all remaining operations
			cancel()

			// Wait for cleanup with timeout
			cleanupTimeout := time.After(5 * time.Second)
			for completed < len(requests) {
				select {
				case <-done:
					completed++
				case <-cleanupTimeout:
					logging.Warn("Batch screenshot cleanup timed out",
						"completed", completed,
						"total", len(requests))
					return results, fmt.Errorf("batch screenshot operation cancelled with cleanup timeout")
				}
			}

			return results, fmt.Errorf("batch screenshot operation cancelled: %w", ctx.Err())
		}
	}

	return results, nil
}

// ScreenshotRequest represents a single screenshot request
type ScreenshotRequest struct {
	VMID       string
	SocketPath string
	OutputFile string // Empty for temporary screenshots
	Options    *ScreenshotOptions
}

// ScreenshotResult represents the result of a screenshot operation
type ScreenshotResult struct {
	Index    int
	Request  ScreenshotRequest
	TempFile *TempFile
	Error    error
}
