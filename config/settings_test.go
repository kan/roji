package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	settings := Defaults()

	if settings.Network != "roji" {
		t.Errorf("expected Network 'roji', got '%s'", settings.Network)
	}
	if settings.Domain != "dev.localhost" {
		t.Errorf("expected Domain 'dev.localhost', got '%s'", settings.Domain)
	}
	if settings.HTTPPort != 80 {
		t.Errorf("expected HTTPPort 80, got %d", settings.HTTPPort)
	}
	if settings.HTTPSPort != 443 {
		t.Errorf("expected HTTPSPort 443, got %d", settings.HTTPSPort)
	}
	if settings.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got '%s'", settings.LogLevel)
	}
	if !settings.AutoCert {
		t.Errorf("expected AutoCert true, got false")
	}
}

func TestLoadWithDefaults(t *testing.T) {
	// Load with no config file and no overrides
	settings, err := Load("/nonexistent/config.yaml", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Network != "roji" {
		t.Errorf("expected Network 'roji', got '%s'", settings.Network)
	}
	if settings.Domain != "dev.localhost" {
		t.Errorf("expected Domain 'dev.localhost', got '%s'", settings.Domain)
	}
	// Dashboard should default to "roji." + Domain
	expectedDashboard := "roji." + settings.Domain
	if settings.Dashboard != expectedDashboard {
		t.Errorf("expected Dashboard '%s', got '%s'", expectedDashboard, settings.Dashboard)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `network: custom-network
domain: custom.localhost
http_port: 8080
https_port: 8443
log_level: debug
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	settings, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Network != "custom-network" {
		t.Errorf("expected Network 'custom-network', got '%s'", settings.Network)
	}
	if settings.Domain != "custom.localhost" {
		t.Errorf("expected Domain 'custom.localhost', got '%s'", settings.Domain)
	}
	if settings.HTTPPort != 8080 {
		t.Errorf("expected HTTPPort 8080, got %d", settings.HTTPPort)
	}
	if settings.HTTPSPort != 8443 {
		t.Errorf("expected HTTPSPort 8443, got %d", settings.HTTPSPort)
	}
	if settings.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got '%s'", settings.LogLevel)
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("ROJI_NETWORK", "env-network")
	os.Setenv("ROJI_DOMAIN", "env.localhost")
	os.Setenv("ROJI_LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("ROJI_NETWORK")
		os.Unsetenv("ROJI_DOMAIN")
		os.Unsetenv("ROJI_LOG_LEVEL")
	}()

	settings, err := Load("/nonexistent/config.yaml", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Network != "env-network" {
		t.Errorf("expected Network 'env-network', got '%s'", settings.Network)
	}
	if settings.Domain != "env.localhost" {
		t.Errorf("expected Domain 'env.localhost', got '%s'", settings.Domain)
	}
	if settings.LogLevel != "warn" {
		t.Errorf("expected LogLevel 'warn', got '%s'", settings.LogLevel)
	}
}

func TestLoadWithCLIOverrides(t *testing.T) {
	// CLI overrides should take highest priority
	overrides := map[string]any{
		"network":   "cli-network",
		"domain":    "cli.localhost",
		"http_port": 9080,
	}

	settings, err := Load("/nonexistent/config.yaml", overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Network != "cli-network" {
		t.Errorf("expected Network 'cli-network', got '%s'", settings.Network)
	}
	if settings.Domain != "cli.localhost" {
		t.Errorf("expected Domain 'cli.localhost', got '%s'", settings.Domain)
	}
	if settings.HTTPPort != 9080 {
		t.Errorf("expected HTTPPort 9080, got %d", settings.HTTPPort)
	}
}

func TestLoadPriority(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `network: file-network
domain: file.localhost
http_port: 8080
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variable
	os.Setenv("ROJI_DOMAIN", "env.localhost")
	defer os.Unsetenv("ROJI_DOMAIN")

	// CLI override for http_port
	overrides := map[string]any{
		"http_port": 9999,
	}

	settings, err := Load(configPath, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Network should come from file (no env or CLI override)
	if settings.Network != "file-network" {
		t.Errorf("expected Network 'file-network', got '%s'", settings.Network)
	}
	// Domain should come from env (overrides file)
	if settings.Domain != "env.localhost" {
		t.Errorf("expected Domain 'env.localhost', got '%s'", settings.Domain)
	}
	// HTTPPort should come from CLI (overrides env and file)
	if settings.HTTPPort != 9999 {
		t.Errorf("expected HTTPPort 9999, got %d", settings.HTTPPort)
	}
}

func TestNetworks(t *testing.T) {
	tests := []struct {
		network  string
		expected []string
	}{
		{"roji", []string{"roji"}},
		{"net1,net2", []string{"net1", "net2"}},
		{"net1, net2, net3", []string{"net1", "net2", "net3"}},
		{"", []string{"roji"}},         // Empty defaults to roji
		{"  ,  ,  ", []string{"roji"}}, // Only whitespace
	}

	for _, tt := range tests {
		s := &Settings{Network: tt.network}
		got := s.Networks()

		if len(got) != len(tt.expected) {
			t.Errorf("Networks(%q): expected %v, got %v", tt.network, tt.expected, got)
			continue
		}

		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("Networks(%q)[%d]: expected %q, got %q", tt.network, i, tt.expected[i], v)
			}
		}
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", home + "/test"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandPath(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestToYAML(t *testing.T) {
	settings := &Settings{
		Network:   "test-network",
		Domain:    "test.localhost",
		HTTPPort:  80,
		HTTPSPort: 443,
		LogLevel:  "info",
		AutoCert:  true,
	}

	yaml, err := settings.ToYAML()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that key fields are present
	if !contains(yaml, "network: test-network") {
		t.Errorf("YAML should contain 'network: test-network', got:\n%s", yaml)
	}
	if !contains(yaml, "domain: test.localhost") {
		t.Errorf("YAML should contain 'domain: test.localhost', got:\n%s", yaml)
	}
}

func TestSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	settings := Defaults()
	settings.Network = "saved-network"

	if err := settings.SaveToFile(configPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if !contains(string(content), "network: saved-network") {
		t.Errorf("saved file should contain 'network: saved-network', got:\n%s", content)
	}
	if !contains(string(content), "# roji configuration file") {
		t.Errorf("saved file should contain header comment, got:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestStaticSiteAuth(t *testing.T) {
	// Test GetBasicAuth
	tests := []struct {
		name    string
		site    StaticSite
		hasAuth bool
		user    string
		pass    string
		realm   string
	}{
		{
			name:    "no auth",
			site:    StaticSite{Host: "test", Root: "/tmp"},
			hasAuth: false,
		},
		{
			name: "with basic auth",
			site: StaticSite{
				Host: "test",
				Root: "/tmp",
				Auth: &SiteAuth{
					Basic: &BasicAuth{
						User:  "admin",
						Pass:  "secret",
						Realm: "Test Realm",
					},
				},
			},
			hasAuth: true,
			user:    "admin",
			pass:    "secret",
			realm:   "Test Realm",
		},
		{
			name: "auth without basic",
			site: StaticSite{
				Host: "test",
				Root: "/tmp",
				Auth: &SiteAuth{},
			},
			hasAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := tt.site.GetBasicAuth()

			if tt.hasAuth {
				if auth == nil {
					t.Fatal("expected BasicAuth, got nil")
				}
				if auth.User != tt.user {
					t.Errorf("User = %q, want %q", auth.User, tt.user)
				}
				if auth.Pass != tt.pass {
					t.Errorf("Pass = %q, want %q", auth.Pass, tt.pass)
				}
				if auth.Realm != tt.realm {
					t.Errorf("Realm = %q, want %q", auth.Realm, tt.realm)
				}
			} else {
				if auth != nil {
					t.Errorf("expected nil BasicAuth, got %+v", auth)
				}
			}
		})
	}
}

func TestLoadStaticSiteWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `static_sites:
  - host: docs
    root: /tmp/docs
    auth:
      basic:
        user: admin
        pass: secret123
        realm: Documentation
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	settings, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(settings.StaticSites) != 1 {
		t.Fatalf("expected 1 static site, got %d", len(settings.StaticSites))
	}

	site := settings.StaticSites[0]
	auth := site.GetBasicAuth()

	if auth == nil {
		t.Fatal("expected BasicAuth, got nil")
	}
	if auth.User != "admin" {
		t.Errorf("User = %q, want %q", auth.User, "admin")
	}
	if auth.Pass != "secret123" {
		t.Errorf("Pass = %q, want %q", auth.Pass, "secret123")
	}
	if auth.Realm != "Documentation" {
		t.Errorf("Realm = %q, want %q", auth.Realm, "Documentation")
	}
}

func TestBindAddrs(t *testing.T) {
	tests := []struct {
		bind     string
		expected []string
	}{
		{DefaultBind, []string{"127.0.0.1", "::1"}},
		{"127.0.0.1", []string{"127.0.0.1"}},
		{"127.0.0.1, ::1 ", []string{"127.0.0.1", "::1"}},
		// An empty setting asks for every interface, which is one listener on
		// the wildcard address rather than none.
		{"", []string{""}},
		{"  ,  ", []string{""}},
	}

	for _, tt := range tests {
		s := &Settings{Bind: tt.bind}
		if got := s.BindAddrs(); !slices.Equal(got, tt.expected) {
			t.Errorf("BindAddrs(%q) = %v, want %v", tt.bind, got, tt.expected)
		}
	}
}

func TestListensBeyondLoopback(t *testing.T) {
	tests := []struct {
		bind string
		want bool
	}{
		{DefaultBind, false},
		{"127.0.0.1", false},
		{"::1", false},
		{"127.0.0.2", false}, // the whole 127/8 block is loopback
		{"", true},           // wildcard
		{"0.0.0.0", true},
		{"::", true},
		{"192.168.1.5", true},
		{"127.0.0.1,192.168.1.5", true}, // one exposed address is enough
		// A hostname cannot be judged without resolving it, so it counts as
		// exposed rather than being waved through.
		{"localhost", true},
	}

	for _, tt := range tests {
		s := &Settings{Bind: tt.bind}
		if got := s.ListensBeyondLoopback(); got != tt.want {
			t.Errorf("ListensBeyondLoopback(%q) = %v, want %v", tt.bind, got, tt.want)
		}
	}
}

func TestLoadBindFromEnv(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		env  string
		want string
	}{
		{"unset leaves the default", false, "", DefaultBind},
		{"a value replaces the default", true, "127.0.0.1", "127.0.0.1"},
		// An empty ROJI_BIND is a value, not an absence: it asks for every
		// interface. Reading it with os.Getenv would silently keep the default.
		{"an empty value means all interfaces", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("ROJI_BIND", tt.env)
			} else {
				os.Unsetenv("ROJI_BIND")
			}

			settings, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if settings.Bind != tt.want {
				t.Errorf("Bind = %q, want %q", settings.Bind, tt.want)
			}
		})
	}
}

func TestLocalAddr(t *testing.T) {
	tests := []struct {
		bind string
		want string
	}{
		// The wildcard has no address of its own to connect to.
		{"", "localhost"},
		{"127.0.0.1", "127.0.0.1"},
		{"::1", "::1"},
		// Loopback wins wherever it appears, so reordering the setting — which
		// is documented as an unordered list — does not change the answer.
		{"127.0.0.1,::1", "127.0.0.1"},
		{"192.168.1.5,127.0.0.1", "127.0.0.1"},
		{"127.0.0.1,192.168.1.5", "127.0.0.1"},
		// Without loopback, any bound address reaches the server from here.
		{"192.168.1.5", "192.168.1.5"},
	}

	for _, tt := range tests {
		s := &Settings{Bind: tt.bind}
		if got := s.LocalAddr(); got != tt.want {
			t.Errorf("LocalAddr(%q) = %q, want %q", tt.bind, got, tt.want)
		}
	}
}

func TestTunnelEnabled(t *testing.T) {
	tests := []struct {
		name   string
		tunnel *Tunnel
		want   bool
	}{
		{"nil", nil, false},
		{"empty", &Tunnel{}, false},
		{"domain only", &Tunnel{Domain: "example.com"}, false},
		{"name only", &Tunnel{Name: "roji"}, false},
		{"both", &Tunnel{Domain: "example.com", Name: "roji"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tunnel.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTunnelListenPort(t *testing.T) {
	tests := []struct {
		name   string
		tunnel *Tunnel
		want   int
	}{
		{"nil", nil, DefaultTunnelPort},
		{"unset", &Tunnel{Domain: "example.com", Name: "roji"}, DefaultTunnelPort},
		{"explicit", &Tunnel{Domain: "example.com", Name: "roji", Port: 9000}, 9000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tunnel.ListenPort(); got != tt.want {
				t.Errorf("ListenPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLoadTunnelFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `domain: dev.localhost
tunnel:
  domain: example.com
  name: roji
  port: 9000
  auto_start: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !settings.Tunnel.Enabled() {
		t.Fatal("expected the tunnel to be enabled")
	}
	if settings.Tunnel.Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", settings.Tunnel.Domain)
	}
	if settings.Tunnel.Name != "roji" {
		t.Errorf("Name = %q, want roji", settings.Tunnel.Name)
	}
	if settings.Tunnel.ListenPort() != 9000 {
		t.Errorf("ListenPort() = %d, want 9000", settings.Tunnel.ListenPort())
	}
	if !settings.Tunnel.AutoStart {
		t.Error("expected AutoStart true")
	}
}

func TestLoadProxySitesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `proxy_sites:
  - host: api
    target: http://127.0.0.1:8787
    tunnel: true
    auth:
      basic:
        user: admin
        pass: secret
  - host: app
    target: http://localhost:5173
static_sites:
  - host: docs
    root: /var/www/docs
    tunnel: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(settings.ProxySites) != 2 {
		t.Fatalf("ProxySites = %+v, want 2 sites", settings.ProxySites)
	}
	api := settings.ProxySites[0]
	if api.Host != "api" || api.Target != "http://127.0.0.1:8787" || !api.Tunnel {
		t.Errorf("first site = %+v", api)
	}
	if auth := api.Auth.BasicAuth(); auth == nil || auth.User != "admin" {
		t.Errorf("first site auth = %+v, want user admin", auth)
	}
	app := settings.ProxySites[1]
	if app.Tunnel || app.Auth.BasicAuth() != nil {
		t.Errorf("second site = %+v, want neither tunnel nor auth: both are opt-in", app)
	}
	if !settings.StaticSites[0].Tunnel {
		t.Error("the static site's tunnel key was not read")
	}
}

func TestParseProxyTarget(t *testing.T) {
	tests := []struct {
		target  string
		wantErr bool
	}{
		{"http://127.0.0.1:8787", false},
		{"http://localhost:5173", false},
		{"http://[::1]:3000", false},
		{"http://127.0.0.1:8787/base", false},
		{"http://127.0.0.1", false},
		{"127.0.0.1:8787", true},
		{"8787", true},
		{"", true},
		{"http://:8787", true},
		{"http://localhost:", true},
		{"http://localhost:0", true},
		{"http://localhost:99999", true},
		{"http://localhost:65535", false},
		{"https://127.0.0.1:8787", true},
		{"ws://127.0.0.1:8787", true},
		{"http://user:pass@127.0.0.1:8787", true},
		{"http://127.0.0.1:8787/?debug=1", true},
		{"http://127.0.0.1:8787/#top", true},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			u, err := ParseProxyTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseProxyTarget(%q) = %v, %v; wantErr %v", tt.target, u, err, tt.wantErr)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1":       true,
		"127.0.0.2":       true,
		"::1":             true,
		"localhost":       true,
		"LOCALHOST":       true,
		"app.localhost":   true,
		"192.168.1.20":    false,
		"0.0.0.0":         false,
		"example.com":     false,
		"notlocalhost":    false,
		"localhost.evil.": false,
	}

	for host, want := range tests {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestResolveSiteHost(t *testing.T) {
	tests := []struct {
		host, want string
	}{
		{"api", "api.dev.localhost"},
		{"API", "api.dev.localhost"},
		{"api.dev.localhost", "api.dev.localhost"},
		{"private.example.com", "private.example.com"},
	}

	for _, tt := range tests {
		if got := ResolveSiteHost(tt.host, "dev.localhost"); got != tt.want {
			t.Errorf("ResolveSiteHost(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestCertCoversHost(t *testing.T) {
	tests := []struct {
		hostname, baseDomain string
		want                 bool
	}{
		{"api.dev.localhost", "dev.localhost", true},
		{"API.Dev.Localhost", "dev.localhost", true},
		{"dev.localhost", "dev.localhost", true},
		// A wildcard covers one label, whatever the certificate lists.
		{"a.b.dev.localhost", "dev.localhost", false},
		{"private.example.com", "dev.localhost", false},
		{"apidev.localhost", "dev.localhost", false},
		{".dev.localhost", "dev.localhost", false},
		{"api.localhost", "localhost", true},
		{"a.b.localhost", "localhost", false},
	}

	for _, tt := range tests {
		if got := CertCoversHost(tt.hostname, tt.baseDomain); got != tt.want {
			t.Errorf("CertCoversHost(%q, %q) = %v, want %v", tt.hostname, tt.baseDomain, got, tt.want)
		}
	}
}

func TestLoadWithoutTunnel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("domain: dev.localhost\n"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settings.Tunnel != nil {
		t.Errorf("expected no tunnel, got %+v", settings.Tunnel)
	}
	if settings.Tunnel.Enabled() {
		t.Error("a nil tunnel must not report itself enabled")
	}
}

// writeConfigFiles writes the config file and, unless local is empty, the
// per-machine file next to it, and returns the config file's path.
func writeConfigFiles(t *testing.T, shared, local string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if shared != "" {
		if err := os.WriteFile(path, []byte(shared), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
	}
	if local != "" {
		if err := os.WriteFile(filepath.Join(dir, LocalFileName), []byte(local), 0644); err != nil {
			t.Fatalf("failed to write local config: %v", err)
		}
	}
	return path
}

func TestLoadMergesLocalFile(t *testing.T) {
	path := writeConfigFiles(t, `domain: dev.localhost
http_port: 8080
tunnel:
  domain: example.com
  name: shared
  auto_start: true
static_sites:
  - host: docs
    root: /srv/docs
  - host: blog
    root: /srv/blog
`, `tunnel:
  name: this-machine
  routes: [web]
static_sites:
  - host: notes
    root: /srv/notes
http_port: null
`)

	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mappings merge key by key: the local file names the tunnel and the
	// shared one still supplies the rest of it.
	tun := settings.Tunnel
	if tun == nil || tun.Domain != "example.com" || tun.Name != "this-machine" || !tun.AutoStart {
		t.Errorf("tunnel = %+v, want the shared domain and auto_start with the local name", tun)
	}
	if !slices.Equal(tun.Routes, []string{"web"}) {
		t.Errorf("tunnel.routes = %v, want [web]", tun.Routes)
	}

	// A list is replaced whole.
	if len(settings.StaticSites) != 1 || settings.StaticSites[0].Host != "notes" {
		t.Errorf("static_sites = %+v, want only the local file's", settings.StaticSites)
	}

	// null removes a key, which leaves the default.
	if settings.HTTPPort != 80 {
		t.Errorf("http_port = %d, want the default 80 after the local file cleared it", settings.HTTPPort)
	}

	// Values keep the text they were written as. Merging used to go through
	// map[string]any, which read `pass: 0123` as the number 123.
	path = writeConfigFiles(t, `static_sites:
  - host: docs
    root: /srv/docs
    auth:
      basic:
        user: admin
        pass: 0123
`, `tunnel:
  domain: example.com
  name: 1.10
`)
	settings, err = Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := settings.StaticSites[0].GetBasicAuth().Pass; got != "0123" {
		t.Errorf("pass = %q, want 0123 as written", got)
	}
	if got := settings.Tunnel.Name; got != "1.10" {
		t.Errorf("tunnel.name = %q, want 1.10 as written", got)
	}

	// A mapping only the local file has loses its nulls as well.
	path = writeConfigFiles(t, "domain: dev.localhost\n", "tunnel:\n  domain: example.com\n  name: roji\n  port: null\n")
	settings, err = Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Tunnel.ListenPort() != DefaultTunnelPort {
		t.Errorf("tunnel port = %d, want the default", settings.Tunnel.ListenPort())
	}

	// A machine that should not run the shared tunnel turns it off.
	path = writeConfigFiles(t, "tunnel:\n  domain: example.com\n  name: shared\n", "tunnel: null\n")
	settings, err = Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Tunnel.Enabled() {
		t.Errorf("tunnel = %+v, want none after the local file cleared it", settings.Tunnel)
	}
}

func TestReadConfigFiles(t *testing.T) {
	t.Run("only the local file", func(t *testing.T) {
		path := writeConfigFiles(t, "", "domain: local.localhost\n")
		_, files, err := ReadConfigFiles(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(files, []string{LocalFilePath(path)}) {
			t.Errorf("files = %v, want the local file alone", files)
		}

		settings, err := Load(path, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.Domain != "local.localhost" {
			t.Errorf("domain = %q, want the local file's", settings.Domain)
		}
	})

	t.Run("neither file", func(t *testing.T) {
		path := writeConfigFiles(t, "", "")
		if _, _, err := ReadConfigFiles(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("err = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("empty files", func(t *testing.T) {
		path := writeConfigFiles(t, "# nothing yet\n", "~\n")
		if _, files, err := ReadConfigFiles(path); err != nil || len(files) != 2 {
			t.Errorf("files = %v, err = %v, want both read without error", files, err)
		}
	})

	t.Run("a top level that is not a mapping", func(t *testing.T) {
		path := writeConfigFiles(t, "domain: dev.localhost\n", "- a\n- b\n")
		if _, _, err := ReadConfigFiles(path); err == nil || !strings.Contains(err.Error(), LocalFilePath(path)) {
			t.Errorf("err = %v, want one naming %s", err, LocalFilePath(path))
		}
	})

	t.Run("bad YAML names the file", func(t *testing.T) {
		path := writeConfigFiles(t, "domain: dev.localhost\n", "tunnel: [\n")
		_, _, err := ReadConfigFiles(path)
		if err == nil || !strings.Contains(err.Error(), LocalFilePath(path)) {
			t.Errorf("err = %v, want one naming %s", err, LocalFilePath(path))
		}
		if _, err := Load(path, nil); err == nil {
			t.Error("Load accepted a local file that is not YAML")
		}
	})
}

func TestLocalFilePath(t *testing.T) {
	// Next to the path as given: a config.yaml linked in from elsewhere still
	// pairs with the local file in the directory holding the link.
	dir := filepath.Join("home", "u", ".config", "roji")
	tests := []struct{ config, local string }{
		{"config.yaml", "local.yaml"},
		// Any other name pairs with a file named after it, so a config file
		// passed with --config does not pick up a local.yaml that belongs to
		// something else in the same directory.
		{"roji.yaml", "roji.local.yaml"},
		{"roji.yml", "roji.local.yml"},
		{"local.yaml", "local.local.yaml"},
		{"rojirc", "rojirc.local"},
	}
	for _, tt := range tests {
		if got := LocalFilePath(filepath.Join(dir, tt.config)); got != filepath.Join(dir, tt.local) {
			t.Errorf("LocalFilePath(%s) = %q, want %s", tt.config, got, tt.local)
		}
	}
}

func TestLoadIgnoresUnrelatedLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roji.yaml")
	if err := os.WriteFile(path, []byte("domain: dev.localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Not roji's: not even YAML.
	if err := os.WriteFile(filepath.Join(dir, LocalFileName), []byte("[[\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roji.local.yaml"), []byte("domain: here.localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}

	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Domain != "here.localhost" {
		t.Errorf("domain = %q, want roji.local.yaml's", settings.Domain)
	}
}

func TestAutoTunnelName(t *testing.T) {
	tests := []struct {
		hostname string
		want     string
	}{
		{"devbox", "roji-devbox"},
		{"DevBox", "roji-devbox"},
		{"devbox.example.com", "roji-devbox"},
		{"Kan's MacBook", "roji-kan-s-macbook"},
		{"my_host", "roji-my-host"},
		{"-x-", "roji-x"},
		// Nothing usable gives no name rather than one shared by every
		// such machine.
		{"", ""},
		{"___", ""},
		{"開発機", ""},
	}
	for _, tt := range tests {
		if got := AutoTunnelName(tt.hostname); got != tt.want {
			t.Errorf("AutoTunnelName(%q) = %q, want %q", tt.hostname, got, tt.want)
		}
	}
}

func TestTunnelResolveAutoName(t *testing.T) {
	hostname := func(name string, err error) func() (string, error) {
		return func() (string, error) { return name, err }
	}

	tun := &Tunnel{Domain: "example.com", Name: TunnelNameAuto}
	tun.resolveAutoName(hostname("devbox.local", nil))
	if tun.Name != "roji-devbox" || !tun.AutoName {
		t.Errorf("tunnel = %+v, want roji-devbox marked as derived", tun)
	}

	// A name written out is left alone.
	tun = &Tunnel{Domain: "example.com", Name: "roji"}
	tun.resolveAutoName(hostname("devbox", nil))
	if tun.Name != "roji" || tun.AutoName {
		t.Errorf("tunnel = %+v, want the written name untouched", tun)
	}

	// Without a usable hostname the tunnel stays off, and says why.
	for _, host := range []func() (string, error){
		hostname("", errors.New("no hostname")),
		hostname("開発機", nil),
	} {
		tun = &Tunnel{Domain: "example.com", Name: TunnelNameAuto}
		tun.resolveAutoName(host)
		if tun.Enabled() || !tun.AutoNameFailed() {
			t.Errorf("tunnel = %+v, want it disabled with AutoNameFailed", tun)
		}
	}
	if (&Tunnel{Name: "roji"}).AutoNameFailed() {
		t.Error("a written name reported AutoNameFailed")
	}

	var none *Tunnel
	none.resolveAutoName(hostname("devbox", nil)) // must not panic
}

func TestLoadAutoTunnelName(t *testing.T) {
	path := writeConfigFiles(t, "tunnel:\n  domain: example.com\n  name: auto\n", "")
	settings, err := Load(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Skipf("no hostname: %v", err)
	}
	if want := AutoTunnelName(host); settings.Tunnel.Name != want || !settings.Tunnel.AutoName {
		t.Errorf("tunnel = %+v, want name %q", settings.Tunnel, want)
	}
}

func TestTunnelResolveRoutes(t *testing.T) {
	tun := &Tunnel{Routes: []string{"Web", " api.dev.localhost ", "", "other.example.com", "roji"}}
	sites := tun.ResolveRoutes("dev.localhost", "roji.dev.localhost")

	var hosts []string
	for _, site := range sites {
		hosts = append(hosts, site.Hostname)
	}
	want := []string{"web.dev.localhost", "api.dev.localhost", "other.example.com", "roji.dev.localhost"}
	if !slices.Equal(hosts, want) {
		t.Fatalf("hostnames = %v, want %v", hosts, want)
	}

	// The rules a declared site's name is held to apply here too.
	if sites[0].Err != nil || sites[0].Unpublishable {
		t.Errorf("web = %+v, want a publishable name", sites[0])
	}
	if !sites[2].Unpublishable {
		t.Errorf("other.example.com = %+v, want it unpublishable", sites[2])
	}
	if !errors.Is(sites[3].Err, ErrSiteReservedHost) {
		t.Errorf("roji = %+v, want the dashboard's name refused", sites[3])
	}

	var none *Tunnel
	if got := none.ResolveRoutes("dev.localhost", "roji.dev.localhost"); got != nil {
		t.Errorf("a nil tunnel returned %v", got)
	}
}
