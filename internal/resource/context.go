package resource

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
)

// ContextManager manages application-wide context and resource cleanup
type ContextManager struct {
	rootContext    context.Context
	cancelFunc     context.CancelFunc
	resourceMgr    *ResourceManager
	cleanupTimeout time.Duration
	mu             sync.RWMutex
}

// NewContextManager creates a new context manager with signal handling
func NewContextManager() *ContextManager {
	rootCtx, cancel := context.WithCancel(context.Background())

	cm := &ContextManager{
		rootContext:    rootCtx,
		cancelFunc:     cancel,
		cleanupTimeout: 10 * time.Second,
	}

	// Create resource manager
	cm.resourceMgr = NewResourceManager(rootCtx)

	// Set up signal handling for graceful shutdown
	cm.setupSignalHandling()

	return cm
}

// GetContext returns the root context for operations
func (cm *ContextManager) GetContext() context.Context {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.rootContext
}

// GetResourceManager returns the resource manager
func (cm *ContextManager) GetResourceManager() *ResourceManager {
	return cm.resourceMgr
}

// WithTimeout creates a context with timeout
func (cm *ContextManager) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(cm.rootContext, timeout)
}

// WithDeadline creates a context with deadline
func (cm *ContextManager) WithDeadline(deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(cm.rootContext, deadline)
}

// WithCancel creates a cancellable context
func (cm *ContextManager) WithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(cm.rootContext)
}

// setupSignalHandling sets up signal handlers for graceful shutdown
func (cm *ContextManager) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		logging.Info("Received shutdown signal, initiating graceful shutdown",
			"signal", sig.String())

		// Cancel root context
		cm.cancelFunc()

		// Start cleanup with timeout
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cm.cleanupTimeout)
		defer cleanupCancel()

		// Wait for cleanup or timeout
		done := make(chan error, 1)
		go func() {
			done <- cm.resourceMgr.CleanupAll()
		}()

		select {
		case err := <-done:
			if err != nil {
				logging.Error("Resource cleanup completed with errors", "error", err)
			} else {
				logging.Info("Resource cleanup completed successfully")
			}
		case <-cleanupCtx.Done():
			logging.Warn("Resource cleanup timed out", "timeout", cm.cleanupTimeout)
		}

		// Force exit
		os.Exit(0)
	}()
}

// Shutdown initiates graceful shutdown
func (cm *ContextManager) Shutdown() error {
	logging.Info("Initiating manual shutdown")

	cm.mu.Lock()
	cm.cancelFunc()
	cm.mu.Unlock()

	// Perform cleanup
	return cm.resourceMgr.CleanupAll()
}

// SetCleanupTimeout sets the timeout for cleanup operations
func (cm *ContextManager) SetCleanupTimeout(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cleanupTimeout = timeout
}

// IsActive returns whether the context manager is still active
func (cm *ContextManager) IsActive() bool {
	select {
	case <-cm.rootContext.Done():
		return false
	default:
		return true
	}
}

// WaitForShutdown waits for the context to be cancelled
func (cm *ContextManager) WaitForShutdown() error {
	<-cm.rootContext.Done()
	return cm.rootContext.Err()
}

// RunWithContext executes a function with managed context and resources
func (cm *ContextManager) RunWithContext(fn func(context.Context, *ResourceManager) error) error {
	return fn(cm.rootContext, cm.resourceMgr)
}

// RunWithTimeout executes a function with a timeout context
func (cm *ContextManager) RunWithTimeout(timeout time.Duration, fn func(context.Context, *ResourceManager) error) error {
	ctx, cancel := cm.WithTimeout(timeout)
	defer cancel()

	return fn(ctx, cm.resourceMgr)
}

// GetStats returns comprehensive statistics about the context manager and resources
func (cm *ContextManager) GetStats() map[string]interface{} {
	stats := cm.resourceMgr.GetStats()

	stats["context_active"] = cm.IsActive()
	stats["cleanup_timeout"] = cm.cleanupTimeout.String()

	return stats
}
