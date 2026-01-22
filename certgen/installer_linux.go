//go:build linux

package certgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LinuxInstaller installs CA certificates to the Linux system trust store
type LinuxInstaller struct {
	distro string // "debian" or "rhel"
}

// NewSystemInstaller returns the system CA installer for the current platform
func NewSystemInstaller() CAInstaller {
	return &LinuxInstaller{distro: detectDistro()}
}

// NewUserInstaller returns a user-level CA installer
// Note: On Linux, there's no standard user-level CA store, so this still uses system store
func NewUserInstaller() CAInstaller {
	return NewSystemInstaller()
}

func detectDistro() string {
	// Check for Debian/Ubuntu (update-ca-certificates)
	if _, err := exec.LookPath("update-ca-certificates"); err == nil {
		return "debian"
	}
	// Check for RHEL/Fedora/CentOS (update-ca-trust)
	if _, err := exec.LookPath("update-ca-trust"); err == nil {
		return "rhel"
	}
	// Default to Debian-style
	return "debian"
}

func (i *LinuxInstaller) Install(caCertPath string) error {
	// Verify the certificate file exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found: %s", caCertPath)
	}

	var destPath string
	var updateCmd []string

	switch i.distro {
	case "rhel":
		// RHEL/Fedora/CentOS
		destPath = "/etc/pki/ca-trust/source/anchors/roji-ca.pem"
		updateCmd = []string{"sudo", "update-ca-trust", "extract"}
	default:
		// Debian/Ubuntu
		destPath = "/usr/local/share/ca-certificates/roji-ca.crt"
		updateCmd = []string{"sudo", "update-ca-certificates"}
	}

	// Copy the certificate to the appropriate location
	copyCmd := exec.Command("sudo", "cp", caCertPath, destPath)
	copyCmd.Stdin = os.Stdin
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr

	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy certificate: %w", err)
	}

	// Update the trust store
	cmd := exec.Command(updateCmd[0], updateCmd[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update CA trust: %w", err)
	}

	return nil
}

func (i *LinuxInstaller) Uninstall(caCertPath string) error {
	var certPath string
	var updateCmd []string

	switch i.distro {
	case "rhel":
		certPath = "/etc/pki/ca-trust/source/anchors/roji-ca.pem"
		updateCmd = []string{"sudo", "update-ca-trust", "extract"}
	default:
		certPath = "/usr/local/share/ca-certificates/roji-ca.crt"
		updateCmd = []string{"sudo", "update-ca-certificates", "--fresh"}
	}

	// Remove the certificate
	rmCmd := exec.Command("sudo", "rm", "-f", certPath)
	rmCmd.Stdin = os.Stdin
	rmCmd.Stdout = os.Stdout
	rmCmd.Stderr = os.Stderr

	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove certificate: %w", err)
	}

	// Update the trust store
	cmd := exec.Command(updateCmd[0], updateCmd[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update CA trust: %w", err)
	}

	return nil
}

func (i *LinuxInstaller) IsInstalled(caCertPath string) (bool, error) {
	var certPath string

	switch i.distro {
	case "rhel":
		certPath = "/etc/pki/ca-trust/source/anchors/roji-ca.pem"
	default:
		certPath = "/usr/local/share/ca-certificates/roji-ca.crt"
	}

	// Check if the file exists
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func (i *LinuxInstaller) NeedsSudo() bool {
	return true
}

func (i *LinuxInstaller) Description() string {
	switch i.distro {
	case "rhel":
		return "Linux system CA store (RHEL/Fedora: /etc/pki/ca-trust/source/anchors/)"
	default:
		return "Linux system CA store (Debian/Ubuntu: /usr/local/share/ca-certificates/)"
	}
}

// FirefoxInstaller installs CA certificates to Firefox's NSS store
type FirefoxInstaller struct{}

// NewFirefoxInstaller returns a Firefox-specific CA installer
func NewFirefoxInstaller() *FirefoxInstaller {
	return &FirefoxInstaller{}
}

func (i *FirefoxInstaller) Install(caCertPath string) error {
	// Check if certutil (nss-tools) is available
	if _, err := exec.LookPath("certutil"); err != nil {
		return fmt.Errorf("certutil (nss-tools) not found. Install it with:\n  Debian/Ubuntu: sudo apt install libnss3-tools\n  RHEL/Fedora: sudo dnf install nss-tools")
	}

	// Find Firefox profile directories
	profiles, err := i.findFirefoxProfiles()
	if err != nil {
		return fmt.Errorf("failed to find Firefox profiles: %w", err)
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no Firefox profiles found")
	}

	// Install to each profile
	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-A",
			"-n", "roji CA",
			"-t", "C,,",
			"-i", caCertPath,
			"-d", "sql:"+profile)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install certificate to %s: %w\n%s", profile, err, stderr.String())
		}
	}

	return nil
}

func (i *FirefoxInstaller) Uninstall(caCertPath string) error {
	profiles, err := i.findFirefoxProfiles()
	if err != nil {
		return fmt.Errorf("failed to find Firefox profiles: %w", err)
	}

	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-D",
			"-n", "roji CA",
			"-d", "sql:"+profile)

		// Ignore errors (cert might not exist in this profile)
		cmd.Run()
	}

	return nil
}

func (i *FirefoxInstaller) IsInstalled(caCertPath string) (bool, error) {
	profiles, err := i.findFirefoxProfiles()
	if err != nil {
		return false, nil
	}

	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-L",
			"-n", "roji CA",
			"-d", "sql:"+profile)

		if err := cmd.Run(); err == nil {
			return true, nil
		}
	}

	return false, nil
}

func (i *FirefoxInstaller) NeedsSudo() bool {
	return false
}

func (i *FirefoxInstaller) Description() string {
	return "Firefox NSS certificate store"
}

func (i *FirefoxInstaller) findFirefoxProfiles() ([]string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return nil, fmt.Errorf("HOME environment variable not set")
	}

	var profiles []string

	// Common Firefox profile locations
	locations := []string{
		filepath.Join(home, ".mozilla/firefox"),
		filepath.Join(home, "snap/firefox/common/.mozilla/firefox"),
	}

	for _, loc := range locations {
		entries, err := os.ReadDir(loc)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				profilePath := filepath.Join(loc, entry.Name())
				// Check if it looks like a Firefox profile
				if _, err := os.Stat(filepath.Join(profilePath, "cert9.db")); err == nil {
					profiles = append(profiles, profilePath)
				}
			}
		}
	}

	return profiles, nil
}
