// Package proxy provides HTTP/HTTPS reverse proxy functionality.
//
// It handles:
//   - Dynamic routing based on hostname and path
//   - TLS termination with auto-generated certificates
//   - Real-time dashboard with Server-Sent Events (SSE)
//   - Health check and status endpoints
package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/docker"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/project"
	"golang.org/x/net/http2"
)

// sharedTransport is used for connection pooling across all proxied requests (HTTP/1.1)
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// http2Transport is used for gRPC proxying (HTTP/2)
var http2Transport = &http2.Transport{
	AllowHTTP: true, // Allow h2c (HTTP/2 without TLS) for backend connections
	DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
		// Use plain TCP for backend connections (h2c)
		return net.Dial(network, addr)
	},
}

// isGRPCRequest checks if the request is a gRPC request
func isGRPCRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/grpc")
}

// clientIP extracts the bare client IP from an http.Request RemoteAddr,
// or an empty string when RemoteAddr cannot be parsed.
//
// Go reports IPv6 peers as "[::1]:54321", and the X-Forwarded-* headers carry
// bare addresses by convention, so the brackets have to go.
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return ""
	}
	return host
}

// hostWithoutPort strips the port from a Host header value, always returning a
// bare host: an IPv6 literal loses its brackets ("[::1]:443" -> "::1"), so use
// hostForURL to put one back in a URL.
//
// Unlike RemoteAddr, a Host header carrying no port is normal rather than
// malformed, so such a value is kept.
func hostWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// No port to split off; an IPv6 literal is still bracketed here.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

// hostForURL wraps a bare IPv6 address, as returned by hostWithoutPort, in the
// brackets a URL authority requires. Other hosts are returned unchanged.
func hostForURL(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// stripURLPathPrefix removes a route's path prefix from an outbound URL,
// keeping the decoded and encoded forms of the path in step.
//
// url.URL holds the client's encoding in RawPath only while it agrees with
// Path, and EscapedPath goes back to re-escaping Path once the two diverge.
// Rewriting Path on its own therefore leaves a stale RawPath behind and the
// backend receives a re-escaped path: "/api/files/a%2Fb" arrives as
// "/files/a/b", with one segment turned into two.
func stripURLPathPrefix(u *url.URL, prefix string) {
	path, ok := matchPathPrefix(u.Path, prefix)
	if !ok {
		return
	}
	u.Path = path

	// An empty RawPath means Path re-escapes to exactly what the client sent,
	// so there is nothing to keep in step.
	if u.RawPath == "" {
		return
	}
	// A RawPath the prefix does not match is encoded in some way this cannot
	// follow. matchPathPrefix yields "" there, which falls back to escaping
	// Path — what a route without a prefix does anyway.
	u.RawPath, _ = matchPathPrefix(u.RawPath, prefix)
}

// setProtoAndRealIP sets the two forwarding headers neither httputil's
// SetXForwarded nor roji's own scrubbing gets right on its own.
//
// The scheme is always https: roji terminates TLS in front of every backend,
// and its HTTP listener only redirects, so what the inbound connection used
// does not matter. X-Real-IP is not a standard header, so it is replaced here
// rather than left to the client.
func setProtoAndRealIP(out http.Header, remoteAddr string) {
	out.Set("X-Forwarded-Proto", "https")

	out.Del("X-Real-IP")
	if ip := clientIP(remoteAddr); ip != "" {
		out.Set("X-Real-IP", ip)
	}
}

// backendRewrite states what a backend receives, once, for every path that
// proxies to a Docker backend. serveGRPC calls it and then overrides the one
// thing it needs differently.
func backendRewrite(targetURL *url.URL, route *Route) func(*httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.SetURL(targetURL)
		// SetURL points the outbound Host header at the backend. Keep the
		// hostname the client asked for instead: dev servers routinely key on
		// it for virtual hosts and host allowlists.
		pr.Out.Host = pr.In.Host
		preserveRawQuery(pr)

		stripURLPathPrefix(pr.Out.URL, route.PathPrefix)

		rewriteForwardedHeaders(pr)
	}
}

// preserveRawQuery restores the query string the client sent.
//
// httputil.ReverseProxy drops query parameters it cannot parse before calling
// a Rewrite func, so a search for "50%" ("?q=50%&page=2") would reach the
// backend as "?page=2", and a semicolon-separated query would arrive empty.
// roji forwards to local dev servers and has to be transparent about what the
// browser sent, so the raw value wins.
func preserveRawQuery(pr *httputil.ProxyRequest) {
	pr.Out.URL.RawQuery = pr.In.URL.RawQuery
}

