//go:build !windows

package cmd

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/proxy"
	"github.com/spf13/cobra"
)

// testPort stands in for the port roji run picked.
const testPort = 5173

// runTestSettings points the CLI at a roji listening on httpsPort.
func runTestSettings(httpsPort int) *config.Settings {
	return &config.Settings{
		Domain:    "dev.localhost",
		Dashboard: "roji.dev.localhost",
		Bind:      "127.0.0.1",
		HTTPSPort: httpsPort,
	}
}

// runTestServer stands in for a running roji: the real handler and router
// behind TLS, and settings that point the CLI at it.
func runTestServer(t *testing.T) (*proxy.Router, *config.Settings) {
	t.Helper()

	router := proxy.NewRouter()
	handler := proxy.NewHandler(router, nil, "roji.dev.localhost", "dev.localhost", nil, nil)
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	httpsPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}

	return router, runTestSettings(httpsPort)
}

func TestLeaseHolder_AcquireAndRelease(t *testing.T) {
	router, settings := runTestServer(t)
	l := newLeaseHolder(settings, "app", testPort)

	lease, err := l.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	if lease.Hostname != "app.dev.localhost" {
		t.Errorf("Hostname = %q, want app.dev.localhost", lease.Hostname)
	}
	l.hostname = lease.Hostname

	route := router.Resolve("app.dev.localhost", "/")
	if route == nil || route.LocalBackend.Target.Host != "localhost:5173" {
		t.Fatalf("Resolve returned %+v, want the route to localhost:5173", route)
	}

	// Renewing is the same call, and what brings the route back after roji
	// restarts and forgets it.
	if _, err := l.acquire(context.Background()); err != nil {
		t.Errorf("renewing: acquire() error = %v", err)
	}

	l.release()
	if router.Resolve("app.dev.localhost", "/") != nil {
		t.Error("the route is still there after release")
	}
}

// A name that cannot be had stops roji run before it starts the command.
func TestLeaseHolder_Refusals(t *testing.T) {
	router, settings := runTestServer(t)

	target := &url.URL{Scheme: "http", Host: "localhost:9000"}
	if _, err := router.Lease("taken.dev.localhost", target, "someone-else"); err != nil {
		t.Fatal(err)
	}
	router.ReplaceDeclared([]*proxy.Route{{
		Hostname:     "declared.dev.localhost",
		LocalBackend: &proxy.LocalBackend{Target: target},
	}})

	for _, host := range []string{"taken", "declared", "roji", "not a hostname"} {
		if _, err := newLeaseHolder(settings, host, testPort).acquire(context.Background()); err == nil {
			t.Errorf("acquire(%q) succeeded, want it refused", host)
		}
	}
}

func TestLeaseHolder_NotRunning(t *testing.T) {
	// A port nothing listens on, which is what roji being stopped looks like.
	l := newLeaseHolder(runTestSettings(freePort(t)), "app", testPort)

	if _, err := l.acquire(context.Background()); err == nil {
		t.Error("acquire() succeeded with no roji to talk to")
	}
}

// The tests below run commands that finish at once, well before the first
// renewal, so runWithLease never talks to the roji these settings point at.

