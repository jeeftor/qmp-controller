package cmd

import (
	"os"

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
            logging.Debug("Using socket path", "path", socketPath)
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
    if cfgFile != "" {
        // Use config file from the flag.
        viper.SetConfigFile(cfgFile)
    } else {
        // Find home directory.
        home, err := os.UserHomeDir()
        if err != nil {
            return
        }

        // Search config in home directory with name ".qmp" (without extension).
        viper.AddConfigPath(home)
        viper.SetConfigType("yaml")
        viper.SetConfigName(".qmp")
    }

    viper.AutomaticEnv() // read in environment variables that match

    // If a config file is found, read it in.
    viper.ReadInConfig()
}

// GetSocketPath returns the custom socket path if specified
func GetSocketPath() string {
    return socketPath
}
