// Package test contains integration tests for roji.
//
// These tests require Docker to be running and will start/stop containers
// using the docker-compose.yml in this directory.
//
// Run with: go test -v -tags=integration ./test/...
//
//go:build integration

package test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kan/roji/docker"
	"github.com/kan/roji/proxy"
)

const (
	testNetwork    = "roji-test"
	testBaseDomain = "test.localhost"
)

// TestMain handles setup and teardown for integration tests
func TestMain(m *testing.M) {
	// Start test containers
	if err := startContainers(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start test containers: %v\n", err)
		os.Exit(1)
	}

	// Wait for containers to be ready
	time.Sleep(3 * time.Second)

	// Run tests
	code := m.Run()

	// Stop test containers
	if err := stopContainers(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop test containers: %v\n", err)
	}

	os.Exit(code)
}

func startContainers() error {
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "up", "-d")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopContainers() error {
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "down", "-v")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// setupTestHandler creates a handler with discovered backends from test containers
func setupTestHandler(t *testing.T) (*proxy.Handler, *proxy.Router, *docker.Client) {
	t.Helper()

	dockerClient, err := docker.NewClient([]string{testNetwork}, testBaseDomain)
	if err != nil {
		t.Fatalf("Failed to create docker client: %v", err)
	}

	router := proxy.NewRouter()

	// Discover backends
	ctx := context.Background()
	backends, err := dockerClient.DiscoverBackends(ctx)
	if err != nil {
		t.Fatalf("Failed to discover backends: %v", err)
	}

	for _, backend := range backends {
		router.AddBackend(backend)
	}

	statusConfig := &proxy.StatusConfig{
		Version:    "test",
		StartTime:  time.Now(),
		Networks:   []string{testNetwork},
		BaseDomain: testBaseDomain,
	}

	handler := proxy.NewHandler(router, dockerClient, "roji."+testBaseDomain, testBaseDomain, statusConfig, nil)

	return handler, router, dockerClient
}

// TestDiscoverBackends verifies that all test containers are discovered
func TestDiscoverBackends(t *testing.T) {
	dockerClient, err := docker.NewClient([]string{testNetwork}, testBaseDomain)
	if err != nil {
		t.Fatalf("Failed to create docker client: %v", err)
	}
	defer dockerClient.Close()

	ctx := context.Background()
	backends, err := dockerClient.DiscoverBackends(ctx)
	if err != nil {
		t.Fatalf("Failed to discover backends: %v", err)
	}

	// We expect 5 services: web, api, app, noport, mock
	expectedServices := map[string]bool{
		"web":    false,
		"api":    false,
		"app":    false,
		"noport": false,
		"mock":   false,
	}

	for _, backend := range backends {
		if _, ok := expectedServices[backend.ServiceName]; ok {
			expectedServices[backend.ServiceName] = true
			t.Logf("Found service: %s (hostname: %s, port: %d, warning: %s)",
				backend.ServiceName, backend.Hostname, backend.Port, backend.Warning)
		}
	}

	for service, found := range expectedServices {
		if !found {
			t.Errorf("Expected service %q not found", service)
		}
	}
}

// TestRouteDiscovery verifies routes are created correctly
func TestRouteDiscovery(t *testing.T) {
	handler, router, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()
	_ = handler // handler is set up for potential future use

	routes := router.ListRoutes()

	// Check for expected routes
	// Note: Services without roji.host label get {service}-{project}.{domain} format
	// when there are multiple services in a project
	tests := []struct {
		hostname    string
		shouldExist bool
		hasWarning  bool
	}{
		{"web-test.test.localhost", true, false},    // No custom label, uses {service}-{project}
		{"api.test.localhost", true, false},          // Has roji.host label
		{"app-test.test.localhost", true, false},     // No custom label, uses {service}-{project}
		{"noport.test.localhost", true, true},        // Has roji.host, but no port = warning
		{"mock.test.localhost", true, true},          // Has roji.host, no port = warning (but mocks work)
	}

	routeMap := make(map[string]proxy.RouteInfo)
	for _, r := range routes {
		routeMap[r.Hostname] = r
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			route, exists := routeMap[tt.hostname]
			if exists != tt.shouldExist {
				t.Errorf("Route %q exists=%v, want %v", tt.hostname, exists, tt.shouldExist)
				return
			}
			if exists && tt.hasWarning && route.Warning == "" {
				t.Errorf("Route %q should have warning but doesn't", tt.hostname)
			}
			if exists && !tt.hasWarning && route.Warning != "" {
				t.Errorf("Route %q has unexpected warning: %s", tt.hostname, route.Warning)
			}
		})
	}
}

