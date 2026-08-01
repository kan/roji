// Package apiclient talks to the API of a locally running roji server.
//
// Requests always connect to localhost and carry the dashboard hostname in the
// Host header. That avoids depending on *.localhost DNS resolution while still
// reaching the dashboard virtual host, which is the only one that serves the
// /_api endpoints.
package apiclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/kan/roji/config"
)

// NewClient returns an HTTP client for talking to the local roji server.
// TLS verification is disabled because roji serves a self-signed certificate.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// NewRequest builds a GET request against the local roji API endpoint at path,
// using the resolved settings for the HTTPS port and the dashboard hostname.
func NewRequest(settings *config.Settings, path string) (*http.Request, error) {
	url := fmt.Sprintf("https://localhost:%d%s", settings.HTTPSPort, path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Host = settings.Dashboard
	return req, nil
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
