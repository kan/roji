package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check if roji is healthy",
	Long:  "Performs a health check against the local roji instance. Exits with 0 if healthy, 1 otherwise.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := checkHealth(); err != nil {
			fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("healthy")
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func checkHealth() error {
	// Health check via HTTPS (allow self-signed certificates)
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://localhost/_api/health")
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
