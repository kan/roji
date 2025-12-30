package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	// Version is set by build flags
	Version = "dev"

	// Config flags
	networkName   string
	baseDomain    string
	httpPort      int
	httpsPort     int
	certsDir      string
	autoCert      bool
	dashboardHost string
	logLevel      string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "roji",
	Short: "Reverse proxy for local development",
	Long: `roji - Reverse proxy for local development

Automatically discovers Docker Compose services and makes them accessible via *.localhost with HTTPS.`,
	RunE: runServer,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Server flags
	rootCmd.Flags().StringVarP(&networkName, "network", "n", getEnv("ROJI_NETWORK", "roji"),
		"Docker network name to watch")
	rootCmd.Flags().StringVarP(&baseDomain, "domain", "d", getEnv("ROJI_DOMAIN", "dev.localhost"),
		"Base domain for auto-generated hostnames")
	rootCmd.Flags().IntVar(&httpPort, "http-port", 80,
		"HTTP port (for redirect)")
	rootCmd.Flags().IntVar(&httpsPort, "https-port", 443,
		"HTTPS port")
	rootCmd.Flags().StringVar(&certsDir, "certs-dir", getEnv("ROJI_CERTS_DIR", "/certs"),
		"Directory for TLS certificates")
	rootCmd.Flags().BoolVar(&autoCert, "auto-cert", true,
		"Auto-generate certificates if not present")
	rootCmd.Flags().StringVar(&dashboardHost, "dashboard", getEnv("ROJI_DASHBOARD", ""),
		"Dashboard hostname (e.g., dev.localhost)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", getEnv("ROJI_LOG_LEVEL", "info"),
		"Log level (debug, info, warn, error)")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func runServer(cmd *cobra.Command, args []string) error {
	// Import here to avoid circular dependencies
	setupLogging(logLevel)

	// Default dashboard hostname
	if dashboardHost == "" {
		// Use the base domain itself as dashboard
		dashboardHost = baseDomain
	}

	cfg := Config{
		NetworkName:   networkName,
		BaseDomain:    baseDomain,
		HTTPPort:      httpPort,
		HTTPSPort:     httpsPort,
		CertsDir:      certsDir,
		AutoCert:      autoCert,
		DashboardHost: dashboardHost,
		LogLevel:      logLevel,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println() // Print newline after ^C
		cancel()
	}()

	return run(ctx, cfg)
}
