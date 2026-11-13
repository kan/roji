package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SiteAuth holds authentication configuration for a site declared in the
// config file.
type SiteAuth struct {
	Basic *BasicAuth `yaml:"basic,omitempty"` // Basic authentication
}

// BasicAuth returns the basic auth configuration if set
func (a *SiteAuth) BasicAuth() *BasicAuth {
	if a == nil {
		return nil
	}
	return a.Basic
}

// StaticSite represents a static file hosting configuration
type StaticSite struct {
	Host   string    `yaml:"host"`             // Hostname or subdomain (e.g., "docs" or "docs.example.com")
	Root   string    `yaml:"root"`             // Root directory path (supports ~ expansion)
	Index  *bool     `yaml:"index,omitempty"`  // Enable directory listing (default: true, set false to disable)
	Auth   *SiteAuth `yaml:"auth,omitempty"`   // Authentication configuration
	Tunnel bool      `yaml:"tunnel,omitempty"` // Publish through the tunnel (default: false)
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
	return s.Auth.BasicAuth()
}

// ProxySite gives a hostname to a server already listening on the host — a dev
// server started outside Docker, such as `vite dev` or `wrangler dev`.
type ProxySite struct {
	Host string `yaml:"host"` // Hostname or subdomain, resolved like StaticSite.Host

	// Target is the URL of the server, e.g. "http://127.0.0.1:8787". A URL is
	// the only spelling: a `port:` shorthand would leave two ways to write the
	// same site.
	Target string `yaml:"target"`

	Auth   *SiteAuth `yaml:"auth,omitempty"`   // Authentication configuration
	Tunnel bool      `yaml:"tunnel,omitempty"` // Publish through the tunnel (default: false)
}

// ParseProxyTarget parses the target of a ProxySite.
//
// The scheme is required and has to be http: roji terminates TLS itself, and a
// dev server on the same machine has no certificate a proxy could verify. A
// path is allowed and is prepended to every request. Credentials, a query and a
// fragment have no meaning for a proxy target, so they are refused rather than
// silently dropped.
func ParseProxyTarget(target string) (*url.URL, error) {
	// The usual mistake is leaving the scheme off, and what url.Parse makes of
	// "127.0.0.1:8787" or "localhost:8787" — a parse error, or a URL whose
	// scheme is "localhost" — explains nothing. Say what is wanted instead.
	//
	// Hostname rather than Host: "http://:8787" has a Host and names nothing.
	u, err := url.Parse(target)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("want a URL such as http://127.0.0.1:8787")
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("scheme %q is not supported, use http", u.Scheme)
	}
	// url.Parse checks that a port is digits and nothing more, so "localhost:"
	// and "localhost:99999" both get through it and neither can be dialed.
	if port := u.Port(); port != "" || strings.HasSuffix(u.Host, ":") {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("port %q is not between 1 and 65535", port)
		}
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("credentials, a query and a fragment are not supported")
	}
	return u, nil
}

// IsLoopbackHost reports whether host, as written in a URL, names this machine:
// a loopback address, "localhost", or a name under it. RFC 6761 reserves the
// last two for loopback, so no resolving is needed to say so.
func IsLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	return IsLoopbackAddr(host) || host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// ResolveSiteHost returns the full hostname of a site declared in the config
// file: a name without a dot is a subdomain of the base domain.
func ResolveSiteHost(host, baseDomain string) string {
	if !strings.Contains(host, ".") {
		host = host + "." + baseDomain
	}
	return strings.ToLower(host)
}

// CertCoversHost reports whether the certificate roji generates for baseDomain
// is valid for hostname: the base domain itself, or one label under it. Any
// other name is served with a certificate the browser rejects.
//
// One label only. The certificate also lists "*.*.{domain}", but a wildcard
// stands for a single leftmost label, so that entry matches nothing. A test in
// certgen holds this function to what the generated certificate accepts.
func CertCoversHost(hostname, baseDomain string) bool {
	hostname = strings.ToLower(hostname)
	baseDomain = strings.ToLower(baseDomain)
	if hostname == baseDomain {
		return true
	}
	label, ok := strings.CutSuffix(hostname, "."+baseDomain)
	return ok && label != "" && !strings.Contains(label, ".")
}

// DefaultTunnelPort is the port the tunnel listener takes when the tunnel is
// configured without one.
//
// It is deliberately not 80 or 443: the tunnel listener is a separate door into
// roji with its own guard, and telling the two apart is what makes the guard
// possible at all. See Tunnel.Port.
const DefaultTunnelPort = 8080

