package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Settings holds all configuration settings for roji
type Settings struct {
	Network   string `yaml:"network"`    // Docker network name(s) (comma-separated)
	Domain    string `yaml:"domain"`     // Base domain (e.g., dev.localhost)
	HTTPPort  int    `yaml:"http_port"`  // HTTP port (for redirect)
	HTTPSPort int    `yaml:"https_port"` // HTTPS port
	CertsDir  string `yaml:"certs_dir"`  // Directory for TLS certificates
	DataDir   string `yaml:"data_dir"`   // Directory for persistent data
	Dashboard string `yaml:"dashboard"`  // Dashboard hostname
	LogLevel  string `yaml:"log_level"`  // Log level (debug, info, warn, error)
	AutoCert  bool   `yaml:"auto_cert"`  // Auto-generate certificates
}

// Defaults returns settings with default values
func Defaults() *Settings {
	paths := DefaultPaths()
	return &Settings{
		Network:   "roji",
		Domain:    "dev.localhost",
		HTTPPort:  80,
		HTTPSPort: 443,
		CertsDir:  paths.CertsDir,
		DataDir:   paths.DataDir,
		Dashboard: "",
		LogLevel:  "info",
		AutoCert:  true,
	}
}

// Load loads configuration with the following priority (highest to lowest):
// 1. CLI overrides (passed in cliOverrides)
// 2. Environment variables (ROJI_*)
// 3. Config file
// 4. Defaults
func Load(configPath string, cliOverrides map[string]any) (*Settings, error) {
	// Start with defaults
	settings := Defaults()

	// Try to load config file
	if configPath == "" {
		configPath = ConfigFilePath()
	}
	if err := settings.loadFromFile(configPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Apply environment variables
	settings.applyEnvVars()

	// Apply CLI overrides (highest priority)
	settings.applyOverrides(cliOverrides)

	// Set dashboard default if not specified
	if settings.Dashboard == "" {
		settings.Dashboard = settings.Domain
	}

	return settings, nil
}

// loadFromFile loads settings from a YAML file
func (s *Settings) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse YAML into a temporary struct to preserve defaults for unset fields
	var fileSettings Settings
	if err := yaml.Unmarshal(data, &fileSettings); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Only override fields that are explicitly set in the file
	// We need to re-parse to check which fields were actually present
	var rawMap map[string]any
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	if _, ok := rawMap["network"]; ok {
		s.Network = fileSettings.Network
	}
	if _, ok := rawMap["domain"]; ok {
		s.Domain = fileSettings.Domain
	}
	if _, ok := rawMap["http_port"]; ok {
		s.HTTPPort = fileSettings.HTTPPort
	}
	if _, ok := rawMap["https_port"]; ok {
		s.HTTPSPort = fileSettings.HTTPSPort
	}
	if _, ok := rawMap["certs_dir"]; ok {
		s.CertsDir = expandPath(fileSettings.CertsDir)
	}
	if _, ok := rawMap["data_dir"]; ok {
		s.DataDir = expandPath(fileSettings.DataDir)
	}
	if _, ok := rawMap["dashboard"]; ok {
		s.Dashboard = fileSettings.Dashboard
	}
	if _, ok := rawMap["log_level"]; ok {
		s.LogLevel = fileSettings.LogLevel
	}
	if _, ok := rawMap["auto_cert"]; ok {
		s.AutoCert = fileSettings.AutoCert
	}

	return nil
}

// applyEnvVars applies environment variables to settings
func (s *Settings) applyEnvVars() {
	if v := os.Getenv("ROJI_NETWORK"); v != "" {
		s.Network = v
	}
	if v := os.Getenv("ROJI_DOMAIN"); v != "" {
		s.Domain = v
	}
	if v := os.Getenv("ROJI_HTTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			s.HTTPPort = port
		}
	}
	if v := os.Getenv("ROJI_HTTPS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			s.HTTPSPort = port
		}
	}
	if v := os.Getenv("ROJI_CERTS_DIR"); v != "" {
		s.CertsDir = expandPath(v)
	}
	if v := os.Getenv("ROJI_DATA_DIR"); v != "" {
		s.DataDir = expandPath(v)
	}
	if v := os.Getenv("ROJI_DASHBOARD"); v != "" {
		s.Dashboard = v
	}
	if v := os.Getenv("ROJI_LOG_LEVEL"); v != "" {
		s.LogLevel = v
	}
	if v := os.Getenv("ROJI_AUTO_CERT"); v != "" {
		s.AutoCert = v == "true" || v == "1"
	}
}

// applyOverrides applies CLI overrides (highest priority)
func (s *Settings) applyOverrides(overrides map[string]any) {
	if overrides == nil {
		return
	}

	if v, ok := overrides["network"].(string); ok {
		s.Network = v
	}
	if v, ok := overrides["domain"].(string); ok {
		s.Domain = v
	}
	if v, ok := overrides["http_port"].(int); ok {
		s.HTTPPort = v
	}
	if v, ok := overrides["https_port"].(int); ok {
		s.HTTPSPort = v
	}
	if v, ok := overrides["certs_dir"].(string); ok {
		s.CertsDir = expandPath(v)
	}
	if v, ok := overrides["data_dir"].(string); ok {
		s.DataDir = expandPath(v)
	}
	if v, ok := overrides["dashboard"].(string); ok {
		s.Dashboard = v
	}
	if v, ok := overrides["log_level"].(string); ok {
		s.LogLevel = v
	}
	if v, ok := overrides["auto_cert"].(bool); ok {
		s.AutoCert = v
	}
}

// Networks returns the network names as a slice
func (s *Settings) Networks() []string {
	var networks []string
	for _, n := range strings.Split(s.Network, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			networks = append(networks, n)
		}
	}
	if len(networks) == 0 {
		networks = []string{"roji"}
	}
	return networks
}

// ToYAML returns the settings as YAML string
func (s *Settings) ToYAML() (string, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveToFile saves settings to a YAML file
func (s *Settings) SaveToFile(path string) error {
	// Ensure the parent directory exists
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Add header comment
	header := "# roji configuration file\n# See: https://github.com/kan/roji\n\n"
	content := header + string(data)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
