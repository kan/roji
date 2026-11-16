package checks

import (
	"strings"
	"testing"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

func tunnelSettings(tunnel *config.Tunnel) *config.Settings {
	return &config.Settings{Domain: "dev.localhost", Dashboard: "roji.dev.localhost", Tunnel: tunnel}
}

// hintFor resolves tunnel.routes as Run does and returns the DNS hint.
func hintFor(s *config.Settings) string {
	return strings.Join(dnsHint(s, s.Tunnel.ResolveRoutes(s.Domain, s.Dashboard)), "\n")
}

func TestDNSHint_Wildcard(t *testing.T) {
	// Without routes doctor cannot tell which names are published, and the
	// wildcard is the record to make.
	hint := hintFor(tunnelSettings(&config.Tunnel{Domain: "example.com", Name: "roji"}))
	if !strings.Contains(hint, `cloudflared tunnel route dns roji "*.example.com"`) {
		t.Errorf("hint does not give the wildcard command:\n%s", hint)
	}
	if strings.Contains(hint, i18n.T("doctor.check.tunnel.dns_hint_auto")) {
		t.Errorf("hint warns about several machines for a name written out:\n%s", hint)
	}
}

func TestDNSHint_WildcardAutoName(t *testing.T) {
	// `name: auto` means one machine of several, where a wildcard set from
	// each machine in turn ends up on whichever ran it last.
	hint := hintFor(tunnelSettings(&config.Tunnel{Domain: "example.com", Name: "roji-laptop", AutoName: true}))
	if !strings.Contains(hint, `cloudflared tunnel route dns roji-laptop "*.example.com"`) {
		t.Errorf("hint does not give the wildcard command:\n%s", hint)
	}
	if !strings.Contains(hint, i18n.T("doctor.check.tunnel.dns_hint_auto")) {
		t.Errorf("hint does not point at tunnel.routes:\n%s", hint)
	}
}

func TestDNSHint_Routes(t *testing.T) {
	// With routes, one record per public name, and no wildcard: setting it
	// from this machine would pull every other machine's names here.
	hint := hintFor(tunnelSettings(&config.Tunnel{
		Domain: "example.com",
		Name:   "roji-laptop",
		Routes: []string{
			"api",
			"api.dev.localhost", // the same route written out
			"Web.Dev.Localhost",
			"roji",       // the dashboard: reported elsewhere, no command
			"other.test", // outside the base domain: no public name
		},
	}))

	want := []string{
		"cloudflared tunnel route dns --overwrite-dns roji-laptop api.example.com",
		"cloudflared tunnel route dns --overwrite-dns roji-laptop web.example.com",
	}
	for _, cmd := range want {
		if strings.Count(hint, cmd) != 1 {
			t.Errorf("hint gives %q %d times, want once:\n%s", cmd, strings.Count(hint, cmd), hint)
		}
	}
	if got := strings.Count(hint, "cloudflared tunnel route dns"); got != len(want) {
		t.Errorf("hint gives %d commands, want %d:\n%s", got, len(want), hint)
	}
	if strings.Contains(hint, "*.example.com") {
		t.Errorf("hint gives the wildcard alongside routes:\n%s", hint)
	}
}

func TestDNSHint_RoutesWithDeclaredSites(t *testing.T) {
	// A declared site with `tunnel: true` is published alongside the routes,
	// so it needs a record of its own too. One without it is not published.
	s := tunnelSettings(&config.Tunnel{Domain: "example.com", Name: "roji", Routes: []string{"api"}})
	s.ProxySites = []config.ProxySite{
		{Host: "hook", Target: "http://127.0.0.1:8787", Tunnel: true},
		{Host: "api", Target: "http://127.0.0.1:8788", Tunnel: true}, // also in routes
		{Host: "private", Target: "http://127.0.0.1:8789"},
	}
	hint := hintFor(s)

	for _, name := range []string{"api.example.com", "hook.example.com"} {
		cmd := "cloudflared tunnel route dns --overwrite-dns roji " + name
		if strings.Count(hint, cmd) != 1 {
			t.Errorf("hint gives %q %d times, want once:\n%s", cmd, strings.Count(hint, cmd), hint)
		}
	}
	if strings.Contains(hint, "private.example.com") {
		t.Errorf("hint gives a command for a site that is not published:\n%s", hint)
	}
}

func TestDNSHint_RoutesNonePublishable(t *testing.T) {
	// Routes that are all left out get no hint at all, not a wildcard: the
	// machine still means to publish by name. Run reports why they are out.
	hint := hintFor(tunnelSettings(&config.Tunnel{Domain: "example.com", Name: "roji", Routes: []string{"other.test"}}))
	if hint != "" {
		t.Errorf("hint = %q, want none", hint)
	}
}

func TestTunnel_AutoNameFailed(t *testing.T) {
	// `name: auto` on a machine whose hostname gives no name leaves the tunnel
	// off. That is not "no tunnel configured", and doctor must not say so.
	settings := &config.Settings{Tunnel: &config.Tunnel{Domain: "example.com", AutoName: true}}
	result := (&Tunnel{}).Run(t.Context(), &doctor.Config{Settings: settings})
	if result.Status != doctor.Warn {
		t.Errorf("status = %v, want warn: %s", result.Status, result.Message)
	}
}

func TestTunnel_NotConfigured(t *testing.T) {
	result := (&Tunnel{}).Run(t.Context(), &doctor.Config{Settings: &config.Settings{}})
	if result.Status != doctor.Pass {
		t.Errorf("status = %v, want pass: %s", result.Status, result.Message)
	}
}
