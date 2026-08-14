package cmd

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/proxy"
)

// freePort returns a port nothing is listening on. There is a race between
// closing the listener and the caller binding it, but no other way to ask the
// OS for a port without holding it.
func freePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestListenAll(t *testing.T) {
	tests := []struct {
		name      string
		binds     []string
		want      int
		needsIPv6 bool
	}{
		{name: "single address", binds: []string{"127.0.0.1"}, want: 1},
		// An empty address is how the config spells "every interface"; it has
		// to reach net.Listen as ":port", not as a listener that fails to open.
		{name: "wildcard", binds: []string{""}, want: 1},
		{name: "both loopback addresses", binds: []string{"127.0.0.1", "::1"}, want: 2, needsIPv6: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needsIPv6 {
				requireIPv6Loopback(t)
			}

			// A fixed port for the multi-address case, where binding the same
			// port on two addresses is the point; port 0 elsewhere, so nothing
			// has to be reserved and released first.
			port := 0
			if len(tt.binds) > 1 {
				port = freePort(t)
			}

			listeners, err := listenAll(tt.binds, port, "test")
			if err != nil {
				t.Fatalf("listenAll: %v", err)
			}
			defer closeListeners(listeners)

			if len(listeners) != tt.want {
				t.Fatalf("got %d listeners, want %d", len(listeners), tt.want)
			}
			for i, ln := range listeners {
				host, p, err := net.SplitHostPort(ln.Addr().String())
				if err != nil {
					t.Fatalf("listener %d address %q: %v", i, ln.Addr(), err)
				}
				if want := tt.binds[i]; want != "" && host != want {
					t.Errorf("listener %d bound %q, want %q", i, host, want)
				}
				if port != 0 && p != strconv.Itoa(port) {
					t.Errorf("listener %d bound port %s, want %d", i, p, port)
				}
			}
		})
	}
}

func requireIPv6Loopback(t *testing.T) {
	t.Helper()

	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	ln.Close()
}

func TestListenAll_SkipsUnbindableLoopback(t *testing.T) {
	port := freePort(t)

	// Hold 127.0.0.2 so binding it fails the way ::1 does on a host with IPv6
	// disabled. The other loopback address still serves the same intent, so
	// startup carries on.
	held, err := net.Listen("tcp", net.JoinHostPort("127.0.0.2", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("cannot hold 127.0.0.2: %v", err)
	}
	defer held.Close()

	listeners, err := listenAll([]string{"127.0.0.2", "127.0.0.1"}, port, "test")
	if err != nil {
		t.Fatalf("listenAll: %v", err)
	}
	defer closeListeners(listeners)

	if len(listeners) != 1 {
		t.Fatalf("got %d listeners, want 1 (the unbindable loopback address is skipped)", len(listeners))
	}
}

func TestListenAll_FailsOnUnbindableNonLoopback(t *testing.T) {
	port := freePort(t)

	// 240.0.0.1 is reserved and assigned to no interface. Nothing substitutes
	// for an address asked for by name, so this is an error rather than a
	// quieter startup — even though a loopback address after it would bind.
	listeners, err := listenAll([]string{"240.0.0.1", "127.0.0.1"}, port, "test")
	if err == nil {
		closeListeners(listeners)
		t.Fatal("expected an error for an unbindable non-loopback address")
	}
	if !strings.Contains(err.Error(), "240.0.0.1") {
		t.Errorf("error = %q, want it to name the address", err)
	}

	// The listeners opened before the failure must not be left holding the port.
	probe, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port still held after a failed listenAll: %v", err)
	}
	probe.Close()
}

