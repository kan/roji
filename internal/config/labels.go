package config

import (
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
)

// RouteConfig holds the configuration for a single route
type RouteConfig struct {
	Host       string // e.g., "myapp.localhost"
	Port       int    // Target port
	PathPrefix string // e.g., "/api" (optional)
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

	if path, ok := labels[LabelPath]; ok {
		cfg.PathPrefix = strings.TrimSpace(path)
	}

	return cfg
}

// DefaultHostname generates a default hostname from service name and base domain
// e.g., ("myapp", "kan.localhost") -> "myapp.kan.localhost"
func DefaultHostname(serviceName, baseDomain string) string {
	return serviceName + "." + baseDomain
}
