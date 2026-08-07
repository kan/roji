// Package config provides Docker container label parsing for roji configuration.
//
// It extracts routing configuration from Docker labels such as:
//   - roji.host: Custom hostname
//   - roji.port: Target port
//   - roji.path: Path prefix for path-based routing
package config

import (
	"path"
	"strconv"
	"strings"
)

const (
	// Label prefix for all roji-related labels
	LabelPrefix = "roji."

	// Supported labels
	LabelHost = LabelPrefix + "host" // Custom hostname (default: {service}.{domain})
	LabelPort = LabelPrefix + "port" // Target port when multiple ports exposed
	LabelPath = LabelPrefix + "path" // Path prefix for routing (optional)

	// Mock labels prefix
	LabelMockPrefix       = LabelPrefix + "mock."        // roji.mock.GET./path = response body
	LabelMockStatusPrefix = LabelMockPrefix + "status."  // roji.mock.status.GET./path = status code

	// Basic auth labels
	LabelAuthBasicUser  = LabelPrefix + "auth.basic.user"  // Basic auth username
	LabelAuthBasicPass  = LabelPrefix + "auth.basic.pass"  // Basic auth password
	LabelAuthBasicRealm = LabelPrefix + "auth.basic.realm" // Basic auth realm (optional)
)

// MockRoute defines a mock response for a specific method and path
type MockRoute struct {
	Method     string // HTTP method (GET, POST, etc.)
	Path       string // URL path (e.g., "/api/users")
	Body       string // Response body
	StatusCode int    // HTTP status code (default: 200)
}

// BasicAuth holds basic authentication credentials
type BasicAuth struct {
	User  string // Username
	Pass  string // Password
	Realm string // Authentication realm (optional, default: "Restricted")
}

// RouteConfig holds the configuration for a single route
type RouteConfig struct {
	Host       string       // e.g., "myapp.localhost"
	Port       int          // Target port
	PathPrefix string       // e.g., "/api" (optional)
	MockRoutes []*MockRoute // Mock responses for this container
	BasicAuth  *BasicAuth   // Basic authentication (optional)
}

// ParseLabels extracts roji configuration from container labels
func ParseLabels(labels map[string]string) *RouteConfig {
	cfg := &RouteConfig{}

	if host, ok := labels[LabelHost]; ok {
		cfg.Host = strings.TrimSpace(host)
	}

	if portStr, ok := labels[LabelPort]; ok {
		if port, err := strconv.Atoi(strings.TrimSpace(portStr)); err == nil {
			cfg.Port = port
		}
	}

	if rawPath, ok := labels[LabelPath]; ok {
		trimmed := strings.TrimSpace(rawPath)
		// Path traversal prevention: reject if ".." is present in original input
		if strings.Contains(trimmed, "..") {
			// Dangerous path, leave PathPrefix empty (default behavior)
		} else {
			// Normalize the path. This is a URL path, so path.Clean rather
			// than filepath.Clean: the latter rewrites every slash to the
			// OS separator, turning "/api" into "\api" on Windows and
			// leaving the route unable to match any request.
			cfg.PathPrefix = path.Clean("/" + trimmed)
		}
	}

	// Parse mock labels
	cfg.MockRoutes = parseMockLabels(labels)

	// Parse basic auth labels
	cfg.BasicAuth = parseBasicAuthLabels(labels)

	return cfg
}

// parseBasicAuthLabels extracts basic authentication configuration from labels
func parseBasicAuthLabels(labels map[string]string) *BasicAuth {
	user, hasUser := labels[LabelAuthBasicUser]
	pass, hasPass := labels[LabelAuthBasicPass]

	// Both user and pass are required
	if !hasUser || !hasPass {
		return nil
	}

	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)

	// Both must be non-empty
	if user == "" || pass == "" {
		return nil
	}

	auth := &BasicAuth{
		User:  user,
		Pass:  pass,
		Realm: "Restricted", // Default realm
	}

	// Optional realm
	if realm, ok := labels[LabelAuthBasicRealm]; ok {
		realm = strings.TrimSpace(realm)
		if realm != "" {
			auth.Realm = realm
		}
	}

	return auth
}

// parseMockLabels extracts mock route configurations from labels
// Label format:
//   - roji.mock.GET./api/users = {"id": 1}  (response body)
//   - roji.mock.status.GET./api/users = 201 (status code)
func parseMockLabels(labels map[string]string) []*MockRoute {
	// Map to collect mock routes by method+path
	mockMap := make(map[string]*MockRoute)

	for key, value := range labels {
		// Check for status code labels first (roji.mock.status.METHOD.PATH)
		if strings.HasPrefix(key, LabelMockStatusPrefix) {
			rest := strings.TrimPrefix(key, LabelMockStatusPrefix)
			method, path := parseMethodPath(rest)
			if method == "" || path == "" {
				continue
			}

			mapKey := method + " " + path
			if mock, ok := mockMap[mapKey]; ok {
				if code, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
					mock.StatusCode = code
				}
			} else {
				code, _ := strconv.Atoi(strings.TrimSpace(value))
				mockMap[mapKey] = &MockRoute{
					Method:     method,
					Path:       path,
					StatusCode: code,
				}
			}
			continue
		}

		// Check for body labels (roji.mock.METHOD.PATH)
		if strings.HasPrefix(key, LabelMockPrefix) {
			rest := strings.TrimPrefix(key, LabelMockPrefix)
			// Skip if it's a status label (already handled above)
			if strings.HasPrefix(rest, "status.") {
				continue
			}

			method, path := parseMethodPath(rest)
			if method == "" || path == "" {
				continue
			}

			mapKey := method + " " + path
			if mock, ok := mockMap[mapKey]; ok {
				mock.Body = value
			} else {
				mockMap[mapKey] = &MockRoute{
					Method:     method,
					Path:       path,
					Body:       value,
					StatusCode: 200, // Default status code
				}
			}
		}
	}

	// Convert map to slice
	var mocks []*MockRoute
	for _, mock := range mockMap {
		// Set default status code if not specified
		if mock.StatusCode == 0 {
			mock.StatusCode = 200
		}
		mocks = append(mocks, mock)
	}

	return mocks
}

// parseMethodPath extracts HTTP method and path from a label suffix
// e.g., "GET./api/users" -> "GET", "/api/users"
func parseMethodPath(s string) (method, path string) {
	// Find the first dot followed by a slash (method separator)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && i+1 < len(s) && s[i+1] == '/' {
			method = strings.ToUpper(s[:i])
			// Method must not be empty
			if method == "" {
				return "", ""
			}
			path = s[i+1:]
			return
		}
	}
	return "", ""
}

// DefaultHostname generates a default hostname from service name and base domain
// e.g., ("myapp", "kan.localhost") -> "myapp.kan.localhost"
func DefaultHostname(serviceName, baseDomain string) string {
	return serviceName + "." + baseDomain
}