// Tunnel configures exposing selected routes to the internet through a
// Cloudflare Tunnel.
//
// roji runs cloudflared as a child process rather than linking it: the library
// pulls in a QUIC stack and several megabytes of binary, and roji already
// shells out to `docker compose` for the same reason.
type Tunnel struct {
	// Domain is a zone on the Cloudflare account, e.g. "example.com". A route
	// published as "web.dev.localhost" locally answers to "web.example.com".
	//
	// One level only. Cloudflare's Universal SSL covers "example.com" and
	// "*.example.com"; a two-level name like "*.tunnel.example.com" needs the
	// paid Advanced Certificate Manager.
	Domain string `yaml:"domain"`

	// Name is the named tunnel to run, created beforehand with
	// `cloudflared tunnel create`.
	//
	// TunnelNameAuto stands for a name derived from this machine's hostname
	// (AutoTunnelName), and Load replaces it with that name. A tunnel's
	// credentials live only on the machine that created it, and Cloudflare
	// does not choose between two machines running the same one, so each
	// machine needs a tunnel of its own while the config file is shared.
	Name string `yaml:"name"`

	// AutoName records that Name was derived from the hostname rather than
	// written in the file.
	AutoName bool `yaml:"-"`

	// Port is the tunnel listener's port on 127.0.0.1. cloudflared reaches roji
	// through it, and every request arriving there is treated as coming from
	// the internet.
	//
	// A separate port is the whole mechanism: cloudflared connects from
	// 127.0.0.1 like any local browser, so nothing in the request distinguishes
	// the two. The port does.
	Port int `yaml:"port,omitempty"`

	// AutoStart runs cloudflared alongside roji. With it off the listener still
	// opens and whoever wants the tunnel runs `cloudflared tunnel run` himself.
	AutoStart bool `yaml:"auto_start,omitempty"`

	// Routes publishes the routes on these hostnames without a label or a
	// `tunnel` key, in addition to those that have one. A name is written as
	// a declared site's host: without a dot it is a subdomain of the base
	// domain.
	//
	// It belongs in the per-machine file (LocalFilePath): a public name's DNS
	// record points at one tunnel, so which machine publishes a route is that
	// machine's choice, and a label in a compose file shared between machines
	// cannot make it.
	Routes []string `yaml:"routes,omitempty"`
}

// TunnelNameAuto is the value of Tunnel.Name that asks for AutoTunnelName.
const TunnelNameAuto = "auto"

// AutoTunnelName derives a tunnel name from a hostname: "roji-" and the
// hostname's first label, lowercased, with anything but letters, digits and
// hyphens replaced by a hyphen.
//
// The first label only, since whether the rest of an FQDN appears depends on
// how the machine's DNS is set up rather than on the machine.
//
// It returns an empty string for a hostname with nothing usable in its first
// label (one written in non-Latin letters, say). Falling back to a fixed name
// would give every such machine the same tunnel, which is what `name: auto`
// is for avoiding.
func AutoTunnelName(hostname string) string {
	label, _, _ := strings.Cut(strings.ToLower(hostname), ".")
	label = strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, label), "-")
	if label == "" {
		return ""
	}
	return "roji-" + label
}

// ResolveRoutes resolves each name in Routes through ResolveSiteName, so a
// hostname listed here is held to the rules a declared site's is: Err is set
// for one roji keeps for itself, and Unpublishable for one outside the base
// domain, which has no public name. Empty names are left out.
func (t *Tunnel) ResolveRoutes(baseDomain, dashboardHost string) []DeclaredSite {
	if t == nil {
		return nil
	}
	var sites []DeclaredSite
	for _, name := range t.Routes {
		if name = strings.TrimSpace(name); name != "" {
			sites = append(sites, ResolveSiteName(name, true, baseDomain, dashboardHost))
		}
	}
	return sites
}

// resolveAutoName replaces TunnelNameAuto with the name derived from this
// machine's hostname. Without a usable hostname there is no name, and the
// tunnel stays off (Name empty, AutoName set) rather than running one some
// other machine might also run.
func (t *Tunnel) resolveAutoName(hostname func() (string, error)) {
	if t == nil || t.Name != TunnelNameAuto {
		return
	}
	t.AutoName = true
	host, err := hostname()
	if err == nil {
		t.Name = AutoTunnelName(host)
	}
	if t.Name == TunnelNameAuto || t.Name == "" {
		slog.Warn("cannot derive the tunnel name from the hostname, continuing without the tunnel",
			"hostname", host, "error", err)
		t.Name = ""
	}
}

// AutoNameFailed reports whether `name: auto` was asked for and no name could
// be derived, which leaves the tunnel off.
func (t *Tunnel) AutoNameFailed() bool {
	return t != nil && t.AutoName && t.Name == ""
}

// Enabled reports whether the tunnel is configured well enough to open the
// listener. Both names are required: the domain decides which public hostnames
// resolve to which route, and the tunnel name decides what cloudflared runs.
func (t *Tunnel) Enabled() bool {
	return t != nil && t.Domain != "" && t.Name != ""
}

