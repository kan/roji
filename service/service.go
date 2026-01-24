// Package service provides cross-platform service management for roji.
package service

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Manager provides service management operations
type Manager interface {
	// Install registers roji as a system service
	Install() error

	// Uninstall removes roji from system services
	Uninstall() error

	// Start starts the roji service
	Start() error

	// Stop stops the roji service
	Stop() error

	// Restart restarts the roji service
	Restart() error

	// Status returns the current service status
	Status() (*Status, error)

	// IsInstalled checks if the service is installed
	IsInstalled() bool
}

// Status represents the service status
type Status struct {
	Installed   bool
	Running     bool
	Enabled     bool   // Auto-start on boot
	Description string // Human-readable status
}

// Options for creating a service manager
type Options struct {
	User string // User to run service as (Linux only)
}

// NewManager returns a platform-specific service manager
func NewManager() (Manager, error) {
	return NewManagerWithOptions(Options{})
}

// NewManagerWithOptions returns a platform-specific service manager with options
func NewManagerWithOptions(opts Options) (Manager, error) {
	switch runtime.GOOS {
	case "linux":
		return newSystemdManagerWithOptions(ServiceOptions{User: opts.User})
	case "darwin":
		return newLaunchdManagerWithOptions(LaunchdOptions{User: opts.User})
	case "windows":
		return nil, fmt.Errorf("Windows service management not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
