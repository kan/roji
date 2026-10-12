package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/proxy"
	"github.com/spf13/cobra"
)

// useEnglish pins the language the assertions are written in.
func useEnglish(t *testing.T) {
	t.Helper()

	previous := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(previous) })
}

// pointCLIAt writes a config file that says roji listens on httpsPort, and
// makes it the one the CLI reads.
func pointCLIAt(t *testing.T, httpsPort string) {
	t.Helper()

	// Ambient ROJI_* variables would override the file. Unset rather than
	// emptied: an empty ROJI_BIND is itself a setting, meaning every interface.
	for _, key := range []string{"ROJI_DOMAIN", "ROJI_HTTPS_PORT", "ROJI_DASHBOARD", "ROJI_BIND"} {
		if value, ok := os.LookupEnv(key); ok {
			os.Unsetenv(key)
			t.Cleanup(func() { os.Setenv(key, value) })
		}
	}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, fmt.Appendf(nil, "bind: 127.0.0.1\nhttps_port: %s\n", httpsPort), 0644); err != nil {
		t.Fatal(err)
	}

	old := configFile
	configFile = configPath
	t.Cleanup(func() { configFile = old })
}

// serveAsRoji stands in for a running roji with handler answering on the
// dashboard's hostname, and points the CLI at it.
func serveAsRoji(t *testing.T, handler http.Handler) {
	t.Helper()
	useEnglish(t)

	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	pointCLIAt(t, port)
}

// reloadTestServer is serveAsRoji with the real handler, whose reload does
// what reload says.
func reloadTestServer(t *testing.T, reload proxy.ConfigReloadFunc) {
	t.Helper()

	handler := proxy.NewHandler(proxy.NewRouter(), nil, "roji.dev.localhost", "dev.localhost", nil, nil)
	handler.SetConfigReloader(reload)
	serveAsRoji(t, handler)
}

// reloadReturning is a reload that reports sites as read from /etc/roji.yaml.
func reloadReturning(sites ...proxy.ReloadedSite) proxy.ConfigReloadFunc {
	return func() (proxy.ConfigReload, error) {
		return proxy.ConfigReload{ConfigPath: "/etc/roji.yaml", Sites: sites}, nil
	}
}

// runReload runs `roji config reload` and returns what it printed.
func runReload(t *testing.T) (string, error) {
	t.Helper()

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context()) // cobra sets one when it executes a command
	cmd.SetOut(&out)
	err := runConfigReload(cmd, nil)
	return out.String(), err
}

