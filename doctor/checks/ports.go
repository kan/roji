package checks

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/kan/roji/doctor"
)

// Ports checks if required ports are available
type Ports struct{}

func (c *Ports) Name() string {
	return "Port availability"
}

func (c *Ports) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	if cfg.Settings == nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Configuration not available",
			Fixable: false,
		}
	}

	ports := []int{cfg.Settings.HTTPPort, cfg.Settings.HTTPSPort}
	var unavailable []string
	var available []string

	for _, port := range ports {
		if c.isPortAvailable(port) {
			available = append(available, fmt.Sprintf("%d", port))
		} else {
			unavailable = append(unavailable, fmt.Sprintf("%d", port))
		}
	}

	if len(unavailable) > 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: fmt.Sprintf("Port(s) %s already in use", strings.Join(unavailable, ", ")),
			Details: "Stop any services using these ports or configure different ports",
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: fmt.Sprintf("Ports %s are available", strings.Join(available, ", ")),
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
