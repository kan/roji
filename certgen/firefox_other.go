//go:build !linux

package certgen

import (
	"context"
	"errors"
)

// FirefoxInstaller is a stub for non-Linux platforms
type FirefoxInstaller struct{}

// NewFirefoxInstaller returns a Firefox-specific CA installer
// On non-Linux platforms, Firefox uses the system certificate store
func NewFirefoxInstaller() *FirefoxInstaller {
	return &FirefoxInstaller{}
}

func (i *FirefoxInstaller) Install(caCertPath string) error {
	return errors.New("Firefox certificate installation is only supported on Linux. On macOS/Windows, Firefox uses the system certificate store")
}

func (i *FirefoxInstaller) Uninstall(caCertPath string) error {
	return errors.New("Firefox certificate uninstallation is only supported on Linux. On macOS/Windows, Firefox uses the system certificate store")
}

func (i *FirefoxInstaller) IsInstalled(caCertPath string) (bool, error) {
	// On non-Linux, Firefox uses system store, so defer to system installer
	installer := NewSystemInstaller()
	return installer.IsInstalled(caCertPath)
}

func (i *FirefoxInstaller) NeedsSudo() bool {
	return false
}

func (i *FirefoxInstaller) Description() string {
	return "Firefox (uses system certificate store on this platform)"
}

// Ensure FirefoxInstaller implements CAInstaller
var _ CAInstaller = (*FirefoxInstaller)(nil)

// Stub for context (not used but matches interface pattern)
var _ = context.Background
