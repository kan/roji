package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// StaticSiteAuth holds authentication configuration for a static site
type StaticSiteAuth struct {
	Basic *BasicAuth `yaml:"basic,omitempty"` // Basic authentication
}

// StaticSite represents a static file hosting configuration
type StaticSite struct {
	Host  string          `yaml:"host"`            // Hostname or subdomain (e.g., "docs" or "docs.example.com")
	Root  string          `yaml:"root"`            // Root directory path (supports ~ expansion)
	Index *bool           `yaml:"index,omitempty"` // Enable directory listing (default: true, set false to disable)
	Auth  *StaticSiteAuth `yaml:"auth,omitempty"`  // Authentication configuration
}

// IndexEnabled returns whether directory listing is enabled (default: true)
func (s *StaticSite) IndexEnabled() bool {
	if s.Index == nil {
		return true // default is enabled
	}
	return *s.Index
}

// GetBasicAuth returns the basic auth configuration if set
func (s *StaticSite) GetBasicAuth() *BasicAuth {
	if s.Auth == nil {
		return nil
	}
	return s.Auth.Basic
}

// DefaultBind is the address set roji listens on when nothing else is
// configured: both loopback addresses, and nothing beyond the machine itself.
//
// Both are needed because the wildcard listener this replaced accepted IPv4 and
// IPv6 alike, and a browser resolving a *.localhost name may pick either.
const DefaultBind = "127.0.0.1,::1"

// Settings holds all configuration settings for roji
type Settings struct {
	Network     string       `yaml:"network"`                // Docker network name(s) (comma-separated)
	Domain      string       `yaml:"domain"`                 // Base domain (e.g., dev.localhost)
	Bind        string       `yaml:"bind"`                   // Listen address(es) (comma-separated); empty means all interfaces
	HTTPPort    int          `yaml:"http_port"`              // HTTP port (for redirect)
	HTTPSPort   int          `yaml:"https_port"`             // HTTPS port
	CertsDir    string       `yaml:"certs_dir"`              // Directory for TLS certificates
	DataDir     string       `yaml:"data_dir"`               // Directory for persistent data
	Dashboard   string       `yaml:"dashboard"`              // Dashboard hostname
	LogLevel    string       `yaml:"log_level"`              // Log level (debug, info, warn, error)
	AutoCert    bool         `yaml:"auto_cert"`              // Auto-generate certificates
	StaticSites []StaticSite `yaml:"static_sites,omitempty"` // Static file hosting sites
}

// Defaults returns settings with default values
func Defaults() *Settings {
	paths := DefaultPaths()
	return &Settings{
		Network:   "roji",
		Domain:    "dev.localhost",
		Bind:      DefaultBind,
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
		settings.Dashboard = "roji." + settings.Domain
	}

	return settings, nil
}

// loadFromFile loads settings from a YAML file
func (s *Settings) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Run validation and log any warnings
	validationResult := ValidateConfigYAML(data)
	if validationResult.HasIssues() {
		validationResult.LogWarnings()
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
	if _, ok := rawMap["bind"]; ok {
		s.Bind = fileSettings.Bind
	}
	if _, ok := rawMap["http_port"]; ok {
		s.HTTPPort = fileSettings.HTTPPort
	}
	if _, ok := rawMap["https_port"]; ok {
		s.HTTPSPort = fileSettings.HTTPSPort
	}
	if _, ok := rawMap["certs_dir"]; ok {
		s.CertsDir = ExpandPath(fileSettings.CertsDir)
	}
	if _, ok := rawMap["data_dir"]; ok {
		s.DataDir = ExpandPath(fileSettings.DataDir)
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
	if _, ok := rawMap["static_sites"]; ok {
		// Expand paths for static sites
		for i := range fileSettings.StaticSites {
			fileSettings.StaticSites[i].Root = ExpandPath(fileSettings.StaticSites[i].Root)
		}
		s.StaticSites = fileSettings.StaticSites
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
	// LookupEnv rather than Getenv: an empty ROJI_BIND is a meaningful value,
	// asking for every interface rather than leaving the default alone.
	if v, ok := os.LookupEnv("ROJI_BIND"); ok {
		s.Bind = v
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
		s.CertsDir = ExpandPath(v)
	}
	if v := os.Getenv("ROJI_DATA_DIR"); v != "" {
		s.DataDir = ExpandPath(v)
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
	if v, ok := overrides["bind"].(string); ok {
		s.Bind = v
	}
	if v, ok := overrides["http_port"].(int); ok {
		s.HTTPPort = v
	}
	if v, ok := overrides["https_port"].(int); ok {
		s.HTTPSPort = v
	}
	if v, ok := overrides["certs_dir"].(string); ok {
		s.CertsDir = ExpandPath(v)
	}
	if v, ok := overrides["data_dir"].(string); ok {
		s.DataDir = ExpandPath(v)
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

// splitCSV splits a comma-separated setting into trimmed, non-empty parts.
func splitCSV(s string) []string {
	var parts []string
	for v := range strings.SplitSeq(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return parts
}

// Networks returns the network names as a slice
func (s *Settings) Networks() []string {
	networks := splitCSV(s.Network)
	if len(networks) == 0 {
		return []string{"roji"}
	}
	return networks
}

// IsWildcardAddr reports whether addr is the wildcard. The bind setting spells
// it as the empty string, which net.JoinHostPort turns into ":443" — the
// address that accepts connections on every interface.
func IsWildcardAddr(addr string) bool {
	return addr == ""
}

// IsLoopbackAddr reports whether addr names the machine roji runs on and
// nothing else.
//
// A hostname is not judged here: deciding what it points at means resolving
// it, which belongs to whoever is about to connect or listen. The wildcard is
// not loopback either, since it accepts connections from anywhere.
func IsLoopbackAddr(addr string) bool {
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}

// BindAddrs returns the addresses to listen on, one per listener.
//
// An empty setting means every interface, returned as a single wildcard entry;
// it is the only case where the list holds one, since parsing drops empty
// parts.
func (s *Settings) BindAddrs() []string {
	addrs := splitCSV(s.Bind)
	if len(addrs) == 0 {
		return []string{""}
	}
	return addrs
}

// ListensBeyondLoopback reports whether any listener accepts connections from
// outside the machine. That matters because roji publishes every container on
// its network without requiring a label, and the dashboard's Compose endpoints
// have no authentication: reaching roji is enough to control containers.
func (s *Settings) ListensBeyondLoopback() bool {
	for _, a := range s.BindAddrs() {
		if !IsLoopbackAddr(a) {
			return true
		}
	}
	return false
}

// LocalAddr returns an address this machine can reach the server on.
//
// Loopback is preferred because a connection to it never leaves the machine.
// The wildcard has no address of its own, so localhost stands in. Otherwise any
// configured address will do: the server bound it, so it names an interface
// here. Order within the setting is not meaningful, so nothing depends on it
// beyond that last case, where every candidate is equivalent.
func (s *Settings) LocalAddr() string {
	addrs := s.BindAddrs()
	for _, a := range addrs {
		if IsLoopbackAddr(a) {
			return a
		}
	}
	if IsWildcardAddr(addrs[0]) {
		return "localhost"
	}
	return addrs[0]
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

// ExpandPath expands ~ to home directory
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
