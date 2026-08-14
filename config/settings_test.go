package config

import (
	"os"
	"path/filepath"
	"slices"
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
				Auth: &StaticSiteAuth{
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
				Auth: &StaticSiteAuth{},
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
