package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/kan/roji/docker"
)

// RouteEvent represents a route change event for SSE subscribers
type RouteEvent struct {
	Type   string      `json:"type"`   // "routes"
	Routes []RouteInfo `json:"routes"` // Current route list
}

// Subscriber represents an SSE subscriber
type Subscriber struct {
	ch     chan RouteEvent
	ctx    context.Context
	cancel context.CancelFunc
}

// Route represents a single route configuration
type Route struct {
	Hostname      string
	PathPrefix    string
	Backend       *docker.Backend
	StaticBackend *StaticBackend // For static file hosting (mutually exclusive with Backend)
}

// Router manages routes and provides thread-safe access
type Router struct {
	mu     sync.RWMutex
	routes map[string]*Route // key: hostname (lowercase)

	// For path-based routing: hostname -> []*Route (sorted by path length desc)
	pathRoutes map[string][]*Route

	// Static file hosting routes
	staticRoutes map[string]*Route // key: hostname (lowercase)

	// Pub/Sub for SSE subscribers
	subMu       sync.RWMutex
	subscribers map[*Subscriber]struct{}
}

// NewRouter creates a new route manager
func NewRouter() *Router {
	return &Router{
		routes:       make(map[string]*Route),
		pathRoutes:   make(map[string][]*Route),
		staticRoutes: make(map[string]*Route),
		subscribers:  make(map[*Subscriber]struct{}),
	}
}

// AddBackend adds or updates a route for a backend
func (r *Router) AddBackend(backend *docker.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname := strings.ToLower(backend.Hostname)

	// Check for hostname conflict with different container
	if backend.PathPrefix == "" {
		if existing, ok := r.routes[hostname]; ok {
			if existing.Backend.ContainerID != backend.ContainerID {
				// Conflict detected - different container is using the same hostname
				conflictMsg := fmt.Sprintf("hostname conflict: overwrites %s", existing.Backend.ServiceName)
				if backend.Warning != "" {
					backend.Warning += "; " + conflictMsg
				} else {
					backend.Warning = conflictMsg
				}
				slog.Warn("hostname conflict detected",
					"hostname", hostname,
					"new_service", backend.ServiceName,
					"existing_service", existing.Backend.ServiceName)
			}
		}
	}

	route := &Route{
		Hostname:   hostname,
		PathPrefix: backend.PathPrefix,
		Backend:    backend,
	}

	if backend.PathPrefix != "" {
		// Path-based routing
		r.pathRoutes[hostname] = append(r.pathRoutes[hostname], route)
		// Sort by path length descending (longest match first)
		sort.Slice(r.pathRoutes[hostname], func(i, j int) bool {
			return len(r.pathRoutes[hostname][i].PathPrefix) > len(r.pathRoutes[hostname][j].PathPrefix)
		})
	} else {
		// Simple hostname routing
		r.routes[hostname] = route
	}

	slog.Info("route added",
		"hostname", backend.Hostname,
		"path", backend.PathPrefix,
		"target", fmt.Sprintf("%s:%d", backend.Host, backend.Port),
		"container", backend.ContainerName)

	// Notify SSE subscribers (must be called after unlocking mu)
	go r.notifySubscribers()
}

// RemoveBackend removes routes for a container
func (r *Router) RemoveBackend(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from simple routes
	for hostname, route := range r.routes {
		if route.Backend.ContainerID == containerID {
			delete(r.routes, hostname)
			slog.Info("route removed",
				"hostname", route.Hostname,
				"container", route.Backend.ContainerName)
		}
	}

	// Remove from path routes
	for hostname, routes := range r.pathRoutes {
		filtered := routes[:0]
		for _, route := range routes {
			if route.Backend.ContainerID != containerID {
				filtered = append(filtered, route)
			} else {
				slog.Info("route removed",
					"hostname", route.Hostname,
					"path", route.PathPrefix,
					"container", route.Backend.ContainerName)
			}
		}
		if len(filtered) == 0 {
			delete(r.pathRoutes, hostname)
		} else {
			r.pathRoutes[hostname] = filtered
		}
	}

	// Notify SSE subscribers
	go r.notifySubscribers()
}

// RemoveProject removes all routes for a given project
func (r *Router) RemoveProject(projectName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from simple routes
	for hostname, route := range r.routes {
		if route.Backend.ProjectName == projectName {
			delete(r.routes, hostname)
			slog.Debug("route removed for project update",
				"hostname", route.Hostname,
				"project", projectName)
		}
	}

	// Remove from path routes
	for hostname, routes := range r.pathRoutes {
		filtered := routes[:0]
		for _, route := range routes {
			if route.Backend.ProjectName != projectName {
				filtered = append(filtered, route)
			}
		}
		if len(filtered) == 0 {
			delete(r.pathRoutes, hostname)
		} else {
			r.pathRoutes[hostname] = filtered
		}
	}

	// Notify SSE subscribers
	go r.notifySubscribers()
}

// AddStaticSite adds a static file hosting route
func (r *Router) AddStaticSite(site *StaticBackend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname := strings.ToLower(site.Hostname)

	// Check for conflict with Docker routes
	if existing, ok := r.routes[hostname]; ok {
		slog.Warn("static site conflicts with Docker route, Docker route takes priority",
			"hostname", hostname,
			"docker_service", existing.Backend.ServiceName,
			"static_root", site.Root)
		return
	}

	route := &Route{
		Hostname:      hostname,
		StaticBackend: site,
	}

	r.staticRoutes[hostname] = route

	slog.Info("static site added",
		"hostname", site.Hostname,
		"root", site.Root)

	// Notify SSE subscribers
	go r.notifySubscribers()
}

