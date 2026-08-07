//go:build !darwin

package service

import "fmt"

// LaunchdOptions for non-macOS platforms (stub)
type LaunchdOptions struct {
	User string
}

func newLaunchdManagerWithOptions(opts LaunchdOptions) (*launchdManager, error) {
	return nil, fmt.Errorf("launchd is only available on macOS")
}

type launchdManager struct{}

func (m *launchdManager) Install() error           { return nil }
func (m *launchdManager) Uninstall() error         { return nil }
func (m *launchdManager) Start() error             { return nil }
func (m *launchdManager) Stop() error              { return nil }
func (m *launchdManager) Restart() error           { return nil }
func (m *launchdManager) Status() (*Status, error) { return nil, nil }
func (m *launchdManager) IsInstalled() bool        { return false }
