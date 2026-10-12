package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kan/roji/docker"
)

// fakeClock lets a test move a router's time forward.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newLeaseRouter() (*Router, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	router := NewRouter()
	router.now = clock.now
	return router, clock
}

func leaseTarget(port int) *url.URL {
	return &url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(port)}
}

func TestRouter_LeaseRoutesUntilItLapses(t *testing.T) {
	router, clock := newLeaseRouter()

	if _, err := router.Lease("App.dev.localhost", leaseTarget(5173), "owner-a"); err != nil {
		t.Fatalf("Lease() error = %v", err)
	}

	route := router.Resolve("app.dev.localhost", "/")
	if route == nil || route.Kind() != KindLocal || !route.LocalBackend.Temporary {
		t.Fatalf("Resolve returned %+v, want the leased route", route)
	}
	if route.Published() {
		t.Error("a leased route must not be published through the tunnel")
	}

	clock.advance(LeaseTTL - time.Second)
	if router.Resolve("app.dev.localhost", "/") == nil {
		t.Error("the lease lapsed before its TTL")
	}

	// Routing stops on time whether or not anything has swept the lease away:
	// the sweep exists for the dashboard.
	clock.advance(2 * time.Second)
	if route := router.Resolve("app.dev.localhost", "/"); route != nil {
		t.Errorf("Resolve returned %+v after the lease lapsed", route)
	}
	if n := len(router.ListRoutes()); n != 0 {
		t.Errorf("ListRoutes has %d entries after the lease lapsed, want 0", n)
	}

	router.ExpireLeases()
	if n := len(router.leases); n != 0 {
		t.Errorf("%d leases left after ExpireLeases, want 0", n)
	}
}

func TestRouter_LeaseRenewal(t *testing.T) {
	router, clock := newLeaseRouter()

	for range 5 {
		if _, err := router.Lease("app.dev.localhost", leaseTarget(5173), "owner-a"); err != nil {
			t.Fatalf("Lease() error = %v", err)
		}
		clock.advance(LeaseTTL - time.Second)
	}

	if router.Resolve("app.dev.localhost", "/") == nil {
		t.Error("a lease renewed inside its TTL lapsed")
	}
}

func TestRouter_LeaseOwnership(t *testing.T) {
	router, clock := newLeaseRouter()

	if _, err := router.Lease("app.dev.localhost", leaseTarget(5173), "owner-a"); err != nil {
		t.Fatalf("Lease() error = %v", err)
	}

	// Two commands asked for one name. The second is told, rather than quietly
	// taking the first one's traffic.
	if _, err := router.Lease("app.dev.localhost", leaseTarget(6000), "owner-b"); !errors.Is(err, ErrLeaseHeld) {
		t.Errorf("Lease() by another owner: error = %v, want ErrLeaseHeld", err)
	}

	// A command that exits late must not take down a route it no longer holds.
	router.Release("app.dev.localhost", "owner-b")
	if router.Resolve("app.dev.localhost", "/") == nil {
		t.Error("Release by another owner removed the lease")
	}

	// The name is free once the holder is gone, by lapsing or by giving it back.
	clock.advance(LeaseTTL + time.Second)
	if _, err := router.Lease("app.dev.localhost", leaseTarget(6000), "owner-b"); err != nil {
		t.Fatalf("Lease() after the first lapsed: error = %v", err)
	}
	if got := router.Resolve("app.dev.localhost", "/").LocalBackend.Target.Host; got != "localhost:6000" {
		t.Errorf("target = %s, want the new owner's", got)
	}

	router.Release("app.dev.localhost", "owner-b")
	if router.Resolve("app.dev.localhost", "/") != nil {
		t.Error("the lease is still routed after its owner released it")
	}
}

// The config file was written deliberately, so it outranks a command started
// later — on registration, and when a reload declares a hostname a lease holds.
func TestRouter_LeaseYieldsToConfig(t *testing.T) {
	router, _ := newLeaseRouter()
	declared := func() []*Route {
		return []*Route{localRoute(t, "api.dev.localhost", "http://127.0.0.1:8787")}
	}

	router.ReplaceDeclared(declared())
	if _, err := router.Lease("api.dev.localhost", leaseTarget(5173), "owner-a"); !errors.Is(err, ErrLeaseDeclared) {
		t.Errorf("Lease() on a declared hostname: error = %v, want ErrLeaseDeclared", err)
	}

	if _, err := router.Lease("app.dev.localhost", leaseTarget(5173), "owner-a"); err != nil {
		t.Fatalf("Lease() error = %v", err)
	}

	// A reload replaces what the config file declares and nothing else.
	router.ReplaceDeclared(declared())
	if router.Resolve("app.dev.localhost", "/") == nil {
		t.Fatal("a config reload dropped the lease")
	}

	router.ReplaceDeclared(append(declared(), localRoute(t, "app.dev.localhost", "http://127.0.0.1:9000")))
	route := router.Resolve("app.dev.localhost", "/")
	if route == nil || route.LocalBackend.Temporary {
		t.Errorf("Resolve returned %+v, want the site the config file now declares", route)
	}
	if n := len(router.ListRoutes()); n != 2 {
		t.Errorf("ListRoutes has %d entries, want 2: the shadowed lease is not a route", n)
	}
}

