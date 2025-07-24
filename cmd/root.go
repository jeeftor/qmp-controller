package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/qmp"
	"github.com/jeeftor/qmp-controller/internal/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
    cfgFile    string
    logLevel   string
    socketPath string

    // Global context and resource management
    contextManager *resource.ContextManager
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
    Use:   "qmp",
    Short: "QMP Controller is a CLI tool for managing QEMU virtual machines",
    Long: `QMP Controller provides a command-line interface to interact with
QEMU's QMP (QEMU Machine Protocol) for managing virtual machines.`,
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        // Default to info level if not specified
        if logLevel == "" {
            logLevel = "info"
        }

        // Initialize logging with the specified level
        logging.InitWithLevel(logLevel)

        logging.Debug("Logging initialized", "level", logLevel)
        logging.Debug("Using socket path", "path", GetSocketPath())
    },
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig, initResourceManagement)

    // Global flags
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.qmp.yaml)")
    rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level (trace, debug, info, warn, error)")
    rootCmd.PersistentFlags().StringVarP(&socketPath, "socket", "s", "", "custom socket path (for SSH tunneling)")

    // Bind flags to Viper
    viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
    viper.BindPFlag("socket", rootCmd.PersistentFlags().Lookup("socket"))
}

// initResourceManagement initializes the global resource management system
func initResourceManagement() {
    if contextManager == nil {
        contextManager = resource.NewContextManager()
        logging.Debug("Resource management initialized")
    }
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
    // Set environment variable prefix for our app
    viper.SetEnvPrefix("QMP")

    // Enable automatic environment variable binding (QMP_DEBUG, QMP_SOCKET, etc.)
    viper.AutomaticEnv()

    // Set default values
    viper.SetDefault("log_level", "info")
    viper.SetDefault("socket", "")

    // Config file setup
    if cfgFile != "" {
        // Use config file from the flag
        viper.SetConfigFile(cfgFile)
    } else {
        // Search for config in standard locations

        // 1. Current directory
        viper.AddConfigPath(".")

        // 2. User's home directory
        home, err := os.UserHomeDir()
        if err == nil {
            viper.AddConfigPath(filepath.Join(home))
        }

        // 3. System config directories
        viper.AddConfigPath("/etc/qmp")

        // Set config name and type
        viper.SetConfigType("yaml")
        viper.SetConfigName(".qmp")
    }

    // Read the config file
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            // Config file was found but another error occurred
            fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
        }
        // It's okay if no config file is found - we'll use defaults and env vars
    }

    // Update variables from viper
    // This ensures they reflect values from config file or env vars
    if logLevel == "" {
        logLevel = viper.GetString("log_level")
    }
    socketPath = viper.GetString("socket")
}

// GetSocketPath returns the socket path from config, env var, or flag
func GetSocketPath() string {
    // First check if the flag was explicitly set
    if socketPath != "" {
        return socketPath
    }

    // Otherwise return from viper (which includes env vars and config file)
    return viper.GetString("socket")
}

// ConnectToVM creates and connects a QMP client for the specified VM
// This centralizes the common pattern used across all commands
func ConnectToVM(vmid string) (*qmp.Client, error) {
    socketPath := GetSocketPath()
    var client *qmp.Client

    err := logging.LogOperation("qmp_connect", vmid, func() error {
        if socketPath != "" {
            client = qmp.NewWithSocketPath(vmid, socketPath)
        } else {
            client = qmp.New(vmid)
        }

        if err := client.Connect(); err != nil {
            logging.LogConnection(vmid, socketPath, false, err)
            return fmt.Errorf("error connecting to VM %s: %v", vmid, err)
        }

        logging.LogConnection(vmid, socketPath, true, nil)
        return nil
    })

    if err != nil {
        return nil, err
    }

    return client, nil
}

// TakeScreenshot captures a screenshot from the VM and saves it to the specified file
// This centralizes the common screenshot pattern used across OCR and screenshot commands
func TakeScreenshot(client *qmp.Client, outputFile string, format string) error {
    return logging.LogOperation("screenshot", "", func() error {
        // Get remote temp path from config
        remotePath := getRemoteTempPath()
        start := time.Now()

        var err error
        if format == "png" {
            logging.Debug("Taking screenshot in PNG format", "output", outputFile, "remoteTempPath", remotePath)
            err = client.ScreenDumpAndConvert(outputFile, remotePath)
        } else {
            logging.Debug("Taking screenshot in PPM format", "output", outputFile, "remoteTempPath", remotePath)
            err = client.ScreenDump(outputFile, remotePath)
        }

        if err != nil {
            return fmt.Errorf("error taking screenshot: %v", err)
        }

        // Log screenshot details with file size
        if stat, statErr := os.Stat(outputFile); statErr == nil {
            logging.LogScreenshot("", outputFile, format, stat.Size(), time.Since(start))
        }

        return nil
    })
}

// TakeTemporaryScreenshot creates a temporary PPM file and captures a screenshot to it
// Returns the temporary file handle and any error. Uses resource management for cleanup.
func TakeTemporaryScreenshot(client *qmp.Client, prefix string) (*os.File, error) {
    // Fallback to legacy implementation if context manager not available
    if contextManager == nil {
        return takeTemporaryScreenshotLegacy(client, prefix)
    }

    // Use resource manager for better cleanup
    ctx := contextManager.GetContext()
    rm := contextManager.GetResourceManager()

    opts := &resource.ScreenshotOptions{
        Format:         "ppm",
        Timeout:        30 * time.Second,
        RemoteTempPath: getRemoteTempPath(),
        Prefix:         prefix,
    }

    // We need the VMID and socket path, but they're not available in this legacy interface
    // For now, create a temporary file the legacy way but with resource tracking
    tempFile, err := rm.CreateTempFile(ctx, prefix)
    if err != nil {
        return nil, fmt.Errorf("error creating temporary file: %v", err)
    }

    // Take screenshot using the client directly (legacy compatibility)
    if err := client.ScreenDump(tempFile.Path, opts.RemoteTempPath); err != nil {
        rm.CleanupTempFile(tempFile.Path)
        return nil, fmt.Errorf("error taking screenshot: %v", err)
    }

    return tempFile.File, nil
}

// takeTemporaryScreenshotLegacy is the original implementation for backward compatibility
func takeTemporaryScreenshotLegacy(client *qmp.Client, prefix string) (*os.File, error) {
    // Create temporary file
    tmpFile, err := os.CreateTemp("", prefix+"-*.ppm")
    if err != nil {
        return nil, fmt.Errorf("error creating temporary file: %v", err)
    }

    // Get remote temp path from config
    remotePath := getRemoteTempPath()

    // Take screenshot to temporary file
    if err := client.ScreenDump(tmpFile.Name(), remotePath); err != nil {
        tmpFile.Close()
        os.Remove(tmpFile.Name())
        return nil, fmt.Errorf("error taking screenshot: %v", err)
    }

    return tmpFile, nil
}

// getRemoteTempPath determines the remote temp path to use based on flags or config
// This centralizes the logic from screenshot.go for use by multiple commands
func getRemoteTempPath() string {
    // Priority 1: Viper (includes command line flags and config file)
    if viper.IsSet("screenshot.remote_temp_path") {
        return viper.GetString("screenshot.remote_temp_path")
    }

    // Default to empty string (local temp file)
    return ""
}
