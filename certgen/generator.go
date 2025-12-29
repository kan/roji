package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// Generator handles TLS certificate generation
type Generator struct {
	certsDir   string
	baseDomain string
}

// NewGenerator creates a new certificate generator
func NewGenerator(certsDir, baseDomain string) *Generator {
	return &Generator{
		certsDir:   certsDir,
		baseDomain: baseDomain,
	}
}

// CertPaths returns the paths to certificate files
func (g *Generator) CertPaths() (caCert, caKey, serverCert, serverKey string) {
	return filepath.Join(g.certsDir, "ca.pem"),
		filepath.Join(g.certsDir, "ca-key.pem"),
		filepath.Join(g.certsDir, "cert.pem"),
		filepath.Join(g.certsDir, "key.pem")
}

// CACrtPath returns the path to Windows-compatible CA certificate (DER format)
func (g *Generator) CACrtPath() string {
	return filepath.Join(g.certsDir, "ca.crt")
}

// EnsureCerts generates certificates if they don't exist
// If server cert/key already exist (e.g., from mkcert), they are used as-is
func (g *Generator) EnsureCerts() error {
	caCertPath, caKeyPath, serverCertPath, serverKeyPath := g.CertPaths()

	// Check if server certificate files exist (cert.pem and key.pem)
	serverCertExists := fileExists(serverCertPath)
	serverKeyExists := fileExists(serverKeyPath)

	// If server cert/key exist, use them (likely from mkcert or manual setup)
	if serverCertExists && serverKeyExists {
		return nil
	}

	// If only one of cert/key exists, that's an error state
	if serverCertExists != serverKeyExists {
		return fmt.Errorf("incomplete certificate setup: only one of cert.pem/key.pem exists in %s", g.certsDir)
	}

	// No server certs exist - we need to generate them
	// Ensure directory exists
	if err := os.MkdirAll(g.certsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Check CA status
	caCertExists := fileExists(caCertPath)
	caKeyExists := fileExists(caKeyPath)

	var caCert *x509.Certificate
	var caKey *ecdsa.PrivateKey

	if caCertExists && caKeyExists {
		// Load existing CA
		var err error
		caCert, caKey, err = loadCA(caCertPath, caKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load existing CA: %w", err)
		}
	} else if !caCertExists && !caKeyExists {
		// Generate new CA
		var err error
		caCert, caKey, err = g.generateCA()
		if err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}

		// Save CA (PEM format)
		if err := saveCertificate(caCertPath, caCert); err != nil {
			return fmt.Errorf("failed to save CA certificate: %w", err)
		}
		if err := savePrivateKey(caKeyPath, caKey); err != nil {
			return fmt.Errorf("failed to save CA key: %w", err)
		}
		// Save CA in DER format for Windows (.crt)
		if err := saveCertificateDER(g.CACrtPath(), caCert); err != nil {
			return fmt.Errorf("failed to save CA certificate (DER): %w", err)
		}
	} else {
		// Only one of CA cert/key exists - error state
		return fmt.Errorf("incomplete CA setup: only one of ca.pem/ca-key.pem exists in %s", g.certsDir)
	}

	// Generate server certificate
	if err := g.generateServerCert(caCert, caKey, serverCertPath, serverKeyPath); err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}

	return nil
}

// generateCA creates a new CA certificate and private key
func (g *Generator) generateCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"roji Dev CA"},
			CommonName:   "roji CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, privateKey, nil
}

// generateServerCert creates a server certificate signed by the CA
func (g *Generator) generateServerCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, certPath, keyPath string) error {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Build DNS names for the certificate
	dnsNames := []string{
		"*." + g.baseDomain, // *.localhost or *.kan.localhost
		g.baseDomain,        // localhost or kan.localhost
		"localhost",         // Always include localhost
	}

	// Add wildcard for nested subdomains if baseDomain is not just "localhost"
	if g.baseDomain != "localhost" {
		dnsNames = append(dnsNames, "*.*."+g.baseDomain) // *.*.kan.localhost
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"roji"},
			CommonName:   "*." + g.baseDomain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              dnsNames,
		BasicConstraintsValid: true,
	}

	// Sign with CA
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Save certificate and key
	if err := saveCertificate(certPath, cert); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}
	if err := savePrivateKey(keyPath, privateKey); err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}

	return nil
}

// loadCA loads an existing CA certificate and private key
func loadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	// Load certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("invalid certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Load private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ = pem.Decode(keyPEM)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, nil, fmt.Errorf("invalid private key PEM")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return cert, key, nil
}

// saveCertificate saves a certificate to a PEM file
func saveCertificate(path string, cert *x509.Certificate) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// saveCertificateDER saves a certificate to a DER file (for Windows)
func saveCertificateDER(path string, cert *x509.Certificate) error {
	return os.WriteFile(path, cert.Raw, 0644)
}

// savePrivateKey saves a private key to a PEM file
func savePrivateKey(path string, key *ecdsa.PrivateKey) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}

	return pem.Encode(file, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
