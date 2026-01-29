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
	"http_port":    "int",
	"https_port":   "int",
	"certs_dir":    "string",
	"data_dir":     "string",
	"dashboard":    "string",
	"log_level":    "string",
	"auto_cert":    "bool",
	"static_sites": "array",
}

// validateTopLevel validates top-level config keys
func validateTopLevel(rawMap map[string]any, result *ValidationResult) {
	// Check for unknown keys
	for key := range rawMap {
		expectedType, ok := validTopLevelKeys[key]
		if !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    key,
				Type:    "unknown_key",
				Message: fmt.Sprintf("unknown config key '%s', valid keys: %s", key, formatValidKeys(validTopLevelKeys)),
			})
			continue
		}

		// Type validation
		value := rawMap[key]
		if !validateType(value, expectedType) {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    key,
				Type:    "invalid_type",
				Message: fmt.Sprintf("expected %s, got %T", expectedType, value),
			})
		}

		// Specific validation for static_sites
		if key == "static_sites" {
			if sites, ok := value.([]any); ok {
				validateStaticSites(sites, result)
			}
		}
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

		// Check for unknown keys
		for key := range siteMap {
			expectedType, ok := validStaticSiteKeys[key]
			if !ok {
				result.Issues = append(result.Issues, ValidationIssue{
					Path:    fmt.Sprintf("%s.%s", pathPrefix, key),
					Type:    "unknown_key",
					Message: fmt.Sprintf("unknown key '%s', valid keys: %s", key, formatValidKeys(validStaticSiteKeys)),
				})
				continue
			}

			// Type validation
			value := siteMap[key]
			if !validateType(value, expectedType) {
				result.Issues = append(result.Issues, ValidationIssue{
					Path:    fmt.Sprintf("%s.%s", pathPrefix, key),
					Type:    "invalid_type",
					Message: fmt.Sprintf("expected %s, got %T", expectedType, value),
				})
			}
		}

		// Required fields
		if _, ok := siteMap["host"]; !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    pathPrefix,
				Type:    "missing_required",
				Message: "missing required key 'host'",
			})
		}
		if _, ok := siteMap["root"]; !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    pathPrefix,
				Type:    "missing_required",
				Message: "missing required key 'root'",
			})
		}

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

	// Check for unknown basic auth keys
	for key := range basicMap {
		expectedType, ok := validBasicAuthKeys[key]
		if !ok {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    fmt.Sprintf("%s.auth.basic.%s", pathPrefix, key),
				Type:    "unknown_key",
				Message: fmt.Sprintf("unknown key '%s', valid keys: %s", key, formatValidKeys(validBasicAuthKeys)),
			})
			continue
		}

		// Type validation
		value := basicMap[key]
		if !validateType(value, expectedType) {
			result.Issues = append(result.Issues, ValidationIssue{
				Path:    fmt.Sprintf("%s.auth.basic.%s", pathPrefix, key),
				Type:    "invalid_type",
				Message: fmt.Sprintf("expected %s, got %T", expectedType, value),
			})
		}
	}

	// Required fields for basic auth (user and pass)
	if _, ok := basicMap["user"]; !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    fmt.Sprintf("%s.auth.basic", pathPrefix),
			Type:    "missing_required",
			Message: "missing required key 'user'",
		})
	}
	if _, ok := basicMap["pass"]; !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Path:    fmt.Sprintf("%s.auth.basic", pathPrefix),
			Type:    "missing_required",
			Message: "missing required key 'pass'",
		})
	}
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
