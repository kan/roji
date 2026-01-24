package checks

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kan/roji/certgen"
	"github.com/kan/roji/doctor"
)

// ServerCert checks if server certificate exists and is valid
type ServerCert struct{}

func (c *ServerCert) Name() string {
	return "Server certificate"
}

func (c *ServerCert) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
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

	certPath := filepath.Join(certsDir, "cert.pem")
	keyPath := filepath.Join(certsDir, "key.pem")

	// Check if cert exists
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Server certificate not found",
			Details: fmt.Sprintf("Expected at: %s", certPath),
			Fixable: true,
		}
	}

	// Check if key exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Server private key not found",
			Details: fmt.Sprintf("Expected at: %s", keyPath),
			Fixable: true,
		}
	}

	// Parse and validate the certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Failed to read server certificate",
			Details: err.Error(),
			Fixable: true,
		}
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Invalid server certificate format",
			Fixable: true,
		}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Failed to parse server certificate",
			Details: err.Error(),
			Fixable: true,
		}
	}

	// Check if certificate matches configured domain
	domain := "dev.localhost"
	if cfg.Settings != nil {
		domain = cfg.Settings.Domain
	}
	expectedWildcard := "*." + domain

	domainMatches := false
	for _, dnsName := range cert.DNSNames {
		if dnsName == expectedWildcard || dnsName == domain {
			domainMatches = true
			break
		}
	}

	if !domainMatches {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Certificate domain mismatch",
			Details: fmt.Sprintf("Expected: %s\nCertificate has: %v\nRun 'roji doctor --fix' to regenerate", expectedWildcard, cert.DNSNames),
			Fixable: true,
		}
	}

	// Check expiration
	now := time.Now()
	if now.After(cert.NotAfter) {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Server certificate has expired",
			Details: fmt.Sprintf("Expired on: %s", cert.NotAfter.Format("2006-01-02")),
			Fixable: true,
		}
	}

	// Warn if expiring soon (within 30 days)
	daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysUntilExpiry <= 30 {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Warn,
			Message: fmt.Sprintf("Server certificate expires in %d days", daysUntilExpiry),
			Details: fmt.Sprintf("Expires on: %s", cert.NotAfter.Format("2006-01-02")),
			Fixable: true,
		}
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: fmt.Sprintf("Server certificate is valid (expires in %d days)", daysUntilExpiry),
		Details: certPath,
		Fixable: false,
	}
}

func (c *ServerCert) CanFix() bool {
	return true
}

func (c *ServerCert) Fix(ctx context.Context, cfg *doctor.Config) error {
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

	// Delete existing server cert/key to force regeneration
	os.Remove(filepath.Join(certsDir, "cert.pem"))
	os.Remove(filepath.Join(certsDir, "key.pem"))

	gen := certgen.NewGenerator(certsDir, domain)
	if err := gen.EnsureCerts(); err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	return nil
}
