package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kan/roji/docker"
)

func TestTunnelDomains_ToLocal(t *testing.T) {
	domains := TunnelDomains{Base: "dev.localhost", Public: "example.com"}

	tests := []struct {
		name   string
		public string
		want   string
		ok     bool
	}{
		{"published route", "web.example.com", "web.dev.localhost", true},
		{"uppercase", "WEB.Example.COM", "web.dev.localhost", true},
		{"multi-label subdomain", "api-web.example.com", "api-web.dev.localhost", true},
		// The apex is not a route, and neither is a name from another domain.
		{"apex", "example.com", "", false},
		{"empty label", ".example.com", "", false},
		{"other domain", "web.evil.com", "", false},
		// A suffix that only looks right: "notexample.com" ends with the same
		// letters but is a different domain.
		{"suffix lookalike", "web.notexample.com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := domains.ToLocal(tt.public)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ToLocal(%q) = (%q, %v), want (%q, %v)", tt.public, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTunnelDomains_ToPublic(t *testing.T) {
	domains := TunnelDomains{Base: "dev.localhost", Public: "example.com"}

	got, ok := domains.ToPublic("web.dev.localhost")
	if !ok || got != "web.example.com" {
		t.Errorf("ToPublic() = (%q, %v), want (web.example.com, true)", got, ok)
	}

	if _, ok := domains.ToPublic("dev.localhost"); ok {
		t.Error("the base domain itself must not map to a public name")
	}
}

func TestTunnelDomains_Unconfigured(t *testing.T) {
	var zero TunnelDomains
	if zero.Configured() {
		t.Error("a zero TunnelDomains must not report itself configured")
	}
	if _, ok := zero.ToLocal("web.example.com"); ok {
		t.Error("an unconfigured mapping must not resolve anything")
	}
	if _, ok := (TunnelDomains{Base: "dev.localhost"}).ToPublic("web.dev.localhost"); ok {
		t.Error("a half-configured mapping must not resolve anything")
	}
}

// tunnelTestBackend builds a Docker backend for the guard tests. The target is
// never dialed: every request these tests make is refused before it reaches the
// proxy, except the one case that checks a published route gets through.
func tunnelTestBackend(hostname string, published bool) *docker.Backend {
	return &docker.Backend{
		ContainerID:   "c-" + hostname,
		ContainerName: hostname,
		ServiceName:   hostname,
		Host:          "127.0.0.1",
		Port:          9,
		Hostname:      hostname,
		Tunnel:        published,
	}
}

// newTunnelGuard wires a TunnelHandler in front of a handler that records
// whether it was reached and on which hostname.
func newTunnelGuard(t *testing.T, router *Router) (*TunnelHandler, *string) {
	t.Helper()

	var reachedHost string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("reached"))
	})

	domains := TunnelDomains{Base: "dev.localhost", Public: "example.com"}
	return NewTunnelHandler(inner, router, domains, "roji.dev.localhost"), &reachedHost
}

func TestTunnelHandler_PublishedRoutePassesThrough(t *testing.T) {
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))

	guard, reachedHost := newTunnelGuard(t, router)

	req := httptest.NewRequest("GET", "http://web.example.com/page", nil)
	req.Host = "web.example.com"
	w := httptest.NewRecorder()
	guard.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", w.Code, http.StatusOK, w.Body.String())
	}
	// The local hostname is the routing key everything downstream is written
	// against, and what a dev server's host allowlist expects.
	if *reachedHost != "web.dev.localhost" {
		t.Errorf("inner handler saw Host %q, want web.dev.localhost", *reachedHost)
	}
}

func TestTunnelHandler_Refuses(t *testing.T) {
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))
	router.AddBackend(tunnelTestBackend("private.dev.localhost", false))
	router.AddStaticSite(&StaticBackend{Hostname: "docs.dev.localhost", Root: t.TempDir(), Index: true})

	tests := []struct {
		name string
		host string
		path string
		why  string
	}{
		{
			name: "route without the label",
			host: "private.example.com",
			path: "/",
			why:  "publishing is opt-in per container",
		},
		{
			name: "unknown hostname",
			host: "nothing.example.com",
			path: "/",
		},
		{
			name: "dashboard",
			host: "roji.example.com",
			path: "/",
			why:  "the dashboard has no authentication",
		},
		{
			name: "base domain",
			host: "example.com",
			path: "/",
		},
		{
			name: "another domain entirely",
			host: "web.evil.com",
			path: "/",
		},
		{
			name: "compose up on a published hostname",
			host: "web.example.com",
			path: "/_api/projects/myapp/up",
			why:  "the API starts containers without asking for credentials",
		},
		{
			name: "request log on a published hostname",
			host: "web.example.com",
			path: "/_api/logs",
		},
		{
			name: "status on a published hostname",
			host: "web.example.com",
			path: "/_api/status",
		},
		{
			name: "dashboard assets",
			host: "web.example.com",
			path: "/_assets/petite-vue.min.js",
		},
		{
			name: "static site",
			host: "docs.example.com",
			path: "/",
			why:  "static sites carry no opt-in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard, reachedHost := newTunnelGuard(t, router)

			req := httptest.NewRequest("GET", "http://"+tt.host+tt.path, nil)
			req.Host = tt.host
			w := httptest.NewRecorder()
			guard.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d (%s)", w.Code, http.StatusNotFound, tt.why)
			}
			if *reachedHost != "" {
				t.Errorf("the request reached the inner handler as %q", *reachedHost)
			}
			// The same bare answer whatever the reason, so a refusal says
			// nothing about which hostnames or paths exist.
			if body := strings.TrimSpace(w.Body.String()); body != "Not Found" {
				t.Errorf("body = %q, want %q", body, "Not Found")
			}
		})
	}
}