// TestDashboard verifies the dashboard is accessible
func TestDashboard(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://roji.test.localhost/", nil)
	req.Host = "roji.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Dashboard status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "roji") {
		t.Error("Dashboard should contain 'roji'")
	}
	if !strings.Contains(body, "Dashboard") || !strings.Contains(body, "routes") {
		t.Error("Dashboard should contain route information")
	}
}

// TestRoutesAPI verifies the routes API endpoint
func TestRoutesAPI(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://roji.test.localhost/_api/routes", nil)
	req.Host = "roji.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Routes API status = %d, want %d", w.Code, http.StatusOK)
	}

	var routes []proxy.RouteInfo
	if err := json.NewDecoder(w.Body).Decode(&routes); err != nil {
		t.Fatalf("Failed to decode routes: %v", err)
	}

	if len(routes) < 5 {
		t.Errorf("Expected at least 5 routes, got %d", len(routes))
	}
}

// TestHealthEndpoint verifies the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	endpoints := []string{"/_api/health", "/healthz"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", "https://roji.test.localhost"+endpoint, nil)
			req.Host = "roji.test.localhost"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Health endpoint %s status = %d, want %d", endpoint, w.Code, http.StatusOK)
			}

			var health struct {
				Status string `json:"status"`
				Routes int    `json:"routes"`
			}
			if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
				t.Fatalf("Failed to decode health response: %v", err)
			}

			if health.Status != "healthy" {
				t.Errorf("Health status = %q, want %q", health.Status, "healthy")
			}
		})
	}
}