func TestListenAll_FailsWhenNoLoopbackAddressBinds(t *testing.T) {
	port := freePort(t)

	held := make([]net.Listener, 0, 2)
	for _, addr := range []string{"127.0.0.2", "127.0.0.3"} {
		ln, err := net.Listen("tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
		if err != nil {
			t.Skipf("cannot hold %s: %v", addr, err)
		}
		held = append(held, ln)
	}
	defer closeListeners(held)

	listeners, err := listenAll([]string{"127.0.0.2", "127.0.0.3"}, port, "test")
	if err == nil {
		closeListeners(listeners)
		t.Fatal("expected an error when every address is skipped")
	}
	if !strings.Contains(err.Error(), "127.0.0.2,127.0.0.3") {
		t.Errorf("error = %q, want it to name the addresses that were tried", err)
	}
}

// TestReopenIfRotated covers how `roji log` notices a rotation.
//
// rotateLogFile renames the log and the server recreates it, so a handle held
// by a follower keeps referring to the renamed file. Watching its size cannot
// see that: nothing truncates it, and Stat on the handle reports the renamed
// inode, so the size never goes backwards.
func TestReopenIfRotated(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "roji.log")
	if err := os.WriteFile(logPath, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	held, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	t.Run("nothing has changed", func(t *testing.T) {
		reopened, err := reopenIfRotated(held, logPath)
		if err != nil {
			t.Fatalf("reopenIfRotated: %v", err)
		}
		if reopened != nil {
			reopened.Close()
			t.Error("reopened a file that was not rotated")
		}
	})

	t.Run("path is gone", func(t *testing.T) {
		// The server recreates the log on its next write, so the follower has
		// to keep the handle it holds rather than fail.
		gone := filepath.Join(dir, "missing.log")
		reopened, err := reopenIfRotated(held, gone)
		if err != nil {
			t.Fatalf("reopenIfRotated: %v", err)
		}
		if reopened != nil {
			reopened.Close()
			t.Error("reopened a path that does not exist")
		}
	})

	t.Run("rotated", func(t *testing.T) {
		if err := os.Rename(logPath, logPath+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logPath, []byte("second\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		reopened, err := reopenIfRotated(held, logPath)
		if err != nil {
			t.Fatalf("reopenIfRotated: %v", err)
		}
		if reopened == nil {
			t.Fatal("did not reopen after a rotation")
		}
		defer reopened.Close()

		got, err := io.ReadAll(reopened)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "second\n" {
			t.Errorf("reopened file holds %q, want %q", got, "second\n")
		}
	})
}

func tunnelTestConfig(t *testing.T, tunnelCfg *config.Tunnel) Config {
	t.Helper()
	return Config{
		BaseDomain:    "dev.localhost",
		DashboardHost: "roji.dev.localhost",
		DataDir:       t.TempDir(),
		Tunnel:        tunnelCfg,
	}
}

func TestStartTunnel_NotConfigured(t *testing.T) {
	tests := []struct {
		name   string
		tunnel *config.Tunnel
	}{
		{"absent", nil},
		{"empty", &config.Tunnel{}},
		{"domain without a tunnel name", &config.Tunnel{Domain: "example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, runner := startTunnel(tunnelTestConfig(t, tt.tunnel), proxy.NewRouter(), nil)
			if server != nil {
				t.Error("an unconfigured tunnel must not open a listener")
			}
			if runner != nil {
				t.Error("an unconfigured tunnel must not run cloudflared")
			}
		})
	}
}

func TestStartTunnel_RefusesAProxyPort(t *testing.T) {
	// Which listener wins a shared port depends on bind, and one of the two
	// outcomes puts the tunnel guard in front of local browsers. Neither is a
	// state to start in.
	tests := []struct {
		name string
		port int
	}{
		{"the HTTP port", 8080},
		{"the HTTPS port", 8443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tunnelTestConfig(t, &config.Tunnel{
				Domain: "example.com",
				Name:   "roji",
				Port:   tt.port,
			})
			cfg.HTTPPort = 8080
			cfg.HTTPSPort = 8443

			server, runner := startTunnel(cfg, proxy.NewRouter(), nil)
			if server != nil {
				t.Error("a tunnel sharing a proxy port must not open a listener")
			}
			if runner != nil {
				t.Error("a tunnel sharing a proxy port must not run cloudflared")
			}
		})
	}
}

func TestStartTunnel_ListensOnLoopbackOnly(t *testing.T) {
	port := freePort(t)
	cfg := tunnelTestConfig(t, &config.Tunnel{
		Domain: "example.com",
		Name:   "roji",
		Port:   port,
		// cloudflared is not installed in CI, and starting it is a separate
		// concern from opening the listener it connects to.
		AutoStart: false,
	})

	router := proxy.NewRouter()
	server, runner := startTunnel(cfg, router, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	if server == nil {
		t.Fatal("startTunnel did not open a listener")
	}
	t.Cleanup(func() { _ = server.Close() })
	if runner != nil {
		t.Error("auto_start is off, so cloudflared must not be started")
	}

	// The guard depends on the tunnel arriving through a port of its own, so
	// the listener has to be there and answer.
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		t.Fatalf("the tunnel listener is not accepting connections: %v", err)
	}
	_ = conn.Close()
}

func TestStartTunnel_GuardsTheListener(t *testing.T) {
	port := freePort(t)
	cfg := tunnelTestConfig(t, &config.Tunnel{Domain: "example.com", Name: "roji", Port: port})

	reached := make(chan string, 1)
	server, _ := startTunnel(cfg, proxy.NewRouter(), http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) { reached <- r.Host }))
	if server == nil {
		t.Fatal("startTunnel did not open a listener")
	}
	t.Cleanup(func() { _ = server.Close() })

	// Nothing is published, so nothing reaches the proxy behind the guard.
	// proxy.TunnelHandler covers the individual refusals; this checks the
	// listener is wired to the guard rather than straight to the handler.
	req, err := http.NewRequest("GET", "http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(port))+"/", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Host = "roji.example.com"

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("request to the tunnel listener: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	select {
	case host := <-reached:
		t.Errorf("the dashboard request reached the proxy as %q", host)
	default:
	}
}

func TestStartTunnel_PortInUse(t *testing.T) {
	// Something else already holding the port must not stop roji: the tunnel
	// is an addition to a proxy that works without it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()

	cfg := tunnelTestConfig(t, &config.Tunnel{
		Domain: "example.com",
		Name:   "roji",
		Port:   ln.Addr().(*net.TCPAddr).Port,
	})

	server, runner := startTunnel(cfg, proxy.NewRouter(), nil)
	if server != nil || runner != nil {
		t.Error("startTunnel should give up quietly when the port is taken")
	}
}
