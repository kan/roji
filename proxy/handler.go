package proxy

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Handler is the main HTTP handler for the reverse proxy
type Handler struct {
	router        *Router
	dashboardHost string // hostname for dashboard (e.g., "roji.localhost")
}

// NewHandler creates a new proxy handler
func NewHandler(router *Router, dashboardHost string) *Handler {
	return &Handler{
		router:        router,
		dashboardHost: strings.ToLower(dashboardHost),
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Extract hostname (remove port if present)
	hostname := r.Host
	if idx := strings.LastIndex(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}
	hostname = strings.ToLower(hostname)

	// Check if this is the dashboard
	if h.dashboardHost != "" && hostname == h.dashboardHost {
		h.serveDashboard(w, r)
		return
	}

	// Look up route
	route := h.router.Lookup(hostname, r.URL.Path)
	if route == nil {
		h.handleNotFound(w, r, hostname)
		return
	}

	// Create reverse proxy for this request
	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the director to handle path prefixes
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip path prefix if configured
		if route.PathPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, route.PathPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		// Set X-Forwarded-* headers
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", "https")
		if r.RemoteAddr != "" {
			if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
				req.Header.Set("X-Forwarded-For", r.RemoteAddr[:idx])
			}
		}
		req.Header.Set("X-Real-IP", req.Header.Get("X-Forwarded-For"))
	}

	// Error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("proxy error",
			"hostname", hostname,
			"path", r.URL.Path,
			"target", targetURL.String(),
			"error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Log the request
	proxy.ModifyResponse = func(resp *http.Response) error {
		duration := time.Since(startTime)
		slog.Info("request",
			"method", r.Method,
			"host", hostname,
			"path", r.URL.Path,
			"status", resp.StatusCode,
			"duration", duration.Round(time.Millisecond),
			"target", route.Backend.ServiceName)
		return nil
	}

	proxy.ServeHTTP(w, r)
}

func (h *Handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	routes := h.router.ListRoutes()

	data := struct {
		Routes []RouteInfo
	}{
		Routes: routes,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		slog.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleNotFound(w http.ResponseWriter, r *http.Request, hostname string) {
	slog.Warn("no route found",
		"hostname", hostname,
		"path", r.URL.Path)

	routes := h.router.ListRoutes()

	data := struct {
		Hostname      string
		Routes        []RouteInfo
		DashboardHost string
	}{
		Hostname:      hostname,
		Routes:        routes,
		DashboardHost: h.dashboardHost,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := templates.ExecuteTemplate(w, "notfound.html", data); err != nil {
		slog.Error("failed to render notfound template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RedirectHandler redirects HTTP to HTTPS
type RedirectHandler struct {
	HTTPSPort int
}

// ServeHTTP implements http.Handler for HTTP->HTTPS redirect
func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	targetURL := fmt.Sprintf("https://%s", host)
	if h.HTTPSPort != 443 {
		targetURL = fmt.Sprintf("https://%s:%d", host, h.HTTPSPort)
	}
	targetURL += r.URL.RequestURI()

	http.Redirect(w, r, targetURL, http.StatusMovedPermanently)
}
