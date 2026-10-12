package checks

import (
	"net"
	"strings"
	"testing"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
)

func runSites(t *testing.T, settings *config.Settings) doctor.CheckResult {
	t.Helper()
	settings.Domain = "dev.localhost"
	return (&Sites{}).Run(t.Context(), &doctor.Config{Settings: settings})
}

func TestSites_NoneDeclared(t *testing.T) {
	if result := runSites(t, &config.Settings{}); result.Status != doctor.Pass {
		t.Errorf("status = %v, want pass: %s", result.Status, result.Message)
	}
}

func TestSites_Ready(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	result := runSites(t, &config.Settings{
		StaticSites: []config.StaticSite{{Host: "docs", Root: t.TempDir()}},
		ProxySites:  []config.ProxySite{{Host: "api", Target: "http://" + ln.Addr().String()}},
	})

	if result.Status != doctor.Pass {
		t.Errorf("status = %v, want pass: %s\n%s", result.Status, result.Message, result.Details)
	}
}

func TestSites_Findings(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := ln.Addr().String()
	ln.Close()

	result := runSites(t, &config.Settings{
		StaticSites: []config.StaticSite{
			{Host: "gone", Root: "/nonexistent/roji-test"},
			{Host: "docs.example.com", Root: t.TempDir()},
		},
		ProxySites: []config.ProxySite{
			{Host: "api", Target: "http://" + closed},
			{Host: "bad", Target: "127.0.0.1:8787"},
		},
	})

	// A dev server that is not running must not fail the whole diagnosis.
	if result.Status != doctor.Warn {
		t.Errorf("status = %v, want warn", result.Status)
	}
	// Hostnames and addresses only: the wording follows the machine's language.
	for _, want := range []string{
		"gone.dev.localhost",
		"docs.example.com",
		"api.dev.localhost",
		closed,
		"bad.dev.localhost",
	} {
		if !strings.Contains(result.Details, want) {
			t.Errorf("details do not mention %q:\n%s", want, result.Details)
		}
	}
}
