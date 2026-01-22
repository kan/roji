package checks

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/kan/roji/doctor"
)

// DockerDaemon checks if the Docker daemon is running
type DockerDaemon struct{}

func (c *DockerDaemon) Name() string {
	return "Docker daemon"
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
			Message: "Docker daemon is not running",
			Details: err.Error(),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: "Docker daemon is running",
		Fixable: false,
	}
}

func (c *DockerDaemon) CanFix() bool {
	return false
}

func (c *DockerDaemon) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}
