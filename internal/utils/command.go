package utils

import (
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
)

// CommandExecutor provides standardized execution patterns for QMP commands
type CommandExecutor struct {
	VMID      string
	Operation string
	Timer     *logging.Timer
	Logger    *logging.ContextualLogger
	Client    *qmp.Client
}

// NewCommandExecutor creates a new standardized command executor
func NewCommandExecutor(vmid string, operation string) *CommandExecutor {
	timer := logging.StartTimer(operation, vmid)
	logger := logging.NewContextualLogger(vmid, operation)

	return &CommandExecutor{
		VMID:      vmid,
		Operation: operation,
		Timer:     timer,
		Logger:    logger,
	}
}

// ConnectToVM establishes QMP connection with standardized error handling and timing
// Note: This will need to be called with the cmd.ConnectToVM function passed in
func (ce *CommandExecutor) ConnectToVM(connectFn func(string) (*qmp.Client, error)) error {
	ce.Logger.Debug("Establishing QMP connection")

	client, err := connectFn(ce.VMID)
	if err != nil {
		ce.Logger.Error("Failed to connect to VM", "error", err)
		ce.Timer.StopWithError(err, map[string]interface{}{
			"stage": "connection",
		})
		return err
	}

	ce.Client = client
	ce.Logger.Debug("QMP connection established successfully")
	return nil
}

// ExecuteWithMetrics executes a function with standardized timing and error handling
func (ce *CommandExecutor) ExecuteWithMetrics(stage string, fn func() error, metrics map[string]interface{}) error {
	ce.Logger.Debug("Starting execution stage", "stage", stage)

	err := fn()
	if err != nil {
		ce.Logger.Error("Execution stage failed", "stage", stage, "error", err)

		stageMetrics := map[string]interface{}{
			"stage": stage,
		}
		// Merge additional metrics
		if metrics != nil {
			for k, v := range metrics {
				stageMetrics[k] = v
			}
		}

		ce.Timer.StopWithError(err, stageMetrics)
		return err
	}

	ce.Logger.Debug("Execution stage completed successfully", "stage", stage)
	return nil
}

// FinishSuccess completes the command execution successfully with metrics
func (ce *CommandExecutor) FinishSuccess(metrics map[string]interface{}) time.Duration {
	if ce.Client != nil {
		ce.Client.Close()
	}

	return ce.Timer.Stop(true, metrics)
}

// FinishError completes the command execution with an error
func (ce *CommandExecutor) FinishError(err error, metrics map[string]interface{}) time.Duration {
	if ce.Client != nil {
		ce.Client.Close()
	}

	return ce.Timer.StopWithError(err, metrics)
}

// Close ensures proper cleanup (defer-safe)
func (ce *CommandExecutor) Close() {
	if ce.Client != nil {
		ce.Client.Close()
	}
}

// WithConnection executes a function with an established QMP connection
func (ce *CommandExecutor) WithConnection(connectFn func(string) (*qmp.Client, error), fn func(*qmp.Client) error) error {
	if err := ce.ConnectToVM(connectFn); err != nil {
		return err
	}
	defer ce.Client.Close()

	return fn(ce.Client)
}

// ExecuteCommand provides a complete standardized command execution pattern
func (ce *CommandExecutor) ExecuteCommand(stages []CommandStage) error {
	for _, stage := range stages {
		if err := ce.ExecuteWithMetrics(stage.Name, stage.Function, stage.Metrics); err != nil {
			return err
		}
	}
	return nil
}

// CommandStage represents a single stage in command execution
type CommandStage struct {
	Name     string
	Function func() error
	Metrics  map[string]interface{}
}

// NewCommandStage creates a new command stage
func NewCommandStage(name string, fn func() error, metrics map[string]interface{}) CommandStage {
	return CommandStage{
		Name:     name,
		Function: fn,
		Metrics:  metrics,
	}
}

// Helper functions for common patterns

// ExecuteWithTimer executes a function with basic timer management
func ExecuteWithTimer(vmid string, operation string, fn func() error) error {
	ce := NewCommandExecutor(vmid, operation)
	defer ce.Close()

	return fn()
}

// ExecuteWithConnection executes a function with QMP connection and timer management
func ExecuteWithConnection(vmid string, operation string, connectFn func(string) (*qmp.Client, error), fn func(*qmp.Client) error) error {
	ce := NewCommandExecutor(vmid, operation)
	defer ce.Close()

	return ce.WithConnection(connectFn, fn)
}

// ExecuteSimpleCommand executes a simple command with connection, timing, and metrics
func ExecuteSimpleCommand(vmid string, operation string, connectFn func(string) (*qmp.Client, error), fn func(*qmp.Client) (map[string]interface{}, error)) error {
	ce := NewCommandExecutor(vmid, operation)
	defer ce.Close()

	if err := ce.ConnectToVM(connectFn); err != nil {
		return err
	}

	metrics, err := fn(ce.Client)
	if err != nil {
		ce.FinishError(err, metrics)
		return err
	}

	ce.FinishSuccess(metrics)
	return nil
}
