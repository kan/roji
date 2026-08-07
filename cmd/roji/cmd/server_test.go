package cmd

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
