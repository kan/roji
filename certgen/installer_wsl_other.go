//go:build !linux

package certgen

// IsWSL returns false on non-Linux platforms
func IsWSL() bool {
	return false
}

// WSLInstaller is a stub for non-Linux platforms
type WSLInstaller struct{}

// NewWSLInstaller returns nil on non-Linux platforms
func NewWSLInstaller() *WSLInstaller {
	return nil
}

// NewWSLUserInstaller returns nil on non-Linux platforms
func NewWSLUserInstaller() *WSLInstaller {
	return nil
}

func (i *WSLInstaller) Install(caCertPath string) error {
	return nil
}

func (i *WSLInstaller) Uninstall(caCertPath string) error {
	return nil
}

func (i *WSLInstaller) IsInstalled(caCertPath string) (bool, error) {
	return false, nil
}

func (i *WSLInstaller) NeedsSudo() bool {
	return false
}

func (i *WSLInstaller) Description() string {
	return "WSL (not available on this platform)"
}
