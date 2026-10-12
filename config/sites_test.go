package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A hostname declared twice goes to the later site that can be served. The
// earlier one is marked, so that it is reported as not served instead of being
// dropped where nobody looks.
func TestResolveSites_DuplicateHostname(t *testing.T) {
	root := t.TempDir()

	sites := ResolveSites(
		[]StaticSite{
			{Host: "docs", Root: root},
			{Host: "api", Root: root},
			{Host: "app", Root: root},
		},
		[]ProxySite{
			{Host: "docs", Target: "http://127.0.0.1:3000"},
			{Host: "DOCS.dev.localhost", Target: "http://127.0.0.1:4000"},
			// Cannot be served itself, so it supersedes nothing.
			{Host: "api", Target: "127.0.0.1:8787"},
		},
		"dev.localhost", "roji.dev.localhost",
	)

	want := []struct {
		where  string
		reason string
	}{
		{root, SiteSkipDuplicate},
		{root, ""},
		{root, ""},
		{"http://127.0.0.1:3000", SiteSkipDuplicate},
		{"http://127.0.0.1:4000", ""},
		{"127.0.0.1:8787", SiteSkipTarget},
	}
	for i, w := range want {
		if got := sites[i]; got.Where() != w.where || got.SkipReason() != w.reason {
			t.Errorf("sites[%d] (%s): Where = %q, SkipReason = %q; want %q, %q",
				i, got.Hostname, got.Where(), got.SkipReason(), w.where, w.reason)
		}
	}
}

func TestResolveSites(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}

	sites := ResolveSites(
		[]StaticSite{
			{Host: "docs", Root: root},
			{Host: "missing", Root: filepath.Join(root, "nope")},
			{Host: "notadir", Root: file},
		},
		[]ProxySite{
			{Host: "API", Target: "http://127.0.0.1:8787", Tunnel: true},
			{Host: "bad", Target: "127.0.0.1:8787"},
			{Host: "roji", Target: "http://127.0.0.1:9000"},
			{Host: "dev.localhost", Target: "http://127.0.0.1:9000"},
			{Host: "a.b.dev.localhost", Target: "http://127.0.0.1:9000", Tunnel: true},
			{Host: "hooks.example.test", Target: "http://127.0.0.1:9000", Tunnel: true},
			{Host: "plain.example.test", Target: "http://127.0.0.1:9000"},
		},
		"dev.localhost", "roji.dev.localhost",
	)

	// Static sites first, each list in the order written: the order they are
	// registered in, which decides who wins a hostname declared twice.
	tests := []struct {
		hostname      string
		err           error
		certCovered   bool
		unpublishable bool
	}{
		{"docs.dev.localhost", nil, true, false},
		{"missing.dev.localhost", ErrSiteRoot, true, false},
		{"notadir.dev.localhost", ErrSiteRoot, true, false},
		{"api.dev.localhost", nil, true, false},
		{"bad.dev.localhost", ErrSiteTarget, true, false},
		// The dashboard answers on these before any route is looked at.
		{"roji.dev.localhost", ErrSiteReservedHost, true, false},
		{"dev.localhost", ErrSiteReservedHost, true, false},
		// Under the base domain, so it has a public name, but two levels deep,
		// so the certificate does not cover it.
		{"a.b.dev.localhost", nil, false, false},
		// Outside the base domain there is no public name to publish under.
		{"hooks.example.test", nil, false, true},
		// Nothing to say about the tunnel for a site that did not ask for it.
		{"plain.example.test", nil, false, false},
	}

	if len(sites) != len(tests) {
		t.Fatalf("resolved %d sites, want %d", len(sites), len(tests))
	}
	for i, tt := range tests {
		site := sites[i]
		if site.Hostname != tt.hostname {
			t.Errorf("sites[%d].Hostname = %q, want %q", i, site.Hostname, tt.hostname)
		}
		if !errors.Is(site.Err, tt.err) || (tt.err == nil && site.Err != nil) {
			t.Errorf("%s: Err = %v, want %v", tt.hostname, site.Err, tt.err)
		}
		if site.CertCovered != tt.certCovered || site.Unpublishable != tt.unpublishable {
			t.Errorf("%s: CertCovered = %v, Unpublishable = %v; want %v, %v",
				tt.hostname, site.CertCovered, site.Unpublishable, tt.certCovered, tt.unpublishable)
		}
		if (site.Static == nil) == (site.Proxy == nil) {
			t.Errorf("%s: exactly one of Static and Proxy should be set", tt.hostname)
		}
	}

	if sites[0].Root != root {
		t.Errorf("docs Root = %q, want %q", sites[0].Root, root)
	}
	if target := sites[3].Target; target == nil || target.Host != "127.0.0.1:8787" {
		t.Errorf("api Target = %v", target)
	}
	if sites[4].Target != nil {
		t.Errorf("bad Target = %v, want nil for a target that does not parse", sites[4].Target)
	}
}
