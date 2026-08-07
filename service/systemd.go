//go:build linux

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	systemdServicePath = "/etc/systemd/system/roji.service"
	systemdServiceName = "roji"
)

// systemd service unit template
const systemdTemplate = `[Unit]
Description=roji - reverse proxy for local development
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User={{.User}}
Group={{.Group}}
ExecStart={{.ExecPath}}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=roji

# Allow binding to ports 80/443
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
`

type systemdManager struct {
	execPath string
	user     string
	group    string
}

// ServiceOptions contains options for service installation
type ServiceOptions struct {
	User string // User to run service as (empty = auto-detect)
}

// commandExists checks if a command is available in PATH.
//
// Lives here rather than in service.go because systemd is its only caller, and
// service.go is built on every platform.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func newSystemdManagerWithOptions(opts ServiceOptions) (*systemdManager, error) {
	if !commandExists("systemctl") {
		return nil, fmt.Errorf("systemctl not found - systemd is required")
	}

	// Find the roji executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine user
	user := opts.User
	if user == "" {
		// Try to auto-detect
		user = os.Getenv("SUDO_USER")
		if user == "" {
			user = os.Getenv("USER")
		}
	}

	// Allow root if explicitly specified or if running as root
	if user == "" {
		return nil, fmt.Errorf("cannot determine user - specify with --user flag")
	}

	// Get user's primary group (same as username on most systems)
	group := user

	return &systemdManager{
		execPath: execPath,
		user:     user,
		group:    group,
	}, nil
}

func (m *systemdManager) Install() error {
	if m.IsInstalled() {
		return fmt.Errorf("service is already installed")
	}

	// Generate service file content
	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"ExecPath": m.execPath,
		"User":     m.user,
		"Group":    m.group,
	})
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write service file (requires root)
	if err := os.WriteFile(systemdServicePath, buf.Bytes(), 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied - run with sudo")
		}
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd daemon
	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service (auto-start on boot)
	if err := runSystemctl("enable", systemdServiceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func (m *systemdManager) Uninstall() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	// Stop service if running
	_ = runSystemctl("stop", systemdServiceName)

	// Disable service
	if err := runSystemctl("disable", systemdServiceName); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	// Remove service file
	if err := os.Remove(systemdServicePath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied - run with sudo")
		}
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func (m *systemdManager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed - run 'roji service install' first")
	}

	if err := runSystemctl("start", systemdServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (m *systemdManager) Stop() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	if err := runSystemctl("stop", systemdServiceName); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

func (m *systemdManager) Restart() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	if err := runSystemctl("restart", systemdServiceName); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	return nil
}

func (m *systemdManager) Status() (*Status, error) {
	status := &Status{
		Installed: m.IsInstalled(),
	}

	if !status.Installed {
		status.Description = "not installed"
		return status, nil
	}

	// Check if enabled
	err := runSystemctl("is-enabled", "--quiet", systemdServiceName)
	status.Enabled = err == nil

	// Check if running
	err = runSystemctl("is-active", "--quiet", systemdServiceName)
	status.Running = err == nil

	// Get detailed status
	output, _ := exec.Command("systemctl", "status", systemdServiceName, "--no-pager", "-l").Output()
	if len(output) > 0 {
		// Extract first line as description
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			status.Description = strings.TrimSpace(lines[0])
		}
	}

	if status.Description == "" {
		if status.Running {
			status.Description = "running"
		} else {
			status.Description = "stopped"
		}
		if status.Enabled {
			status.Description += " (enabled)"
		} else {
			status.Description += " (disabled)"
		}
	}

	return status, nil
}

func (m *systemdManager) IsInstalled() bool {
	_, err := os.Stat(systemdServicePath)
	return err == nil
}

// runSystemctl executes a systemctl command
func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
