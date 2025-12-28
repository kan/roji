package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>roji - Dashboard</title>
    <style>
        * { box-sizing: border-box; }
        body { 
            font-family: system-ui, -apple-system, sans-serif; 
            max-width: 800px; 
            margin: 0 auto; 
            padding: 40px 20px;
            background: #f5f5f5;
        }
        h1 { 
            color: #333;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .subtitle {
            color: #666;
            font-weight: normal;
            font-size: 0.9rem;
            margin-left: 8px;
        }
        .routes {
            background: white;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .route {
            padding: 16px 20px;
            border-bottom: 1px solid #eee;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .route:last-child { border-bottom: none; }
        .route:hover { background: #fafafa; }
        .route-url {
            font-family: monospace;
            font-size: 0.95rem;
        }
        .route-url a {
            color: #0066cc;
            text-decoration: none;
        }
        .route-url a:hover { text-decoration: underline; }
        .route-target {
            color: #666;
            font-size: 0.85rem;
        }
        .service-name {
            background: #e8f4e8;
            color: #2d5a2d;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.8rem;
        }
        .empty {
            padding: 40px;
            text-align: center;
            color: #666;
        }
        .count {
            background: #333;
            color: white;
            padding: 2px 10px;
            border-radius: 12px;
            font-size: 0.85rem;
        }
    </style>
</head>
<body>
    <h1>
        🛤️ roji
        <span class="subtitle">reverse proxy for local development</span>
    </h1>
`)

	if len(routes) > 0 {
		fmt.Fprintf(w, `    <p><span class="count">%d</span> routes registered</p>
    <div class="routes">
`, len(routes))
		for _, route := range routes {
			path := route.PathPrefix
			if path == "" {
				path = ""
			}
			fullURL := fmt.Sprintf("https://%s%s", route.Hostname, path)
			fmt.Fprintf(w, `        <div class="route">
            <div>
                <div class="route-url"><a href="%s" target="_blank">%s%s</a></div>
                <div class="route-target">→ %s</div>
            </div>
            <span class="service-name">%s</span>
        </div>
`, fullURL, route.Hostname, path, route.Target, route.ServiceName)
		}
		fmt.Fprintf(w, `    </div>
`)
	} else {
		fmt.Fprintf(w, `    <div class="routes">
        <div class="empty">
            <p>🔍 No routes registered yet</p>
            <p>Start some containers on the roji network!</p>
        </div>
    </div>
`)
	}

	fmt.Fprintf(w, `</body>
</html>`)
}

func (h *Handler) handleNotFound(w http.ResponseWriter, r *http.Request, hostname string) {
	slog.Warn("no route found",
		"hostname", hostname,
		"path", r.URL.Path)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	routes := h.router.ListRoutes()

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>No Route Found - roji</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        h1 { color: #e74c3c; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        .routes { background: #f9f9f9; padding: 15px; border-radius: 5px; margin-top: 20px; }
        .route { margin: 5px 0; font-family: monospace; }
        .route a { color: #0066cc; }
    </style>
</head>
<body>
    <h1>🚫 No Route Found</h1>
    <p>No backend is configured for <code>%s</code></p>
`, hostname)

	if len(routes) > 0 {
		fmt.Fprintf(w, `    <div class="routes">
        <h3>Available Routes:</h3>
`)
		for _, route := range routes {
			path := route.PathPrefix
			if path == "" {
				path = "/"
			}
			fmt.Fprintf(w, `        <div class="route">• <a href="https://%s%s">%s%s</a> → %s</div>
`, route.Hostname, path, route.Hostname, path, route.ServiceName)
		}
		fmt.Fprintf(w, `    </div>
`)
	} else {
		fmt.Fprintf(w, `    <p>No routes are currently registered. Start some containers on the roji network!</p>
`)
	}

	if h.dashboardHost != "" {
		fmt.Fprintf(w, `    <p><a href="https://%s">View Dashboard</a></p>
`, h.dashboardHost)
	}

	fmt.Fprintf(w, `</body>
</html>`)
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
