package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// Network checks if the required Docker network(s) exist
type Network struct{}

func (c *Network) Name() string {
	return i18n.T("doctor.check.network")
}

func (c *Network) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	if cfg.Settings == nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.network.not_available"),
			Fixable: false,
		}
	}

	networks := cfg.Settings.Networks()
	if len(networks) == 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.network.none_configured"),
			Fixable: false,
		}
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Failed to create Docker client",
			Details: err.Error(),
			Fixable: false,
		}
	}
	defer cli.Close()

	var missing []string
	var existing []string

	for _, netName := range networks {
		_, err := cli.NetworkInspect(ctx, netName, network.InspectOptions{})
		if err != nil {
			if client.IsErrNotFound(err) {
				missing = append(missing, netName)
			} else {
				return doctor.CheckResult{
					Name:    c.Name(),
					Status:  doctor.Fail,
					Message: "Failed to inspect network: " + netName,
					Details: err.Error(),
					Fixable: false,
				}
			}
		} else {
			existing = append(existing, netName)
		}
	}

	if len(missing) > 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.Tf("doctor.check.network.missing", strings.Join(missing, ", ")),
			Details: i18n.Tf("doctor.check.network.fix_hint", missing[0]),
			Fixable: true,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.Tf("doctor.check.network.exist", strings.Join(existing, ", ")),
		Fixable: false,
	}
}

func (c *Network) CanFix() bool {
	return true
}

func (c *Network) Fix(ctx context.Context, cfg *doctor.Config) error {
	if cfg.Settings == nil {
		return fmt.Errorf("configuration not available")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	networks := cfg.Settings.Networks()
	for _, netName := range networks {
		_, err := cli.NetworkInspect(ctx, netName, network.InspectOptions{})
		if err != nil && client.IsErrNotFound(err) {
			// Network doesn't exist, create it
			_, err = cli.NetworkCreate(ctx, netName, network.CreateOptions{
				Driver: "bridge",
			})
			if err != nil {
				return fmt.Errorf("failed to create network %s: %w", netName, err)
			}
		}
	}

	return nil
}
