package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/config"
	"github.com/kan/roji/docker"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/project"
	"github.com/kan/roji/proxy"
	"golang.org/x/net/http2"
)

// Config holds the server configuration
type Config struct {
	Networks      []string // Docker networks to watch (e.g., ["roji", "custom"])
	BaseDomain    string
	BindAddrs     []string // Addresses to listen on; a single "" means every interface
	HTTPPort      int
	HTTPSPort     int
	CertsDir      string
	AutoCert      bool
	DashboardHost string
	LogLevel      string
	DataDir       string              // Directory for persistent data (project history)
	StaticSites   []config.StaticSite // Static file hosting sites
	ConfigPath    string              // Path to config file (for reload)
}

const (
	// MaxLogFileSize is the maximum size of log file before rotation (10MB)
	MaxLogFileSize = 10 * 1024 * 1024
)

// setupLogging configures logging to stdout and optionally to a file.
// Returns the log file handle (caller should close it) or nil if file logging failed.
func setupLogging(level string, enableFileLogging bool) *os.File {
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

	var writer io.Writer = os.Stdout
	var logFile *os.File

	if enableFileLogging {
		logPath := config.LogFilePath()

		// Ensure log directory exists
		if err := config.EnsureDir(filepath.Dir(logPath)); err == nil {
			// Rotate log file if too large
			rotateLogFile(logPath)

			// Open log file for appending
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				logFile = f
				writer = io.MultiWriter(os.Stdout, f)
			}
		}
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	return logFile
}

// rotateLogFile rotates the log file if it exceeds MaxLogFileSize
func rotateLogFile(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil {
		return // File doesn't exist, no rotation needed
	}

	if info.Size() < MaxLogFileSize {
		return // File is small enough
	}

	// Rename current log to .old (overwrite any existing .old file). Remove
	// first because Windows will not rename onto an existing file; a missing
	// .old is the normal case, so its error says nothing.
	oldPath := logPath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(logPath, oldPath); err != nil {
		// Logging is set up after this runs, so there is nowhere to report it
		// but stderr. Silence would leave the log growing without bound.
		fmt.Fprintf(os.Stderr, "warning: cannot rotate log file %s: %v\n", logPath, err)
	}
}