// rewriteForwardedHeaders sets the forwarding headers of a request proxied
// through httputil.ReverseProxy.
//
// ReverseProxy strips the client's Forwarded and X-Forwarded-* headers before
// calling a Rewrite func, and SetXForwarded fills them back in from the
// inbound request — including X-Forwarded-For as a bare client IP, dropped
// entirely when RemoteAddr does not parse.
func rewriteForwardedHeaders(pr *httputil.ProxyRequest) {
	pr.SetXForwarded()
	setProtoAndRealIP(pr.Out.Header, pr.In.RemoteAddr)
}

// checkBasicAuth validates basic authentication credentials.
// Returns true if authentication is successful or not required.
// If authentication fails, it writes the 401 response and returns false.
func checkBasicAuth(w http.ResponseWriter, r *http.Request, auth *config.BasicAuth) bool {
	if auth == nil {
		return true // No authentication required
	}

	// Skip authentication for CORS preflight requests (OPTIONS)
	// Browsers send OPTIONS without credentials before the actual request
	if r.Method == http.MethodOptions {
		slog.Debug("skipping auth for CORS preflight", "path", r.URL.Path)
		return true
	}

	user, pass, ok := r.BasicAuth()
	if !ok || user != auth.User || pass != auth.Pass {
		realm := auth.Realm
		if realm == "" {
			realm = "Restricted"
		}
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		slog.Debug("basic auth failed",
			"path", r.URL.Path,
			"has_credentials", ok,
			"realm", realm)
		return false
	}

	return true
}

//go:embed templates/*.html templates/*.js templates/*.svg templates/*.css
var templateFS embed.FS

var templates = template.Must(
	template.New("").Delims("[[", "]]").ParseFS(templateFS, "templates/*.html"),
)

// StatusConfig contains configuration for the status endpoint
type StatusConfig struct {
	Version       string
	Commit        string
	Date          string
	BuiltBy       string
	StartTime     time.Time
	CertsDir      string
	AutoGenerated bool
	Networks      []string // Docker networks being watched
	BaseDomain    string
	HTTPPort      int
	HTTPSPort     int
}

// ConfigReloadFunc is a callback function to reload configuration
type ConfigReloadFunc func() error

// Handler is the main HTTP handler for the reverse proxy
type Handler struct {
	router        *Router
	dockerClient  *docker.Client
	logBuffer     *LogBuffer
	dashboardHost string // hostname for dashboard (e.g., "roji.dev.localhost")
	baseDomain    string // base domain (e.g., "dev.localhost")
	statusConfig  *StatusConfig
	projectStore  *project.Store
	reloadConfig  ConfigReloadFunc // callback to reload config
}

// NewHandler creates a new proxy handler
func NewHandler(router *Router, dockerClient *docker.Client, dashboardHost string, baseDomain string, statusConfig *StatusConfig, projectStore *project.Store) *Handler {
	return &Handler{
		router:        router,
		dockerClient:  dockerClient,
		logBuffer:     NewLogBuffer(100), // Keep last 100 requests
		dashboardHost: strings.ToLower(dashboardHost),
		baseDomain:    strings.ToLower(baseDomain),
		statusConfig:  statusConfig,
		projectStore:  projectStore,
	}
}