// ListenPort returns the port the tunnel listener binds, filling in the default
// for a tunnel configured without one.
func (t *Tunnel) ListenPort() int {
	if t == nil || t.Port == 0 {
		return DefaultTunnelPort
	}
	return t.Port
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
	ProxySites  []ProxySite  `yaml:"proxy_sites,omitempty"`  // Servers on the host given a hostname
	Tunnel      *Tunnel      `yaml:"tunnel,omitempty"`       // Cloudflare Tunnel for outside access
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
// 3. The per-machine file next to the config file (LocalFilePath)
// 4. Config file
// 5. Defaults
func Load(configPath string, cliOverrides map[string]any) (*Settings, error) {
	// Start with defaults
	settings := Defaults()

	// Try to load config file
	if configPath == "" {
		configPath = ConfigFilePath()
	}
	if err := settings.loadFromFile(configPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}
	settings.Tunnel.resolveAutoName(os.Hostname)

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

// ReadConfigFiles reads the config file and the per-machine file next to it
// (LocalFilePath), and returns them merged into one YAML document along with
// the paths of the files it read.
//
// The local file goes on top. Mappings merge key by key at every level, so it
// can set tunnel.name alone and keep tunnel.domain from the shared file;
// anything else, a list included, replaces what the config file says. A key
// set to null in the local file removes it.
//
// Either file may be missing. The error satisfies errors.Is(err,
// fs.ErrNotExist) only when both are.
func ReadConfigFiles(configPath string) ([]byte, []string, error) {
	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	var files []string

	for _, path := range []string{configPath, LocalFilePath(configPath)} {
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		files = append(files, path)

		// Nodes rather than map[string]any: a node keeps the text a scalar
		// was written as, and a map does not. Through a map, `pass: 0123`
		// comes back out as 123, and a password field reads "123".
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, nil, fmt.Errorf("invalid YAML in %s: %w", path, err)
		}
		if len(doc.Content) == 0 {
			continue // empty, or comments only
		}
		layer := doc.Content[0]
		if layer.Kind != yaml.MappingNode {
			if layer.ShortTag() == "!!null" {
				continue
			}
			return nil, nil, fmt.Errorf("invalid YAML in %s: want a mapping at the top level", path)
		}
		mergeMapping(merged, layer)
	}

	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no config file at %s: %w", configPath, fs.ErrNotExist)
	}
	data, err := yaml.Marshal(merged)
	if err != nil {
		return nil, nil, err
	}
	return data, files, nil
}

// mergeMapping lays the mapping top over the mapping base, which is modified:
// mappings on both sides merge recursively, a null in top removes the key, and
// anything else in top replaces what base has.
//
// Removing rather than keeping the null matters to loadFromFile, which takes a
// key that is present as set: `http_port: null` would otherwise mean port 0
// instead of the default, and `tunnel: null` would not turn the tunnel off. A
// mapping only top has is merged into an empty one for the same reason, so
// the nulls inside it go too.
func mergeMapping(base, top *yaml.Node) {
	for i := 0; i+1 < len(top.Content); i += 2 {
		key, value := top.Content[i], top.Content[i+1]

		at := -1
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				at = j + 1
				break
			}
		}

		switch {
		case value.ShortTag() == "!!null":
			if at >= 0 {
				base.Content = append(base.Content[:at-1], base.Content[at+1:]...)
			}
		case value.Kind == yaml.MappingNode:
			target := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if at >= 0 && base.Content[at].Kind == yaml.MappingNode {
				target = base.Content[at]
			}
			mergeMapping(target, value)
			if at >= 0 {
				base.Content[at] = target
			} else {
				base.Content = append(base.Content, key, target)
			}
		case at >= 0:
			base.Content[at] = value
		default:
			base.Content = append(base.Content, key, value)
		}
	}
}

// loadFromFile loads settings from the config file and the per-machine file
// next to it, merged by ReadConfigFiles.
func (s *Settings) loadFromFile(path string) error {
	data, files, err := ReadConfigFiles(path)
	if err != nil {
		return err
	}

	// Run validation and log any warnings. The merged document is what is
	// checked: a key required in tunnel may well be in the other file. Its
	// paths do not say which file a key came from, so name the files read.
	validationResult := ValidateConfigYAML(data)
	if validationResult.HasIssues() {
		slog.Warn("config files have validation issues", "files", files)
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
	if _, ok := rawMap["proxy_sites"]; ok {
		s.ProxySites = fileSettings.ProxySites
	}
	if _, ok := rawMap["tunnel"]; ok {
		s.Tunnel = fileSettings.Tunnel
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
