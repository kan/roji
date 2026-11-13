package checks

import (
	"testing"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
)

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
