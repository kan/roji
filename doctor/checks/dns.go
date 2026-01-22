package checks

import (
	"context"
	"fmt"
	"net"

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
		// .localhost domains should resolve to 127.0.0.1 on most systems
		// If they don't, it might still work if we're using /etc/hosts
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: fmt.Sprintf("Cannot resolve %s", testHost),
			Details: "*.localhost domains should resolve to 127.0.0.1.\nThis is usually automatic on modern systems.\nIf using a custom domain, ensure DNS is configured.",
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
