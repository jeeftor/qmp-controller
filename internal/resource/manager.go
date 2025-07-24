package resource

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// ResourceManager handles resource lifecycle and cleanup
type ResourceManager struct {
	mu            sync.RWMutex
	tempFiles     map[string]*TempFile
	connections   map[string]*ManagedConnection
	cleanupFuncs  []func() error
	cancelled     bool
	cancelFunc    context.CancelFunc
}

// TempFile represents a managed temporary file
type TempFile struct {
	Path     string
	File     *os.File
	Created  time.Time
	Prefix   string
	Cleaned  bool
	mutex    sync.Mutex
}

// ManagedConnection represents a managed QMP connection
type ManagedConnection struct {
	Client      *qmp.Client
	VMID        string
	LastUsed    time.Time
	InUse       bool
	CreatedAt   time.Time
	SocketPath  string
	mutex       sync.RWMutex
}

// NewResourceManager creates a new resource manager with context support
func NewResourceManager(ctx context.Context) *ResourceManager {
	ctx, cancel := context.WithCancel(ctx)

	rm := &ResourceManager{
		tempFiles:    make(map[string]*TempFile),
		connections:  make(map[string]*ManagedConnection),
		cleanupFuncs: make([]func() error, 0),
		cancelFunc:   cancel,
	}

	// Start cleanup goroutine
	go rm.cleanupRoutine(ctx)

	return rm
}

// CreateTempFile creates and tracks a temporary file
func (rm *ResourceManager) CreateTempFile(ctx context.Context, prefix string) (*TempFile, error) {
	if rm.cancelled {
		return nil, fmt.Errorf("resource manager is cancelled")
	}

	// Create temporary file
	osFile, err := os.CreateTemp("", prefix+"-*.ppm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	tempFile := &TempFile{
		Path:    osFile.Name(),
		File:    osFile,
		Created: time.Now(),
		Prefix:  prefix,
		Cleaned: false,
	}

	rm.mu.Lock()
	rm.tempFiles[tempFile.Path] = tempFile
	rm.mu.Unlock()

	logging.Debug("Created temporary file",
		"path", tempFile.Path,
		"prefix", prefix)

	// Set up context cancellation
	go func() {
		<-ctx.Done()
		rm.CleanupTempFile(tempFile.Path)
	}()

	return tempFile, nil
}

// CleanupTempFile removes and cleans up a temporary file
func (rm *ResourceManager) CleanupTempFile(path string) error {
	rm.mu.Lock()
	tempFile, exists := rm.tempFiles[path]
	if !exists {
		rm.mu.Unlock()
		return nil // Already cleaned up
	}
	delete(rm.tempFiles, path)
	rm.mu.Unlock()

	tempFile.mutex.Lock()
	defer tempFile.mutex.Unlock()

	if tempFile.Cleaned {
		return nil // Already cleaned up
	}

	var err error
	if tempFile.File != nil {
		if closeErr := tempFile.File.Close(); closeErr != nil {
			err = fmt.Errorf("error closing file: %w", closeErr)
		}
	}

	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		if err != nil {
			err = fmt.Errorf("%w; error removing file: %v", err, removeErr)
		} else {
			err = fmt.Errorf("error removing file: %w", removeErr)
		}
	}

	tempFile.Cleaned = true

	if err != nil {
		logging.Warn("Error cleaning up temporary file", "path", path, "error", err)
	} else {
		logging.Debug("Cleaned up temporary file", "path", path)
	}

	return err
}

// GetOrCreateConnection gets an existing connection or creates a new one
func (rm *ResourceManager) GetOrCreateConnection(ctx context.Context, vmid string, socketPath string) (*ManagedConnection, error) {
	if rm.cancelled {
		return nil, fmt.Errorf("resource manager is cancelled")
	}

	connectionKey := fmt.Sprintf("%s:%s", vmid, socketPath)

	rm.mu.RLock()
	if conn, exists := rm.connections[connectionKey]; exists {
		conn.mutex.RLock()
		if !conn.InUse {
			conn.mutex.RUnlock()
			conn.mutex.Lock()
			conn.InUse = true
			conn.LastUsed = time.Now()
			conn.mutex.Unlock()
			rm.mu.RUnlock()
			logging.Debug("Reusing existing QMP connection", "vmid", vmid, "socket_path", socketPath)
			return conn, nil
		}
		conn.mutex.RUnlock()
	}
	rm.mu.RUnlock()

	// Create new connection
	var client *qmp.Client
	if socketPath != "" {
		client = qmp.NewWithSocketPath(vmid, socketPath)
	} else {
		client = qmp.New(vmid)
	}

	// Connect with context support
	if err := rm.connectWithContext(ctx, client); err != nil {
		return nil, fmt.Errorf("failed to connect to VM %s: %w", vmid, err)
	}

	managedConn := &ManagedConnection{
		Client:     client,
		VMID:       vmid,
		LastUsed:   time.Now(),
		InUse:      true,
		CreatedAt:  time.Now(),
		SocketPath: socketPath,
	}

	rm.mu.Lock()
	rm.connections[connectionKey] = managedConn
	rm.mu.Unlock()

	logging.Debug("Created new QMP connection", "vmid", vmid, "socket_path", socketPath)

	// Set up context cancellation for connection
	go func() {
		<-ctx.Done()
		rm.ReleaseConnection(vmid, socketPath)
	}()

	return managedConn, nil
}

