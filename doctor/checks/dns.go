package checks

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/kan/roji/doctor"
)

// DNS checks if *.localhost domains resolve correctly
type DNS struct{}

func (c *DNS) Name() string {
	return "DNS resolution"
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
				Message: fmt.Sprintf("*.%s (browser auto-resolve)", domain),
				Details: "Chrome-based browsers automatically resolve *.localhost to 127.0.0.1.\nNote: curl and other CLI tools may require /etc/hosts entry.",
				Fixable: false,
			}
		}

		// Non-.localhost domains require proper DNS configuration
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: fmt.Sprintf("Cannot resolve %s", testHost),
			Details: "Custom domains require DNS configuration.\nAdd entries to /etc/hosts or configure a local DNS server.",
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
			Message: fmt.Sprintf("%s resolves to non-local address", testHost),
			Details: fmt.Sprintf("Resolved to: %v\nExpected: 127.0.0.1 or ::1", addrs),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: fmt.Sprintf("*.%s resolves to localhost", domain),
		Details: fmt.Sprintf("Tested: %s -> %v", testHost, addrs),
		Fixable: false,
	}
}

func (c *DNS) CanFix() bool {
	return false
}

func (c *DNS) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}
