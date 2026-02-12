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

// CACert checks if CA certificate exists and is valid
type CACert struct{}

func (c *CACert) Name() string {
	return i18n.T("doctor.check.ca_cert")
}

func (c *CACert) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	certsDir := cfg.CertsDir
	if certsDir == "" && cfg.Settings != nil {
		certsDir = cfg.Settings.CertsDir
	}

	if certsDir == "" {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: i18n.T("doctor.check.ca_cert.dir_not_configured"),
			Fixable: false,
		}
	}

	caCertPath := filepath.Join(certsDir, "ca.pem")
	caKeyPath := filepath.Join(certsDir, "ca-key.pem")

	// Check if CA cert exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.ca_cert.not_found"),
			Details: i18n.Tf("doctor.check.ca_cert.not_found_hint", caCertPath),
			Fixable: true,
		}
	}

	// Check if CA key exists
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.ca_cert.key_not_found"),
			Details: i18n.Tf("doctor.check.ca_cert.expected_at", caKeyPath),
			Fixable: true,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.T("doctor.check.ca_cert.exists"),
		Details: caCertPath,
		Fixable: false,
	}
}

func (c *CACert) CanFix() bool {
	return true
}

func (c *CACert) Fix(ctx context.Context, cfg *doctor.Config) error {
	certsDir := cfg.CertsDir
	if certsDir == "" && cfg.Settings != nil {
		certsDir = cfg.Settings.CertsDir
	}

	if certsDir == "" {
		return fmt.Errorf("certificates directory not configured")
	}

	domain := "dev.localhost"
	if cfg.Settings != nil {
		domain = cfg.Settings.Domain
	}

	gen := certgen.NewGenerator(certsDir, domain)
	if err := gen.EnsureCerts(); err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	return nil
}
