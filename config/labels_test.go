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
		{"root path", "/", "/"},
		{"empty path", "", "/"},
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
