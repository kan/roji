//go:build windows

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	windowsServiceName = "roji"
)

// windowsManager manages Windows services using NSSM
type windowsManager struct {
	execPath string
	nssmPath string
	user     string
}

// WindowsOptions contains options for Windows service
type WindowsOptions struct {
	User string // Not used for Windows (runs as LocalSystem or specified account)
}

func newWindowsManager() (*windowsManager, error) {
	return newWindowsManagerWithOptions(WindowsOptions{})
}

func newWindowsManagerWithOptions(opts WindowsOptions) (*windowsManager, error) {
	// Find NSSM
	nssmPath, err := findNSSM()
	if err != nil {
		return nil, err
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

	return &windowsManager{
		execPath: execPath,
		nssmPath: nssmPath,
		user:     opts.User,
	}, nil
}

// findNSSM locates the NSSM executable
func findNSSM() (string, error) {
	// Check if nssm is in PATH
	path, err := exec.LookPath("nssm.exe")
	if err == nil {
		return path, nil
	}

	// Check common installation locations
	commonPaths := []string{
		`C:\Program Files\nssm\nssm.exe`,
		`C:\Program Files (x86)\nssm\nssm.exe`,
		`C:\nssm\nssm.exe`,
		`C:\Tools\nssm\nssm.exe`,
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("NSSM not found. Please install NSSM from https://nssm.cc/ and add it to PATH")
}

func (m *windowsManager) Install() error {
	if m.IsInstalled() {
		return fmt.Errorf("service is already installed")
	}

	// Install service using NSSM
	cmd := exec.Command(m.nssmPath, "install", windowsServiceName, m.execPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install service: %w\n%s", err, string(output))
	}

	// Set service description
	cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "Description", "roji - reverse proxy for local development")
	_ = cmd.Run()

	// Set start type to auto
	cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "Start", "SERVICE_AUTO_START")
	_ = cmd.Run()

	// Set stdout/stderr log files
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		logDir := filepath.Join(homeDir, ".local", "share", "roji")
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "roji.log")

		cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "AppStdout", logPath)
		_ = cmd.Run()

		cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "AppStderr", logPath)
		_ = cmd.Run()

		// Rotate logs
		cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "AppRotateFiles", "1")
		_ = cmd.Run()

		cmd = exec.Command(m.nssmPath, "set", windowsServiceName, "AppRotateBytes", "10485760") // 10MB
		_ = cmd.Run()
	}

	return nil
}

func (m *windowsManager) Uninstall() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	// Stop service first
	_ = m.Stop()

	// Remove service using NSSM
	cmd := exec.Command(m.nssmPath, "remove", windowsServiceName, "confirm")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to uninstall service: %w\n%s", err, string(output))
	}

	return nil
}

func (m *windowsManager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed - run 'roji service install' first")
	}

	cmd := exec.Command(m.nssmPath, "start", windowsServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service: %w\n%s", err, string(output))
	}

	return nil
}

func (m *windowsManager) Stop() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command(m.nssmPath, "stop", windowsServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop service: %w\n%s", err, string(output))
	}

	return nil
}

func (m *windowsManager) Restart() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command(m.nssmPath, "restart", windowsServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service: %w\n%s", err, string(output))
	}

	return nil
}

func (m *windowsManager) Status() (*Status, error) {
	status := &Status{
		Installed: m.IsInstalled(),
	}

	if !status.Installed {
		status.Description = "not installed"
		return status, nil
	}

	// Check service status using NSSM
	cmd := exec.Command(m.nssmPath, "status", windowsServiceName)
	output, err := cmd.Output()
	if err != nil {
		status.Description = "unknown"
		return status, nil
	}

	outputStr := strings.TrimSpace(string(output))

	switch {
	case strings.Contains(outputStr, "SERVICE_RUNNING"):
		status.Running = true
		status.Description = "running"
	case strings.Contains(outputStr, "SERVICE_STOPPED"):
		status.Running = false
		status.Description = "stopped"
	case strings.Contains(outputStr, "SERVICE_PAUSED"):
		status.Running = false
		status.Description = "paused"
	default:
		status.Description = outputStr
	}

	// Check if auto-start is enabled
	cmd = exec.Command(m.nssmPath, "get", windowsServiceName, "Start")
	output, err = cmd.Output()
	if err == nil {
		if bytes.Contains(output, []byte("SERVICE_AUTO_START")) {
			status.Enabled = true
			status.Description += " (auto-start enabled)"
		}
	}

	return status, nil
}

func (m *windowsManager) IsInstalled() bool {
	cmd := exec.Command(m.nssmPath, "status", windowsServiceName)
	err := cmd.Run()
	return err == nil
}