// SetConfigReloader sets the callback function for reloading configuration
func (h *Handler) SetConfigReloader(fn ConfigReloadFunc) {
	h.reloadConfig = fn
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Extract hostname (remove port if present)
	hostname := strings.ToLower(hostWithoutPort(r.Host))

	// Check if this is a dashboard-related host
	isDashboardHost := h.dashboardHost != "" && hostname == h.dashboardHost
	isBaseDomain := h.baseDomain != "" && hostname == h.baseDomain

	if isDashboardHost || isBaseDomain {
		// If accessing via base domain, redirect to dashboard host
		if isBaseDomain && !isDashboardHost {
			// Construct redirect URL
			scheme := "https"
			redirectURL := fmt.Sprintf("%s://%s%s", scheme, h.dashboardHost, r.URL.RequestURI())
			http.Redirect(w, r, redirectURL, http.StatusFound) // 302 Temporary Redirect
			return
		}

		// Health check endpoints
		if r.URL.Path == "/_api/health" || r.URL.Path == "/healthz" {
			h.serveHealth(w, r)
			return
		}
		// Status endpoint
		if r.URL.Path == "/_api/status" {
			h.serveStatus(w, r)
			return
		}
		// API endpoint for route listing
		if r.URL.Path == "/_api/routes" {
			h.serveRoutesAPI(w, r)
			return
		}
		// API endpoint for project listing
		if r.URL.Path == "/_api/projects" {
			h.serveProjectsAPI(w, r)
			return
		}
		// SSE endpoint for real-time route updates
		if r.URL.Path == "/_api/events" {
			h.serveSSE(w, r)
			return
		}
		// Container restart endpoint
		if strings.HasPrefix(r.URL.Path, "/_api/containers/") && strings.HasSuffix(r.URL.Path, "/restart") {
			h.serveContainerRestart(w, r)
			return
		}
		// Project operations endpoint (must come before /_api/logs)
		if strings.HasPrefix(r.URL.Path, "/_api/projects/") {
			h.serveProjectOperation(w, r)
			return
		}
		// Request logs endpoint
		if r.URL.Path == "/_api/logs" {
			h.serveLogsAPI(w, r)
			return
		}
		// SSE endpoint for real-time log updates
		if r.URL.Path == "/_api/logs/events" {
			h.serveLogsSSE(w, r)
			return
		}
		// Log export endpoint
		if r.URL.Path == "/_api/logs/export" {
			h.serveLogsExport(w, r)
			return
		}
		// Config reload endpoint
		if r.URL.Path == "/_api/config/reload" {
			h.serveConfigReload(w, r)
			return
		}
		// Static assets (petite-vue.min.js, etc.)
		if strings.HasPrefix(r.URL.Path, "/_assets/") {
			h.serveAsset(w, r)
			return
		}
		h.serveDashboard(w, r)
		return
	}

	// Check for static site route first
	staticRoute := h.router.LookupStatic(hostname)
	if staticRoute != nil {
		// Check basic auth if configured
		if staticRoute.StaticBackend != nil && !checkBasicAuth(w, r, staticRoute.StaticBackend.BasicAuth) {
			return
		}
		h.serveStaticSite(w, r, staticRoute, hostname, startTime)
		return
	}

	// Look up Docker route
	route := h.router.Lookup(hostname, r.URL.Path)
	if route == nil {
		h.handleNotFound(w, r, hostname)
		return
	}

	// Check basic auth if configured
	if route.Backend != nil && !checkBasicAuth(w, r, route.Backend.BasicAuth) {
		return
	}

	// Check if this request matches a mock route (before warning check,
	// since mocks can work without a real backend)
	if mock := h.findMockRoute(route, r.Method, r.URL.Path); mock != nil {
		h.serveMockResponse(w, r, mock, hostname, route)
		return
	}

	// Check if route has a warning (e.g., no port exposed)
	if route.Backend.Warning != "" {
		h.handleRouteWarning(w, r, hostname, route)
		return
	}

	// Create target URL for backend
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port),
	}

	// Handle gRPC requests (HTTP/2)
	if isGRPCRequest(r) {
		h.serveGRPC(w, r, targetURL, route, hostname, startTime)
		return
	}

	// Create reverse proxy for this request
	proxy := &httputil.ReverseProxy{
		// Use shared transport for connection pooling
		Transport: sharedTransport,
		// SSE support: flush responses immediately (disable buffering)
		FlushInterval: -1,
		Rewrite:       backendRewrite(targetURL, route),
	}

	// upgraded records that a WebSocket was established, which only the two
	// hooks below can tell apart from an ordinary exchange.
	upgraded := false

	// Error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// Also reached after ModifyResponse: ReverseProxy refuses a 101 whose
		// Upgrade token disagrees with the request's, and a ResponseWriter it
		// cannot hijack.
		upgraded = false

		slog.Error("proxy error",
			"hostname", hostname,
			"path", r.URL.Path,
			"target", targetURL.String(),
			"error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Log the request
	proxy.ModifyResponse = func(resp *http.Response) error {
		method := r.Method
		if resp.StatusCode == http.StatusSwitchingProtocols {
			// Labelled from the response rather than from sniffing the
			// request: an upgrade the backend refuses is an ordinary HTTP
			// exchange, and reads better logged as one.
			method = "WS"
			upgraded = true
		}

		duration := time.Since(startTime)
		slog.Info("request",
			"method", method,
			"host", hostname,
			"path", r.URL.Path,
			"status", resp.StatusCode,
			"duration", duration.Round(time.Millisecond),
			"target", route.Backend.ServiceName)

		// Add to log buffer
		h.logBuffer.Add(RequestLog{
			Timestamp: startTime,
			Method:    method,
			Host:      hostname,
			Path:      r.URL.Path,
			Status:    resp.StatusCode,
			Duration:  duration.Milliseconds(),
			Service:   route.Backend.ServiceName,
		})
		return nil
	}

	proxy.ServeHTTP(w, r)

	// ReverseProxy blocks until an upgraded connection closes in both
	// directions, so this is the connection's lifetime rather than the
	// handshake's.
	if upgraded {
		slog.Info("websocket disconnected",
			"host", hostname,
			"path", r.URL.Path,
			"duration", time.Since(startTime).Round(time.Millisecond),
			"target", route.Backend.ServiceName)
	}
}

