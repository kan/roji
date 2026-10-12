package checks

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// siteDialTimeout bounds the connection attempt to each proxy site's target.
// Targets are on this machine in all but unusual setups, where a refusal comes
// back at once; the timeout is for the one that points somewhere unreachable.
const siteDialTimeout = 2 * time.Second

// Sites checks the sites declared in the config file: that each static site has
// a directory to serve, that each proxy site has a server listening behind it,
// and that the hostnames they are given will work.
//
// Which sites roji serves is config.ResolveSites' answer, the same one the
// server registers routes from, so this reports what the server did rather
// than a second opinion on it.
//
// Every finding is a warning. A dev server that is not running is the ordinary
// state of a proxy site rather than a fault, and none of this stops roji from
// serving everything else.
type Sites struct{}

func (c *Sites) Name() string {
	return i18n.T("doctor.check.sites")
}

func (c *Sites) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	if cfg.Settings == nil || len(cfg.Settings.StaticSites)+len(cfg.Settings.ProxySites) == 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.T("doctor.check.sites.none"),
		}
	}

	settings := cfg.Settings
	sites := config.ResolveSites(settings.StaticSites, settings.ProxySites, settings.Domain, settings.Dashboard)

	var findings []string
	for _, site := range sites {
		if site.Err != nil {
			// roji does not serve the site, so nothing else about it matters.
			findings = append(findings, SiteSkipMessage(site.SkipReason(), site.Hostname, site.Where(), site.Err.Error()))
			continue
		}

		if !site.CertCovered {
			findings = append(findings, i18n.Tf("doctor.check.sites.cert", site.Hostname, settings.Domain))
		}
		if site.Unpublishable {
			findings = append(findings, i18n.Tf("doctor.check.sites.unpublishable", site.Hostname, settings.Domain))
		}
		if site.Target != nil && !listening(ctx, site.Target) {
			findings = append(findings, i18n.Tf("doctor.check.sites.not_listening", site.Hostname, site.Target.Host))
		}
	}

	if len(findings) == 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.Tf("doctor.check.sites.ok", len(sites)),
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Warn,
		Message: i18n.Tf("doctor.check.sites.issues", len(findings), len(sites)),
		// The report indents the first line of the details and no other.
		Details: strings.Join(findings, "\n  "),
	}
}

// SiteSkipMessage says, in the user's language, why a declared site is not
// served. reason is one of config's SiteSkip codes, where is what the site was
// to serve, and detail is the error as roji reported it.
//
// `roji config reload` uses it too, for the sites the running roji left out:
// the same finding from the same source should read the same.
func SiteSkipMessage(reason, hostname, where, detail string) string {
	switch reason {
	case config.SiteSkipReserved:
		return i18n.Tf("doctor.check.sites.reserved", hostname)
	case config.SiteSkipRoot:
		return i18n.Tf("doctor.check.sites.bad_root", hostname, where)
	case config.SiteSkipTarget:
		return i18n.Tf("doctor.check.sites.bad_target", hostname, where, detail)
	case config.SiteSkipDuplicate:
		return i18n.Tf("doctor.check.sites.duplicate", hostname, where)
	default:
		// A reason this version does not know, from a newer roji. What it
		// said is all there is to go on.
		return i18n.Tf("doctor.check.sites.skipped", hostname, detail)
	}
}

func (c *Sites) CanFix() bool {
	return false
}

func (c *Sites) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}

// listening reports whether something accepts TCP connections where target
// points. A URL without a port means port 80.
func listening(ctx context.Context, target *url.URL) bool {
	port := target.Port()
	if port == "" {
		port = "80"
	}

	dialer := net.Dialer{Timeout: siteDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target.Hostname(), port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
