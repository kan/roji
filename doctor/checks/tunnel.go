package checks

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
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
			Details: i18n.Tf("doctor.check.tunnel.create_hint", cfgTunnel.Name),
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.Tf("doctor.check.tunnel.ready", cfgTunnel.Name, cfgTunnel.Domain),
		// The one step roji cannot do or verify: the DNS record has to point
		// at the tunnel, and roji holds no Cloudflare credentials to check it.
		Details: i18n.Tf("doctor.check.tunnel.dns_hint", cfgTunnel.Domain, cfgTunnel.Name, cfgTunnel.Domain),
	}
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
