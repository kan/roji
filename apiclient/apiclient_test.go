package apiclient

import (
	"testing"

	"github.com/kan/roji/config"
)

func TestNewRequest(t *testing.T) {
	settings := &config.Settings{
		HTTPSPort: 8443,
		Dashboard: "panel.example.localhost",
	}

	req, err := NewRequest(settings, "/_api/routes")
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	// The connection targets localhost so that *.localhost DNS resolution is
	// never required...
	if got, want := req.URL.String(), "https://localhost:8443/_api/routes"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	// ...while the Host header selects the dashboard virtual host, which is the
	// only one serving the API.
	if got, want := req.Host, "panel.example.localhost"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
}
