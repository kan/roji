package config

import (
	"testing"
)

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected RouteConfig
	}{
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: RouteConfig{},
		},
		{
			name: "host only",
			labels: map[string]string{
				"roji.host": "api.localhost",
			},
			expected: RouteConfig{
				Host: "api.localhost",
			},
		},
		{
			name: "port only",
			labels: map[string]string{
				"roji.port": "8080",
			},
			expected: RouteConfig{
				Port: 8080,
			},
		},
		{
			name: "path only",
			labels: map[string]string{
				"roji.path": "/api",
			},
			expected: RouteConfig{
				PathPrefix: "/api",
			},
		},
		{
			name: "all labels",
			labels: map[string]string{
				"roji.host": "myapp.localhost",
				"roji.port": "3000",
				"roji.path": "/v1",
			},
			expected: RouteConfig{
				Host:       "myapp.localhost",
				Port:       3000,
				PathPrefix: "/v1",
			},
		},
		{
			name: "with whitespace",
			labels: map[string]string{
				"roji.host": "  api.localhost  ",
				"roji.port": " 8080 ",
				"roji.path": " /api ",
			},
			expected: RouteConfig{
				Host:       "api.localhost",
				Port:       8080,
				PathPrefix: "/api",
			},
		},
		{
			name: "invalid port",
			labels: map[string]string{
				"roji.port": "not-a-number",
			},
			expected: RouteConfig{
				Port: 0,
			},
		},
		{
			name: "mixed with other labels",
			labels: map[string]string{
				"roji.host":                  "api.localhost",
				"com.docker.compose.service": "api",
				"com.docker.compose.project": "myproject",
				"some.other.label":           "value",
			},
			expected: RouteConfig{
				Host: "api.localhost",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLabels(tt.labels)

			if result.Host != tt.expected.Host {
				t.Errorf("Host = %q, want %q", result.Host, tt.expected.Host)
			}
			if result.Port != tt.expected.Port {
				t.Errorf("Port = %d, want %d", result.Port, tt.expected.Port)
			}
			if result.PathPrefix != tt.expected.PathPrefix {
				t.Errorf("PathPrefix = %q, want %q", result.PathPrefix, tt.expected.PathPrefix)
			}
		})
	}
}

func TestParseLabels_PathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"normal path", "/api", "/api"},
		{"path with trailing slash", "/api/", "/api"},
		{"path traversal attempt", "/../etc/passwd", ""},
		{"path traversal in middle", "/api/../secret", ""},
		{"double dots only", "..", ""},
		{"complex traversal", "/api/v1/../../admin", ""},
		{"clean path", "/api/v1/users", "/api/v1/users"},
		// A label naming the root means the same as no label at all, and is
		// spelled the same way.
		{"root path", "/", ""},
		{"root path with extra slashes", "///", ""},
		{"empty path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{
				"roji.path": tt.path,
			}
			cfg := ParseLabels(labels)
			if cfg.PathPrefix != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, cfg.PathPrefix)
			}
		})
	}
}

func TestDefaultHostname(t *testing.T) {
	tests := []struct {
		serviceName string
		baseDomain  string
		expected    string
	}{
		{"api", "localhost", "api.localhost"},
		{"web", "kan.localhost", "web.kan.localhost"},
		{"my-service", "dev.local", "my-service.dev.local"},
		{"", "localhost", ".localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName+"/"+tt.baseDomain, func(t *testing.T) {
			result := DefaultHostname(tt.serviceName, tt.baseDomain)
			if result != tt.expected {
				t.Errorf("DefaultHostname(%q, %q) = %q, want %q",
					tt.serviceName, tt.baseDomain, result, tt.expected)
			}
		})
	}
}

func TestParseMockLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected []*MockRoute
	}{
		{
			name:     "no mock labels",
			labels:   map[string]string{},
			expected: nil,
		},
		{
			name: "single GET mock",
			labels: map[string]string{
				"roji.mock.GET./api/users": `[{"id":1,"name":"Test"}]`,
			},
			expected: []*MockRoute{
				{Method: "GET", Path: "/api/users", Body: `[{"id":1,"name":"Test"}]`, StatusCode: 200},
			},
		},
		{
			name: "mock with custom status code",
			labels: map[string]string{
				"roji.mock.POST./api/users":        `{"created":true}`,
				"roji.mock.status.POST./api/users": "201",
			},
			expected: []*MockRoute{
				{Method: "POST", Path: "/api/users", Body: `{"created":true}`, StatusCode: 201},
			},
		},
		{
			name: "multiple mocks",
			labels: map[string]string{
				"roji.mock.GET./api/users":  `[]`,
				"roji.mock.GET./api/health": `{"status":"ok"}`,
			},
			expected: []*MockRoute{
				{Method: "GET", Path: "/api/users", Body: `[]`, StatusCode: 200},
				{Method: "GET", Path: "/api/health", Body: `{"status":"ok"}`, StatusCode: 200},
			},
		},
		{
			name: "status code only (no body)",
			labels: map[string]string{
				"roji.mock.status.DELETE./api/users/1": "204",
			},
			expected: []*MockRoute{
				{Method: "DELETE", Path: "/api/users/1", Body: "", StatusCode: 204},
			},
		},
		{
			name: "case insensitive method",
			labels: map[string]string{
				"roji.mock.get./api/test": `test`,
			},
			expected: []*MockRoute{
				{Method: "GET", Path: "/api/test", Body: `test`, StatusCode: 200},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ParseLabels(tt.labels)

			if len(cfg.MockRoutes) != len(tt.expected) {
				t.Fatalf("got %d mock routes, want %d", len(cfg.MockRoutes), len(tt.expected))
			}

			// Create a map for easier comparison (order-independent)
			expectedMap := make(map[string]*MockRoute)
			for _, m := range tt.expected {
				key := m.Method + " " + m.Path
				expectedMap[key] = m
			}

			for _, got := range cfg.MockRoutes {
				key := got.Method + " " + got.Path
				exp, ok := expectedMap[key]
				if !ok {
					t.Errorf("unexpected mock route: %s %s", got.Method, got.Path)
					continue
				}
				if got.Body != exp.Body {
					t.Errorf("mock %s: body = %q, want %q", key, got.Body, exp.Body)
				}
				if got.StatusCode != exp.StatusCode {
					t.Errorf("mock %s: status = %d, want %d", key, got.StatusCode, exp.StatusCode)
				}
			}
		})
	}
}

