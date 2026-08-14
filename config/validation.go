package config

import (
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Path    string // YAML path (e.g., "static_sites[0].auth.basic")
	Type    string // Issue type: "unknown_key", "invalid_type"
	Message string // Human-readable message
}

// ValidationResult holds the result of config validation
type ValidationResult struct {
	Issues []ValidationIssue
}

// HasIssues returns true if any validation issues were found
func (r *ValidationResult) HasIssues() bool {
	return len(r.Issues) > 0
}

// LogWarnings logs all validation issues as warnings
func (r *ValidationResult) LogWarnings() {
	for _, issue := range r.Issues {
		slog.Warn("config validation issue",
			"path", issue.Path,
			"type", issue.Type,
			"message", issue.Message)
	}
}

// FormatMessages returns all issues as formatted strings
func (r *ValidationResult) FormatMessages() []string {
	var messages []string
	for _, issue := range r.Issues {
		messages = append(messages, fmt.Sprintf("[%s] %s: %s", issue.Type, issue.Path, issue.Message))
	}
	return messages
}

// ValidateConfigYAML validates raw YAML config data
func ValidateConfigYAML(data []byte) *ValidationResult {
	result := &ValidationResult{}

	var rawMap map[string]any
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    "",
			Type:    "parse_error",
			Message: fmt.Sprintf("invalid YAML: %v", err),
		})
		return result
	}

	validateTopLevel(rawMap, result)
	return result
}

// Valid top-level keys
var validTopLevelKeys = map[string]string{
	"network":      "string",
	"domain":       "string",
	"bind":         "string",
	"http_port":    "int",
	"https_port":   "int",
	"certs_dir":    "string",
	"data_dir":     "string",
	"dashboard":    "string",
	"log_level":    "string",
	"auto_cert":    "bool",
	"static_sites": "array",
	"tunnel":       "object",
}

// validateKeys reports every unknown key and type mismatch in one object.
//
// prefix names the object in the reported path, and is empty for the top level.
func validateKeys(values map[string]any, valid map[string]string, prefix string, result *ValidationResult) {
	for key, value := range values {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		expectedType, ok := valid[key]
		if !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    path,
				Type:    "unknown_key",
				Message: fmt.Sprintf("unknown key '%s', valid keys: %s", key, formatValidKeys(valid)),
			})
			continue
		}

		if !validateType(value, expectedType) {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    path,
				Type:    "invalid_type",
				Message: fmt.Sprintf("expected %s, got %T", expectedType, value),
			})
		}
	}
}

// requireKeys reports each of keys that values does not have. They are all
// reported against path, since a missing key has no path of its own.
func requireKeys(values map[string]any, keys []string, path string, result *ValidationResult) {
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    path,
				Type:    "missing_required",
				Message: fmt.Sprintf("missing required key '%s'", key),
			})
		}
	}
}

// validateTopLevel validates top-level config keys
func validateTopLevel(rawMap map[string]any, result *ValidationResult) {
	validateKeys(rawMap, validTopLevelKeys, "", result)

	for key, value := range rawMap {
		// Specific validation for static_sites
		if key == "static_sites" {
			if sites, ok := value.([]any); ok {
				validateStaticSites(sites, result)
			}
		}

		// Specific validation for tunnel
		if key == "tunnel" {
			if tunnel, ok := value.(map[string]any); ok {
				validateTunnel(tunnel, result)
			}
		}
	}
}

// Valid tunnel keys
var validTunnelKeys = map[string]string{
	"domain":     "string",
	"name":       "string",
	"port":       "int",
	"auto_start": "bool",
}

// validateTunnel validates the tunnel configuration
func validateTunnel(tunnel map[string]any, result *ValidationResult) {
	validateKeys(tunnel, validTunnelKeys, "tunnel", result)

	// Both names are required: see Tunnel.Enabled.
	requireKeys(tunnel, []string{"domain", "name"}, "tunnel", result)

	// A two-level domain needs Cloudflare's paid certificate manager, since
	// Universal SSL covers one level of wildcard. Nothing here can tell a
	// subscriber from a non-subscriber, so this reports rather than refuses.
	if domain, ok := tunnel["domain"].(string); ok && strings.Count(domain, ".") > 1 {
		result.Issues = append(result.Issues, ValidationIssue{
			Path: "tunnel.domain",
			Type: "suspicious_value",
			Message: fmt.Sprintf("'%s' has more than one level; Cloudflare's Universal SSL covers only *.example.com, "+
				"so *.%s needs Advanced Certificate Manager", domain, domain),
		})
	}
}

