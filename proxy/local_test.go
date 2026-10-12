package proxy

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/kan/roji/config"
	"github.com/kan/roji/docker"
)

// localRoute declares a server on the host under hostname.
func localRoute(t *testing.T, hostname, target string) *Route {
	t.Helper()

	u, err := url.Parse(target)
	if err != nil {
		t.Fatalf("bad target %q: %v", target, err)
	}
	return &Route{Hostname: hostname, LocalBackend: &LocalBackend{Target: u}}
}

// declare adds one route to the declared set. It goes through ReplaceDeclared,
// the way startup and a reload do, with what is already declared kept.
func declare(router *Router, route *Route) {
	router.mu.RLock()
	routes := slices.Collect(maps.Values(router.declaredRoutes))
	router.mu.RUnlock()

	router.ReplaceDeclared(append(routes, route))
}

// closedAddr returns a loopback address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot find a free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// tunneled marks a request the way TunnelHandler does once it lets one through.
func tunneled(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tunneledKey{}, true))
}

func TestHandler_LocalProxy(t *testing.T) {
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		_, _ = w.Write([]byte("from the dev server"))
	}))
	defer backend.Close()

	router := NewRouter()
	declare(router, localRoute(t, "api.dev.localhost", backend.URL))
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	req := httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/v1/items?q=50%&page=2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "from the dev server" {
		t.Errorf("body = %q", body)
	}

	// The same treatment a container gets, since both go through proxyTo.
	if got.Host != "api.dev.localhost" {
		t.Errorf("backend saw Host %q, want the hostname the client asked for", got.Host)
	}
	if got.URL.Path != "/v1/items" {
		t.Errorf("backend saw path %q, want /v1/items", got.URL.Path)
	}
	if got.URL.RawQuery != "q=50%&page=2" {
		t.Errorf("backend saw query %q, want it verbatim", got.URL.RawQuery)
	}
	if proto := got.Header.Get("X-Forwarded-Proto"); proto != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want https", proto)
	}

	logs := handler.logBuffer.ListRecent(1)
	if len(logs) != 1 || logs[0].Service != KindLocal || logs[0].Host != "api.dev.localhost" {
		t.Errorf("request log = %+v, want one entry for api.dev.localhost with service %q", logs, KindLocal)
	}
}

func TestHandler_LocalProxy_TargetPath(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer backend.Close()

	router := NewRouter()
	declare(router, localRoute(t, "api.dev.localhost", backend.URL+"/base"))
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/items", nil))

	if gotPath != "/base/items" {
		t.Errorf("backend saw path %q, want the target's path in front: /base/items", gotPath)
	}
}

// vite's HMR is the reason this matters: a dev server on the host is the
// commonest thing to put behind a local route, and it talks WebSocket.
func TestHandler_LocalProxy_WebSocket(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("backend upgrade error: %v", err)
			return
		}
		defer conn.Close()

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(messageType, message)
	}))
	defer backend.Close()

	router := NewRouter()
	declare(router, localRoute(t, "app.dev.localhost", backend.URL))
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	header := http.Header{}
	header.Set("Host", "app.dev.localhost")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(proxyServer.URL, "http")+"/hmr", header)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if string(message) != "ping" {
		t.Errorf("message = %q, want ping", message)
	}
}

func TestHandler_LocalProxy_BasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()

	route := localRoute(t, "api.dev.localhost", backend.URL)
	route.LocalBackend.BasicAuth = &config.BasicAuth{User: "admin", Pass: "secret"}

	router := NewRouter()
	declare(router, route)
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status without credentials = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status with credentials = %d, want 200", rec.Code)
	}
}

// A dev server that has not been started is the usual state of a local route,
// so the answer explains what to do. Through the tunnel the same page would
// tell a stranger which address roji forwards to.
func TestHandler_LocalProxy_NotRunning(t *testing.T) {
	addr := closedAddr(t)

	route := localRoute(t, "api.dev.localhost", "http://"+addr)
	route.LocalBackend.Tunnel = true

	router := NewRouter()
	declare(router, route)
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, addr) || !strings.Contains(body, "proxy_sites") {
		t.Errorf("the page names neither the target nor where to change it:\n%s", body)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, tunneled(httptest.NewRequest(http.MethodGet, "http://api.dev.localhost/", nil)))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status over the tunnel = %d, want 404", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "Not Found" {
		t.Errorf("body over the tunnel = %q, want the bare 404", body)
	}
}

