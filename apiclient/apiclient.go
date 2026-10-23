// Package apiclient talks to the API of a locally running roji server.
//
// Requests connect to an address the server listens on and carry the dashboard
// hostname in the Host header. That avoids depending on *.localhost DNS
// resolution while still reaching the dashboard virtual host, which is the only
// one that serves the /_api endpoints.
package apiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/kan/roji/config"
)

// NewClient returns an HTTP client for talking to the local roji server.
// TLS verification is disabled because roji serves a self-signed certificate.
//
// Verifying against roji's own CA was considered and left out. The connection
// is to an address on this machine (loopback unless bind says otherwise) and
// carries no secrets, while verification would break the CLI whenever the
// certificate does not cover the dashboard hostname, has expired, or the CA is
// not readable by the user running the CLI.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// NewRequest builds a GET request against the local roji API endpoint at path,
// using the resolved settings for the listen address and the dashboard hostname.
func NewRequest(settings *config.Settings, path string) (*http.Request, error) {
	return newRequest(context.Background(), settings, http.MethodGet, path, nil)
}

func newRequest(ctx context.Context, settings *config.Settings, method, path string, body io.Reader) (*http.Request, error) {
	// LocalAddr rather than localhost: a server configured not to listen on
	// loopback is unreachable there.
	addr := net.JoinHostPort(settings.LocalAddr(), strconv.Itoa(settings.HTTPSPort))
	url := "https://" + addr + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Host = settings.Dashboard
	return req, nil
}

// SendJSON sends a request with an optional JSON body to the local roji API
// endpoint at path, through client. A nil payload sends no body. The caller is
// responsible for closing the response body.
//
// The client is the caller's so that one making many requests — `roji run`
// renewing its route — can keep a connection open instead of making a new
// one, TLS handshake included, every time.
func SendJSON(ctx context.Context, client *http.Client, settings *config.Settings, method, path string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := newRequest(ctx, settings, method, path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

// Get sends a GET request to the local roji API endpoint at path.
// The caller is responsible for closing the response body.
func Get(settings *config.Settings, path string, timeout time.Duration) (*http.Response, error) {
	req, err := NewRequest(settings, path)
	if err != nil {
		return nil, err
	}
	return NewClient(timeout).Do(req)
}
