package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version variables set via ldflags during build
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
	BuiltBy = ""
)

// getVersionInfo returns version information, using VCS info as fallback
func getVersionInfo() (version, commit, date, builtBy string) {
	version = Version
	commit = Commit
	date = Date
	builtBy = BuiltBy

	// If not set via ldflags, try to get from build info (Go 1.18+)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == "" {
					commit = setting.Value
					if len(commit) > 12 {
						commit = commit[:12] // Short hash
					}
				}
			case "vcs.time":
				if date == "" {
					date = setting.Value
				}
			case "vcs.modified":
				if setting.Value == "true" && commit != "" {
					commit += "-dirty"
				}
			}
		}
	}

	// Set defaults for any remaining empty values
	if commit == "" {
		commit = "unknown"
	}
	if date == "" {
		date = "unknown"
	}
	if builtBy == "" {
		builtBy = "go build"
	}

	return
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the current version of roji.`,
	Run: func(cmd *cobra.Command, args []string) {
		version, commit, date, builtBy := getVersionInfo()
		fmt.Printf("roji version %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  built:   %s\n", date)
		fmt.Printf("  by:      %s\n", builtBy)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