// TestMockRoutes verifies mock route responses
func TestMockRoutes(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "GET /api/users",
			method:     "GET",
			path:       "/api/users",
			wantStatus: 200,
			wantBody:   `[{"id":1,"name":"Test User"}]`,
		},
		{
			name:       "GET /api/health",
			method:     "GET",
			path:       "/api/health",
			wantStatus: 200,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "POST /api/users with custom status",
			method:     "POST",
			path:       "/api/users",
			wantStatus: 201,
			wantBody:   `{"created":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://mock.test.localhost"+tt.path, nil)
			req.Host = "mock.test.localhost"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}

			body := strings.TrimSpace(w.Body.String())
			if body != tt.wantBody {
				t.Errorf("Body = %q, want %q", body, tt.wantBody)
			}

			// Check X-Roji-Mock header
			if w.Header().Get("X-Roji-Mock") != "true" {
				t.Error("Expected X-Roji-Mock header to be 'true'")
			}
		})
	}
}

// TestNotFound verifies 404 handling for unknown hosts
func TestNotFound(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://unknown.test.localhost/", nil)
	req.Host = "unknown.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}

	body := w.Body.String()
	if !strings.Contains(body, "No Route Found") {
		t.Error("404 page should contain 'No Route Found'")
	}
}

// TestWarningPage verifies warning page for containers without ports
func TestWarningPage(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://noport.test.localhost/", nil)
	req.Host = "noport.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 502 Bad Gateway for containers without ports
	if w.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	body := w.Body.String()
	if !strings.Contains(body, "no port") {
		t.Error("Warning page should mention 'no port'")
	}
}

// TestProxyToNginx verifies actual proxying to nginx containers
func TestProxyToNginx(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	// Test proxying to the web service (nginx)
	// Uses {service}-{project}.{domain} format since no roji.host label
	req := httptest.NewRequest("GET", "https://web-test.test.localhost/", nil)
	req.Host = "web-test.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Nginx should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Proxy status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	// Nginx default page contains "Welcome to nginx"
	if !strings.Contains(body, "nginx") {
		t.Errorf("Expected nginx response, got: %s", body[:min(len(body), 200)])
	}
}

// TestStatusAPI verifies the status endpoint
func TestStatusAPI(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://roji.test.localhost/_api/status", nil)
	req.Host = "roji.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status API status = %d, want %d", w.Code, http.StatusOK)
	}

	var status proxy.StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode status: %v", err)
	}

	if status.Build.Version != "test" {
		t.Errorf("Version = %q, want %q", status.Build.Version, "test")
	}

	if len(status.Docker.Networks) == 0 {
		t.Error("Expected at least one network in status")
	}

	if status.Proxy.RoutesCount < 5 {
		t.Errorf("Expected at least 5 routes, got %d", status.Proxy.RoutesCount)
	}
}

// TestLogsAPI verifies the logs endpoint
func TestLogsAPI(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	// First make a request to generate a log (use correct hostname)
	req := httptest.NewRequest("GET", "https://web-test.test.localhost/", nil)
	req.Host = "web-test.test.localhost"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Now check the logs API
	req = httptest.NewRequest("GET", "https://roji.test.localhost/_api/logs", nil)
	req.Host = "roji.test.localhost"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Logs API status = %d, want %d", w.Code, http.StatusOK)
	}

	var logs []proxy.RequestLog
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("Failed to decode logs: %v", err)
	}

	// Should have at least one log from the request above
	if len(logs) < 1 {
		t.Error("Expected at least 1 log entry")
	}
}

// TestBaseDomainRedirect verifies redirect from base domain to dashboard
func TestBaseDomainRedirect(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://test.localhost/", nil)
	req.Host = "test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusFound)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "roji.test.localhost") {
		t.Errorf("Redirect location = %q, should contain roji.test.localhost", location)
	}
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestTLSCertificate tests HTTPS with self-signed certificates
// This test is more of a smoke test and may need adjustment based on cert setup
func TestTLSCertificate(t *testing.T) {
	// Skip if no certs are configured
	t.Skip("TLS certificate test requires running roji server with certs")

	// This would test against a real running server
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get("https://localhost:443/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.TLS == nil {
		t.Error("Expected TLS connection")
	}
}

// TestSSEConnection tests Server-Sent Events endpoint
func TestSSEConnection(t *testing.T) {
	handler, _, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	req := httptest.NewRequest("GET", "https://roji.test.localhost/_api/events", nil)
	req.Host = "roji.test.localhost"
	w := httptest.NewRecorder()

	// We need to handle this differently since SSE is a streaming response
	// For now, just verify headers are set correctly
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Cancel the request after a short time
	}()

	// Start the handler in a goroutine with a timeout
	done := make(chan bool)
	go func() {
		handler.ServeHTTP(w, req)
		done <- true
	}()

	// Wait a bit for headers to be set
	time.Sleep(50 * time.Millisecond)

	// Check SSE headers
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/event-stream")
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cacheControl, "no-cache")
	}
}

// TestContainerRestart tests the container restart endpoint
func TestContainerRestart(t *testing.T) {
	handler, router, dockerClient := setupTestHandler(t)
	defer dockerClient.Close()

	// Get a container ID from the routes
	routes := router.ListRoutes()
	var containerID string
	for _, r := range routes {
		if r.ContainerID != "" && r.ServiceName == "web" {
			containerID = r.ContainerID
			break
		}
	}

	if containerID == "" {
		t.Skip("No container ID found for testing restart")
	}

	// Test restart endpoint
	req := httptest.NewRequest("POST", "https://roji.test.localhost/_api/containers/"+containerID+"/restart", nil)
	req.Host = "roji.test.localhost"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body, _ := io.ReadAll(w.Body)
		t.Errorf("Restart status = %d, want %d, body: %s", w.Code, http.StatusOK, string(body))
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "restarted" {
		t.Errorf("Status = %q, want %q", result["status"], "restarted")
	}

	// Wait for container to be ready again
	time.Sleep(3 * time.Second)
}
