package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the current version of roji.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("roji %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
