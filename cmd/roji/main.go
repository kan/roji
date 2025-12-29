package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/docker"
	"github.com/kan/roji/proxy"
)

var version = "dev"

type Config struct {
	NetworkName   string
	BaseDomain    string
	HTTPPort      int
	HTTPSPort     int
	CertsDir      string
	AutoCert      bool
	DashboardHost string
	LogLevel      string
}

func main() {
	cfg := parseFlags()
	setupLogging(cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down...")
		cancel()
	}()

	if err := run(ctx, cfg); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	cfg := Config{}

	flag.StringVar(&cfg.NetworkName, "network", "roji", "Docker network name to watch")
	flag.StringVar(&cfg.BaseDomain, "domain", "localhost", "Base domain for auto-generated hostnames")
	flag.IntVar(&cfg.HTTPPort, "http-port", 80, "HTTP port (for redirect)")
	flag.IntVar(&cfg.HTTPSPort, "https-port", 443, "HTTPS port")
	flag.StringVar(&cfg.CertsDir, "certs-dir", "/certs", "Directory for TLS certificates")
	flag.BoolVar(&cfg.AutoCert, "auto-cert", true, "Auto-generate certificates if not present")
	flag.StringVar(&cfg.DashboardHost, "dashboard", "", "Dashboard hostname (e.g., roji.localhost)")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("roji %s\n", version)
		os.Exit(0)
	}

	// Allow override from environment
	if v := os.Getenv("ROJI_NETWORK"); v != "" {
		cfg.NetworkName = v
	}
	if v := os.Getenv("ROJI_DOMAIN"); v != "" {
		cfg.BaseDomain = v
	}
	if v := os.Getenv("ROJI_CERTS_DIR"); v != "" {
		cfg.CertsDir = v
	}
	if v := os.Getenv("ROJI_DASHBOARD"); v != "" {
		cfg.DashboardHost = v
	}
	if v := os.Getenv("ROJI_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	// Default dashboard hostname
	if cfg.DashboardHost == "" {
		cfg.DashboardHost = "roji." + cfg.BaseDomain
	}

	return cfg
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))
}

func run(ctx context.Context, cfg Config) error {
	printBanner(cfg)

	// Auto-generate certificates if enabled
	if cfg.AutoCert {
		certGen := certgen.NewGenerator(cfg.CertsDir, cfg.BaseDomain)
		if err := certGen.EnsureCerts(); err != nil {
			return fmt.Errorf("failed to ensure certificates: %w", err)
		}
		slog.Info("certificates ready", "dir", cfg.CertsDir)
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient(cfg.NetworkName, cfg.BaseDomain)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	slog.Info("starting roji",
		"network", cfg.NetworkName,
		"domain", cfg.BaseDomain,
		"http_port", cfg.HTTPPort,
		"https_port", cfg.HTTPSPort,
		"dashboard", cfg.DashboardHost)

	// Initialize router and handler
	router := proxy.NewRouter()
	handler := proxy.NewHandler(router, cfg.DashboardHost)

	// Discover existing containers
	if err := discoverExisting(ctx, dockerClient, router); err != nil {
		return fmt.Errorf("failed to discover containers: %w", err)
	}

	// Start watching for container events
	watcher := docker.NewWatcher(dockerClient)
	eventCh := watcher.Watch(ctx)

	go handleEvents(ctx, dockerClient, router, eventCh)

	// Start HTTP and HTTPS servers
	httpServer := startHTTPServer(cfg)
	httpsServer, err := startHTTPSServer(cfg, handler)
	if err != nil {
		return err
	}

	// Print registered routes
	printRoutes(router)

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	shutdownServers(context.Background(), httpServer, httpsServer)

	slog.Info("shutdown complete")
	return nil
}

func startHTTPServer(cfg Config) *http.Server {
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: &proxy.RedirectHandler{HTTPSPort: cfg.HTTPSPort},
	}

	go func() {
		slog.Info("starting HTTP redirect server", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	return httpServer
}

func startHTTPSServer(cfg Config, handler http.Handler) (*http.Server, error) {
	tlsConfig, err := loadTLSConfig(cfg.CertsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}

	httpsServer := &http.Server{
		Addr:      fmt.Sprintf(":%d", cfg.HTTPSPort),
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	go func() {
		slog.Info("starting HTTPS server", "port", cfg.HTTPSPort)
		if err := httpsServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			slog.Error("HTTPS server error", "error", err)
		}
	}()

	return httpsServer, nil
}

func shutdownServers(ctx context.Context, httpServer, httpsServer *http.Server) {
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	httpServer.Shutdown(shutdownCtx)
	httpsServer.Shutdown(shutdownCtx)
}

func loadTLSConfig(certsDir string) (*tls.Config, error) {
	certFile := certsDir + "/cert.pem"
	keyFile := certsDir + "/key.pem"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func discoverExisting(ctx context.Context, client *docker.Client, router *proxy.Router) error {
	backends, err := client.DiscoverBackends(ctx)
	if err != nil {
		return err
	}

	for _, backend := range backends {
		router.AddBackend(backend)
	}

	slog.Info("discovered existing containers", "count", len(backends))
	return nil
}

func handleEvents(ctx context.Context, client *docker.Client, router *proxy.Router, eventCh <-chan docker.ContainerEvent) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-eventCh:
			if !ok {
				return
			}

			switch event.Type {
			case docker.EventStart:
				handleStartEvent(ctx, client, router, event.ContainerID)
			case docker.EventStop:
				handleStopEvent(ctx, client, router, event.ContainerID)
			}
		}
	}
}