func TestConfigReload_ReportsEverySite(t *testing.T) {
	reloadTestServer(t, reloadReturning(
		proxy.ReloadedSite{Hostname: "docs.dev.localhost", Kind: proxy.KindStatic, Target: "/var/www/docs"},
		proxy.ReloadedSite{Hostname: "far.example.com", Kind: proxy.KindLocal, Target: "http://127.0.0.1:9000", CertUncovered: true, Unpublishable: true},
		proxy.ReloadedSite{Hostname: "bad.dev.localhost", Kind: proxy.KindLocal, Target: "127.0.0.1:8787",
			Skipped: config.SiteSkipTarget, Detail: "target is not usable: want a URL"},
		proxy.ReloadedSite{Hostname: "gone.dev.localhost", Kind: proxy.KindStatic, Target: "/nonexistent", Skipped: config.SiteSkipRoot, Detail: "x"},
		proxy.ReloadedSite{Hostname: "roji.dev.localhost", Kind: proxy.KindLocal, Target: "http://127.0.0.1:1", Skipped: config.SiteSkipReserved, Detail: "x"},
		proxy.ReloadedSite{Hostname: "twice.dev.localhost", Kind: proxy.KindStatic, Target: "/var/www/old", Skipped: config.SiteSkipDuplicate, Detail: "x"},
		// A reason from a roji newer than this CLI.
		proxy.ReloadedSite{Hostname: "new.dev.localhost", Kind: proxy.KindStatic, Target: "/srv", Skipped: "quota", Detail: "over the site quota"},
	))

	out, err := runReload(t)
	// The sites that could be applied were, so a skipped one is reported and
	// is not a failure.
	if err != nil {
		t.Fatalf("runConfigReload() error = %v", err)
	}

	for _, want := range []string{
		"2 site(s) registered, 5 skipped",
		"✓ docs.dev.localhost -> /var/www/docs",
		"✓ far.example.com",
		// Judged against the running roji's domain, which is where the
		// handler above was told it is.
		"⚠ far.example.com: outside *.dev.localhost",
		"⚠ far.example.com: tunnel is set",
		"✗ bad.dev.localhost: target '127.0.0.1:8787' cannot be used (target is not usable: want a URL)",
		"✗ gone.dev.localhost: root '/nonexistent'",
		"✗ roji.dev.localhost: this hostname is the dashboard's",
		"✗ twice.dev.localhost: declared again later",
		// Said in the server's words rather than passed off as a bad target.
		"✗ new.dev.localhost: over the site quota",
		// The file is roji's, which may not be the one that was edited.
		"roji read: /etc/roji.yaml",
		// What a reload does not apply, which the command's name hides.
		"Only static_sites and proxy_sites are reloaded",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

func TestConfigReload_Summaries(t *testing.T) {
	tests := []struct {
		name  string
		sites []proxy.ReloadedSite
		want  string
	}{
		{"nothing declared", nil, "No sites are declared"},
		{"all registered", []proxy.ReloadedSite{{Hostname: "a.dev.localhost"}, {Hostname: "b.dev.localhost"}}, "2 site(s) registered."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reloadTestServer(t, reloadReturning(tt.sites...))

			out, err := runReload(t)
			if err != nil {
				t.Fatalf("runConfigReload() error = %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("output does not contain %q:\n%s", tt.want, out)
			}
		})
	}
}

// A config file roji cannot read at all changes nothing, and is a failure.
func TestConfigReload_ServerCannotReadTheFile(t *testing.T) {
	reloadTestServer(t, func() (proxy.ConfigReload, error) {
		return proxy.ConfigReload{}, errors.New("failed to load config: invalid YAML: line 3")
	})

	out, err := runReload(t)
	if err == nil || !strings.Contains(err.Error(), "invalid YAML: line 3") {
		t.Errorf("runConfigReload() error = %v, want one carrying the server's reason", err)
	}
	if strings.Contains(out, "reloaded") {
		t.Errorf("a failed reload was reported as done:\n%s", out)
	}
}

func TestConfigReload_NotRunning(t *testing.T) {
	useEnglish(t)
	pointCLIAt(t, fmt.Sprint(freePort(t)))

	if _, err := runReload(t); err == nil || !strings.Contains(err.Error(), "Nothing was reloaded") {
		t.Errorf("runConfigReload() error = %v, want it to say nothing was reloaded", err)
	}
}

// Whatever answered was not the reload endpoint: another web server on the
// port, or a roji that does not know the hostname the config file now names.
// That is not a fault in the config file, and its page is not worth printing.
func TestConfigReload_NotTheReloadEndpoint(t *testing.T) {
	page := "<html>" + strings.Repeat("some other server's page ", 200) + "</html>"
	serveAsRoji(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(page))
	}))

	_, err := runReload(t)
	if err == nil {
		t.Fatal("runConfigReload() succeeded against something that is not roji")
	}
	msg := err.Error()
	if !strings.Contains(msg, "status 404") || !strings.Contains(msg, "restart roji") {
		t.Errorf("error = %q, want the status and what to do about it", msg)
	}
	if strings.Contains(msg, "some other server") || strings.Contains(msg, "could not reload the config file") {
		t.Errorf("error = %q, want neither the page nor the blame on the config file", msg)
	}
}

// A roji from before the reload reported sites answers with the status alone.
// The command still has to say the reload happened.
func TestConfigReload_OlderServer(t *testing.T) {
	serveAsRoji(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"reloaded"}`))
	}))

	out, err := runReload(t)
	if err != nil {
		t.Fatalf("runConfigReload() error = %v", err)
	}
	if !strings.Contains(out, "Config reloaded.") || !strings.Contains(out, "Only static_sites and proxy_sites") {
		t.Errorf("output = %q, want the plain confirmation and the scope note", out)
	}
}

func TestHandler_ConfigReloadResponse(t *testing.T) {
	handler := proxy.NewHandler(proxy.NewRouter(), nil, "roji.dev.localhost", "dev.localhost", nil, nil)
	handler.SetConfigReloader(func() (proxy.ConfigReload, error) {
		return proxy.ConfigReload{ConfigPath: "/etc/roji.yaml"}, nil
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://roji.dev.localhost/_api/config/reload", nil))

	// "sites" is present and empty, which is how a client tells "nothing
	// declared" from a roji that does not report sites. "status" is what the
	// response has always carried.
	body := strings.Join(strings.Fields(rec.Body.String()), "")
	want := `{"status":"reloaded","domain":"dev.localhost","configPath":"/etc/roji.yaml","sites":[]}`
	if rec.Code != http.StatusOK || body != want {
		t.Errorf("status %d, body %s, want %s", rec.Code, body, want)
	}
}
