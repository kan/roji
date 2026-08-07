//go:build !linux

package service

import "fmt"

// ServiceOptions for non-Linux platforms (stub)
type ServiceOptions struct {
	User string
}

func newSystemdManagerWithOptions(opts ServiceOptions) (*systemdManager, error) {
	return nil, fmt.Errorf("systemd is only available on Linux")
}

type systemdManager struct{}

func (m *systemdManager) Install() error           { return nil }
func (m *systemdManager) Uninstall() error         { return nil }
func (m *systemdManager) Start() error             { return nil }
func (m *systemdManager) Stop() error              { return nil }
func (m *systemdManager) Restart() error           { return nil }
func (m *systemdManager) Status() (*Status, error) { return nil, nil }
func (m *systemdManager) IsInstalled() bool        { return false }
