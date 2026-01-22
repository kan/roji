package certgen

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// CAInstaller is the interface for installing CA certificates into system trust stores
type CAInstaller interface {
	// Install installs the CA certificate to the system trust store
	Install(caCertPath string) error

	// Uninstall removes the CA certificate from the system trust store
	Uninstall(caCertPath string) error

	// IsInstalled checks if the CA certificate is installed in the system trust store
	IsInstalled(caCertPath string) (bool, error)

	// NeedsSudo returns whether this installer requires elevated privileges
	NeedsSudo() bool

	// Description returns a human-readable description of where certs are installed
	Description() string
}

// InstallStatus represents the installation status of a CA certificate
type InstallStatus struct {
	Installed   bool   `json:"installed"`
	Location    string `json:"location"`
	Description string `json:"description"`
	NeedsSudo   bool   `json:"needs_sudo"`
}

// GetCAFingerprint returns the SHA256 fingerprint of a CA certificate
func GetCAFingerprint(certPath string) (string, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Return Subject Key Identifier as hex string
	return fmt.Sprintf("%X", cert.SubjectKeyId), nil
}

// GetCASubject returns the subject (CN) of a CA certificate
func GetCASubject(certPath string) (string, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.Subject.CommonName, nil
}

// ExportToDER exports a PEM certificate to DER format
func ExportToDER(pemPath, derPath string) error {
	certPEM, err := os.ReadFile(pemPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode PEM")
	}

	if err := os.WriteFile(derPath, block.Bytes, 0644); err != nil {
		return fmt.Errorf("failed to write DER file: %w", err)
	}

	return nil
}

// ExportToPEM copies a PEM certificate to a new location
func ExportToPEM(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write PEM file: %w", err)
	}

	return nil
}