// The page is about dev servers on the host. A container that refuses the
// connection is a different situation with a different fix.
func TestHandler_DockerBackendNotRunningStaysPlain(t *testing.T) {
	router := NewRouter()
	addBackendFor(t, router, "web.dev.localhost", "http://"+closedAddr(t))
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://web.dev.localhost/", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "Bad Gateway" {
		t.Errorf("body = %q, want the plain Bad Gateway", body)
	}
}

// The guard and Handler resolve the route separately. Whatever changed in
// between — a reload, a container stopping — the route that answers a tunneled
// request has to be one that opted in.
func TestHandler_TunneledRequestNeedsPublishedRoute(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("private"))
	}))
	defer backend.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter()
	declare(router, localRoute(t, "api.dev.localhost", backend.URL))
	declare(router, &Route{Hostname: "docs.dev.localhost", StaticBackend: &StaticBackend{Root: root, Index: true}})
	addBackendFor(t, router, "web.dev.localhost", backend.URL)
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	for _, host := range []string{"api.dev.localhost", "docs.dev.localhost", "web.dev.localhost"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, tunneled(httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)))

		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "private") {
			t.Errorf("%s over the tunnel: status %d, body %q; want the bare 404", host, rec.Code, rec.Body.String())
		}
	}
}

// A published container can still carry a warning, and the page for it names
// the service and suggests compose edits.
func TestHandler_RouteWarningOverTunnelNamesNothing(t *testing.T) {
	router := NewRouter()
	router.AddBackend(&docker.Backend{
		ContainerID: "web", ContainerName: "web", ServiceName: "secret-service",
		Hostname: "web.dev.localhost", Tunnel: true,
		Warning: "no port exposed",
	})
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://web.dev.localhost/", nil))
	if !strings.Contains(rec.Body.String(), "secret-service") {
		t.Fatal("the local warning page stopped naming the service; this test no longer proves anything")
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, tunneled(httptest.NewRequest(http.MethodGet, "http://web.dev.localhost/", nil)))
	if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != "Not Found" {
		t.Errorf("over the tunnel: status %d, body %q; want the bare 404", rec.Code, rec.Body.String())
	}
}

func TestTunnelHandler_PublishedDeclaredSites(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("webhook received on " + r.Host))
	}))
	defer backend.Close()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}

	hook := localRoute(t, "hook.dev.localhost", backend.URL)
	hook.LocalBackend.Tunnel = true

	router := NewRouter()
	declare(router, hook)
	declare(router, &Route{
		Hostname:      "docs.dev.localhost",
		StaticBackend: &StaticBackend{Root: root, Index: true, Tunnel: true},
	})
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)
	guard := NewTunnelHandler(handler, router, TunnelDomains{Base: "dev.localhost", Public: "example.com"}, "roji.dev.localhost")

	get := func(host string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Body)
		return rec.Code, string(body)
	}

	// The backend is handed the local name, as a published container is.
	if code, body := get("hook.example.com"); code != http.StatusOK || body != "webhook received on hook.dev.localhost" {
		t.Errorf("server on the host: status %d, body %q", code, body)
	}

	code, body := get("docs.example.com")
	if code != http.StatusOK || !strings.Contains(body, "sub") {
		t.Errorf("static site: status %d, want the directory listing:\n%s", code, body)
	}
	// The listing links to the dashboard for whoever runs the machine. Outside
	// it is a dead link that names a local hostname.
	if strings.Contains(body, "roji.dev.localhost") {
		t.Errorf("the listing served through the tunnel names the dashboard:\n%s", body)
	}
}

