//go:build windows

package certgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// WindowsInstaller installs CA certificates to the Windows certificate store
type WindowsInstaller struct {
	CurrentUser bool // If true, install to current user store; otherwise machine store
}

// NewSystemInstaller returns the system CA installer for the current platform
func NewSystemInstaller() CAInstaller {
	return &WindowsInstaller{CurrentUser: false}
}

// NewUserInstaller returns a user-level CA installer (no admin required)
func NewUserInstaller() CAInstaller {
	return &WindowsInstaller{CurrentUser: true}
}

func (i *WindowsInstaller) Install(caCertPath string) error {
	// Verify the certificate file exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found: %s", caCertPath)
	}

	// Windows prefers .crt (DER) format, but can handle PEM too
	// Use certutil to add the certificate

	var store string
	if i.CurrentUser {
		store = "ROOT" // Current user's Trusted Root store
	} else {
		store = "ROOT" // Machine's Trusted Root store (requires admin)
	}

	var args []string
	if i.CurrentUser {
		args = []string{"-addstore", "-user", "-f", store, caCertPath}
	} else {
		args = []string{"-addstore", "-f", store, caCertPath}
	}

	cmd := exec.Command("certutil", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install certificate: %w\n%s", err, stderr.String())
	}

	return nil
}

func (i *WindowsInstaller) Uninstall(caCertPath string) error {
	// Get the certificate's subject name
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to get certificate subject: %w", err)
	}

	var store string
	if i.CurrentUser {
		store = "ROOT"
	} else {
		store = "ROOT"
	}

	var args []string
	if i.CurrentUser {
		args = []string{"-delstore", "-user", store, subject}
	} else {
		args = []string{"-delstore", store, subject}
	}

	cmd := exec.Command("certutil", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w\n%s", err, stderr.String())
	}

	return nil
}

func (i *WindowsInstaller) IsInstalled(caCertPath string) (bool, error) {
	// Get the certificate's subject name
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return false, fmt.Errorf("failed to get certificate subject: %w", err)
	}

	var store string
	if i.CurrentUser {
		store = "ROOT"
	} else {
		store = "ROOT"
	}

	var args []string
	if i.CurrentUser {
		args = []string{"-store", "-user", store, subject}
	} else {
		args = []string{"-store", store, subject}
	}

	cmd := exec.Command("certutil", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Certificate not found
		return false, nil
	}

	return stdout.Len() > 0, nil
}

func (i *WindowsInstaller) NeedsSudo() bool {
	return !i.CurrentUser // Machine store requires admin
}

func (i *WindowsInstaller) Description() string {
	if i.CurrentUser {
		return "Windows User Certificate Store (CurrentUser\\ROOT)"
	}
	return "Windows Machine Certificate Store (LocalMachine\\ROOT)"
}
