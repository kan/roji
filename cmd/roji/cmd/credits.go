package cmd

import (
	"fmt"
	"strings"

	"github.com/kan/roji"
	"github.com/spf13/cobra"
)

var creditsCmd = &cobra.Command{
	Use:   "credits",
	Short: "Show licenses of roji and its dependencies",
	Long: `Show roji's license, followed by the licenses of the Go modules linked
into the binary and the notices for other bundled software.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		texts := []string{roji.License, roji.Credits, roji.ThirdPartyNotices}
		for i, text := range texts {
			texts[i] = strings.TrimRight(text, "\n")
		}
		separator := "\n\n" + strings.Repeat("=", 64) + "\n\n"
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(texts, separator))
	},
}

func init() {
	rootCmd.AddCommand(creditsCmd)
}