func run(ctx context.Context, cfg Config) error {
	printBanner(cfg)

	// Auto-generate certificates if enabled
	if cfg.AutoCert {
		certGen := certgen.NewGenerator(cfg.CertsDir, cfg.BaseDomain)

		// Check if existing certificate matches configured domain
		matches, certDNSNames, err := certGen.CheckCertDomain()
		if err != nil {
			return fmt.Errorf("failed to check certificate domain: %w", err)
		}

		if !matches {
			fmt.Println()
			fmt.Println("⚠️  " + i18n.T("server.cert.mismatch"))
			fmt.Println(i18n.Tf("server.cert.current", certDNSNames))
			fmt.Println(i18n.Tf("server.cert.expected", cfg.BaseDomain))
			fmt.Println()
			fmt.Println("   " + i18n.T("server.cert.regenerating"))

			if err := certGen.RegenerateCerts(); err != nil {
				return fmt.Errorf("failed to regenerate certificates: %w", err)
			}

			fmt.Println("   ✓ " + i18n.T("server.cert.regenerated"))
			fmt.Println()
			fmt.Println("   " + i18n.T("server.cert.ca_unchanged"))
			fmt.Println("   " + i18n.T("server.cert.ca_hint"))
			fmt.Println()
			fmt.Println("     " + i18n.T("server.cert.ca_command"))
			fmt.Println()
			return fmt.Errorf("certificate regenerated for new domain - please restart roji")
		}

		if err := certGen.EnsureCerts(); err != nil {
			return fmt.Errorf("failed to ensure certificates: %w", err)
		}
		slog.Info("certificates ready", "dir", cfg.CertsDir)
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient(cfg.Networks, cfg.BaseDomain)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	slog.Info("starting roji",
		"networks", cfg.Networks,
		"domain", cfg.BaseDomain,
		"http_port", cfg.HTTPPort,
		"https_port", cfg.HTTPSPort,
		"dashboard", cfg.DashboardHost)

	// Initialize router and handler
	router := proxy.NewRouter()

	// Initialize project store for history
	projectStorePath := ""
	if cfg.DataDir != "" {
		projectStorePath = cfg.DataDir + "/projects.json"
	}
	projectStore := project.NewStore(projectStorePath)

	// Create status configuration
	version, commit, date, builtBy := getVersionInfo()
	statusConfig := &proxy.StatusConfig{
		Version:       version,
		Commit:        commit,
		Date:          date,
		BuiltBy:       builtBy,
		StartTime:     time.Now(),
		CertsDir:      cfg.CertsDir,
		AutoGenerated: cfg.AutoCert,
		Networks:      cfg.Networks,
		BaseDomain:    cfg.BaseDomain,
		HTTPPort:      cfg.HTTPPort,
		HTTPSPort:     cfg.HTTPSPort,
	}

	handler := proxy.NewHandler(router, dockerClient, cfg.DashboardHost, cfg.BaseDomain, statusConfig, projectStore)

	// Register static sites
	registerStaticSites(cfg, router)

	// Set up config reload callback
	handler.SetConfigReloader(func() error {
		return reloadStaticSites(cfg.ConfigPath, cfg.BaseDomain, cfg.DashboardHost, router)
	})

	// Discover existing containers and projects
	if err := discoverExisting(ctx, dockerClient, router, projectStore); err != nil {
		return fmt.Errorf("failed to discover containers: %w", err)
	}

	// Start watching for container events
	watcher := docker.NewWatcher(dockerClient)
	eventCh := watcher.Watch(ctx)

	go handleEvents(ctx, dockerClient, router, projectStore, eventCh)

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

// closeListeners closes every listener, for giving up on a partially opened set.
func closeListeners(listeners []net.Listener) {
	for _, ln := range listeners {
		ln.Close()
	}
}

// listenAll opens one listener per bind address on port.
//
// A loopback address that will not bind is logged and skipped: the default
// names both 127.0.0.1 and ::1, and a machine with IPv6 disabled still serves
// the same intent — reachable from here and nowhere else — over the other one.
//
// Any other address was asked for deliberately and nothing else stands in for
// it, so failing to bind it is an error rather than a quieter startup.
func listenAll(binds []string, port int, name string) ([]net.Listener, error) {
	var listeners []net.Listener
	for _, bind := range binds {
		addr := net.JoinHostPort(bind, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if config.IsLoopbackAddr(bind) {
				slog.Warn("cannot listen, skipping", "server", name, "address", addr, "error", err)
				continue
			}
			closeListeners(listeners)
			return nil, fmt.Errorf("%s server: cannot listen on %s: %w", name, addr, err)
		}
		listeners = append(listeners, ln)
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("%s server: no address in %q could be bound on port %d",
			name, strings.Join(binds, ","), port)
	}
	return listeners, nil
}

// startHTTPServer starts the HTTP to HTTPS redirect server, or returns nil when
// it cannot listen.
//
// The redirect is a convenience: without it an http:// URL fails instead of
// being sent to https://. Another server already holding port 80 should not
// stop the proxy roji exists to run, so this reports the problem and carries on.
func startHTTPServer(cfg Config) *http.Server {
	listeners, err := listenAll(cfg.BindAddrs, cfg.HTTPPort, "HTTP")
	if err != nil {
		slog.Error("HTTP redirect server unavailable, continuing without it", "error", err)
		return nil
	}

	httpServer := &http.Server{
		Handler:     &proxy.RedirectHandler{HTTPSPort: cfg.HTTPSPort},
		ReadTimeout: 10 * time.Second, // Short timeout for redirect server
		IdleTimeout: 60 * time.Second,
	}

	for _, ln := range listeners {
		slog.Info("starting HTTP redirect server", "address", ln.Addr())
		go func() {
			if err := httpServer.Serve(ln); err != http.ErrServerClosed {
				slog.Error("HTTP server error", "address", ln.Addr(), "error", err)
			}
		}()
	}

	return httpServer
}

func startHTTPSServer(cfg Config, handler http.Handler) (*http.Server, error) {
	tlsConfig, err := loadTLSConfig(cfg.CertsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}

	httpsServer := &http.Server{
		Handler:      handler,
		TLSConfig:    tlsConfig,
		ReadTimeout:  0, // No limit (support large uploads)
		WriteTimeout: 0, // No limit (support SSE/Long Polling)
		IdleTimeout:  120 * time.Second,
	}

	// Enable HTTP/2 support for gRPC
	if err := http2.ConfigureServer(httpsServer, &http2.Server{}); err != nil {
		return nil, fmt.Errorf("failed to configure HTTP/2: %w", err)
	}

	listeners, err := listenAll(cfg.BindAddrs, cfg.HTTPSPort, "HTTPS")
	if err != nil {
		return nil, err
	}

	for _, ln := range listeners {
		slog.Info("starting HTTPS server", "address", ln.Addr(), "http2", true)
		go func() {
			if err := httpsServer.ServeTLS(ln, "", ""); err != http.ErrServerClosed {
				slog.Error("HTTPS server error", "address", ln.Addr(), "error", err)
			}
		}()
	}

	return httpsServer, nil
}

func shutdownServers(ctx context.Context, httpServer, httpsServer *http.Server) {
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	// A Shutdown error means the deadline passed with connections still open,
	// so roji is about to drop them. Worth saying, since the alternative
	// reading of a slow exit is that roji hung.
	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Warn("HTTP server did not shut down cleanly", "error", err)
		}
	}
	if err := httpsServer.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTPS server did not shut down cleanly", "error", err)
	}
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