func TestRunWithLease_PassesThePort(t *testing.T) {
	l := newLeaseHolder(runTestSettings(freePort(t)), "app", testPort)

	out := filepath.Join(t.TempDir(), "out")
	// Writes what it was given, so both ways of receiving the port are checked:
	// the environment, and {port} wherever it appears in an argument.
	code := runWithLease(l, []string{
		"sh", "-c", `printf '%s %s %s' "$PORT" "$1" "$2" > "$0"`, out, "{port}", "--listen=:{port}",
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "5173 5173 --listen=:5173" {
		t.Errorf("the command received %q, want %q", got, "5173 5173 --listen=:5173")
	}
}

// roji run is a wrapper, so a script that checks the command's exit code has
// to see the command's.
func TestRunWithLease_ExitCode(t *testing.T) {
	l := newLeaseHolder(runTestSettings(freePort(t)), "app", testPort)

	if code := runWithLease(l, []string{"sh", "-c", "exit 7"}); code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
	if code := runWithLease(l, []string{filepath.Join(t.TempDir(), "no-such-command")}); code != 127 {
		t.Errorf("exit code for a command that cannot start = %d, want 127", code)
	}
}

// A dev server is rarely roji run's direct child: `npx vite` is npm running a
// shell running node. A signal that stops at the direct child leaves the
// server holding its port.
func TestRunWithLease_SignalReachesGrandchildren(t *testing.T) {
	l := newLeaseHolder(runTestSettings(freePort(t)), "app", testPort)

	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	stopped := filepath.Join(dir, "stopped")

	// The outer shell is the direct child. The inner one, a grandchild, says
	// when it is told to stop.
	script := `sh -c 'trap "touch \"$1\"; exit 0" TERM; touch "$0"; while :; do sleep 0.05; done' "$0" "$1" & wait`

	done := make(chan int, 1)
	go func() { done <- runWithLease(l, []string{"sh", "-c", script, started, stopped}) }()

	waitForFile(t, started)
	// To roji run alone, the way `kill` or a supervisor sends it.
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runWithLease did not return after SIGTERM")
	}
	waitForFile(t, stopped)
}

// A command stopped by a signal is reported the way a shell reports it, so
// that whatever started roji run can tell Ctrl-C from a crash.
func TestRunWithLease_ExitCodeForSignal(t *testing.T) {
	l := newLeaseHolder(runTestSettings(freePort(t)), "app", testPort)

	if code := runWithLease(l, []string{"sh", "-c", "kill -INT $$"}); code != 128+int(syscall.SIGINT) {
		t.Errorf("exit code = %d, want %d", code, 128+int(syscall.SIGINT))
	}
}

func TestDescendants(t *testing.T) {
	// A shell with a child of its own, both of which stay around.
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, pid := range descendants(cmd.Process.Pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	})

	// The shell needs a moment to start its child.
	deadline := time.Now().Add(5 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		if pids = descendants(cmd.Process.Pid); len(pids) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if len(pids) != 2 || pids[0] != cmd.Process.Pid {
		t.Errorf("descendants = %v, want the shell (%d) followed by its one child", pids, cmd.Process.Pid)
	}
	// This test process is the shell's parent, not its descendant.
	if slices.Contains(pids, os.Getpid()) {
		t.Errorf("descendants = %v includes the parent", pids)
	}
}

// roji reaches the command through "localhost", which is both loopback
// addresses, so the port has to be free on both.
func TestPickFreePort(t *testing.T) {
	port, err := pickFreePort()
	if err != nil {
		t.Fatalf("pickFreePort() error = %v", err)
	}

	for _, host := range []string{"127.0.0.1", "::1"} {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			if host == "::1" && !errors.Is(err, syscall.EADDRINUSE) {
				continue // No IPv6 on this machine
			}
			t.Errorf("port %d is not free on %s: %v", port, host, err)
			continue
		}
		ln.Close()
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", filepath.Base(path))
}

func TestRunCmd_RequiresTheSeparator(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr bool
	}{
		{[]string{"run", "app", "--", "sh", "-c", "true"}, false},
		{[]string{"run", "app", "sh"}, true}, // The command's flags would be read as roji's
		{[]string{"run", "--", "sh"}, true},  // No name
		{[]string{"run", "app", "--"}, true}, // No command
		{[]string{"run", "a", "b", "--", "sh"}, true},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			// Parse the way cobra does for a real invocation, then run only the
			// argument check: RunE would start the command. A fresh command
			// each time, since where the "--" was is remembered across parses.
			c := &cobra.Command{Use: "run"}
			if err := c.ParseFlags(tt.args[1:]); err != nil {
				t.Fatal(err)
			}
			err := runCmd.Args(c, c.Flags().Args())
			if (err != nil) != tt.wantErr {
				t.Errorf("Args() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