func TestParseMethodPath(t *testing.T) {
	tests := []struct {
		input      string
		wantMethod string
		wantPath   string
	}{
		{"GET./api/users", "GET", "/api/users"},
		{"POST./api/users", "POST", "/api/users"},
		{"DELETE./api/users/1", "DELETE", "/api/users/1"},
		{"get./test", "GET", "/test"},
		{"GET/invalid", "", ""}, // missing dot before slash
		{"GET.", "", ""},        // no path after dot
		{"./api", "", ""},       // no method
		{"", "", ""},            // empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			method, path := parseMethodPath(tt.input)
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestParseBasicAuthLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected *BasicAuth
	}{
		{
			name:     "no auth labels",
			labels:   map[string]string{},
			expected: nil,
		},
		{
			name: "user only (incomplete)",
			labels: map[string]string{
				"roji.auth.basic.user": "admin",
			},
			expected: nil,
		},
		{
			name: "pass only (incomplete)",
			labels: map[string]string{
				"roji.auth.basic.pass": "secret",
			},
			expected: nil,
		},
		{
			name: "user and pass",
			labels: map[string]string{
				"roji.auth.basic.user": "admin",
				"roji.auth.basic.pass": "secret",
			},
			expected: &BasicAuth{
				User:  "admin",
				Pass:  "secret",
				Realm: "Restricted",
			},
		},
		{
			name: "user, pass, and realm",
			labels: map[string]string{
				"roji.auth.basic.user":  "admin",
				"roji.auth.basic.pass":  "secret123",
				"roji.auth.basic.realm": "Admin Area",
			},
			expected: &BasicAuth{
				User:  "admin",
				Pass:  "secret123",
				Realm: "Admin Area",
			},
		},
		{
			name: "with whitespace",
			labels: map[string]string{
				"roji.auth.basic.user":  "  admin  ",
				"roji.auth.basic.pass":  "  secret  ",
				"roji.auth.basic.realm": "  My Realm  ",
			},
			expected: &BasicAuth{
				User:  "admin",
				Pass:  "secret",
				Realm: "My Realm",
			},
		},
		{
			name: "empty user",
			labels: map[string]string{
				"roji.auth.basic.user": "",
				"roji.auth.basic.pass": "secret",
			},
			expected: nil,
		},
		{
			name: "empty pass",
			labels: map[string]string{
				"roji.auth.basic.user": "admin",
				"roji.auth.basic.pass": "",
			},
			expected: nil,
		},
		{
			name: "empty realm uses default",
			labels: map[string]string{
				"roji.auth.basic.user":  "admin",
				"roji.auth.basic.pass":  "secret",
				"roji.auth.basic.realm": "",
			},
			expected: &BasicAuth{
				User:  "admin",
				Pass:  "secret",
				Realm: "Restricted",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ParseLabels(tt.labels)

			if tt.expected == nil {
				if cfg.BasicAuth != nil {
					t.Errorf("BasicAuth = %+v, want nil", cfg.BasicAuth)
				}
				return
			}

			if cfg.BasicAuth == nil {
				t.Fatal("BasicAuth = nil, want non-nil")
			}

			if cfg.BasicAuth.User != tt.expected.User {
				t.Errorf("User = %q, want %q", cfg.BasicAuth.User, tt.expected.User)
			}
			if cfg.BasicAuth.Pass != tt.expected.Pass {
				t.Errorf("Pass = %q, want %q", cfg.BasicAuth.Pass, tt.expected.Pass)
			}
			if cfg.BasicAuth.Realm != tt.expected.Realm {
				t.Errorf("Realm = %q, want %q", cfg.BasicAuth.Realm, tt.expected.Realm)
			}
		})
	}
}

func TestParseLabelsTunnel(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"one", "1", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"mixed case", "True", true},
		{"padded", "  true  ", true},
		{"false", "false", false},
		{"zero", "0", false},
		{"empty", "", false},
		// Anything unrecognized leaves the route unpublished: a label that
		// exposes a container to the internet must not turn on by accident.
		{"typo", "ture", false},
		{"other word", "enabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ParseLabels(map[string]string{"roji.tunnel": tt.value})
			if cfg.Tunnel != tt.want {
				t.Errorf("ParseLabels(roji.tunnel=%q).Tunnel = %v, want %v", tt.value, cfg.Tunnel, tt.want)
			}
		})
	}
}

func TestParseLabelsTunnelAbsent(t *testing.T) {
	cfg := ParseLabels(map[string]string{"roji.host": "api.localhost"})
	if cfg.Tunnel {
		t.Error("a container with no roji.tunnel label must not be published")
	}
}