func (h *Handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	routes := h.router.ListRoutes()
	if routes == nil {
		routes = []RouteInfo{}
	}

	// Marshal routes to JSON for Petite Vue initialization
	routesJSON, err := json.Marshal(routes)
	if err != nil {
		slog.Error("failed to marshal routes for dashboard", "error", err)
		routesJSON = []byte("[]")
	}

	// Detect language from Accept-Language header
	lang := i18n.DetectHTTP(r.Header.Get("Accept-Language"))
	allMsgs := i18n.AllMessages()
	allMessagesJSON, err := json.Marshal(allMsgs)
	if err != nil {
		slog.Error("failed to marshal i18n messages", "error", err)
		allMessagesJSON = []byte("{}")
	}

	data := struct {
		Routes          []RouteInfo
		RoutesJSON      template.JS // Safe for embedding in script
		Version         string
		AllMessagesJSON template.JS
		Lang            string
	}{
		Routes:          routes,
		RoutesJSON:      template.JS(routesJSON),
		Version:         h.statusConfig.Version,
		AllMessagesJSON: template.JS(allMessagesJSON),
		Lang:            lang,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		slog.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) serveSSE(w http.ResponseWriter, r *http.Request) {
	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe to route changes
	eventCh, cleanup := h.router.Subscribe(r.Context())
	defer cleanup()

	// Send initial routes immediately
	routes := h.router.ListRoutes()
	h.sendSSEEvent(w, flusher, "routes", routes)

	slog.Debug("SSE client connected", "remote", r.RemoteAddr)

	// Stream events until client disconnects
	for {
		select {
		case <-r.Context().Done():
			slog.Debug("SSE client disconnected", "remote", r.RemoteAddr)
			return

		case event, ok := <-eventCh:
			if !ok {
				return
			}
			h.sendSSEEvent(w, flusher, event.Type, event.Routes)
		}
	}
}

func (h *Handler) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, routes []RouteInfo) {
	data, err := json.Marshal(routes)
	if err != nil {
		slog.Error("failed to marshal SSE event", "error", err)
		return
	}

	// SSE format: event: <type>\ndata: <json>\n\n
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (h *Handler) serveAsset(w http.ResponseWriter, r *http.Request) {
	// Extract filename from path (e.g., "/_assets/petite-vue.min.js" -> "petite-vue.min.js")
	filename := strings.TrimPrefix(r.URL.Path, "/_assets/")

	// Security: prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.NotFound(w, r)
		return
	}

	// Read from embedded FS
	content, err := templateFS.ReadFile("templates/" + filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on extension
	if strings.HasSuffix(filename, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(filename, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(filename, ".svg") {
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// Cache for 1 hour (static asset)
	w.Header().Set("Cache-Control", "public, max-age=3600")

	w.Write(content)
}

func (h *Handler) serveRoutesAPI(w http.ResponseWriter, r *http.Request) {
	routes := h.router.ListRoutes()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(routes); err != nil {
		slog.Error("failed to encode routes as JSON", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) serveProjectsAPI(w http.ResponseWriter, r *http.Request) {
	if h.projectStore == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"active":[],"inactive":[]}`))
		return
	}

	type ProjectResponse struct {
		Active   []*project.Project `json:"active"`
		Inactive []*project.Project `json:"inactive"`
	}

	resp := ProjectResponse{
		Active:   h.projectStore.ListActive(),
		Inactive: h.projectStore.ListInactive(),
	}

	// Ensure non-nil slices for JSON
	if resp.Active == nil {
		resp.Active = []*project.Project{}
	}
	if resp.Inactive == nil {
		resp.Inactive = []*project.Project{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode projects as JSON", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) serveContainerRestart(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract container ID from path: /_api/containers/{id}/restart
	path := strings.TrimPrefix(r.URL.Path, "/_api/containers/")
	path = strings.TrimSuffix(path, "/restart")
	containerID := path

	if containerID == "" {
		http.Error(w, "Container ID is required", http.StatusBadRequest)
		return
	}

	// Check if docker client is available
	if h.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return
	}

	// Restart the container
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.dockerClient.RestartContainer(ctx, containerID); err != nil {
		slog.Error("failed to restart container",
			"container", containerID,
			"error", err)
		http.Error(w, fmt.Sprintf("Failed to restart container: %v", err), http.StatusInternalServerError)
		return
	}

	slog.Info("container restarted", "container", containerID)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "restarted",
		"container": containerID,
	})
}

func (h *Handler) serveLogsAPI(w http.ResponseWriter, r *http.Request) {
	logs := h.logBuffer.ListRecent(100)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		slog.Error("failed to encode logs as JSON", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) serveLogsSSE(w http.ResponseWriter, r *http.Request) {
	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe to log updates
	logCh, cleanup := h.logBuffer.Subscribe()
	defer cleanup()

	// Send initial logs
	logs := h.logBuffer.ListRecent(50) // Send last 50 on connect
	for i := len(logs) - 1; i >= 0; i-- {
		h.sendLogSSEEvent(w, flusher, logs[i])
	}

	slog.Debug("Logs SSE client connected", "remote", r.RemoteAddr)

	// Stream logs until client disconnects
	for {
		select {
		case <-r.Context().Done():
			slog.Debug("Logs SSE client disconnected", "remote", r.RemoteAddr)
			return

		case log, ok := <-logCh:
			if !ok {
				return
			}
			h.sendLogSSEEvent(w, flusher, log)
		}
	}
}

func (h *Handler) sendLogSSEEvent(w http.ResponseWriter, flusher http.Flusher, log RequestLog) {
	data, err := json.Marshal(log)
	if err != nil {
		slog.Error("failed to marshal log SSE event", "error", err)
		return
	}

	// SSE format: event: log\ndata: <json>\n\n
	fmt.Fprintf(w, "event: log\n")
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (h *Handler) serveLogsExport(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	format := query.Get("format")
	if format == "" {
		format = "json"
	}

	// Build filter from query parameters
	filter := LogFilter{
		Service: query.Get("service"),
		Host:    query.Get("host"),
		Method:  query.Get("method"),
	}

	// Parse time range
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = t
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = t
		}
	}

	// Get filtered logs
	logs := h.logBuffer.ListFiltered(filter)

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("roji-logs-%s.%s", timestamp, format)

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		h.writeLogsCSV(w, logs)
	case "json":
		fallthrough
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		if err := json.NewEncoder(w).Encode(logs); err != nil {
			slog.Error("failed to encode logs as JSON", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func (h *Handler) serveConfigReload(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if reload function is set
	if h.reloadConfig == nil {
		http.Error(w, "Config reload not available", http.StatusServiceUnavailable)
		return
	}

	// Reload configuration
	if err := h.reloadConfig(); err != nil {
		slog.Error("failed to reload config", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	slog.Info("configuration reloaded")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "reloaded",
	})
}

func (h *Handler) writeLogsCSV(w http.ResponseWriter, logs []RequestLog) {
	// Write CSV header
	fmt.Fprintln(w, "id,timestamp,method,host,path,status,duration_ms,service,is_mock")

	// Write data rows
	for _, log := range logs {
		fmt.Fprintf(w, "%d,%s,%s,%s,%s,%d,%d,%s,%t\n",
			log.ID,
			log.Timestamp.Format(time.RFC3339),
			log.Method,
			log.Host,
			csvEscape(log.Path),
			log.Status,
			log.Duration,
			log.Service,
			log.IsMock,
		)
	}
}

// csvEscape escapes a string for CSV (handles commas and quotes)
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func (h *Handler) serveHealth(w http.ResponseWriter, r *http.Request) {
	routes := h.router.ListRoutes()

	health := struct {
		Status string `json:"status"`
		Routes int    `json:"routes"`
	}{
		Status: "healthy",
		Routes: len(routes),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		slog.Error("failed to encode health response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) serveStatus(w http.ResponseWriter, r *http.Request) {
	routes := h.router.ListRoutes()

	// Build status response
	status := &StatusResponse{
		Build: BuildInfo{
			Version: h.statusConfig.Version,
			Commit:  h.statusConfig.Commit,
			Date:    h.statusConfig.Date,
			BuiltBy: h.statusConfig.BuiltBy,
		},
		UptimeSeconds: int64(time.Since(h.statusConfig.StartTime).Seconds()),
		Certificates:  getCertificateStatus(h.statusConfig.CertsDir, h.statusConfig.AutoGenerated),
		Docker: DockerStatus{
			Connected: true, // If we're running, Docker is connected
			Networks:  h.statusConfig.Networks,
		},
		Proxy: ProxyStatus{
			RoutesCount:    len(routes),
			DashboardHost:  h.dashboardHost,
			BaseDomain:     h.statusConfig.BaseDomain,
			HTTPPort:       h.statusConfig.HTTPPort,
			HTTPSPort:      h.statusConfig.HTTPSPort,
			SSESubscribers: h.router.SubscriberCount(),
		},
	}

	// Determine overall health
	status.Health = determineHealth(status)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Error("failed to encode status response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) handleNotFound(w http.ResponseWriter, r *http.Request, hostname string) {
	slog.Warn("no route found",
		"hostname", hostname,
		"path", r.URL.Path)

	routes := h.router.ListRoutes()

	// Try to find a matching inactive project for this hostname
	var matchedProject *project.Project
	if h.projectStore != nil && h.baseDomain != "" {
		matchedProject = h.findProjectForHostname(hostname)
	}

	// Detect language from Accept-Language header
	lang := i18n.DetectHTTP(r.Header.Get("Accept-Language"))
	msgs := i18n.Messages(lang)
	tFunc := func(key string) string {
		if v, ok := msgs[key]; ok {
			return v
		}
		// Fallback to English
		enMsgs := i18n.Messages("en")
		if v, ok := enMsgs[key]; ok {
			return v
		}
		return key
	}

	data := struct {
		Hostname      string
		Routes        []RouteInfo
		DashboardHost string
		Project       *project.Project
		T             func(string) string
	}{
		Hostname:      hostname,
		Routes:        routes,
		DashboardHost: h.dashboardHost,
		Project:       matchedProject,
		T:             tFunc,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := templates.ExecuteTemplate(w, "notfound.html", data); err != nil {
		slog.Error("failed to render notfound template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// findProjectForHostname attempts to match a hostname to an inactive project.
// Hostname patterns:
//   - {project}.{baseDomain}         → project name match
//   - {service}-{project}.{baseDomain} → project name match on suffix
func (h *Handler) findProjectForHostname(hostname string) *project.Project {
	// Strip the base domain suffix to get the subdomain part
	suffix := "." + h.baseDomain
	if !strings.HasSuffix(hostname, suffix) {
		return nil
	}
	subdomain := strings.TrimSuffix(hostname, suffix)
	if subdomain == "" {
		return nil
	}

	inactive := h.projectStore.ListInactive()

	// First: exact match — subdomain == project name
	for _, p := range inactive {
		if p.Name == subdomain {
			return p
		}
	}

	// Second: multi-service pattern — subdomain is "service-project"
	// Try matching the suffix after the first hyphen
	if idx := strings.Index(subdomain, "-"); idx != -1 {
		projectPart := subdomain[idx+1:]
		for _, p := range inactive {
			if p.Name == projectPart {
				return p
			}
		}
	}

	return nil
}

func (h *Handler) handleRouteWarning(w http.ResponseWriter, r *http.Request, hostname string, route *Route) {
	slog.Warn("route has warning",
		"hostname", hostname,
		"path", r.URL.Path,
		"warning", route.Backend.Warning,
		"container", route.Backend.ContainerName)

	// Determine warning type for template
	warningType := "unknown"
	if strings.Contains(route.Backend.Warning, "no port") {
		warningType = "no_port"
	} else if strings.Contains(route.Backend.Warning, "hostname conflict") {
		warningType = "conflict"
	}

	// Detect language from Accept-Language header
	lang := i18n.DetectHTTP(r.Header.Get("Accept-Language"))
	msgs := i18n.Messages(lang)
	tFunc := func(key string) string {
		if v, ok := msgs[key]; ok {
			return v
		}
		enMsgs := i18n.Messages("en")
		if v, ok := enMsgs[key]; ok {
			return v
		}
		return key
	}

	data := struct {
		Hostname      string
		ServiceName   string
		Warning       string
		WarningType   string
		DashboardHost string
		T             func(string) string
	}{
		Hostname:      hostname,
		ServiceName:   route.Backend.ServiceName,
		Warning:       route.Backend.Warning,
		WarningType:   warningType,
		DashboardHost: h.dashboardHost,
		T:             tFunc,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	if err := templates.ExecuteTemplate(w, "warning.html", data); err != nil {
		slog.Error("failed to render warning template", "error", err)
		http.Error(w, fmt.Sprintf("Service unavailable: %s", route.Backend.Warning), http.StatusBadGateway)
	}
}

// findMockRoute looks for a mock route that matches the request method and path
func (h *Handler) findMockRoute(route *Route, method, path string) *config.MockRoute {
	if len(route.Backend.MockRoutes) == 0 {
		return nil
	}

	// Router.Lookup already matched this path against the prefix, so the
	// remainder is what mock paths are written against.
	checkPath, _ := matchPathPrefix(path, route.PathPrefix)

	for _, mock := range route.Backend.MockRoutes {
		if mock.Method == method && mock.Path == checkPath {
			return mock
		}
	}
	return nil
}

// serveMockResponse returns a mock response for the request
func (h *Handler) serveMockResponse(w http.ResponseWriter, r *http.Request, mock *config.MockRoute, hostname string, route *Route) {
	startTime := time.Now()

	// Determine content type based on response body
	contentType := "text/plain"
	body := mock.Body
	if len(body) > 0 {
		// Check if body looks like JSON
		trimmed := strings.TrimSpace(body)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			contentType = "application/json"
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Roji-Mock", "true")
	w.WriteHeader(mock.StatusCode)
	w.Write([]byte(body))

	// Log the mock response
	duration := time.Since(startTime)
	slog.Info("mock response",
		"method", r.Method,
		"host", hostname,
		"path", r.URL.Path,
		"status", mock.StatusCode,
		"duration", duration.Round(time.Millisecond),
		"service", route.Backend.ServiceName)

	// Add to log buffer
	h.logBuffer.Add(RequestLog{
		Timestamp: startTime,
		Method:    r.Method,
		Host:      hostname,
		Path:      r.URL.Path,
		Status:    mock.StatusCode,
		Duration:  duration.Milliseconds(),
		Service:   route.Backend.ServiceName,
		IsMock:    true,
	})
}

// serveStaticSite serves static files from a configured directory
func (h *Handler) serveStaticSite(w http.ResponseWriter, r *http.Request, route *Route, hostname string, startTime time.Time) {
	// Create a response recorder to capture the status code
	rw := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

	// Serve the static file
	ServeStaticFile(rw, r, route.StaticBackend.Root, route.StaticBackend.Index, h.dashboardHost)

	// Log the request
	duration := time.Since(startTime)
	slog.Info("request",
		"method", r.Method,
		"host", hostname,
		"path", r.URL.Path,
		"status", rw.statusCode,
		"duration", duration.Round(time.Millisecond),
		"target", "static")

	// Add to log buffer
	h.logBuffer.Add(RequestLog{
		Timestamp: startTime,
		Method:    r.Method,
		Host:      hostname,
		Path:      r.URL.Path,
		Status:    rw.statusCode,
		Duration:  duration.Milliseconds(),
		Service:   "static",
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// RedirectHandler redirects HTTP to HTTPS
type RedirectHandler struct {
	HTTPSPort int
}

// ServeHTTP implements http.Handler for HTTP->HTTPS redirect
func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := hostForURL(hostWithoutPort(r.Host))

	targetURL := fmt.Sprintf("https://%s", host)
	if h.HTTPSPort != 443 {
		targetURL = fmt.Sprintf("https://%s:%d", host, h.HTTPSPort)
	}
	targetURL += r.URL.RequestURI()

	http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
}

// serveGRPC handles gRPC requests by proxying to backend using HTTP/2
func (h *Handler) serveGRPC(w http.ResponseWriter, r *http.Request, targetURL *url.URL, route *Route, hostname string, startTime time.Time) {
	slog.Debug("grpc request",
		"host", hostname,
		"path", r.URL.Path,
		"content-type", r.Header.Get("Content-Type"))

	rewrite := backendRewrite(targetURL, route)

	// Create reverse proxy with HTTP/2 transport
	proxy := &httputil.ReverseProxy{
		Transport:     http2Transport,
		FlushInterval: -1, // Flush immediately for streaming
		Rewrite: func(pr *httputil.ProxyRequest) {
			rewrite(pr)
			// The one place gRPC differs: the :authority pseudo-header names
			// the server being dialed, not the hostname the client typed.
			pr.Out.Host = targetURL.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("grpc proxy error",
				"hostname", hostname,
				"path", r.URL.Path,
				"target", targetURL.String(),
				"error", err)
			// Return gRPC-compatible error
			w.Header().Set("Content-Type", "application/grpc")
			w.Header().Set("Grpc-Status", "14") // UNAVAILABLE
			w.Header().Set("Grpc-Message", "backend unavailable")
			w.WriteHeader(http.StatusBadGateway)
		},
		ModifyResponse: func(resp *http.Response) error {
			duration := time.Since(startTime)
			grpcStatus := resp.Header.Get("Grpc-Status")
			if grpcStatus == "" {
				grpcStatus = "0" // OK
			}
			slog.Info("grpc request",
				"method", r.Method,
				"host", hostname,
				"path", r.URL.Path,
				"grpc-status", grpcStatus,
				"duration", duration.Round(time.Millisecond),
				"target", route.Backend.ServiceName)

			// Add to log buffer
			h.logBuffer.Add(RequestLog{
				Timestamp: startTime,
				Method:    "gRPC",
				Host:      hostname,
				Path:      r.URL.Path,
				Status:    resp.StatusCode,
				Duration:  duration.Milliseconds(),
				Service:   route.Backend.ServiceName,
			})
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
}

// validProjectName matches project names containing only safe characters
var validProjectName = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

// isValidProjectName checks if a project name contains only safe characters
func isValidProjectName(name string) bool {
	return validProjectName.MatchString(name)
}

// getProjectOrError looks up a project and writes error responses if something is wrong.
// Returns nil if an error response was written.
func (h *Handler) getProjectOrError(w http.ResponseWriter, name string) *project.Project {
	if !isValidProjectName(name) {
		http.Error(w, "Invalid project name", http.StatusBadRequest)
		return nil
	}
	if h.projectStore == nil {
		http.Error(w, "Project store not available", http.StatusServiceUnavailable)
		return nil
	}
	if h.dockerClient == nil {
		http.Error(w, "Docker client not available", http.StatusServiceUnavailable)
		return nil
	}

	p := h.projectStore.Get(name)
	if p == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return nil
	}
	if p.WorkingDir == "" {
		http.Error(w, "Project has no working directory", http.StatusBadRequest)
		return nil
	}

	return p
}

// setProjectCORS sets CORS headers for project operation API responses.
// These endpoints may be called cross-origin from notfound pages served on
// project hostnames (e.g., myapp.dev.localhost → roji.dev.localhost).
func (h *Handler) setProjectCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	// Allow requests from any subdomain of the base domain
	if h.baseDomain != "" && strings.HasSuffix(strings.TrimPrefix(origin, "https://"), "."+h.baseDomain) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
}

// serveProjectOperation dispatches project operations based on the URL path.
// Path format: /_api/projects/{name}/{action}
func (h *Handler) serveProjectOperation(w http.ResponseWriter, r *http.Request) {
	// Handle CORS for cross-origin requests from project hostnames
	h.setProjectCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse /_api/projects/{name}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/_api/projects/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Invalid path: expected /_api/projects/{name}/{action}", http.StatusBadRequest)
		return
	}

	name := parts[0]
	action := parts[1]

	switch action {
	case "up":
		h.serveProjectUp(w, r, name)
	case "down":
		h.serveProjectDown(w, r, name)
	case "restart":
		h.serveProjectRestart(w, r, name)
	case "logs":
		h.serveProjectLogs(w, r, name)
	case "delete":
		h.serveProjectDelete(w, r, name)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
	}
}

func (h *Handler) serveProjectUp(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p := h.getProjectOrError(w, name)
	if p == nil {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	output, err := h.dockerClient.ComposeUp(ctx, p.WorkingDir, p.ConfigFiles)
	if err != nil {
		slog.Error("failed to compose up project",
			"project", name,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
			"output": output,
		})
		return
	}

	slog.Info("project started", "project", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"project": name,
		"output":  output,
	})
}

func (h *Handler) serveProjectDown(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p := h.getProjectOrError(w, name)
	if p == nil {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	output, err := h.dockerClient.ComposeDown(ctx, p.WorkingDir, p.ConfigFiles)
	if err != nil {
		slog.Error("failed to compose down project",
			"project", name,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
			"output": output,
		})
		return
	}

	slog.Info("project stopped", "project", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "stopped",
		"project": name,
		"output":  output,
	})
}

func (h *Handler) serveProjectRestart(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p := h.getProjectOrError(w, name)
	if p == nil {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	output, err := h.dockerClient.ComposeRestart(ctx, p.WorkingDir, p.ConfigFiles)
	if err != nil {
		slog.Error("failed to compose restart project",
			"project", name,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
			"output": output,
		})
		return
	}

	slog.Info("project restarted", "project", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "restarted",
		"project": name,
		"output":  output,
	})
}

func (h *Handler) serveProjectDelete(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isValidProjectName(name) {
		http.Error(w, "Invalid project name", http.StatusBadRequest)
		return
	}
	if h.projectStore == nil {
		http.Error(w, "Project store not available", http.StatusServiceUnavailable)
		return
	}

	p := h.projectStore.Get(name)
	if p == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	h.projectStore.Remove(name)

	slog.Info("project removed from history", "project", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "deleted",
		"project": name,
	})
}

func (h *Handler) serveProjectLogs(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p := h.getProjectOrError(w, name)
	if p == nil {
		return
	}

	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	reader, err := h.dockerClient.ComposeLogs(r.Context(), p.WorkingDir, p.ConfigFiles)
	if err != nil {
		slog.Error("failed to start compose logs",
			"project", name,
			"error", err)
		http.Error(w, fmt.Sprintf("Failed to start logs: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
			line := scanner.Text()
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}