// Valid static_site keys
var validStaticSiteKeys = map[string]string{
	"host":  "string",
	"root":  "string",
	"index": "bool",
	"auth":  "object",
}

// validateStaticSites validates the static_sites array
func validateStaticSites(sites []any, result *ValidationResult) {
	for i, site := range sites {
		siteMap, ok := site.(map[string]any)
		if !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    fmt.Sprintf("static_sites[%d]", i),
				Type:    "invalid_type",
				Message: fmt.Sprintf("expected object, got %T", site),
			})
			continue
		}

		// Get host for better error messages
		host, _ := siteMap["host"].(string)
		pathPrefix := fmt.Sprintf("static_sites[%d]", i)
		if host != "" {
			pathPrefix = fmt.Sprintf("static_sites[%d] (host: %s)", i, host)
		}

		validateKeys(siteMap, validStaticSiteKeys, pathPrefix, result)
		requireKeys(siteMap, []string{"host", "root"}, pathPrefix, result)

		// Validate auth if present
		if auth, ok := siteMap["auth"]; ok {
			validateStaticSiteAuth(auth, pathPrefix, result)
		}
	}
}

// Valid auth keys
var validAuthKeys = map[string]string{
	"basic": "object",
}

// Valid basic auth keys
var validBasicAuthKeys = map[string]string{
	"user":  "string",
	"pass":  "string",
	"realm": "string",
}

// validateStaticSiteAuth validates the auth configuration for a static site
func validateStaticSiteAuth(auth any, pathPrefix string, result *ValidationResult) {
	authMap, ok := auth.(map[string]any)
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    fmt.Sprintf("%s.auth", pathPrefix),
			Type:    "invalid_type",
			Message: fmt.Sprintf("expected object, got %T", auth),
		})
		return
	}

	// Check for unknown auth keys
	for key := range authMap {
		expectedType, ok := validAuthKeys[key]
		if !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    fmt.Sprintf("%s.auth.%s", pathPrefix, key),
				Type:    "unknown_key",
				Message: fmt.Sprintf("unknown auth type '%s', valid types: %s", key, formatValidKeys(validAuthKeys)),
			})
			continue
		}

		// Type validation
		value := authMap[key]
		if !validateType(value, expectedType) {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    fmt.Sprintf("%s.auth.%s", pathPrefix, key),
				Type:    "invalid_type",
				Message: fmt.Sprintf("expected %s, got %T", expectedType, value),
			})
		}
	}

	// Validate basic auth if present
	if basic, ok := authMap["basic"]; ok {
		validateBasicAuth(basic, pathPrefix, result)
	}
}

// validateBasicAuth validates basic auth configuration
func validateBasicAuth(basic any, pathPrefix string, result *ValidationResult) {
	basicMap, ok := basic.(map[string]any)
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    fmt.Sprintf("%s.auth.basic", pathPrefix),
			Type:    "invalid_type",
			Message: fmt.Sprintf("expected object, got %T", basic),
		})
		return
	}

	validateKeys(basicMap, validBasicAuthKeys, pathPrefix+".auth.basic", result)

	requireKeys(basicMap, []string{"user", "pass"}, pathPrefix+".auth.basic", result)
}

// validateType checks if a value matches the expected type
func validateType(value any, expectedType string) bool {
	if value == nil {
		return true // nil is acceptable for optional fields
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "int":
		// YAML often parses integers as int, but sometimes as int64 or float64
		switch value.(type) {
		case int, int64, float64:
			return true
		default:
			return false
		}
	case "bool":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return true
	}
}

// formatValidKeys formats a map of valid keys for display
func formatValidKeys(validKeys map[string]string) string {
	keys := make([]string, 0, len(validKeys))
	for k := range validKeys {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}
