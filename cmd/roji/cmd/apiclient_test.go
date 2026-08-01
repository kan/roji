package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestAPIGetUsesConfigFile ensures subcommands reach the server described by
// the config file, instead of a hardcoded host and port.
func TestAPIGetUsesConfigFile(t *testing.T) {
	// Neutralize ambient ROJI_* variables (empty values are ignored by
	// config.Load), so the test only exercises the config file.
	for _, key := range []string{"ROJI_DOMAIN", "ROJI_HTTPS_PORT", "ROJI_DASHBOARD"} {
		t.Setenv(key, "")
	}

	var gotHost, gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath = r.Host, r.URL.Path
	}))
	defer srv.Close()

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to determine test server port: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf("domain: example.localhost\nhttps_port: %s\n", port)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	old := configFile
	configFile = configPath
	t.Cleanup(func() { configFile = old })

	resp, err := apiGet(&cobra.Command{}, "/_api/routes", 5*time.Second)
	if err != nil {
		t.Fatalf("apiGet() error = %v", err)
	}
	resp.Body.Close()

	if gotPath != "/_api/routes" {
		t.Errorf("path = %q, want %q", gotPath, "/_api/routes")
	}
	// Dashboard defaults to roji.<domain> when the config file omits it.
	if got, want := gotHost, "roji.example.localhost"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
}