func discoverExisting(ctx context.Context, client *docker.Client, router *proxy.Router, projectStore *project.Store) error {
	backends, err := client.DiscoverBackends(ctx)
	if err != nil {
		return err
	}

	for _, backend := range backends {
		router.AddBackend(backend)
	}

	// Discover and update projects
	projects, err := client.DiscoverProjects(ctx)
	if err != nil {
		slog.Warn("failed to discover projects", "error", err)
	} else {
		// Mark all existing projects as inactive first
		projectStore.SetAllInactive()

		// Update with discovered active projects
		for _, p := range projects {
			projectStore.Update(&project.Project{
				Name:        p.Name,
				WorkingDir:  p.WorkingDir,
				ConfigFiles: p.ConfigFiles,
				Services:    p.Services,
				LastActive:  time.Now(),
				Active:      true,
			})
		}
		slog.Info("discovered projects", "count", len(projects))
	}

	slog.Info("discovered existing containers", "count", len(backends))
	return nil
}

func handleEvents(ctx context.Context, client *docker.Client, router *proxy.Router, projectStore *project.Store, eventCh <-chan docker.ContainerEvent) {
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
				handleStartEvent(ctx, client, router, projectStore, event.ContainerID)
			case docker.EventStop:
				handleStopEvent(ctx, client, router, projectStore, event.ContainerID)
			}
		}
	}
}

func handleStartEvent(ctx context.Context, client *docker.Client, router *proxy.Router, projectStore *project.Store, containerID string) {
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

		// Update project store
		projectInfo, err := client.GetProjectInfo(ctx, containerID)
		if err == nil && projectInfo != nil {
			var services []string
			for _, b := range backends {
				services = append(services, b.ServiceName)
			}
			projectStore.Update(&project.Project{
				Name:        projectInfo.Name,
				WorkingDir:  projectInfo.WorkingDir,
				ConfigFiles: projectInfo.ConfigFiles,
				Services:    services,
				LastActive:  time.Now(),
				Active:      true,
			})
		}
	} else {
		router.AddBackend(backend)
	}
	printRoutes(router)
}

