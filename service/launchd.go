//go:build darwin

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
	launchdServiceName = "com.roji.agent"
)

// launchdManager manages launchd services on macOS
type launchdManager struct {
	execPath  string
	plistPath string
	user      string
}

// launchd plist template
const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecPath}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
    <key>WorkingDirectory</key>
    <string>{{.HomeDir}}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>{{.HomeDir}}</string>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
`

func newLaunchdManager() (*launchdManager, error) {
	return newLaunchdManagerWithOptions(LaunchdOptions{})
}

// LaunchdOptions contains options for launchd service
type LaunchdOptions struct {
	User string // Not used for launchd (always runs as current user)
}

func newLaunchdManagerWithOptions(opts LaunchdOptions) (*launchdManager, error) {
	// Find the roji executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get current user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine user
	user := opts.User
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" {
		return nil, fmt.Errorf("cannot determine user")
	}

	// LaunchAgents path (user-level service)
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", launchdServiceName+".plist")

	return &launchdManager{
		execPath:  execPath,
		plistPath: plistPath,
		user:      user,
	}, nil
}

func (m *launchdManager) Install() error {
	if m.IsInstalled() {
		return fmt.Errorf("service is already installed")
	}

	// Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Get home directory for log path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Log path
	logDir := filepath.Join(homeDir, "Library", "Logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create Logs directory: %w", err)
	}
	logPath := filepath.Join(logDir, "roji.log")

	// Generate plist content
	tmpl, err := template.New("launchd").Parse(launchdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Label":    launchdServiceName,
		"ExecPath": m.execPath,
		"LogPath":  logPath,
		"HomeDir":  homeDir,
	})
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write plist file
	if err := os.WriteFile(m.plistPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	return nil
}

func (m *launchdManager) Uninstall() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	// Stop service if running
	_ = m.Stop()

	// Unload the service
	_ = runLaunchctl("unload", m.plistPath)

	// Remove plist file
	if err := os.Remove(m.plistPath); err != nil {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

func (m *launchdManager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed - run 'roji service install' first")
	}

	// Load and start the service
	if err := runLaunchctl("load", m.plistPath); err != nil {
		// Try bootstrap if load fails (newer macOS)
		if err2 := runLaunchctl("bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), m.plistPath); err2 != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
	}

	return nil
}

func (m *launchdManager) Stop() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	// Try bootout first (newer macOS), then unload
	if err := runLaunchctl("bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdServiceName)); err != nil {
		if err := runLaunchctl("unload", m.plistPath); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}

	return nil
}

func (m *launchdManager) Restart() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	// Stop then start
	_ = m.Stop()
	return m.Start()
}

func (m *launchdManager) Status() (*Status, error) {
	status := &Status{
		Installed: m.IsInstalled(),
	}

	if !status.Installed {
		status.Description = "not installed"
		return status, nil
	}

	// launchd services are "enabled" when the plist exists with RunAtLoad=true
	status.Enabled = true

	// Check if running using launchctl list
	output, err := exec.Command("launchctl", "list").Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, launchdServiceName) {
				fields := strings.Fields(line)
				if len(fields) >= 1 && fields[0] != "-" {
					// First field is PID if running
					status.Running = true
					break
				}
			}
		}
	}

	// Get status description
	if status.Running {
		status.Description = "running"
	} else {
		status.Description = "stopped"
	}
	status.Description += " (enabled)"

	return status, nil
}

func (m *launchdManager) IsInstalled() bool {
	_, err := os.Stat(m.plistPath)
	return err == nil
}

// runLaunchctl executes a launchctl command
func runLaunchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
