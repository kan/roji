package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/kan/roji/i18n"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check if roji is healthy",
	Long:  "Performs a health check against the local roji instance. Exits with 0 if healthy, 1 otherwise.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := checkHealth(cmd); err != nil {
			fmt.Fprintln(os.Stderr, i18n.Tf("cmd.health.unhealthy", err))
			os.Exit(1)
		}
		fmt.Println(i18n.T("cmd.health.healthy"))
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func checkHealth(cmd *cobra.Command) error {
	// Health check via HTTPS against the configured dashboard host
	resp, err := apiGet(cmd, "/_api/health", 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
