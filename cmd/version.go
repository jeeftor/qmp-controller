package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/jeeftor/qmp-controller/internal/logging"
	"github.com/jeeftor/qmp-controller/internal/styles"
	"github.com/spf13/cobra"
)

// These variables will be set during the build using ldflags
var (
	buildVersion         = "dev"
	buildTag             = "unknown"
	buildCommitsSinceTag = "unknown"
	buildBranch          = "unknown"
	buildCommit          = "none"
	buildTime            = "unknown"
)

var shortOutput bool

// GetFormattedBuildTime returns the build time in a readable format
func GetFormattedBuildTime() string {
	if buildTime == "unknown" {
		return buildTime
	}

	// First try to parse as RFC3339 format
	if t, err := time.Parse(time.RFC3339, buildTime); err == nil {
		return t.Format("2006-01-02 15:04:05 MST")
	}

	// Then try to parse as Unix timestamp
	if unixTime, err := parseInt64(buildTime); err == nil {
		t := time.Unix(unixTime, 0)
		return t.Format("2006-01-02 15:04:05 MST")
	}

	// Return original if parsing fails
	return buildTime
}

// Helper function to parse string to int64
func parseInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// GetDisplayVersion returns a formatted version string
// If on a tagged commit (buildCommitsSinceTag == "0"), returns the tag (e.g., "v0.0.1")
// Otherwise, returns "dev (N commits since <tag>)" (e.g., "dev (6 commits since v0.1.2)")
func GetDisplayVersion() string {
	// Parse buildCommitsSinceTag as an integer
	commits, err := strconv.Atoi(buildCommitsSinceTag)
	if err != nil || buildCommitsSinceTag == "unknown" {
		return "dev" // Fallback if parsing fails or no valid tag
	}

	// If no commits since tag, return the tag as-is
	if commits == 0 {
		return buildTag
	}

	// Otherwise, return dev with commit count and tag
	return fmt.Sprintf("dev (%d commits since %s)", commits, buildTag)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		displayVersion := GetDisplayVersion()

		// Log version request for analytics
		logging.Debug("Version command executed",
			"version", buildVersion,
			"display_version", displayVersion,
			"commit", buildCommit,
			"build_time", buildTime,
			"short_output", shortOutput)

		if shortOutput {
			// For short output, just show the raw buildVersion for scripts
			fmt.Println(buildVersion)
			logging.Debug("Version output", "format", "short", "version", buildVersion)
			return
		}

		// Use lipgloss styles for consistent theming
		labelStyle := styles.MutedStyle
		versionStyle := styles.InfoStyle
		buildStyle := styles.WarningStyle
		commitStyle := styles.SuccessStyle
		osArchStyle := styles.BoldStyle
		goVersionStyle := styles.ErrorStyle
		pathStyle := styles.DebugStyle

		// Get executable path for metrics
		exe, err := os.Executable()
		exePath := "Unknown"
		if err == nil {
			exePath, _ = filepath.Abs(exe)
		} else {
			logging.Warn("Failed to get executable path", "error", err)
		}

		// Log full version info for debugging
		logging.Debug("Version information",
			"display_version", displayVersion,
			"build_time", GetFormattedBuildTime(),
			"commit", buildCommit,
			"os_arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			"go_version", runtime.Version(),
			"executable_path", exePath)

		// Display formatted output
		fmt.Printf("%s %s\n", labelStyle.Render("Version:"), versionStyle.Render(displayVersion))
		fmt.Printf("%s %s\n", labelStyle.Render("Built:  "), buildStyle.Render(GetFormattedBuildTime()))
		fmt.Printf("%s %s\n", labelStyle.Render("Commit: "), commitStyle.Render(fmt.Sprintf("%s (%s)", buildCommit, buildBranch)))
		fmt.Printf("%s %s\n", labelStyle.Render("OS/Arch:"), osArchStyle.Render(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)))
		fmt.Printf("%s %s\n", labelStyle.Render("Go:     "), goVersionStyle.Render(runtime.Version()))
		fmt.Printf("%s %s\n", labelStyle.Render("Binary: "), pathStyle.Render(exePath))
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&shortOutput, "short", "n", false, "Print only version number")
	rootCmd.AddCommand(versionCmd)
}
