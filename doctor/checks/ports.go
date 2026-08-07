package checks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kan/roji/apiclient"
	"github.com/kan/roji/config"
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

	binds := cfg.Settings.BindAddrs()
	httpAvailable := c.isPortAvailable(binds, httpPort)
	httpsAvailable := c.isPortAvailable(binds, httpsPort)

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
	if c.isRojiRunning(cfg.Settings) {
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

// isPortAvailable reports whether roji could start on this port, applying the
// same rule the server does when it opens its listeners: a loopback address
// that will not bind is skipped, any other address is required, and at least
// one has to succeed.
//
// Answering any other way makes doctor disagree with startup. Requiring every
// address would report "port in use" on a machine with IPv6 disabled, which
// cannot bind the default's ::1 but starts fine on 127.0.0.1; accepting any one
// success would call a port free when a configured LAN address is taken, which
// stops the server.
//
// Testing the wildcard instead, as this did before roji had a bind setting,
// fails whenever anything holds the port on an interface roji does not use.
func (c *Ports) isPortAvailable(binds []string, port int) bool {
	bound := 0
	for _, bind := range binds {
		ln, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(port)))
		if err != nil {
			if config.IsLoopbackAddr(bind) {
				continue
			}
			return false
		}
		ln.Close()
		bound++
	}
	return bound > 0
}

// isRojiRunning checks if roji is already listening on the configured HTTPS
// port by calling the health check endpoint
func (c *Ports) isRojiRunning(settings *config.Settings) bool {
	resp, err := apiclient.Get(settings, "/_api/health", 2*time.Second)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// If we get a 200 response, roji is running
	return resp.StatusCode == http.StatusOK
}
