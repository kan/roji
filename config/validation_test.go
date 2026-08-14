package config

import (
	"strings"
	"testing"
)

func TestValidateConfigYAML_ValidConfig(t *testing.T) {
	yaml := `network: roji
domain: dev.localhost
http_port: 80
https_port: 443
certs_dir: ~/.local/share/roji/certs
data_dir: ~/.local/share/roji
dashboard: roji.dev.localhost
log_level: info
auto_cert: true
static_sites:
  - host: docs
    root: /tmp/docs
    index: true
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation
`
	result := ValidateConfigYAML([]byte(yaml))
	if result.HasIssues() {
		t.Errorf("expected no issues, got: %v", result.FormatMessages())
	}
}

func TestValidateConfigYAML_UnknownTopLevelKey(t *testing.T) {
	yaml := `network: roji
unknown_key: value
another_unknown: 123
`
	result := ValidateConfigYAML([]byte(yaml))
	if !result.HasIssues() {
		t.Fatal("expected issues for unknown keys")
	}

	unknownCount := 0
	for _, issue := range result.Issues {
		if issue.Type == "unknown_key" {
			unknownCount++
		}
	}
	if unknownCount != 2 {
		t.Errorf("expected 2 unknown key issues, got %d", unknownCount)
	}
}

func TestValidateConfigYAML_InvalidType(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "http_port should be int",
			yaml: `http_port: "not a number"`,
		},
		{
			name: "https_port should be int",
			yaml: `https_port: abc`,
		},
		{
			name: "auto_cert should be bool",
			yaml: `auto_cert: "yes"`,
		},
		{
			name: "static_sites should be array",
			yaml: `static_sites: not_an_array`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfigYAML([]byte(tt.yaml))
			if !result.HasIssues() {
				t.Fatal("expected issues for invalid type")
			}

			hasTypeError := false
			for _, issue := range result.Issues {
				if issue.Type == "invalid_type" {
					hasTypeError = true
					break
				}
			}
			if !hasTypeError {
				t.Errorf("expected invalid_type issue, got: %v", result.FormatMessages())
			}
		})
	}
}

func TestValidateConfigYAML_StaticSiteValidation(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectedType  string
		expectedCount int
	}{
		{
			name: "unknown static_site key",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    unknown_key: value
`,
			expectedType:  "unknown_key",
			expectedCount: 1,
		},
		{
			name: "missing required host",
			yaml: `static_sites:
  - root: /tmp
`,
			expectedType:  "missing_required",
			expectedCount: 1,
		},
		{
			name: "missing required root",
			yaml: `static_sites:
  - host: docs
`,
			expectedType:  "missing_required",
			expectedCount: 1,
		},
		{
			name: "invalid index type",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    index: "yes"
`,
			expectedType:  "invalid_type",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfigYAML([]byte(tt.yaml))
			if !result.HasIssues() {
				t.Fatal("expected issues")
			}

			count := 0
			for _, issue := range result.Issues {
				if issue.Type == tt.expectedType {
					count++
				}
			}
			if count != tt.expectedCount {
				t.Errorf("expected %d %s issues, got %d. All issues: %v",
					tt.expectedCount, tt.expectedType, count, result.FormatMessages())
			}
		})
	}
}

func TestValidateConfigYAML_AuthValidation(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectedType  string
		expectedCount int
	}{
		{
			name: "unknown auth type",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    auth:
      oauth: {}
`,
			expectedType:  "unknown_key",
			expectedCount: 1,
		},
		{
			name: "unknown basic auth key",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    auth:
      basic:
        user: admin
        pass: secret
        unknown: value
`,
			expectedType:  "unknown_key",
			expectedCount: 1,
		},
		{
			name: "missing basic auth user",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    auth:
      basic:
        pass: secret
`,
			expectedType:  "missing_required",
			expectedCount: 1,
		},
		{
			name: "missing basic auth pass",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    auth:
      basic:
        user: admin
`,
			expectedType:  "missing_required",
			expectedCount: 1,
		},
		{
			name: "invalid basic auth user type",
			yaml: `static_sites:
  - host: docs
    root: /tmp
    auth:
      basic:
        user: 123
        pass: secret
`,
			expectedType:  "invalid_type",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfigYAML([]byte(tt.yaml))
			if !result.HasIssues() {
				t.Fatal("expected issues")
			}

			count := 0
			for _, issue := range result.Issues {
				if issue.Type == tt.expectedType {
					count++
				}
			}
			if count != tt.expectedCount {
				t.Errorf("expected %d %s issues, got %d. All issues: %v",
					tt.expectedCount, tt.expectedType, count, result.FormatMessages())
			}
		})
	}
}

func TestValidateConfigYAML_InvalidYAML(t *testing.T) {
	yaml := `invalid: yaml: content: :`

	result := ValidateConfigYAML([]byte(yaml))
	if !result.HasIssues() {
		t.Fatal("expected issues for invalid YAML")
	}

	hasParseError := false
	for _, issue := range result.Issues {
		if issue.Type == "parse_error" {
			hasParseError = true
			break
		}
	}
	if !hasParseError {
		t.Errorf("expected parse_error issue, got: %v", result.FormatMessages())
	}
}

func TestValidateConfigYAML_EmptyConfig(t *testing.T) {
	yaml := ``
	result := ValidateConfigYAML([]byte(yaml))
	if result.HasIssues() {
		t.Errorf("expected no issues for empty config, got: %v", result.FormatMessages())
	}
}

func TestValidationResult_FormatMessages(t *testing.T) {
	result := &ValidationResult{
		Issues: []ValidationIssue{
			{Path: "network", Type: "unknown_key", Message: "test message"},
			{Path: "http_port", Type: "invalid_type", Message: "expected int"},
		},
	}

	messages := result.FormatMessages()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
	if messages[0] != "[unknown_key] network: test message" {
		t.Errorf("unexpected message format: %s", messages[0])
	}
}

func TestValidateConfigYAML_TunnelValidation(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantIssues []string // substrings expected in the reported issues
		wantClean  bool
	}{
		{
			name: "valid tunnel",
			yaml: `tunnel:
  domain: example.com
  name: roji
  port: 8080
  auto_start: true
`,
			wantClean: true,
		},
		{
			name: "minimal tunnel",
			yaml: `tunnel:
  domain: example.com
  name: roji
`,
			wantClean: true,
		},
		{
			name: "unknown key",
			yaml: `tunnel:
  domain: example.com
  name: roji
  hostname: web
`,
			wantIssues: []string{"tunnel.hostname"},
		},
		{
			name: "wrong types",
			yaml: `tunnel:
  domain: example.com
  name: roji
  port: "8080"
  auto_start: "yes"
`,
			wantIssues: []string{"tunnel.port", "tunnel.auto_start"},
		},
		{
			name: "missing both names",
			yaml: `tunnel:
  port: 8080
`,
			wantIssues: []string{"missing required key 'domain'", "missing required key 'name'"},
		},
		{
			name: "two-level domain needs a paid certificate",
			yaml: `tunnel:
  domain: tunnel.example.com
  name: roji
`,
			wantIssues: []string{"Advanced Certificate Manager"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfigYAML([]byte(tt.yaml))
			messages := result.FormatMessages()

			if tt.wantClean {
				if result.HasIssues() {
					t.Errorf("expected no issues, got: %v", messages)
				}
				return
			}

			for _, want := range tt.wantIssues {
				found := false
				for _, msg := range messages {
					if strings.Contains(msg, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected an issue mentioning %q, got: %v", want, messages)
				}
			}
		})
	}
}
