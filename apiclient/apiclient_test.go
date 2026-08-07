package apiclient

import (
	"testing"

	"github.com/kan/roji/config"
)

// TestNewRequest checks the two things the request has to get right: an address
// the server actually listens on, so *.localhost DNS resolution is never
// required, and the dashboard hostname in the Host header, since that virtual
// host is the only one serving the API.
func TestNewRequest(t *testing.T) {
	tests := []struct {
		name string
		bind string
		want string
	}{
		// Unset means the wildcard, which has no address of its own to dial.
		{"wildcard falls back to localhost", "", "https://localhost:8443/_api/routes"},
		{"loopback among several", "192.168.1.5,::1", "https://[::1]:8443/_api/routes"},
		// An IPv6 literal needs brackets in a URL authority.
		{"ipv6 literal", "::1", "https://[::1]:8443/_api/routes"},
		// A server that does not listen on loopback cannot be reached at
		// localhost, which is what this address is for.
		{"non-loopback", "192.168.1.5", "https://192.168.1.5:8443/_api/routes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &config.Settings{
				Bind:      tt.bind,
				HTTPSPort: 8443,
				Dashboard: "panel.example.localhost",
			}

			req, err := NewRequest(settings, "/_api/routes")
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			if got := req.URL.String(); got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
			// The Host header still selects the dashboard virtual host.
			if got, want := req.Host, "panel.example.localhost"; got != want {
				t.Errorf("Host = %q, want %q", got, want)
			}
		})
	}
}
