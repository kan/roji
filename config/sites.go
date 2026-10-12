package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Reasons roji will not serve a declared site. DeclaredSite.Err wraps one of
// them, with the detail in its message.
var (
	// ErrSiteReservedHost: the hostname is the dashboard's or the base domain,
	// which the dashboard answers before any route is looked at.
	ErrSiteReservedHost = errors.New("hostname belongs to the dashboard")
	// ErrSiteRoot: a static site's root is not a directory that can be read.
	ErrSiteRoot = errors.New("root is not a usable directory")
	// ErrSiteTarget: a proxy site's target is not a URL roji can forward to.
	ErrSiteTarget = errors.New("target is not usable")
)

// DeclaredSite is one entry of static_sites or proxy_sites with everything
// roji decides about it worked out: what it is called, whether it is served,
// and what will not work if it is.
//
// The server registers routes from this and `roji doctor` reports on it. Both
// read the same answer so that what doctor says about a site is what the
// server did with it.
type DeclaredSite struct {
	Hostname string

	// Exactly one of Static and Proxy is set, pointing at the entry as written.
	Static *StaticSite
	Proxy  *ProxySite

	Root   string   // Static: the root with ~ expanded
	Target *url.URL // Proxy: the parsed target, nil when it does not parse

	// Err is why roji does not serve the site, nil when it does.
	Err error

	// CertCovered is false when the hostname is one roji's certificate is not
	// valid for. The site is served all the same; browsers reject it.
	CertCovered bool

	// Unpublishable is true when the site asks for the tunnel and can never
	// get it: a public name is the local one with the base domain swapped for
	// the tunnel's, so a hostname outside the base domain has no public name.
	Unpublishable bool
}

// ResolveSiteName works out what follows from a site's name alone: its
// hostname, whether the certificate and the tunnel can serve it, and whether
// the name is one roji keeps for itself. What the site serves is left unset.
//
// It is the part of ResolveSites that does not need a config file, for a site
// that comes from somewhere else — `roji run` registering one over the API —
// to be held to the same rules.
func ResolveSiteName(host string, tunnel bool, baseDomain, dashboardHost string) DeclaredSite {
	baseDomain = strings.ToLower(baseDomain)

	site := DeclaredSite{Hostname: ResolveSiteHost(host, baseDomain)}
	site.CertCovered = CertCoversHost(site.Hostname, baseDomain)
	site.Unpublishable = tunnel && !strings.HasSuffix(site.Hostname, "."+baseDomain)
	if site.Hostname == strings.ToLower(dashboardHost) || site.Hostname == baseDomain {
		site.Err = fmt.Errorf("%w: %s", ErrSiteReservedHost, site.Hostname)
	}
	return site
}

// ResolveSites works out every site the config file declares, static sites
// first and in the order written, which is the order they are registered in.
func ResolveSites(staticSites []StaticSite, proxySites []ProxySite, baseDomain, dashboardHost string) []DeclaredSite {
	named := func(host string, tunnel bool) DeclaredSite {
		return ResolveSiteName(host, tunnel, baseDomain, dashboardHost)
	}

	sites := make([]DeclaredSite, 0, len(staticSites)+len(proxySites))

	for i := range staticSites {
		site := named(staticSites[i].Host, staticSites[i].Tunnel)
		site.Static = &staticSites[i]
		site.Root = ExpandPath(staticSites[i].Root)
		if site.Err == nil {
			site.Err = checkSiteRoot(site.Root)
		}
		sites = append(sites, site)
	}

	for i := range proxySites {
		site := named(proxySites[i].Host, proxySites[i].Tunnel)
		site.Proxy = &proxySites[i]

		// Whether anything listens on the target is not part of this: a dev
		// server that is not running yet is the normal state, and the route has
		// to exist for the page that says so to be served.
		target, err := ParseProxyTarget(proxySites[i].Target)
		if err != nil {
			if site.Err == nil {
				site.Err = fmt.Errorf("%w: %v", ErrSiteTarget, err)
			}
		} else {
			site.Target = target
		}
		sites = append(sites, site)
	}

	return sites
}

// checkSiteRoot reports why root cannot be served, or nil when it can.
func checkSiteRoot(root string) error {
	info, err := os.Stat(root)
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("%w: %s does not exist", ErrSiteRoot, root)
	case err != nil:
		return fmt.Errorf("%w: %v", ErrSiteRoot, err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s is not a directory", ErrSiteRoot, root)
	}
	return nil
}
