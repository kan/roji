package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kan/roji/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Config flags (values from CLI)
	networkName   string
	baseDomain    string
	httpPort      int
	httpsPort     int
	certsDir      string
	autoCert      bool
	dashboardHost string
	logLevel      string
	dataDir       string
	configFile    string
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
	// Server flags - defaults are empty/zero, actual defaults come from config.Load()
	rootCmd.Flags().StringVarP(&networkName, "network", "n", "",
		"Docker network name(s) to watch (comma-separated for multiple)")
	rootCmd.Flags().StringVarP(&baseDomain, "domain", "d", "",
		"Base domain for auto-generated hostnames")
	rootCmd.Flags().IntVar(&httpPort, "http-port", 0,
		"HTTP port (for redirect)")
	rootCmd.Flags().IntVar(&httpsPort, "https-port", 0,
		"HTTPS port")
	rootCmd.Flags().StringVar(&certsDir, "certs-dir", "",
		"Directory for TLS certificates")
	rootCmd.Flags().BoolVar(&autoCert, "auto-cert", true,
		"Auto-generate certificates if not present")
	rootCmd.Flags().StringVar(&dashboardHost, "dashboard", "",
		"Dashboard hostname (e.g., dev.localhost)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "",
		"Log level (debug, info, warn, error)")
	rootCmd.Flags().StringVar(&dataDir, "data-dir", "",
		"Directory for persistent data (project history)")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"Config file path (default: ~/.config/roji/config.yaml)")
}

// collectCLIOverrides collects flags that were explicitly set on the command line
func collectCLIOverrides(cmd *cobra.Command) map[string]any {
	overrides := make(map[string]any)

	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "network":
			overrides["network"] = networkName
		case "domain":
			overrides["domain"] = baseDomain
		case "http-port":
			overrides["http_port"] = httpPort
		case "https-port":
			overrides["https_port"] = httpsPort
		case "certs-dir":
			overrides["certs_dir"] = certsDir
		case "auto-cert":
			overrides["auto_cert"] = autoCert
		case "dashboard":
			overrides["dashboard"] = dashboardHost
		case "log-level":
			overrides["log_level"] = logLevel
		case "data-dir":
			overrides["data_dir"] = dataDir
		}
	})

	return overrides
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration with priority: CLI > Env > File > Defaults
	overrides := collectCLIOverrides(cmd)
	settings, err := config.Load(configFile, overrides)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Enable file logging for Native Mode
	logFile := setupLogging(settings.LogLevel, true)
	if logFile != nil {
		defer logFile.Close()
	}

	cfg := Config{
		Networks:      settings.Networks(),
		BaseDomain:    settings.Domain,
		HTTPPort:      settings.HTTPPort,
		HTTPSPort:     settings.HTTPSPort,
		CertsDir:      settings.CertsDir,
		AutoCert:      settings.AutoCert,
		DashboardHost: settings.Dashboard,
		LogLevel:      settings.LogLevel,
		DataDir:       settings.DataDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Println() // Print newline after ^C
		fmt.Printf("Received %v, shutting down...\n", sig)
		cancel()

		// Wait for second signal to force exit
		sig = <-sigCh
		fmt.Printf("\nReceived %v again, forcing exit\n", sig)
		os.Exit(1)
	}()

	err = run(ctx, cfg)
	signal.Stop(sigCh)
	return err
}