func TestRouter_LeaseHidesDockerRoute(t *testing.T) {
	router, _ := newLeaseRouter()
	router.AddBackend(&docker.Backend{
		ContainerID: "web", ContainerName: "web", ServiceName: "web",
		Host: "172.17.0.2", Port: 8080, Hostname: "web.dev.localhost",
	})

	hidden, err := router.Lease("web.dev.localhost", leaseTarget(5173), "owner-a")
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if hidden == nil || hidden.Backend.ServiceName != "web" {
		t.Errorf("hidden = %+v, want the container's route so the caller can say so", hidden)
	}
	if route := router.Resolve("web.dev.localhost", "/"); route == nil || route.Kind() != KindLocal {
		t.Errorf("Resolve returned %+v, want the lease: the declared tier comes first", route)
	}

	router.Release("web.dev.localhost", "owner-a")
	if route := router.Resolve("web.dev.localhost", "/"); route == nil || route.Kind() != KindDocker {
		t.Errorf("Resolve returned %+v after release, want the container back", route)
	}
}

func TestRouter_LeaseInListRoutes(t *testing.T) {
	router, _ := newLeaseRouter()
	if _, err := router.Lease("app.dev.localhost", leaseTarget(5173), "owner-a"); err != nil {
		t.Fatalf("Lease() error = %v", err)
	}

	infos := router.ListRoutes()
	if len(infos) != 1 {
		t.Fatalf("ListRoutes has %d entries, want 1", len(infos))
	}
	info := infos[0]
	if info.Kind != KindLocal || !info.Temporary || info.Target != "http://localhost:5173" {
		t.Errorf("info = %+v", info)
	}
}

// leaseAPI sends one request to the lease endpoint on the dashboard host.
func leaseAPI(handler *Handler, method, target string, body any, header http.Header) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	req := httptest.NewRequest(method, "http://roji.dev.localhost"+target, &payload)
	req.RemoteAddr = "127.0.0.1:50000" // The CLI, on this machine
	maps.Copy(req.Header, header)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// With `bind` widened, roji is reachable from the network. This endpoint gives
// a hostname to any local port, loopback-only services included, so it answers
// to this machine whatever `bind` says.
func TestHandler_LeaseAPIFromThisMachineOnly(t *testing.T) {
	handler := NewHandler(NewRouter(), nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	tests := []struct {
		name       string
		remoteAddr string
		localAddr  string // What the connection arrived on, empty for unknown
		wantStatus int
	}{
		{"loopback", "127.0.0.1:50000", "127.0.0.1:443", http.StatusOK},
		{"IPv6 loopback", "[::1]:50000", "[::1]:443", http.StatusOK},
		{"another machine", "192.168.1.50:50000", "192.168.1.20:443", http.StatusForbidden},
		{"another machine, nothing known about this end", "192.168.1.50:50000", "", http.StatusForbidden},
		// bind names an address of this machine and no loopback, so the CLI
		// connects to that address and the kernel uses it as the source too.
		{"this machine's own address", "192.168.1.20:50000", "192.168.1.20:443", http.StatusOK},
		{"unparseable", "not-an-address", "", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			_ = json.NewEncoder(&body).Encode(LeaseRequest{Host: "app", Port: 5173, Owner: "o"})
			req := httptest.NewRequest(http.MethodPost, "http://roji.dev.localhost"+LeasePath, &body)
			req.RemoteAddr = tt.remoteAddr
			if tt.localAddr != "" {
				addr, err := net.ResolveTCPAddr("tcp", tt.localAddr)
				if err != nil {
					t.Fatal(err)
				}
				req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, net.Addr(addr)))
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
		})
	}
}

