package checks

import (
	"context"
	"net"
	"strings"

	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// DNS checks if *.localhost domains resolve correctly
type DNS struct{}

func (c *DNS) Name() string {
	return i18n.T("doctor.check.dns")
}

func (c *DNS) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	domain := "dev.localhost"
	if cfg.Settings != nil {
		domain = cfg.Settings.Domain
	}

	// Test resolving the base domain
	testHost := "test." + domain

	addrs, err := net.LookupHost(testHost)
	if err != nil {
		// Check if this is a .localhost domain
		if strings.HasSuffix(domain, ".localhost") || domain == "localhost" {
			// .localhost domains are automatically resolved to 127.0.0.1 by Chrome-based browsers
			// (Chrome, Edge, Brave, etc.) per RFC 6761, so DNS resolution failure is OK
			return doctor.CheckResult{
				Name:    c.Name(),
				Status:  doctor.Pass,
				Message: i18n.Tf("doctor.check.dns.browser_resolve", domain),
				Details: i18n.T("doctor.check.dns.browser_resolve_detail"),
				Fixable: false,
			}
		}

		// Non-.localhost domains require proper DNS configuration
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.Tf("doctor.check.dns.cannot_resolve", testHost),
			Details: i18n.T("doctor.check.dns.custom_domain_hint"),
			Fixable: false,
		}
	}

	// Check if it resolves to localhost
	isLocal := false
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && (ip.IsLoopback() || ip.String() == "127.0.0.1" || ip.String() == "::1") {
			isLocal = true
			break
		}
	}

	if !isLocal {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.Tf("doctor.check.dns.non_local", testHost),
			Details: i18n.Tf("doctor.check.dns.non_local_detail", addrs),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.Tf("doctor.check.dns.resolves", domain),
		Details: i18n.Tf("doctor.check.dns.resolves_detail", testHost, addrs),
		Fixable: false,
	}
}

func (c *DNS) CanFix() bool {
	return false
}

func (c *DNS) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}
