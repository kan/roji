package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version variables set via ldflags during build
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the current version of roji.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("roji version %s\n", Version)
		fmt.Printf("  commit: %s\n", Commit)
		fmt.Printf("  built:  %s\n", Date)
		fmt.Printf("  by:     %s\n", BuiltBy)
		fmt.Printf("  go:     %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