func TestHandler_LeaseAPI(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("dev server saw " + r.Host))
	}))
	defer backend.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(backend.URL, "http://127.0.0.1:"))

	router := NewRouter()
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := leaseAPI(handler, http.MethodPost, LeasePath, LeaseRequest{Host: "App", Port: port, Owner: "owner-a"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var lease LeaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &lease); err != nil {
		t.Fatal(err)
	}
	if lease.Hostname != "app.dev.localhost" || lease.URL != "https://app.dev.localhost" || !lease.CertCovered {
		t.Errorf("lease = %+v", lease)
	}

	// The route works like any other local one.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.dev.localhost/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "dev server saw app.dev.localhost" {
		t.Errorf("request through the lease: status %d, body %q", rec.Code, rec.Body)
	}

	// Asked again by its owner, it is renewed. Asked by anyone else, refused.
	if rec := leaseAPI(handler, http.MethodPost, LeasePath, LeaseRequest{Host: "app", Port: port, Owner: "owner-a"}, nil); rec.Code != http.StatusOK {
		t.Errorf("renewal: status %d, body %s", rec.Code, rec.Body)
	}
	rec = leaseAPI(handler, http.MethodPost, LeasePath, LeaseRequest{Host: "app", Port: port, Owner: "owner-b"}, nil)
	var refused LeaseError
	_ = json.Unmarshal(rec.Body.Bytes(), &refused)
	if rec.Code != http.StatusConflict || refused.Code != LeaseErrorHeld {
		t.Errorf("another owner: status %d, error %+v; want 409 %s", rec.Code, refused, LeaseErrorHeld)
	}

	// Releasing takes the route away.
	query := url.Values{"hostname": {"app.dev.localhost"}, "owner": {"owner-a"}}
	if rec := leaseAPI(handler, http.MethodDelete, LeasePath+"?"+query.Encode(), nil, nil); rec.Code != http.StatusNoContent {
		t.Errorf("release: status %d", rec.Code)
	}
	if router.Resolve("app.dev.localhost", "/") != nil {
		t.Error("the route is still there after its release")
	}
}

func TestHandler_LeaseAPIRefuses(t *testing.T) {
	router := NewRouter()
	router.ReplaceDeclared([]*Route{localRoute(t, "api.dev.localhost", "http://127.0.0.1:8787")})
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	tests := []struct {
		name       string
		request    any
		header     http.Header
		wantStatus int
		wantCode   string
	}{
		{
			name:       "declared in the config file",
			request:    LeaseRequest{Host: "api", Port: 5173, Owner: "o"},
			wantStatus: http.StatusConflict,
			wantCode:   LeaseErrorDeclared,
		},
		{
			name:       "the dashboard's hostname",
			request:    LeaseRequest{Host: "roji", Port: 5173, Owner: "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "not a hostname",
			request:    LeaseRequest{Host: "my app/../x", Port: 5173, Owner: "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "empty name",
			request:    LeaseRequest{Host: "", Port: 5173, Owner: "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "port out of range",
			request:    LeaseRequest{Host: "app", Port: 70000, Owner: "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "no port",
			request:    LeaseRequest{Host: "app", Owner: "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "no owner",
			request:    LeaseRequest{Host: "app", Port: 5173},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			name:       "a target is not something a caller chooses",
			request:    map[string]any{"host": "app", "target": "http://192.168.1.1", "owner": "o"},
			wantStatus: http.StatusBadRequest,
			wantCode:   LeaseErrorInvalid,
		},
		{
			// The API has no credentials to check. A page open in a browser on
			// this machine can POST to it, and would otherwise be able to give
			// itself a hostname pointing at any local port.
			name:       "sent by a browser",
			request:    LeaseRequest{Host: "app", Port: 5173, Owner: "o"},
			header:     http.Header{"Origin": {"https://evil.example"}},
			wantStatus: http.StatusForbidden,
			wantCode:   LeaseErrorInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := leaseAPI(handler, http.MethodPost, LeasePath, tt.request, tt.header)

			var refused LeaseError
			_ = json.Unmarshal(rec.Body.Bytes(), &refused)
			if rec.Code != tt.wantStatus || refused.Code != tt.wantCode {
				t.Errorf("status %d, error %+v; want %d %s", rec.Code, refused, tt.wantStatus, tt.wantCode)
			}
			if router.Resolve("app.dev.localhost", "/") != nil {
				t.Error("a refused request registered a route")
			}
		})
	}
}

// The endpoint is part of the dashboard's API, which the tunnel never serves.
func TestTunnelHandler_RefusesLeaseAPI(t *testing.T) {
	router := NewRouter()
	router.AddBackend(tunnelTestBackend("web.dev.localhost", true))
	guard, reachedHost := newTunnelGuard(t, router)

	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(LeaseRequest{Host: "app", Port: 5173, Owner: "o"})
	req := httptest.NewRequest(http.MethodPost, "http://web.example.com"+LeasePath, &body)
	req.Host = "web.example.com"
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || *reachedHost != "" {
		t.Errorf("status %d, reached %q; want the bare 404 without reaching the handler", rec.Code, *reachedHost)
	}
}

func TestHandler_LeasedRouteNotRunning(t *testing.T) {
	router := NewRouter()
	port, _ := strconv.Atoi(strings.TrimPrefix(closedAddr(t), "127.0.0.1:"))
	if _, err := router.Lease("app.dev.localhost", leaseTarget(port), "owner-a"); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", testStatusConfig(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.dev.localhost/", nil))

	// The advice for a site in the config file — edit proxy_sites — does not
	// apply to a route nobody wrote there.
	body := rec.Body.String()
	if rec.Code != http.StatusBadGateway || !strings.Contains(body, "{port}") || strings.Contains(body, "proxy_sites") {
		t.Errorf("status %d; the page should explain {port} and not mention proxy_sites:\n%s", rec.Code, body)
	}
}
