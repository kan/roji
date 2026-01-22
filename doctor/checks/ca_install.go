package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/doctor"
)

// CAInstall checks if CA certificate is installed in the system trust store
type CAInstall struct{}

func (c *CAInstall) Name() string {
	return "CA certificate installation"
}

func (c *CAInstall) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	certsDir := cfg.CertsDir
	if certsDir == "" && cfg.Settings != nil {
		certsDir = cfg.Settings.CertsDir
	}

	if certsDir == "" {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: "Certificates directory not configured",
			Fixable: false,
		}
	}

	caCertPath := filepath.Join(certsDir, "ca.pem")

	// Check if CA cert exists first
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: "CA certificate not found (run 'roji doctor --fix' to generate)",
			Fixable: false,
		}
	}

	// Check system installation
	installer := certgen.NewSystemInstaller()
	installed, err := installer.IsInstalled(caCertPath)
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: fmt.Sprintf("Cannot check installation status: %v", err),
			Fixable: false,
		}
	}

	if !installed {
		details := fmt.Sprintf("Run 'roji ca install' to install\n  %s", installer.Description())

		// On WSL, also mention Windows installation
		if certgen.IsWSL() {
			details += "\n\nFor Windows browsers, also run 'roji ca install --windows'"
		}

		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: "CA certificate not installed in system trust store",
			Details: details,
			Fixable: true,
		}
	}

	// If on WSL, also check Windows installation
	if certgen.IsWSL() {
		wslInstaller := certgen.NewWSLInstaller()
		wslInstalled, _ := wslInstaller.IsInstalled(caCertPath)
		if !wslInstalled {
			return doctor.CheckResult{
				Name:    c.Name(),
				Status:  doctor.Warn,
				Message: "CA installed in Linux but not in Windows",
				Details: "Run 'roji ca install --windows' to install to Windows user store",
				Fixable: true,
			}
		}

		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: "CA certificate installed (Linux + Windows)",
			Details: fmt.Sprintf("Linux: %s\nWindows: %s", installer.Description(), wslInstaller.Description()),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: "CA certificate installed in system trust store",
		Details: installer.Description(),
		Fixable: false,
	}
}

func (c *CAInstall) CanFix() bool {
	return true
}

func (c *CAInstall) Fix(ctx context.Context, cfg *doctor.Config) error {
	certsDir := cfg.CertsDir
	if certsDir == "" && cfg.Settings != nil {
		certsDir = cfg.Settings.CertsDir
	}

	if certsDir == "" {
		return fmt.Errorf("certificates directory not configured")
	}

	caCertPath := filepath.Join(certsDir, "ca.pem")

	// Check if CA cert exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found: run 'roji doctor --fix' first to generate")
	}

	// Install to system store
	installer := certgen.NewSystemInstaller()
	installed, _ := installer.IsInstalled(caCertPath)
	if !installed {
		fmt.Printf("Installing CA to: %s\n", installer.Description())
		if err := installer.Install(caCertPath); err != nil {
			return fmt.Errorf("failed to install CA certificate: %w", err)
		}
		fmt.Println("Installed to system trust store")
	}

	// On WSL, also install to Windows
	if certgen.IsWSL() {
		wslInstaller := certgen.NewWSLInstaller()
		wslInstalled, _ := wslInstaller.IsInstalled(caCertPath)
		if !wslInstalled {
			fmt.Printf("Installing CA to: %s\n", wslInstaller.Description())
			if err := wslInstaller.Install(caCertPath); err != nil {
				return fmt.Errorf("failed to install CA certificate to Windows: %w", err)
			}
			fmt.Println("Installed to Windows user trust store")
		}
	}

	return nil
}
