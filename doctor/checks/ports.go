package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// Ports checks if required ports are available
type Ports struct{}

func (c *Ports) Name() string {
	return i18n.T("doctor.check.ports")
}

func (c *Ports) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	if cfg.Settings == nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.ports.not_available"),
			Fixable: false,
		}
	}

	httpPort := cfg.Settings.HTTPPort
	httpsPort := cfg.Settings.HTTPSPort

	httpAvailable := c.isPortAvailable(httpPort)
	httpsAvailable := c.isPortAvailable(httpsPort)

	// If ports are available, pass
	if httpAvailable && httpsAvailable {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.Tf("doctor.check.ports.available", httpPort, httpsPort),
			Fixable: false,
		}
	}

	// Ports are in use - check if roji is already running
	if c.isRojiRunning(httpsPort, cfg.Settings.Dashboard) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.Tf("doctor.check.ports.roji_running", httpPort, httpsPort),
			Details: i18n.T("doctor.check.ports.roji_detected"),
			Fixable: false,
		}
	}

	// Ports are in use by something else
	var unavailable []string
	if !httpAvailable {
		unavailable = append(unavailable, fmt.Sprintf("%d", httpPort))
	}
	if !httpsAvailable {
		unavailable = append(unavailable, fmt.Sprintf("%d", httpsPort))
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Fail,
		Message: i18n.Tf("doctor.check.ports.in_use", strings.Join(unavailable, ", ")),
		Details: i18n.T("doctor.check.ports.in_use_hint"),
		Fixable: false,
	}
}

func (c *Ports) CanFix() bool {
	return false
}

func (c *Ports) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}

func (c *Ports) isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// isRojiRunning checks if roji is running on the given HTTPS port
// by calling the health check endpoint
func (c *Ports) isRojiRunning(httpsPort int, dashboardHost string) bool {
	// Create HTTP client with TLS verification disabled (self-signed cert)
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	url := fmt.Sprintf("https://localhost:%d/_api/health", httpsPort)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	// Set Host header to dashboard host (required when not in /etc/hosts)
	if dashboardHost != "" {
		req.Host = dashboardHost
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// If we get a 200 response, roji is running
	return resp.StatusCode == http.StatusOK
}
