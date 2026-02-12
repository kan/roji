package checks

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// DockerDaemon checks if the Docker daemon is running
type DockerDaemon struct{}

func (c *DockerDaemon) Name() string {
	return i18n.T("doctor.check.docker_daemon")
}

func (c *DockerDaemon) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
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

	_, err = cli.Ping(ctx)
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.docker_daemon.not_running"),
			Details: err.Error(),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.T("doctor.check.docker_daemon.running"),
		Fixable: false,
	}
}

func (c *DockerDaemon) CanFix() bool {
	return false
}

func (c *DockerDaemon) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}
