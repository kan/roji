package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kan/roji/config"
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

// LocalBackend is a server already listening on the host, such as a dev server
// started outside Docker, that the config file gives a hostname to.
type LocalBackend struct {
	Target    *url.URL          // Where the server listens, e.g. http://127.0.0.1:8787
	BasicAuth *config.BasicAuth // Basic authentication (optional)
	Tunnel    bool              // Published through the tunnel

	// Temporary marks a route `roji run` leased rather than one the config
	// file declares: it goes away when the command does.
	Temporary bool
}

// Route kinds, as reported in RouteInfo.Kind.
const (
	KindDocker = "docker" // A container discovered on a watched network
	KindStatic = "static" // A directory declared under static_sites
	KindLocal  = "local"  // A server on the host declared under proxy_sites
)

// Route represents a single route configuration. Exactly one of the three
// backends is set.
type Route struct {
	Hostname      string
	PathPrefix    string
	Backend       *docker.Backend
	StaticBackend *StaticBackend // A directory served from disk
	LocalBackend  *LocalBackend  // A server on the host
}

// Kind names which backend the route has.
func (route *Route) Kind() string {
	switch {
	case route.StaticBackend != nil:
		return KindStatic
	case route.LocalBackend != nil:
		return KindLocal
	default:
		return KindDocker
	}
}

// Published reports whether the route opted in to the tunnel: a Docker route
// with the roji.tunnel label, a declared one with its `tunnel` key. There is no
// other way in, so this is the one question the tunnel guard asks of a route.
func (route *Route) Published() bool {
	switch {
	case route.StaticBackend != nil:
		return route.StaticBackend.Tunnel
	case route.LocalBackend != nil:
		return route.LocalBackend.Tunnel
	default:
		return route.Backend.Tunnel
	}
}

// BasicAuth returns the credentials the route asks for, or nil for none.
func (route *Route) BasicAuth() *config.BasicAuth {
	switch {
	case route.StaticBackend != nil:
		return route.StaticBackend.BasicAuth
	case route.LocalBackend != nil:
		return route.LocalBackend.BasicAuth
	default:
		return route.Backend.BasicAuth
	}
}

// Temporary reports whether `roji run` leased the route, as opposed to the
// config file declaring it or Docker running it.
func (route *Route) Temporary() bool {
	return route.LocalBackend != nil && route.LocalBackend.Temporary
}

// ServiceName is what the route is called in logs and listings. Declared routes
// have no service, so they go by their kind.
func (route *Route) ServiceName() string {
	if route.Backend != nil {
		return route.Backend.ServiceName
	}
	return route.Kind()
}

// Router manages routes and provides thread-safe access
type Router struct {
	mu     sync.RWMutex
	routes map[string]*Route // key: hostname (lowercase)

	// For path-based routing: hostname -> []*Route (sorted by path length desc)
	pathRoutes map[string][]*Route

	// Routes declared in the config file: static sites and servers on the host.
	// They are kept apart from the Docker routes above because they outrank
	// them — see Resolve.
	declaredRoutes map[string]*Route // key: hostname (lowercase)

	// Routes `roji run` registered through the API. Part of the declared tier
	// as far as Resolve is concerned; see lease.
	leases map[string]*lease // key: hostname (lowercase)

	// now is the clock leases expire by, replaced in tests.
	now func() time.Time

	// Domain mapping used to name a published route's public URL. Zero when no
	// tunnel is configured, in which case ListRoutes reports no URLs.
	tunnelDomains TunnelDomains

	// Pub/Sub for SSE subscribers
	subMu       sync.RWMutex
	subscribers map[*Subscriber]struct{}
}

// NewRouter creates a new route manager
func NewRouter() *Router {
	return &Router{
		routes:         make(map[string]*Route),
		pathRoutes:     make(map[string][]*Route),
		declaredRoutes: make(map[string]*Route),
		leases:         make(map[string]*lease),
		now:            time.Now,
		subscribers:    make(map[*Subscriber]struct{}),
	}
}

