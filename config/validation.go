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
	"proxy_sites":  "array",
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

		// Specific validation for proxy_sites
		if key == "proxy_sites" {
			if sites, ok := value.([]any); ok {
				validateProxySites(sites, result)
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
	"host":   "string",
	"root":   "string",
	"index":  "bool",
	"auth":   "object",
	"tunnel": "bool",
}

// Valid proxy_site keys
var validProxySiteKeys = map[string]string{
	"host":   "string",
	"target": "string",
	"auth":   "object",
	"tunnel": "bool",
}

// validateSite runs the checks every site declared in the config file gets,
// whichever array it is in. key names the array, i is the site's index in it,
// and required is the key the site needs besides host.
//
// It returns the site and the path its issues are reported under, for the
// checks that apply to one kind of site only. ok is false for an entry that is
// not an object, which has nothing further to check.
func validateSite(key string, i int, site any, valid map[string]string, required string, result *ValidationResult) (siteMap map[string]any, pathPrefix string, ok bool) {
	siteMap, ok = site.(map[string]any)
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    fmt.Sprintf("%s[%d]", key, i),
			Type:    "invalid_type",
			Message: fmt.Sprintf("expected object, got %T", site),
		})
		return nil, "", false
	}

	// Get host for better error messages
	host, _ := siteMap["host"].(string)
	pathPrefix = fmt.Sprintf("%s[%d]", key, i)
	if host != "" {
		pathPrefix = fmt.Sprintf("%s[%d] (host: %s)", key, i, host)
	}

	validateKeys(siteMap, valid, pathPrefix, result)
	requireKeys(siteMap, []string{"host", required}, pathPrefix, result)

	// Validate auth if present
	if auth, ok := siteMap["auth"]; ok {
		validateSiteAuth(auth, pathPrefix, result)
	}

	return siteMap, pathPrefix, true
}

// validateStaticSites validates the static_sites array
func validateStaticSites(sites []any, result *ValidationResult) {
	for i, site := range sites {
		validateSite("static_sites", i, site, validStaticSiteKeys, "root", result)
	}
}

// validateProxySites validates the proxy_sites array
func validateProxySites(sites []any, result *ValidationResult) {
	for i, site := range sites {
		siteMap, pathPrefix, ok := validateSite("proxy_sites", i, site, validProxySiteKeys, "target", result)
		if !ok {
			continue
		}

		target, ok := siteMap["target"].(string)
		if !ok {
			continue // missing or mistyped, and already reported as such
		}

		u, err := ParseProxyTarget(target)
		if err != nil {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    pathPrefix + ".target",
				Type:    "invalid_value",
				Message: fmt.Sprintf("'%s' is not a usable target: %v", target, err),
			})
			continue
		}

		// Allowed, since which machines to trust is the user's call, but worth
		// saying: requests leave this machine unencrypted, and a typo here is
		// more likely than an intent to proxy across the network.
		if !IsLoopbackHost(u.Hostname()) {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    pathPrefix + ".target",
				Type:    "suspicious_value",
				Message: fmt.Sprintf("'%s' is not on this machine; requests to it are sent unencrypted over the network", target),
			})
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

// validateSiteAuth validates the auth configuration for a declared site
func validateSiteAuth(auth any, pathPrefix string, result *ValidationResult) {
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