// Publishing a directory for what it shows must not publish what sits beside
// it. Locally the same files are the developer's own and stay readable.
func TestHandler_StaticSiteOverTunnelHidesDotfiles(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		".env":                     "SECRET=1",
		".git/config":              "[core]",
		".well-known/security.txt": "Contact: mailto:me@example.com",
		"index.txt":                "public",
	} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	router := NewRouter()
	declare(router, &Route{
		Hostname:      "docs.dev.localhost",
		StaticBackend: &StaticBackend{Root: root, Index: true, Tunnel: true},
	})
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	get := func(path string, viaTunnel bool) (int, string) {
		req := httptest.NewRequest(http.MethodGet, "http://docs.dev.localhost"+path, nil)
		if viaTunnel {
			req = tunneled(req)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	for _, path := range []string{"/.env", "/.git/config", "/sub/../.env", "/.git/"} {
		if code, body := get(path, true); code != http.StatusNotFound || strings.TrimSpace(body) != "Not Found" {
			t.Errorf("%s over the tunnel: status %d, body %q; want the bare 404", path, code, body)
		}
	}
	if code, body := get("/.env", false); code != http.StatusOK || body != "SECRET=1" {
		t.Errorf("/.env locally: status %d, body %q; want the file", code, body)
	}

	if code, body := get("/.well-known/security.txt", true); code != http.StatusOK || !strings.Contains(body, "Contact") {
		t.Errorf("/.well-known over the tunnel: status %d; it exists to be fetched from outside", code)
	}

	_, listing := get("/", true)
	if !strings.Contains(listing, "index.txt") || strings.Contains(listing, ".env") || strings.Contains(listing, ".git") {
		t.Errorf("listing over the tunnel should show index.txt and no hidden names:\n%s", listing)
	}
	if _, local := get("/", false); !strings.Contains(local, ".env") {
		t.Error("the local listing stopped showing hidden files; this test no longer proves anything")
	}
}

// checkBasicAuth lets OPTIONS through for CORS preflight. A backend answers
// that without a body; a static site must not answer it with the file.
func TestHandler_StaticSiteAuthNotBypassedByOptions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.html"), []byte("top secret"), 0644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter()
	declare(router, &Route{
		Hostname: "docs.dev.localhost",
		StaticBackend: &StaticBackend{
			Root:      root,
			BasicAuth: &config.BasicAuth{User: "admin", Pass: "secret"},
		},
	})
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	for _, method := range []string{http.MethodOptions, http.MethodPost} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "http://docs.dev.localhost/secret.html", nil))
		if strings.Contains(rec.Body.String(), "top secret") {
			t.Errorf("%s without credentials was answered with the file (status %d)", method, rec.Code)
		}
	}
}

func TestRouter_DeclaredRoutesInListRoutes(t *testing.T) {
	router := NewRouter()
	router.SetTunnelDomains(TunnelDomains{Base: "dev.localhost", Public: "example.com"})

	api := localRoute(t, "API.dev.localhost", "http://127.0.0.1:8787")
	api.LocalBackend.Tunnel = true
	api.LocalBackend.BasicAuth = &config.BasicAuth{User: "u", Pass: "p"}
	declare(router, api)
	declare(router, &Route{Hostname: "docs.dev.localhost", StaticBackend: &StaticBackend{Root: "/var/www/docs", Index: true}})
	router.AddBackend(&docker.Backend{
		ContainerID: "web", ContainerName: "web", ServiceName: "web",
		Host: "172.17.0.2", Port: 8080, Hostname: "web.dev.localhost",
	})

	infos := make(map[string]RouteInfo)
	for _, info := range router.ListRoutes() {
		infos[info.Hostname] = info
	}

	local, ok := infos["api.dev.localhost"]
	if !ok {
		t.Fatalf("no route for api.dev.localhost in %+v; hostnames are matched lowercase", infos)
	}
	if local.Kind != KindLocal || local.Target != "http://127.0.0.1:8787" || local.ServiceName != KindLocal {
		t.Errorf("local route = %+v", local)
	}
	if !local.HasBasicAuth || local.TunnelURL != "https://api.example.com" {
		t.Errorf("local route = %+v, want basic auth and a tunnel URL", local)
	}

	static := infos["docs.dev.localhost"]
	if static.Kind != KindStatic || !static.IndexEnabled || static.TunnelURL != "" {
		t.Errorf("static route = %+v", static)
	}
	if infos["web.dev.localhost"].Kind != KindDocker {
		t.Errorf("docker route = %+v", infos["web.dev.localhost"])
	}

	// The dashboard and anything else reading /_api/routes switches on kind.
	raw, err := json.Marshal(local)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"kind":"local"`) {
		t.Errorf("JSON = %s, want a kind field", raw)
	}
}

// One hostname, two declarations: the later one wins, whichever keys they came
// from, so a reload gives the same answer as a restart.
func TestRouter_HostnameDeclaredTwice(t *testing.T) {
	router := NewRouter()
	router.ReplaceDeclared([]*Route{
		{Hostname: "docs.dev.localhost", StaticBackend: &StaticBackend{Root: "/var/www/docs"}},
		localRoute(t, "docs.dev.localhost", "http://127.0.0.1:3000"),
	})

	route := router.Resolve("docs.dev.localhost", "/")
	if route == nil || route.Kind() != KindLocal {
		t.Errorf("Resolve returned %+v, want the later declaration", route)
	}
	if n := len(router.ListRoutes()); n != 1 {
		t.Errorf("ListRoutes has %d entries, want 1", n)
	}
}
