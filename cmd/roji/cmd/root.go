package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Config flags (values from CLI)
	networkName   string
	baseDomain    string
	bindAddr      string
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
	rootCmd.Flags().StringVar(&bindAddr, "bind", "",
		"Address(es) to listen on (comma-separated; empty for all interfaces)")
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

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		i18n.Init()
		updateCommandMessages()
	}
}

func updateCommandMessages() {
	rootCmd.Short = i18n.T("cmd.root.short")
	rootCmd.Long = i18n.T("cmd.root.long")
	rootCmd.Flags().Lookup("network").Usage = i18n.T("cmd.root.flag.network")
	rootCmd.Flags().Lookup("domain").Usage = i18n.T("cmd.root.flag.domain")
	rootCmd.Flags().Lookup("bind").Usage = i18n.T("cmd.root.flag.bind")
	rootCmd.Flags().Lookup("http-port").Usage = i18n.T("cmd.root.flag.http_port")
	rootCmd.Flags().Lookup("https-port").Usage = i18n.T("cmd.root.flag.https_port")
	rootCmd.Flags().Lookup("certs-dir").Usage = i18n.T("cmd.root.flag.certs_dir")
	rootCmd.Flags().Lookup("auto-cert").Usage = i18n.T("cmd.root.flag.auto_cert")
	rootCmd.Flags().Lookup("dashboard").Usage = i18n.T("cmd.root.flag.dashboard")
	rootCmd.Flags().Lookup("log-level").Usage = i18n.T("cmd.root.flag.log_level")
	rootCmd.Flags().Lookup("data-dir").Usage = i18n.T("cmd.root.flag.data_dir")
	rootCmd.PersistentFlags().Lookup("config").Usage = i18n.T("cmd.root.flag.config")

	versionCmd.Short = i18n.T("cmd.version.short")
	versionCmd.Long = i18n.T("cmd.version.long")

	healthCmd.Short = i18n.T("cmd.health.short")
	healthCmd.Long = i18n.T("cmd.health.long")

	routesCmd.Short = i18n.T("cmd.routes.short")
	routesCmd.Long = i18n.T("cmd.routes.long")

	doctorCmd.Short = i18n.T("cmd.doctor.short")
	doctorCmd.Long = i18n.T("cmd.doctor.long")
	doctorCmd.Flags().Lookup("fix").Usage = i18n.T("cmd.doctor.flag.fix")
	doctorCmd.Flags().Lookup("json").Usage = i18n.T("cmd.doctor.flag.json")

	caCmd.Short = i18n.T("cmd.ca.short")
	caCmd.Long = i18n.T("cmd.ca.long")

	caInstallCmd.Short = i18n.T("cmd.ca.install.short")
	caInstallCmd.Long = i18n.T("cmd.ca.install.long")
	caInstallCmd.Flags().Lookup("user").Usage = i18n.T("cmd.ca.install.flag.user")
	caInstallCmd.Flags().Lookup("firefox").Usage = i18n.T("cmd.ca.install.flag.firefox")
	caInstallCmd.Flags().Lookup("windows").Usage = i18n.T("cmd.ca.install.flag.windows")
	caInstallCmd.Flags().Lookup("force").Usage = i18n.T("cmd.ca.install.flag.force")

	caUninstallCmd.Short = i18n.T("cmd.ca.uninstall.short")
	caUninstallCmd.Long = i18n.T("cmd.ca.uninstall.long")
	caUninstallCmd.Flags().Lookup("user").Usage = i18n.T("cmd.ca.uninstall.flag.user")
	caUninstallCmd.Flags().Lookup("firefox").Usage = i18n.T("cmd.ca.uninstall.flag.firefox")
	caUninstallCmd.Flags().Lookup("windows").Usage = i18n.T("cmd.ca.uninstall.flag.windows")

	caExportCmd.Short = i18n.T("cmd.ca.export.short")
	caExportCmd.Long = i18n.T("cmd.ca.export.long")

	caStatusCmd.Short = i18n.T("cmd.ca.status.short")
	caStatusCmd.Long = i18n.T("cmd.ca.status.long")

	configCmd.Short = i18n.T("cmd.config.short")
	configCmd.Long = i18n.T("cmd.config.long")
	configShowCmd.Short = i18n.T("cmd.config.show.short")
	configShowCmd.Long = i18n.T("cmd.config.show.long")
	configPathCmd.Short = i18n.T("cmd.config.path.short")
	configPathCmd.Long = i18n.T("cmd.config.path.long")
	configInitCmd.Short = i18n.T("cmd.config.init.short")
	configInitCmd.Long = i18n.T("cmd.config.init.long")
	configEditCmd.Short = i18n.T("cmd.config.edit.short")
	configEditCmd.Long = i18n.T("cmd.config.edit.long")

	serviceCmd.Short = i18n.T("cmd.service.short")
	serviceCmd.Long = i18n.T("cmd.service.long")
	serviceInstallCmd.Short = i18n.T("cmd.service.install.short")
	serviceInstallCmd.Long = i18n.T("cmd.service.install.long")
	serviceInstallCmd.Flags().Lookup("user").Usage = i18n.T("cmd.service.install.flag.user")
	serviceUninstallCmd.Short = i18n.T("cmd.service.uninstall.short")
	serviceUninstallCmd.Long = i18n.T("cmd.service.uninstall.long")
	serviceStartCmd.Short = i18n.T("cmd.service.start.short")
	serviceStopCmd.Short = i18n.T("cmd.service.stop.short")
	serviceRestartCmd.Short = i18n.T("cmd.service.restart.short")
	serviceStatusCmd.Short = i18n.T("cmd.service.status.short")

	logCmd.Short = i18n.T("cmd.log.short")
	logCmd.Long = i18n.T("cmd.log.long")
	logCmd.Flags().Lookup("lines").Usage = i18n.T("cmd.log.flag.lines")
	logCmd.Flags().Lookup("no-follow").Usage = i18n.T("cmd.log.flag.no_follow")
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
		case "bind":
			overrides["bind"] = bindAddr
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

	// Determine actual config path
	actualConfigPath := configFile
	if actualConfigPath == "" {
		actualConfigPath = config.ConfigFilePath()
	}

	cfg := Config{
		Networks:      settings.Networks(),
		BaseDomain:    settings.Domain,
		BindAddrs:     settings.BindAddrs(),
		HTTPPort:      settings.HTTPPort,
		HTTPSPort:     settings.HTTPSPort,
		CertsDir:      settings.CertsDir,
		AutoCert:      settings.AutoCert,
		DashboardHost: settings.Dashboard,
		LogLevel:      settings.LogLevel,
		DataDir:       settings.DataDir,
		StaticSites:   settings.StaticSites,
		ConfigPath:    actualConfigPath,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Println() // Print newline after ^C
		fmt.Printf("%s\n", i18n.Tf("cmd.root.shutdown", sig))
		cancel()

		// Wait for second signal to force exit
		sig = <-sigCh
		fmt.Printf("%s\n", i18n.Tf("cmd.root.force_exit", sig))
		os.Exit(1)
	}()

	err = run(ctx, cfg)
	signal.Stop(sigCh)
	return err
}
