//go:build !windows

package service

import "fmt"

// WindowsOptions for non-Windows platforms (stub)
type WindowsOptions struct {
	User string
}

func newWindowsManagerWithOptions(opts WindowsOptions) (*windowsManager, error) {
	return nil, fmt.Errorf("service management is only available on Windows")
}

type windowsManager struct{}

func (m *windowsManager) Install() error           { return nil }
func (m *windowsManager) Uninstall() error         { return nil }
func (m *windowsManager) Start() error             { return nil }
func (m *windowsManager) Stop() error              { return nil }
func (m *windowsManager) Restart() error           { return nil }
func (m *windowsManager) Status() (*Status, error) { return nil, nil }
func (m *windowsManager) IsInstalled() bool        { return false }