func TestTunnelHandler_HostWithPort(t *testing.T) {
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))

	guard, reachedHost := newTunnelGuard(t, router)

	req := httptest.NewRequest("GET", "http://web.example.com/", nil)
	req.Host = "web.example.com:8080"
	w := httptest.NewRecorder()
	guard.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if *reachedHost != "web.dev.localhost" {
		t.Errorf("inner handler saw Host %q, want web.dev.localhost", *reachedHost)
	}
}

func TestTunnelHandler_PathPrefixRoute(t *testing.T) {
	router := NewRouter()
	api := tunnelTestBackend("shared.dev.localhost", true)
	api.PathPrefix = "/api"
	router.AddBackend(api)

	web := tunnelTestBackend("shared.dev.localhost", false)
	web.ContainerID = "c-web"
	router.AddBackend(web)

	guard, reachedHost := newTunnelGuard(t, router)

	// The prefixed route is published, so it answers.
	req := httptest.NewRequest("GET", "http://shared.example.com/api/users", nil)
	req.Host = "shared.example.com"
	w := httptest.NewRecorder()
	guard.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("prefixed route: status = %d, want %d", w.Code, http.StatusOK)
	}
	if *reachedHost != "shared.dev.localhost" {
		t.Errorf("inner handler saw Host %q, want shared.dev.localhost", *reachedHost)
	}

	// The route serving the rest of that hostname is not, and sharing a
	// hostname with a published route does not publish it.
	guard, reachedHost = newTunnelGuard(t, router)
	req = httptest.NewRequest("GET", "http://shared.example.com/", nil)
	req.Host = "shared.example.com"
	w = httptest.NewRecorder()
	guard.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("unpublished sibling: status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if *reachedHost != "" {
		t.Errorf("the request reached the inner handler as %q", *reachedHost)
	}
}

// The guard resolves a route and Handler resolves it again. A container that
// stops in between drops the request onto the not-found page, which names every
// route roji knows. That page is for whoever runs the machine.
func TestHandler_NotFoundOverTunnelNamesNothing(t *testing.T) {
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", nil, nil)

	request := func(tunneled bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "http://gone.dev.localhost/", nil)
		if tunneled {
			r = r.WithContext(context.WithValue(r.Context(), tunneledKey{}, true))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		return rec
	}

	// Locally the page is worth having, which is what makes the check below
	// mean something rather than pass for want of routes to leak.
	if body := request(false).Body.String(); !strings.Contains(body, "web.dev.localhost") {
		t.Fatal("the local not-found page stopped listing routes; this test no longer proves anything")
	}

	body := request(true).Body.String()
	for _, secret := range []string{"web.dev.localhost", "roji.dev.localhost", "127.0.0.1"} {
		if strings.Contains(body, secret) {
			t.Errorf("the tunnel's 404 named %q:\n%s", secret, body)
		}
	}
}

func TestRouter_TunnelURL(t *testing.T) {
	router := NewRouter()
	router.SetTunnelDomains(TunnelDomains{Base: "dev.localhost", Public: "example.com"})

	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))
	router.AddBackend(tunnelTestBackend("private.dev.localhost", false))

	prefixed := tunnelTestBackend("shared.dev.localhost", true)
	prefixed.PathPrefix = "/api"
	router.AddBackend(prefixed)

	urls := make(map[string]string)
	for _, info := range router.ListRoutes() {
		urls[info.Hostname+info.PathPrefix] = info.TunnelURL
	}

	if got := urls["web.dev.localhost"]; got != "https://web.example.com" {
		t.Errorf("TunnelURL = %q, want https://web.example.com", got)
	}
	if got := urls["shared.dev.localhost/api"]; got != "https://shared.example.com/api" {
		t.Errorf("TunnelURL = %q, want https://shared.example.com/api", got)
	}
	if got := urls["private.dev.localhost"]; got != "" {
		t.Errorf("an unpublished route reported TunnelURL %q", got)
	}
}

func TestRouter_TunnelURL_NoTunnelConfigured(t *testing.T) {
	// The label alone publishes nothing, and the dashboard reads TunnelURL to
	// decide what to say, so an opted-in route with no tunnel behind it has to
	// come back indistinguishable from one that never asked.
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))

	for _, info := range router.ListRoutes() {
		if info.TunnelURL != "" {
			t.Errorf("TunnelURL = %q with no tunnel configured, want empty", info.TunnelURL)
		}
	}
}
