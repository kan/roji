//go:build darwin

package certgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DarwinInstaller installs CA certificates to the macOS Keychain
type DarwinInstaller struct {
	UserKeychain bool // If true, install to user keychain (no sudo required)
}

// NewSystemInstaller returns the system CA installer for the current platform
func NewSystemInstaller() CAInstaller {
	return &DarwinInstaller{UserKeychain: false}
}

// NewUserInstaller returns a user-level CA installer (no sudo required)
func NewUserInstaller() CAInstaller {
	return &DarwinInstaller{UserKeychain: true}
}

func (i *DarwinInstaller) Install(caCertPath string) error {
	// Verify the certificate file exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found: %s", caCertPath)
	}

	var keychain string
	var args []string

	if i.UserKeychain {
		// Install to user's login keychain (no sudo needed)
		keychain = filepath.Join(os.Getenv("HOME"), "Library/Keychains/login.keychain-db")
		args = []string{
			"security", "add-trusted-cert",
			"-r", "trustRoot",
			"-k", keychain,
			caCertPath,
		}
	} else {
		// Install to system keychain (requires sudo)
		keychain = "/Library/Keychains/System.keychain"
		args = []string{
			"sudo", "security", "add-trusted-cert",
			"-d", "-r", "trustRoot",
			"-k", keychain,
			caCertPath,
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install certificate: %w\n%s", err, stderr.String())
	}

	return nil
}

func (i *DarwinInstaller) Uninstall(caCertPath string) error {
	// Get the certificate's subject name
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to get certificate subject: %w", err)
	}

	var keychain string
	var args []string

	if i.UserKeychain {
		keychain = filepath.Join(os.Getenv("HOME"), "Library/Keychains/login.keychain-db")
		args = []string{
			"security", "delete-certificate",
			"-c", subject,
			keychain,
		}
	} else {
		keychain = "/Library/Keychains/System.keychain"
		args = []string{
			"sudo", "security", "delete-certificate",
			"-c", subject,
			keychain,
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w\n%s", err, stderr.String())
	}

	return nil
}

func (i *DarwinInstaller) IsInstalled(caCertPath string) (bool, error) {
	// Get the certificate's subject name
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return false, fmt.Errorf("failed to get certificate subject: %w", err)
	}

	// Search for the certificate in keychains
	cmd := exec.Command("security", "find-certificate", "-c", subject)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Certificate not found
		return false, nil
	}

	return strings.Contains(stdout.String(), subject), nil
}

func (i *DarwinInstaller) NeedsSudo() bool {
	return !i.UserKeychain
}

func (i *DarwinInstaller) Description() string {
	if i.UserKeychain {
		return "macOS User Keychain (login.keychain)"
	}
	return "macOS System Keychain (System.keychain)"
}
