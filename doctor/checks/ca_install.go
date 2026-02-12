package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// CAInstall checks if CA certificate is installed in the system trust store
type CAInstall struct{}

func (c *CAInstall) Name() string {
	return i18n.T("doctor.check.ca_install")
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
			Message: i18n.T("doctor.check.ca_install.dir_not_configured"),
			Fixable: false,
		}
	}

	caCertPath := filepath.Join(certsDir, "ca.pem")

	// Check if CA cert exists first
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.T("doctor.check.ca_install.not_found"),
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
			Message: i18n.Tf("doctor.check.ca_install.check_error", err),
			Fixable: false,
		}
	}

	if !installed {
		details := i18n.T("doctor.check.ca_install.install_hint") + "\n  " + installer.Description()

		// On WSL, also mention Windows installation
		if certgen.IsWSL() {
			details += i18n.T("doctor.check.ca_install.wsl_hint")
		}

		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.T("doctor.check.ca_install.not_installed"),
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
				Message: i18n.T("doctor.check.ca_install.wsl_linux_only"),
				Details: i18n.T("doctor.check.ca_install.wsl_install_hint"),
				Fixable: true,
			}
		}

		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.T("doctor.check.ca_install.wsl_installed"),
			Details: fmt.Sprintf("Linux: %s\nWindows: %s", installer.Description(), wslInstaller.Description()),
			Fixable: false,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.T("doctor.check.ca_install.installed"),
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
		fmt.Println(i18n.Tf("doctor.check.ca_install.fix_installing", installer.Description()))
		if err := installer.Install(caCertPath); err != nil {
			return fmt.Errorf("failed to install CA certificate: %w", err)
		}
		fmt.Println(i18n.T("doctor.check.ca_install.fix_installed"))
	}

	// On WSL, also install to Windows
	if certgen.IsWSL() {
		wslInstaller := certgen.NewWSLInstaller()
		wslInstalled, _ := wslInstaller.IsInstalled(caCertPath)
		if !wslInstalled {
			fmt.Println(i18n.Tf("doctor.check.ca_install.fix_wsl_installing", wslInstaller.Description()))
			if err := wslInstaller.Install(caCertPath); err != nil {
				return fmt.Errorf("failed to install CA certificate to Windows: %w", err)
			}
			fmt.Println(i18n.T("doctor.check.ca_install.fix_wsl_installed"))
		}
	}

	return nil
}
