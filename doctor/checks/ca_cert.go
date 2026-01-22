package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/doctor"
)

// CACert checks if CA certificate exists and is valid
type CACert struct{}

func (c *CACert) Name() string {
	return "CA certificate"
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
			Message: "Certificates directory not configured",
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
			Message: "CA certificate not found",
			Details: fmt.Sprintf("Expected at: %s\nRun 'roji' to auto-generate or 'roji doctor --fix'", caCertPath),
			Fixable: true,
		}
	}

	// Check if CA key exists
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "CA private key not found",
			Details: fmt.Sprintf("Expected at: %s", caKeyPath),
			Fixable: true,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: "CA certificate exists",
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
