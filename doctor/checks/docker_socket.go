package checks

import (
	"context"
	"os"
	"runtime"

	"github.com/kan/roji/doctor"
)

// DockerSocket checks if the Docker socket is accessible
type DockerSocket struct{}

func (c *DockerSocket) Name() string {
	return "Docker socket"
}

func (c *DockerSocket) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	socketPath := c.getSocketPath()

	info, err := os.Stat(socketPath)
	if os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Docker socket not found",
			Details: "Expected socket at: " + socketPath,
			Fixable: false,
		}
	}
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Cannot access Docker socket",
			Details: err.Error(),
			Fixable: false,
		}
	}

	// Check if it's a socket (Unix) or named pipe (Windows)
	mode := info.Mode()
	if runtime.GOOS != "windows" && mode&os.ModeSocket == 0 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Path exists but is not a socket",
			Details: socketPath,
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: "Docker socket is accessible",
		Details: socketPath,
		Fixable: false,
	}
}

func (c *DockerSocket) CanFix() bool {
	return false
}

func (c *DockerSocket) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}

func (c *DockerSocket) getSocketPath() string {
	// Check for DOCKER_HOST environment variable
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}

	// Default paths
	if runtime.GOOS == "windows" {
		return `\\.\pipe\docker_engine`
	}
	return "/var/run/docker.sock"
}