func handleStopEvent(ctx context.Context, client *docker.Client, router *proxy.Router, projectStore *project.Store, containerID string) {
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

		// Update project store - mark as inactive if no backends remain
		if len(backends) == 0 {
			projectStore.SetActive(backend.ProjectName, false)
		} else {
			// Update services list
			var services []string
			for _, b := range backends {
				services = append(services, b.ServiceName)
			}
			if p := projectStore.Get(backend.ProjectName); p != nil {
				p.Services = services
				p.LastActive = time.Now()
				projectStore.Update(p)
			}
		}
	}
	printRoutes(router)
}

func reloadStaticSites(configPath, baseDomain, dashboardHost string, router *proxy.Router) error {
	// Reload settings from config file
	settings, err := config.Load(configPath, nil)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Clear existing static sites
	router.ClearStaticSites()

	// Re-register static sites with new settings
	for _, site := range settings.StaticSites {
		// Resolve hostname: if no dot, append base domain
		hostname := site.Host
		if !strings.Contains(hostname, ".") {
			hostname = hostname + "." + baseDomain
		}

		// Expand path
		root := config.ExpandPath(site.Root)

		// Validate that the directory exists
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Warn("static site root directory does not exist",
					"host", hostname,
					"root", root)
			} else {
				slog.Warn("failed to stat static site root",
					"host", hostname,
					"root", root,
					"error", err)
			}
			continue
		}
		if !info.IsDir() {
			slog.Warn("static site root is not a directory",
				"host", hostname,
				"root", root)
			continue
		}

		// Register the static site
		router.AddStaticSite(&proxy.StaticBackend{
			Hostname:      hostname,
			Root:          root,
			Index:         site.IndexEnabled(),
			DashboardHost: dashboardHost,
			BasicAuth:     site.GetBasicAuth(),
		})
	}

	return nil
}

func registerStaticSites(cfg Config, router *proxy.Router) {
	if len(cfg.StaticSites) == 0 {
		return
	}

	for _, site := range cfg.StaticSites {
		// Resolve hostname: if no dot, append base domain
		hostname := site.Host
		if !strings.Contains(hostname, ".") {
			hostname = hostname + "." + cfg.BaseDomain
		}

		// Expand path (in case it wasn't already expanded)
		root := config.ExpandPath(site.Root)

		// Validate that the directory exists
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Warn("static site root directory does not exist",
					"host", hostname,
					"root", root)
			} else {
				slog.Warn("failed to stat static site root",
					"host", hostname,
					"root", root,
					"error", err)
			}
			continue
		}
		if !info.IsDir() {
			slog.Warn("static site root is not a directory",
				"host", hostname,
				"root", root)
			continue
		}

		// Register the static site
		router.AddStaticSite(&proxy.StaticBackend{
			Hostname:      hostname,
			Root:          root,
			Index:         site.IndexEnabled(),
			DashboardHost: cfg.DashboardHost,
			BasicAuth:     site.GetBasicAuth(),
		})
	}
}

func printBanner(cfg Config) {
	fmt.Println()
	fmt.Println("  " + i18n.T("server.banner.title"))
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Printf("  %s\n", i18n.Tf("server.banner.networks", strings.Join(cfg.Networks, ", ")))
	fmt.Printf("  %s\n", i18n.Tf("server.banner.domain", cfg.BaseDomain))
	fmt.Printf("  %s\n", i18n.Tf("server.banner.dashboard", cfg.DashboardHost))
	if len(cfg.StaticSites) > 0 {
		fmt.Printf("  %s\n", i18n.Tf("server.banner.static", len(cfg.StaticSites)))
	}
	fmt.Println()

	// Show CA certificate install hint if auto-cert is enabled
	if cfg.AutoCert {
		fmt.Printf("  %s\n", i18n.Tf("server.banner.ca_cert", cfg.CertsDir))
		fmt.Println("  " + i18n.T("server.banner.ca_hint"))
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
	fmt.Println(i18n.T("server.routes.header"))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, r := range routes {
		fmt.Printf("  %s\n", r.String())
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
