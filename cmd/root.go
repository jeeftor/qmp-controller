package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jstein/qmp/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
    cfgFile    string
    debug      bool
    socketPath string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
    Use:   "qmp",
    Short: "QMP Controller is a CLI tool for managing QEMU virtual machines",
    Long: `QMP Controller provides a command-line interface to interact with
QEMU's QMP (QEMU Machine Protocol) for managing virtual machines.`,
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        // Initialize logging based on debug flag
        logging.Init(debug)

        if debug {
            logging.Debug("Debug mode enabled")
            logging.Debug("Using socket path", "path", GetSocketPath())
        }
    },
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)

    // Global flags
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.qmp.yaml)")
    rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug output")
    rootCmd.PersistentFlags().StringVarP(&socketPath, "socket", "s", "", "custom socket path (for SSH tunneling)")

    // Bind flags to Viper
    viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
    viper.BindPFlag("socket", rootCmd.PersistentFlags().Lookup("socket"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
    // Set environment variable prefix for our app
    viper.SetEnvPrefix("QMP")

    // Enable automatic environment variable binding (QMP_DEBUG, QMP_SOCKET, etc.)
    viper.AutomaticEnv()

    // Set default values
    viper.SetDefault("debug", false)
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
    } else if debug {
        logging.Debug("Using config file", "path", viper.ConfigFileUsed())
    }

    // Update the debug and socketPath variables from viper
    // This ensures they reflect values from config file or env vars
    debug = viper.GetBool("debug")
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
