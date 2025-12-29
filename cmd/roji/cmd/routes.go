package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kan/roji/proxy"
	"github.com/spf13/cobra"
)

var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "List registered routes",
	Long:  `Display all currently registered routes from the running roji server.`,
	RunE:  runRoutes,
}

func init() {
	rootCmd.AddCommand(routesCmd)
}

func runRoutes(cmd *cobra.Command, args []string) error {
	// Build API URL
	apiURL := fmt.Sprintf("https://%s/_api/routes", dashboardHost)
	if dashboardHost == "" {
		// Use default
		apiURL = fmt.Sprintf("https://roji.%s/_api/routes", baseDomain)
	}
	if httpsPort != 443 {
		if dashboardHost == "" {
			apiURL = fmt.Sprintf("https://roji.%s:%d/_api/routes", baseDomain, httpsPort)
		} else {
			apiURL = fmt.Sprintf("https://%s:%d/_api/routes", dashboardHost, httpsPort)
		}
	}

	// Fetch routes from API (skip TLS verification for self-signed certs)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to connect to roji (is it running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var routes []proxy.RouteInfo
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return fmt.Errorf("failed to parse routes: %w", err)
	}

	// Display routes
	if len(routes) == 0 {
		fmt.Println("No routes registered")
		return nil
	}

	fmt.Println()
	fmt.Printf("📋 Registered Routes (%d):\n", len(routes))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, r := range routes {
		fmt.Printf("  %s\n", r.String())
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	return nil
}