func handleStartEvent(ctx context.Context, client *docker.Client, router *proxy.Router, containerID string) {
	backend, err := client.GetBackend(ctx, containerID)
	if err != nil {
		slog.Error("failed to get backend", "error", err)
		return
	}
	if backend == nil {
		return
	}

	// If this is a compose project, update all backends for the project
	// (hostnames may change based on service count)
	if backend.ProjectName != "" {
		router.RemoveProject(backend.ProjectName)
		backends, err := client.GetProjectBackends(ctx, backend.ProjectName)
		if err != nil {
			slog.Error("failed to get project backends", "error", err)
			return
		}
		for _, b := range backends {
			router.AddBackend(b)
		}
	} else {
		router.AddBackend(backend)
	}
	printRoutes(router)
}

func handleStopEvent(ctx context.Context, client *docker.Client, router *proxy.Router, containerID string) {
	// Get the backend info before removing to check project
	backend, _ := client.GetBackend(ctx, containerID)
	router.RemoveBackend(containerID)

	// If this was part of a project, update remaining siblings' hostnames
	if backend != nil && backend.ProjectName != "" {
		router.RemoveProject(backend.ProjectName)
		backends, err := client.GetProjectBackends(ctx, backend.ProjectName)
		if err != nil {
			slog.Error("failed to get project backends", "error", err)
		} else {
			for _, b := range backends {
				router.AddBackend(b)
			}
		}
	}
	printRoutes(router)
}

func printBanner(cfg Config) {
	fmt.Println()
	fmt.Println("  roji - reverse proxy for local development")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Printf("  Network:   %s\n", cfg.NetworkName)
	fmt.Printf("  Domain:    *.%s\n", cfg.BaseDomain)
	fmt.Printf("  Dashboard: https://%s\n", cfg.DashboardHost)
	fmt.Println()

	// Show CA certificate install hint if auto-cert is enabled
	if cfg.AutoCert {
		fmt.Printf("  CA Cert:   %s/ca.crt (Windows) or ca.pem (macOS/Linux)\n", cfg.CertsDir)
		fmt.Println("  Install the CA certificate in your browser/OS to trust HTTPS.")
		fmt.Println()
	}
}

func printRoutes(router *proxy.Router) {
	routes := router.ListRoutes()
	if len(routes) == 0 {
		slog.Info("no routes registered")
		return
	}

	fmt.Println()
	fmt.Println("📋 Registered Routes:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, r := range routes {
		fmt.Printf("  %s\n", r.String())
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
