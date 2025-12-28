package proxy

import (
	"testing"

	"github.com/kan/roji/internal/docker"
)

func TestRouter_AddAndLookup(t *testing.T) {
	router := NewRouter()

	backend := &docker.Backend{
		ContainerID:   "abc123",
		ContainerName: "web",
		ServiceName:   "web",
		Host:          "172.17.0.2",
		Port:          80,
		Hostname:      "web.localhost",
	}

	router.AddBackend(backend)

	// Test lookup
	route := router.Lookup("web.localhost", "/")
	if route == nil {
		t.Fatal("expected route, got nil")
	}
	if route.Backend.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", route.Backend.ContainerID, "abc123")
	}

	// Test case-insensitive lookup
	route = router.Lookup("WEB.LOCALHOST", "/")
	if route == nil {
		t.Fatal("expected route for uppercase hostname, got nil")
	}

	// Test non-existent route
	route = router.Lookup("api.localhost", "/")
	if route != nil {
		t.Errorf("expected nil for non-existent route, got %v", route)
	}
}

func TestRouter_PathBasedRouting(t *testing.T) {
	router := NewRouter()

	// Add path-based routes
	apiBackend := &docker.Backend{
		ContainerID:   "api123",
		ContainerName: "api",
		ServiceName:   "api",
		Host:          "172.17.0.2",
		Port:          8080,
		Hostname:      "app.localhost",
		PathPrefix:    "/api",
	}

	apiV2Backend := &docker.Backend{
		ContainerID:   "apiv2",
		ContainerName: "api-v2",
		ServiceName:   "api-v2",
		Host:          "172.17.0.3",
		Port:          8080,
		Hostname:      "app.localhost",
		PathPrefix:    "/api/v2",
	}

	webBackend := &docker.Backend{
		ContainerID:   "web123",
		ContainerName: "web",
		ServiceName:   "web",
		Host:          "172.17.0.4",
		Port:          80,
		Hostname:      "app.localhost",
	}

	router.AddBackend(apiBackend)
	router.AddBackend(apiV2Backend)
	router.AddBackend(webBackend)

	tests := []struct {
		path        string
		expectedID  string
		description string
	}{
		{"/api/v2/users", "apiv2", "longest path match"},
		{"/api/users", "api123", "shorter path match"},
		{"/", "web123", "fallback to hostname route"},
		{"/other", "web123", "no path match, fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			route := router.Lookup("app.localhost", tt.path)
			if route == nil {
				t.Fatalf("expected route for path %q, got nil", tt.path)
			}
			if route.Backend.ContainerID != tt.expectedID {
				t.Errorf("path %q: ContainerID = %q, want %q",
					tt.path, route.Backend.ContainerID, tt.expectedID)
			}
		})
	}
}

func TestRouter_RemoveBackend(t *testing.T) {
	router := NewRouter()

	backend := &docker.Backend{
		ContainerID:   "abc123",
		ContainerName: "web",
		ServiceName:   "web",
		Host:          "172.17.0.2",
		Port:          80,
		Hostname:      "web.localhost",
	}

	router.AddBackend(backend)

	// Verify it exists
	if route := router.Lookup("web.localhost", "/"); route == nil {
		t.Fatal("expected route before removal")
	}

	// Remove it
	router.RemoveBackend("abc123")

	// Verify it's gone
	if route := router.Lookup("web.localhost", "/"); route != nil {
		t.Error("expected route to be removed")
	}
}

func TestRouter_RemoveProject(t *testing.T) {
	router := NewRouter()

	// Add multiple backends from same project
	backends := []*docker.Backend{
		{
			ContainerID:   "web1",
			ContainerName: "myproject-web-1",
			ServiceName:   "web",
			ProjectName:   "myproject",
			Host:          "172.17.0.2",
			Port:          80,
			Hostname:      "web.myproject.localhost",
		},
		{
			ContainerID:   "api1",
			ContainerName: "myproject-api-1",
			ServiceName:   "api",
			ProjectName:   "myproject",
			Host:          "172.17.0.3",
			Port:          8080,
			Hostname:      "api.myproject.localhost",
		},
		{
			ContainerID:   "other1",
			ContainerName: "other-web-1",
			ServiceName:   "web",
			ProjectName:   "other",
			Host:          "172.17.0.4",
			Port:          80,
			Hostname:      "web.other.localhost",
		},
	}

	for _, b := range backends {
		router.AddBackend(b)
	}

	// Remove myproject
	router.RemoveProject("myproject")

	// Verify myproject routes are gone
	if route := router.Lookup("web.myproject.localhost", "/"); route != nil {
		t.Error("expected web.myproject.localhost to be removed")
	}
	if route := router.Lookup("api.myproject.localhost", "/"); route != nil {
		t.Error("expected api.myproject.localhost to be removed")
	}

	// Verify other project is still there
	if route := router.Lookup("web.other.localhost", "/"); route == nil {
		t.Error("expected web.other.localhost to still exist")
	}
}

func TestRouter_ListRoutes(t *testing.T) {
	router := NewRouter()

	backends := []*docker.Backend{
		{
			ContainerID:   "web1",
			ContainerName: "web",
			ServiceName:   "web",
			Host:          "172.17.0.2",
			Port:          80,
			Hostname:      "web.localhost",
		},
		{
			ContainerID:   "api1",
			ContainerName: "api",
			ServiceName:   "api",
			Host:          "172.17.0.3",
			Port:          8080,
			Hostname:      "api.localhost",
		},
	}

	for _, b := range backends {
		router.AddBackend(b)
	}

	routes := router.ListRoutes()

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	// Should be sorted by hostname
	if routes[0].Hostname != "api.localhost" {
		t.Errorf("first route hostname = %q, want %q", routes[0].Hostname, "api.localhost")
	}
	if routes[1].Hostname != "web.localhost" {
		t.Errorf("second route hostname = %q, want %q", routes[1].Hostname, "web.localhost")
	}
}

func TestRouteInfo_String(t *testing.T) {
	tests := []struct {
		info     RouteInfo
		expected string
	}{
		{
			info: RouteInfo{
				Hostname:    "api.localhost",
				Target:      "172.17.0.2:8080",
				ServiceName: "api",
			},
			expected: "https://api.localhost/ -> 172.17.0.2:8080 (api)",
		},
		{
			info: RouteInfo{
				Hostname:    "app.localhost",
				PathPrefix:  "/api",
				Target:      "172.17.0.3:3000",
				ServiceName: "backend",
			},
			expected: "https://app.localhost/api -> 172.17.0.3:3000 (backend)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.info.Hostname, func(t *testing.T) {
			result := tt.info.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}
