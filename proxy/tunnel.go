package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

// tunneledKey marks a request as having arrived through the tunnel.
type tunneledKey struct{}

// isTunneled reports whether r reached roji from the internet.
//
// The guard resolves a route and Handler resolves it again, and a container
// that goes away in between drops the request onto pages meant for a developer
// looking at their own machine. This is how those pages know not to answer.
func isTunneled(r *http.Request) bool {
	tunneled, _ := r.Context().Value(tunneledKey{}).(bool)
	return tunneled
}

// TunnelDomains translates between the hostname a route answers to locally and
// the one it answers to from the internet.
//
// The two differ only in their suffix: "web.dev.localhost" is published as
// "web.example.com". Keeping the local name as the routing key means the tunnel
// changes nothing about how routes are stored, matched, or shown.
type TunnelDomains struct {
	Base   string // Local base domain, e.g. "dev.localhost"
	Public string // Cloudflare zone, e.g. "example.com"
}

// Configured reports whether both halves of the mapping are known.
func (d TunnelDomains) Configured() bool {
	return d.Base != "" && d.Public != ""
}

// swapSuffix replaces the trailing ".from" of hostname with ".to".
//
// The label before the suffix must be non-empty, so the bare domain does not
// map to anything: an apex request is not a route, and neither is a name whose
// suffix does not match.
func swapSuffix(hostname, from, to string) (string, bool) {
	label, ok := strings.CutSuffix(strings.ToLower(hostname), "."+strings.ToLower(from))
	if !ok || label == "" {
		return "", false
	}
	return label + "." + strings.ToLower(to), true
}

// ToLocal returns the local hostname a public one stands for.
func (d TunnelDomains) ToLocal(public string) (string, bool) {
	if !d.Configured() {
		return "", false
	}
	return swapSuffix(public, d.Public, d.Base)
}

// ToPublic returns the hostname a local route answers to from the internet.
func (d TunnelDomains) ToPublic(local string) (string, bool) {
	if !d.Configured() {
		return "", false
	}
	return swapSuffix(local, d.Base, d.Public)
}

// TunnelHandler guards the listener cloudflared connects to, and hands what
// survives to the ordinary proxy handler.
//
// Everything roji serves locally is reachable without credentials, which is
// safe only because v1.1.0 made it listen on loopback alone. A tunnel undoes
// that for whatever it publishes, so this decides what "whatever" means:
//
//   - the dashboard and its API are never published. `/_api/projects/{name}/up`
//     starts containers and `/_api/logs` reads traffic, both unauthenticated
//   - a route is published only if it says so, with `roji.tunnel=true`
//
// cloudflared connects from 127.0.0.1 like any local browser, so no property of
// the request tells the two apart. The separate listener does, which is why the
// tunnel gets a port of its own rather than sharing the main one.
type TunnelHandler struct {
	inner         http.Handler
	router        *Router
	domains       TunnelDomains
	dashboardHost string
}

// NewTunnelHandler wraps inner with the tunnel guard.
func NewTunnelHandler(inner http.Handler, router *Router, domains TunnelDomains, dashboardHost string) *TunnelHandler {
	return &TunnelHandler{
		inner:         inner,
		router:        router,
		domains:       domains,
		dashboardHost: strings.ToLower(dashboardHost),
	}
}

// ServeHTTP implements http.Handler.
func (h *TunnelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := strings.ToLower(hostWithoutPort(r.Host))

	// Refused by path before anything else, so no hostname or route
	// configuration can reach these. The dashboard is served from its own
	// hostname, but its API answers on whatever host carries the path.
	if strings.HasPrefix(r.URL.Path, "/_api/") || strings.HasPrefix(r.URL.Path, "/_assets/") {
		slog.Warn("tunnel request for the dashboard API refused",
			"host", hostname,
			"path", r.URL.Path,
			"remote", r.RemoteAddr)
		bareNotFound(w)
		return
	}

	local, ok := h.domains.ToLocal(hostname)
	if !ok {
		slog.Debug("tunnel request for an unknown domain", "host", hostname, "path", r.URL.Path)
		bareNotFound(w)
		return
	}

	if local == h.dashboardHost || local == strings.ToLower(h.domains.Base) {
		slog.Warn("tunnel request for the dashboard refused", "host", hostname, "path", r.URL.Path)
		bareNotFound(w)
		return
	}

	// A static site declared on this hostname wins downstream — Handler checks
	// LookupStatic before Lookup — so approving the Docker route here would
	// publish the static site instead of the route that opted in. Static sites
	// have no opt-in of their own, so the hostname is refused outright.
	if h.router.LookupStatic(local) != nil {
		slog.Warn("tunnel request for a hostname a static site claims refused",
			"host", hostname,
			"local_host", local,
			"path", r.URL.Path)
		bareNotFound(w)
		return
	}

	// Static sites carry no opt-in, so Lookup — Docker routes only — is the
	// whole published set.
	route := h.router.Lookup(local, r.URL.Path)
	if route == nil || route.Backend == nil || !route.Backend.Tunnel {
		slog.Debug("tunnel request for a route that is not published",
			"host", hostname,
			"local_host", local,
			"path", r.URL.Path)
		bareNotFound(w)
		return
	}

	// Hand on the local hostname. Everything downstream — the second Lookup,
	// the request log, the Host the backend receives — is written against it,
	// and a dev server checking its allowed hosts expects it too.
	out := r.Clone(context.WithValue(r.Context(), tunneledKey{}, true))
	out.Host = local
	h.inner.ServeHTTP(w, out)
}

// bareNotFound answers everything the tunnel refuses, here and in
// Handler.handleNotFound once a request has been let through.
//
// Always the same bare 404, whatever the reason: anything else tells whoever is
// probing which hostnames exist and which paths roji treats specially. That
// holds only as long as there is one of these, so there is one.
func bareNotFound(w http.ResponseWriter) {
	http.Error(w, "Not Found", http.StatusNotFound)
}