// RemoveStaticSite removes a static file hosting route
func (r *Router) RemoveStaticSite(hostname string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname = strings.ToLower(hostname)
	if route, ok := r.staticRoutes[hostname]; ok {
		delete(r.staticRoutes, hostname)
		slog.Info("static site removed",
			"hostname", route.Hostname)

		// Notify SSE subscribers
		go r.notifySubscribers()
	}
}

// LookupStatic finds a static route for a given hostname
func (r *Router) LookupStatic(hostname string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostname = strings.ToLower(hostname)
	return r.staticRoutes[hostname]
}

// ClearStaticSites removes all static site routes
func (r *Router) ClearStaticSites() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.staticRoutes = make(map[string]*Route)
	slog.Info("all static sites cleared")

	// Notify SSE subscribers
	go r.notifySubscribers()
}

// Lookup finds a route for a given hostname and path
func (r *Router) Lookup(hostname, path string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostname = strings.ToLower(hostname)

	// First check path-based routes
	if routes, ok := r.pathRoutes[hostname]; ok {
		for _, route := range routes {
			if strings.HasPrefix(path, route.PathPrefix) {
				return route
			}
		}
	}

	// Fall back to simple hostname route
	return r.routes[hostname]
}

// ListRoutes returns all current routes for display
func (r *Router) ListRoutes() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := []RouteInfo{}

	for _, route := range r.routes {
		target := ""
		if route.Backend.Port > 0 {
			target = fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port)
		}
		infos = append(infos, RouteInfo{
			Hostname:      route.Hostname,
			PathPrefix:    route.PathPrefix,
			Target:        target,
			ContainerID:   route.Backend.ContainerID,
			ContainerName: route.Backend.ContainerName,
			ServiceName:   route.Backend.ServiceName,
			Warning:       route.Backend.Warning,
			Network:       route.Backend.Network,
			HasBasicAuth:  route.Backend.BasicAuth != nil,
		})
	}

	for _, routes := range r.pathRoutes {
		for _, route := range routes {
			target := ""
			if route.Backend.Port > 0 {
				target = fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port)
			}
			infos = append(infos, RouteInfo{
				Hostname:      route.Hostname,
				PathPrefix:    route.PathPrefix,
				Target:        target,
				ContainerID:   route.Backend.ContainerID,
				ContainerName: route.Backend.ContainerName,
				ServiceName:   route.Backend.ServiceName,
				Warning:       route.Backend.Warning,
				Network:       route.Backend.Network,
				HasBasicAuth:  route.Backend.BasicAuth != nil,
			})
		}
	}

	// Add static routes
	for _, route := range r.staticRoutes {
		infos = append(infos, RouteInfo{
			Hostname:     route.Hostname,
			Target:       route.StaticBackend.Root,
			ServiceName:  "static",
			IsStatic:     true,
			IndexEnabled: route.StaticBackend.Index,
			HasBasicAuth: route.StaticBackend.BasicAuth != nil,
		})
	}

	// Sort by hostname for consistent output
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Hostname != infos[j].Hostname {
			return infos[i].Hostname < infos[j].Hostname
		}
		return infos[i].PathPrefix < infos[j].PathPrefix
	})

	return infos
}

// RouteInfo is a display-friendly route representation
type RouteInfo struct {
	Hostname      string `json:"hostname"`
	PathPrefix    string `json:"pathPrefix,omitempty"`
	Target        string `json:"target"`
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	ServiceName   string `json:"serviceName"`
	Warning       string `json:"warning,omitempty"`
	Network       string `json:"network,omitempty"`
	IsStatic      bool   `json:"isStatic,omitempty"`
	IndexEnabled  bool   `json:"indexEnabled,omitempty"` // For static sites: directory listing enabled
	HasBasicAuth  bool   `json:"hasBasicAuth,omitempty"` // Whether basic auth is enabled
}

func (ri RouteInfo) String() string {
	path := ri.PathPrefix
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("https://%s%s -> %s (%s)",
		ri.Hostname, path, ri.Target, ri.ServiceName)
}

// Subscribe creates a new SSE subscriber and returns the event channel and cleanup function.
// The cleanup function must be called when the subscriber disconnects.
func (r *Router) Subscribe(ctx context.Context) (<-chan RouteEvent, func()) {
	subCtx, cancel := context.WithCancel(ctx)
	sub := &Subscriber{
		ch:     make(chan RouteEvent, 10), // Buffered to prevent blocking
		ctx:    subCtx,
		cancel: cancel,
	}

	r.subMu.Lock()
	r.subscribers[sub] = struct{}{}
	r.subMu.Unlock()

	slog.Debug("SSE subscriber added", "total", r.SubscriberCount())

	// Cleanup function
	cleanup := func() {
		r.subMu.Lock()
		delete(r.subscribers, sub)
		r.subMu.Unlock()
		cancel()
		close(sub.ch)
		slog.Debug("SSE subscriber removed", "total", r.SubscriberCount())
	}

	return sub.ch, cleanup
}

// SubscriberCount returns the number of active SSE subscribers
func (r *Router) SubscriberCount() int {
	r.subMu.RLock()
	defer r.subMu.RUnlock()
	return len(r.subscribers)
}

// notifySubscribers broadcasts the current routes to all SSE subscribers
func (r *Router) notifySubscribers() {
	r.subMu.RLock()
	defer r.subMu.RUnlock()

	if len(r.subscribers) == 0 {
		return
	}

	// Get current routes
	routes := r.ListRoutes()

	event := RouteEvent{
		Type:   "routes",
		Routes: routes,
	}

	for sub := range r.subscribers {
		select {
		case sub.ch <- event:
			// Sent successfully
		case <-sub.ctx.Done():
			// Subscriber disconnected, will be cleaned up
		default:
			// Channel full, skip this update (subscriber too slow)
			slog.Warn("SSE subscriber channel full, dropping update")
		}
	}
}
