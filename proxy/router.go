package proxy

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/kan/roji/docker"
)

// Route represents a single route configuration
type Route struct {
	Hostname   string
	PathPrefix string
	Backend    *docker.Backend
}

// Router manages routes and provides thread-safe access
type Router struct {
	mu     sync.RWMutex
	routes map[string]*Route // key: hostname (lowercase)

	// For path-based routing: hostname -> []*Route (sorted by path length desc)
	pathRoutes map[string][]*Route
}

// NewRouter creates a new route manager
func NewRouter() *Router {
	return &Router{
		routes:     make(map[string]*Route),
		pathRoutes: make(map[string][]*Route),
	}
}

// AddBackend adds or updates a route for a backend
func (r *Router) AddBackend(backend *docker.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname := strings.ToLower(backend.Hostname)
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

	var infos []RouteInfo

	for _, route := range r.routes {
		infos = append(infos, RouteInfo{
			Hostname:      route.Hostname,
			PathPrefix:    route.PathPrefix,
			Target:        fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port),
			ContainerName: route.Backend.ContainerName,
			ServiceName:   route.Backend.ServiceName,
		})
	}

	for _, routes := range r.pathRoutes {
		for _, route := range routes {
			infos = append(infos, RouteInfo{
				Hostname:      route.Hostname,
				PathPrefix:    route.PathPrefix,
				Target:        fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port),
				ContainerName: route.Backend.ContainerName,
				ServiceName:   route.Backend.ServiceName,
			})
		}
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
	Hostname      string
	PathPrefix    string
	Target        string
	ContainerName string
	ServiceName   string
}

func (ri RouteInfo) String() string {
	path := ri.PathPrefix
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("https://%s%s -> %s (%s)",
		ri.Hostname, path, ri.Target, ri.ServiceName)
}