// connectWithContext connects to QMP with context cancellation support
func (rm *ResourceManager) connectWithContext(ctx context.Context, client *qmp.Client) error {
	done := make(chan error, 1)

	go func() {
		done <- client.Connect()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseConnection marks a connection as available for reuse
func (rm *ResourceManager) ReleaseConnection(vmid string, socketPath string) {
	connectionKey := fmt.Sprintf("%s:%s", vmid, socketPath)

	rm.mu.RLock()
	conn, exists := rm.connections[connectionKey]
	rm.mu.RUnlock()

	if exists {
		conn.mutex.Lock()
		conn.InUse = false
		conn.LastUsed = time.Now()
		conn.mutex.Unlock()
		logging.Debug("Released QMP connection", "vmid", vmid, "socket_path", socketPath)
	}
}

// CloseConnection closes and removes a connection
func (rm *ResourceManager) CloseConnection(vmid string, socketPath string) error {
	connectionKey := fmt.Sprintf("%s:%s", vmid, socketPath)

	rm.mu.Lock()
	conn, exists := rm.connections[connectionKey]
	if exists {
		delete(rm.connections, connectionKey)
	}
	rm.mu.Unlock()

	if !exists {
		return nil
	}

	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.Client != nil {
		if err := conn.Client.Close(); err != nil {
			logging.Warn("Error closing QMP connection", "vmid", vmid, "error", err)
			return err
		}
	}

	logging.Debug("Closed QMP connection", "vmid", vmid, "socket_path", socketPath)
	return nil
}

// AddCleanupFunc adds a custom cleanup function
func (rm *ResourceManager) AddCleanupFunc(fn func() error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.cleanupFuncs = append(rm.cleanupFuncs, fn)
}

// CleanupAll performs comprehensive cleanup of all resources
func (rm *ResourceManager) CleanupAll() error {
	if rm.cancelled {
		return nil
	}

	rm.mu.Lock()
	rm.cancelled = true
	rm.mu.Unlock()

	if rm.cancelFunc != nil {
		rm.cancelFunc()
	}

	var errs []error

	// Cleanup temporary files
	rm.mu.RLock()
	tempFilePaths := make([]string, 0, len(rm.tempFiles))
	for path := range rm.tempFiles {
		tempFilePaths = append(tempFilePaths, path)
	}
	rm.mu.RUnlock()

	for _, path := range tempFilePaths {
		if err := rm.CleanupTempFile(path); err != nil {
			errs = append(errs, fmt.Errorf("temp file cleanup %s: %w", path, err))
		}
	}

	// Close all connections
	rm.mu.RLock()
	connectionKeys := make([]string, 0, len(rm.connections))
	for key := range rm.connections {
		connectionKeys = append(connectionKeys, key)
	}
	rm.mu.RUnlock()

	for _, key := range connectionKeys {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			if err := rm.CloseConnection(parts[0], parts[1]); err != nil {
				errs = append(errs, fmt.Errorf("connection cleanup %s: %w", key, err))
			}
		}
	}

	// Run custom cleanup functions
	rm.mu.RLock()
	cleanupFuncs := make([]func() error, len(rm.cleanupFuncs))
	copy(cleanupFuncs, rm.cleanupFuncs)
	rm.mu.RUnlock()

	for i, fn := range cleanupFuncs {
		if err := fn(); err != nil {
			errs = append(errs, fmt.Errorf("cleanup function %d: %w", i, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	logging.Debug("Resource manager cleanup completed successfully")
	return nil
}

// cleanupRoutine runs periodic cleanup of unused resources
func (rm *ResourceManager) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rm.performPeriodicCleanup()
		}
	}
}

// performPeriodicCleanup cleans up unused resources
func (rm *ResourceManager) performPeriodicCleanup() {
	now := time.Now()
	connectionTimeout := 10 * time.Minute
	tempFileTimeout := 30 * time.Minute

	// Cleanup old temporary files
	rm.mu.RLock()
	oldTempFiles := make([]string, 0)
	for path, tempFile := range rm.tempFiles {
		if now.Sub(tempFile.Created) > tempFileTimeout && !tempFile.Cleaned {
			oldTempFiles = append(oldTempFiles, path)
		}
	}
	rm.mu.RUnlock()

	for _, path := range oldTempFiles {
		logging.Warn("Cleaning up old temporary file", "path", path, "age", now.Sub(rm.tempFiles[path].Created))
		rm.CleanupTempFile(path)
	}

	// Cleanup idle connections
	rm.mu.RLock()
	idleConnections := make([]string, 0)
	for key, conn := range rm.connections {
		conn.mutex.RLock()
		if !conn.InUse && now.Sub(conn.LastUsed) > connectionTimeout {
			idleConnections = append(idleConnections, key)
		}
		conn.mutex.RUnlock()
	}
	rm.mu.RUnlock()

	for _, key := range idleConnections {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			logging.Debug("Cleaning up idle connection", "vmid", parts[0], "age", now.Sub(rm.connections[key].LastUsed))
			rm.CloseConnection(parts[0], parts[1])
		}
	}
}

// GetStats returns resource manager statistics
func (rm *ResourceManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	activeConnections := 0
	idleConnections := 0
	for _, conn := range rm.connections {
		conn.mutex.RLock()
		if conn.InUse {
			activeConnections++
		} else {
			idleConnections++
		}
		conn.mutex.RUnlock()
	}

	cleanTempFiles := 0
	dirtyTempFiles := 0
	for _, tempFile := range rm.tempFiles {
		if tempFile.Cleaned {
			cleanTempFiles++
		} else {
			dirtyTempFiles++
		}
	}

	return map[string]interface{}{
		"temp_files_active":      dirtyTempFiles,
		"temp_files_cleaned":     cleanTempFiles,
		"connections_active":     activeConnections,
		"connections_idle":       idleConnections,
		"cleanup_functions":      len(rm.cleanupFuncs),
		"cancelled":             rm.cancelled,
	}
}
