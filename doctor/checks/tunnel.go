package checks

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/proxy"
	"github.com/kan/roji/tunnel"
)

// tunnelListTimeout bounds `cloudflared tunnel list`, which calls Cloudflare's
// API. Doctor is meant to answer quickly, and a hanging network call says
// nothing useful about the configuration.
const tunnelListTimeout = 10 * time.Second

// Tunnel checks that a configured tunnel could actually run: cloudflared
// installed, an account logged in, and the named tunnel created.
//
// None of it is fixable here. `cloudflared tunnel login` opens a browser and
// `tunnel create` writes credentials to a Cloudflare account, so both are
// deliberate steps rather than something `roji doctor --fix` should take on.
type Tunnel struct{}

func (c *Tunnel) Name() string {
	return i18n.T("doctor.check.tunnel")
}

func (c *Tunnel) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	// The tunnel is off, but not because nobody asked for it.
	if cfg.Settings != nil && cfg.Settings.Tunnel.AutoNameFailed() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.T("doctor.check.tunnel.auto_name_failed"),
			Details: i18n.T("doctor.check.tunnel.auto_name_failed_hint"),
		}
	}

	if cfg.Settings == nil || !cfg.Settings.Tunnel.Enabled() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.T("doctor.check.tunnel.not_configured"),
		}
	}

	cfgTunnel := cfg.Settings.Tunnel

	if _, err := tunnel.LookPath(); err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.tunnel.not_installed"),
			Details: i18n.T("doctor.check.tunnel.install_hint"),
		}
	}

	if !hasCloudflaredCert() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.tunnel.not_logged_in"),
			Details: i18n.T("doctor.check.tunnel.login_hint"),
		}
	}

	switch found, err := namedTunnelExists(ctx, cfgTunnel.Name); {
	case err != nil:
		// Reaching Cloudflare is not something roji requires to run, so a
		// failed query is reported without failing the check.
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.Tf("doctor.check.tunnel.list_failed", err),
		}
	case !found:
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.Tf("doctor.check.tunnel.missing", cfgTunnel.Name),
			Details: withAutoName(cfgTunnel, i18n.Tf("doctor.check.tunnel.create_hint", cfgTunnel.Name)),
		}
	}

	routes := cfgTunnel.ResolveRoutes(cfg.Settings.Domain, cfg.Settings.Dashboard)

	result := doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.Tf("doctor.check.tunnel.ready", cfgTunnel.Name, cfgTunnel.Domain),
	}

	// The one step roji cannot do or verify: the DNS record has to point at
	// the tunnel, and roji holds no Cloudflare credentials to check it.
	details := dnsHint(cfg.Settings, routes)

	// Names in tunnel.routes the server leaves out, which otherwise show up
	// only in its log.
	for _, site := range routes {
		var problem string
		switch {
		case site.Err != nil:
			problem = i18n.Tf("doctor.check.tunnel.route_reserved", site.Hostname)
		case site.Unpublishable:
			problem = i18n.Tf("doctor.check.tunnel.route_unpublishable", site.Hostname, cfg.Settings.Domain)
		default:
			continue
		}
		result.Status = doctor.Warn
		details = append(details, problem)
	}
	result.Details = withAutoName(cfgTunnel, details...)
	return result
}

// dnsHint tells how to point DNS at this machine's tunnel.
//
// With tunnel.routes, it is one record per public name. The routes are what
// this machine publishes, and a name's record points at one tunnel, so a
// wildcard would pull every name without a record of its own to whichever
// machine set it last. Without routes, doctor cannot list the public names
// (a container's label is not in the config file), and the wildcard is the
// record to make. Under `name: auto` that is likely one machine of several,
// which the hint then says.
//
// routes is tunnel.routes resolved. Alongside them the commands cover the
// declared sites with `tunnel: true`, which this machine publishes too.
// Names the server leaves out get no command; Run reports them.
func dnsHint(s *config.Settings, routes []config.DeclaredSite) []string {
	t := s.Tunnel
	if len(routes) == 0 {
		hint := []string{i18n.Tf("doctor.check.tunnel.dns_hint", t.Domain, t.Name, t.Domain)}
		if t.AutoName {
			hint = append(hint, i18n.T("doctor.check.tunnel.dns_hint_auto"))
		}
		return append(hint, i18n.T("doctor.check.tunnel.dns_no_credentials"))
	}

	sites := slices.Clone(routes)
	for _, site := range config.ResolveSites(s.StaticSites, s.ProxySites, s.Domain, s.Dashboard) {
		if (site.Static != nil && site.Static.Tunnel) || (site.Proxy != nil && site.Proxy.Tunnel) {
			sites = append(sites, site)
		}
	}

	domains := proxy.TunnelDomains{Base: s.Domain, Public: t.Domain}
	var names []string
	for _, site := range sites {
		if site.Err != nil || site.Unpublishable {
			continue
		}
		if public, ok := domains.ToPublic(site.Hostname); ok && !slices.Contains(names, public) {
			names = append(names, public)
		}
	}
	if len(names) == 0 {
		return nil
	}
	hint := []string{i18n.T("doctor.check.tunnel.dns_routes_hint")}
	for _, name := range names {
		hint = append(hint, "  cloudflared tunnel route dns --overwrite-dns "+t.Name+" "+name)
	}
	return append(hint,
		i18n.T("doctor.check.tunnel.dns_routes_labels"),
		i18n.T("doctor.check.tunnel.dns_no_credentials"))
}

// withAutoName adds to details where the tunnel's name came from when the
// config file did not write it out: `name: auto` reads the same on every
// machine, and the name it stands for does not.
func withAutoName(t *config.Tunnel, details ...string) string {
	if t.AutoName {
		details = append([]string{i18n.Tf("doctor.check.tunnel.auto_name", t.Name)}, details...)
	}
	return strings.Join(details, "\n")
}

func (c *Tunnel) CanFix() bool {
	return false
}

func (c *Tunnel) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}

// hasCloudflaredCert reports whether `cloudflared tunnel login` has been run.
// It writes the account certificate to ~/.cloudflared/cert.pem, and every other
// tunnel command needs it.
func hasCloudflaredCert() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".cloudflared", "cert.pem"))
	return err == nil
}

// namedTunnelExists reports whether a tunnel of this name exists on the
// account.
func namedTunnelExists(ctx context.Context, name string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, tunnelListTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, tunnel.Binary, "tunnel", "list", "--output", "json").Output()
	if err != nil {
		return false, err
	}

	var tunnels []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &tunnels); err != nil {
		return false, err
	}

	for _, t := range tunnels {
		if t.Name == name {
			return true, nil
		}
	}
	return false, nil
}