// SetTunnelDomains records the domain mapping so ListRoutes can name the public
// URL of each published route. Called once at startup, before serving.
func (r *Router) SetTunnelDomains(domains TunnelDomains) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tunnelDomains = domains
}

// tunnelURL returns the address a published route answers to from the internet,
// or an empty string for a route that is not published. The caller holds r.mu.
func (r *Router) tunnelURL(route *Route) string {
	if !route.Published() {
		return ""
	}
	public, ok := r.tunnelDomains.ToPublic(route.Hostname)
	if !ok {
		return ""
	}
	return "https://" + public + route.PathPrefix
}

// claimedRoute returns the route already serving this hostname and prefix, or
// nil when nothing does. The caller holds r.mu.
//
// The pair is the routing key. A prefix-less backend claims the hostname; a
// prefixed one claims the hostname and that prefix together. Two backends
// sharing a key collide — two on one hostname with different prefixes do not.
func (r *Router) claimedRoute(hostname, prefix string) *Route {
	if prefix == "" {
		return r.routes[hostname]
	}
	for _, route := range r.pathRoutes[hostname] {
		if route.PathPrefix == prefix {
			return route
		}
	}
	return nil
}

// dockerRouteOn returns a Docker route serving this hostname, whatever path it
// claims, or nil when none does. The caller holds r.mu.
func (r *Router) dockerRouteOn(hostname string) *Route {
	if route := r.claimedRoute(hostname, ""); route != nil {
		return route
	}
	if routes := r.pathRoutes[hostname]; len(routes) > 0 {
		return routes[0]
	}
	return nil
}

// setPathRoute stores a path-based route, replacing whatever claims the same
// prefix rather than adding a second entry for it. The caller holds r.mu. AddBackend updates existing
// routes as well as adding new ones, and a Docker event can re-add a container
// that is already routed.
func (r *Router) setPathRoute(hostname string, route *Route) {
	routes := r.pathRoutes[hostname]
	idx := slices.IndexFunc(routes, func(e *Route) bool {
		return e.PathPrefix == route.PathPrefix
	})
	if idx >= 0 {
		// Same prefix, so the order Lookup relies on does not change.
		routes[idx] = route
	} else {
		routes = append(routes, route)
		// Longest prefix first, so Lookup answers with the most specific match.
		slices.SortFunc(routes, func(a, b *Route) int {
			return len(b.PathPrefix) - len(a.PathPrefix)
		})
	}
	r.pathRoutes[hostname] = routes
}

// AddBackend adds or updates a route for a backend
func (r *Router) AddBackend(backend *docker.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname := strings.ToLower(backend.Hostname)

	// Replace whatever already holds this routing key.
	if existing := r.claimedRoute(hostname, backend.PathPrefix); existing != nil &&
		existing.Backend.ContainerID != backend.ContainerID {
		// Name the path, since two backends on one hostname with different
		// prefixes are a normal setup and do not collide. The "hostname
		// conflict" opening stays: handleRouteWarning matches on it to pick
		// the warning page.
		what := "hostname conflict"
		if backend.PathPrefix != "" {
			what += " on " + backend.PathPrefix
		}
		conflictMsg := fmt.Sprintf("%s: overwrites %s", what, existing.Backend.ServiceName)
		if backend.Warning != "" {
			backend.Warning += "; " + conflictMsg
		} else {
			backend.Warning = conflictMsg
		}
		slog.Warn("hostname conflict detected",
			"hostname", hostname,
			"path", backend.PathPrefix,
			"new_service", backend.ServiceName,
			"existing_service", existing.Backend.ServiceName)
	}

	// The other order of the collision setDeclared reports: the site was
	// declared first and the container arrived on its hostname afterwards. Said
	// in the log and not in backend.Warning, which would outlive a reload that
	// removes the site and leave a working container behind a warning page.
	if declared := r.declaredOn(hostname); declared != nil {
		slog.Warn("Docker route is hidden by a declared site on the same hostname",
			"hostname", hostname,
			"docker_service", backend.ServiceName,
			"docker_path", backend.PathPrefix,
			"kind", declared.Kind(),
			"target", declaredTarget(declared))
	}

	route := &Route{
		Hostname:   hostname,
		PathPrefix: backend.PathPrefix,
		Backend:    backend,
	}

	if backend.PathPrefix != "" {
		r.setPathRoute(hostname, route)
	} else {
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

// declaredTarget renders where a declared route sends its requests.
func declaredTarget(route *Route) string {
	if route.StaticBackend != nil {
		return route.StaticBackend.Root
	}
	return route.LocalBackend.Target.String()
}

// setDeclared stores a declared route under its hostname. The caller holds r.mu.
func (r *Router) setDeclared(route *Route) {
	route.Hostname = strings.ToLower(route.Hostname)

	// A declared route takes the hostname. Docker routes are discovered, so a
	// container appearing on a hostname the config file already names must not
	// displace it — Resolve asks the declared routes first for the same reason.
	//
	// Reporting rather than refusing: registration used to return here, so a
	// config reload dropped any static site whose hostname a container had
	// since taken, and it stayed gone until roji restarted.
	if hidden := r.dockerRouteOn(route.Hostname); hidden != nil {
		slog.Warn("declared site hides a Docker route on the same hostname",
			"hostname", route.Hostname,
			"docker_service", hidden.Backend.ServiceName,
			"docker_path", hidden.PathPrefix,
			"kind", route.Kind(),
			"target", declaredTarget(route))
	}

	// The config file can name a hostname twice, under one key or across both.
	// The later one wins, as it always has for static sites.
	if existing, ok := r.declaredRoutes[route.Hostname]; ok {
		slog.Warn("hostname is declared more than once, the later declaration wins",
			"hostname", route.Hostname,
			"dropped_kind", existing.Kind(),
			"dropped_target", declaredTarget(existing),
			"kind", route.Kind(),
			"target", declaredTarget(route))
	}

	r.declaredRoutes[route.Hostname] = route

	slog.Info("declared site added",
		"hostname", route.Hostname,
		"kind", route.Kind(),
		"target", declaredTarget(route))
}

// ReplaceDeclared swaps the whole set of declared routes — those with a
// StaticBackend or a LocalBackend — for routes. It is the only way in: startup
// registers what the config file says and a reload registers what it says now.
// A declared route is keyed by hostname alone, so a PathPrefix on one is
// ignored.
//
// One step rather than a clear followed by adds: in the gap between the two, a
// request for a declared hostname falls through to whatever Docker route the
// declaration was hiding.
func (r *Router) ReplaceDeclared(routes []*Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.declaredRoutes = make(map[string]*Route, len(routes))
	for _, route := range routes {
		r.setDeclared(route)

		// The config file now names a hostname `roji run` was holding. The
		// file wins, and the lease goes rather than lingering unseen behind
		// the site: its owner is refused at its next renewal and says so.
		if _, leased := r.leases[route.Hostname]; leased {
			delete(r.leases, route.Hostname)
			slog.Warn("leased route dropped, the config file now declares its hostname",
				"hostname", route.Hostname)
		}
	}

	// Notify SSE subscribers
	go r.notifySubscribers()
}

// LookupDeclared finds the route the config file declares for a hostname
func (r *Router) LookupDeclared(hostname string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.declaredOn(strings.ToLower(hostname))
}

// Resolve finds the route that serves a hostname and path, which is the order
// of precedence in one place: a declared route owns its hostname outright, and
// only a hostname the config file does not name goes to the Docker routes.
func (r *Router) Resolve(hostname, path string) *Route {
	if route := r.LookupDeclared(hostname); route != nil {
		return route
	}
	return r.Lookup(hostname, path)
}

// matchPathPrefix reports whether path falls under prefix, and returns what
// remains of the path once the prefix is removed.
//
// A prefix only matches on a segment boundary: "/api" covers "/api" and
// "/api/users" but not "/apifoo", which belongs to whatever route would serve
// it without the prefix. The remainder is always rooted, so the prefix on its
// own yields "/" rather than an empty path.
//
// An empty prefix means no prefix: it matches every path and leaves it
// untouched. There is no other exception — "/" is a prefix like any other and
// covers "/" alone, which is why config.RouteConfig.PathPrefix spells "no
// prefix" as the empty string.
func matchPathPrefix(path, prefix string) (rest string, ok bool) {
	if prefix == "" {
		return path, true
	}

	rest, ok = strings.CutPrefix(path, prefix)
	if !ok {
		return "", false
	}
	if rest == "" {
		return "/", true
	}
	if !strings.HasPrefix(rest, "/") {
		return "", false
	}
	return rest, true
}

// Lookup finds a Docker route for a given hostname and path. Use Resolve to
// find what actually serves a request.
func (r *Router) Lookup(hostname, path string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostname = strings.ToLower(hostname)

	// First check path-based routes
	if routes, ok := r.pathRoutes[hostname]; ok {
		for _, route := range routes {
			if _, ok := matchPathPrefix(path, route.PathPrefix); ok {
				return route
			}
		}
	}

	// Fall back to simple hostname route
	return r.routes[hostname]
}

// backendRouteInfo renders a Docker route for display. The caller holds r.mu.
func (r *Router) backendRouteInfo(route *Route) RouteInfo {
	target := ""
	if route.Backend.Port > 0 {
		target = fmt.Sprintf("%s:%d", route.Backend.Host, route.Backend.Port)
	}
	return RouteInfo{
		Kind:          KindDocker,
		Hostname:      route.Hostname,
		PathPrefix:    route.PathPrefix,
		Target:        target,
		ContainerID:   route.Backend.ContainerID,
		ContainerName: route.Backend.ContainerName,
		ServiceName:   route.Backend.ServiceName,
		ProjectName:   route.Backend.ProjectName,
		Warning:       route.Backend.Warning,
		Network:       route.Backend.Network,
		HasBasicAuth:  route.Backend.BasicAuth != nil,
		TunnelURL:     r.tunnelURL(route),
	}
}

// ListRoutes returns all current routes for display
func (r *Router) ListRoutes() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := []RouteInfo{}

	for _, route := range r.routes {
		infos = append(infos, r.backendRouteInfo(route))
	}

	for _, routes := range r.pathRoutes {
		for _, route := range routes {
			infos = append(infos, r.backendRouteInfo(route))
		}
	}

	declaredInfo := func(route *Route) RouteInfo {
		return RouteInfo{
			Kind:         route.Kind(),
			Hostname:     route.Hostname,
			Target:       declaredTarget(route),
			ServiceName:  route.ServiceName(),
			IndexEnabled: route.StaticBackend != nil && route.StaticBackend.Index,
			HasBasicAuth: route.BasicAuth() != nil,
			Temporary:    route.Temporary(),
			TunnelURL:    r.tunnelURL(route),
		}
	}

	for _, route := range r.declaredRoutes {
		infos = append(infos, declaredInfo(route))
	}

	for hostname := range r.leases {
		if held := r.liveLease(hostname); held != nil {
			infos = append(infos, declaredInfo(held.route))
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
	// Kind is one of KindDocker, KindStatic and KindLocal. Which of the fields
	// below mean anything follows from it.
	Kind          string `json:"kind"`
	Hostname      string `json:"hostname"`
	PathPrefix    string `json:"pathPrefix,omitempty"`
	Target        string `json:"target"`
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	ServiceName   string `json:"serviceName"`
	ProjectName   string `json:"projectName,omitempty"`
	Warning       string `json:"warning,omitempty"`
	Network       string `json:"network,omitempty"`
	IndexEnabled  bool   `json:"indexEnabled,omitempty"` // For static sites: directory listing enabled
	Temporary     bool   `json:"temporary,omitempty"`    // For local routes: registered by `roji run`, gone when it exits
	HasBasicAuth  bool   `json:"hasBasicAuth,omitempty"` // Whether basic auth is enabled
	// TunnelURL is the address a route answers to from the internet, empty for
	// one that does not. It is the only spelling of "published": a route that
	// opted in while no tunnel is configured reaches nobody, and saying so twice
	// invites the two answers to disagree.
	TunnelURL string `json:"tunnelUrl,omitempty"`
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
