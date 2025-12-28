package certs

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerator_EnsureCerts_NewCerts(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	gen := NewGenerator(tempDir, "test.localhost")

	// Generate certificates
	if err := gen.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts() error = %v", err)
	}

	// Verify all files exist
	files := []string{"ca.pem", "ca-key.pem", "ca.crt", "cert.pem", "key.pem"}
	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify CA certificate
	caCertPEM, err := os.ReadFile(filepath.Join(tempDir, "ca.pem"))
	if err != nil {
		t.Fatalf("failed to read ca.pem: %v", err)
	}

	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		t.Fatal("failed to decode CA PEM")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	if !caCert.IsCA {
		t.Error("CA certificate IsCA = false, want true")
	}
	if caCert.Subject.CommonName != "roji CA" {
		t.Errorf("CA CommonName = %q, want %q", caCert.Subject.CommonName, "roji CA")
	}

	// Verify server certificate
	serverCertPEM, err := os.ReadFile(filepath.Join(tempDir, "cert.pem"))
	if err != nil {
		t.Fatalf("failed to read cert.pem: %v", err)
	}

	block, _ = pem.Decode(serverCertPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}

	serverCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}

	// Check DNS names
	expectedDNS := []string{"*.test.localhost", "test.localhost", "localhost", "*.*.test.localhost"}
	for _, expected := range expectedDNS {
		found := false
		for _, dns := range serverCert.DNSNames {
			if dns == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DNS name %q not found in certificate", expected)
		}
	}

	// Verify DER format file
	derData, err := os.ReadFile(filepath.Join(tempDir, "ca.crt"))
	if err != nil {
		t.Fatalf("failed to read ca.crt: %v", err)
	}

	derCert, err := x509.ParseCertificate(derData)
	if err != nil {
		t.Fatalf("failed to parse DER certificate: %v", err)
	}

	if derCert.Subject.CommonName != "roji CA" {
		t.Errorf("DER CA CommonName = %q, want %q", derCert.Subject.CommonName, "roji CA")
	}
}

func TestGenerator_EnsureCerts_ExistingCerts(t *testing.T) {
	tempDir := t.TempDir()

	// Create dummy cert.pem and key.pem
	dummyCert := []byte("dummy cert")
	dummyKey := []byte("dummy key")

	if err := os.WriteFile(filepath.Join(tempDir, "cert.pem"), dummyCert, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "key.pem"), dummyKey, 0600); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(tempDir, "test.localhost")

	// Should not generate new certs
	if err := gen.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts() error = %v", err)
	}

	// Verify original files are unchanged
	cert, _ := os.ReadFile(filepath.Join(tempDir, "cert.pem"))
	if string(cert) != "dummy cert" {
		t.Error("cert.pem was modified when it should have been preserved")
	}

	// CA files should not be created
	if _, err := os.Stat(filepath.Join(tempDir, "ca.pem")); !os.IsNotExist(err) {
		t.Error("ca.pem should not exist when using existing certs")
	}
}

func TestGenerator_EnsureCerts_IncompleteCerts(t *testing.T) {
	tempDir := t.TempDir()

	// Create only cert.pem (missing key.pem)
	if err := os.WriteFile(filepath.Join(tempDir, "cert.pem"), []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(tempDir, "test.localhost")

	// Should return error
	err := gen.EnsureCerts()
	if err == nil {
		t.Error("expected error for incomplete certificate setup")
	}
}

func TestGenerator_EnsureCerts_IncompleteCA(t *testing.T) {
	tempDir := t.TempDir()

	// Create only ca.pem (missing ca-key.pem)
	if err := os.WriteFile(filepath.Join(tempDir, "ca.pem"), []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(tempDir, "test.localhost")

	// Should return error
	err := gen.EnsureCerts()
	if err == nil {
		t.Error("expected error for incomplete CA setup")
	}
}

func TestGenerator_CertPaths(t *testing.T) {
	gen := NewGenerator("/certs", "localhost")

	caCert, caKey, serverCert, serverKey := gen.CertPaths()

	if caCert != "/certs/ca.pem" {
		t.Errorf("caCert = %q, want %q", caCert, "/certs/ca.pem")
	}
	if caKey != "/certs/ca-key.pem" {
		t.Errorf("caKey = %q, want %q", caKey, "/certs/ca-key.pem")
	}
	if serverCert != "/certs/cert.pem" {
		t.Errorf("serverCert = %q, want %q", serverCert, "/certs/cert.pem")
	}
	if serverKey != "/certs/key.pem" {
		t.Errorf("serverKey = %q, want %q", serverKey, "/certs/key.pem")
	}
}

func TestGenerator_CACrtPath(t *testing.T) {
	gen := NewGenerator("/certs", "localhost")

	path := gen.CACrtPath()
	if path != "/certs/ca.crt" {
		t.Errorf("CACrtPath() = %q, want %q", path, "/certs/ca.crt")
	}
}

func TestGenerator_LocalhostDomain(t *testing.T) {
	tempDir := t.TempDir()

	// Test with just "localhost" as domain
	gen := NewGenerator(tempDir, "localhost")

	if err := gen.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts() error = %v", err)
	}

	// Read and check DNS names
	serverCertPEM, _ := os.ReadFile(filepath.Join(tempDir, "cert.pem"))
	block, _ := pem.Decode(serverCertPEM)
	serverCert, _ := x509.ParseCertificate(block.Bytes)

	// Should have *.localhost, localhost, localhost
	// Should NOT have *.*.localhost (only added for non-localhost domains)
	hasNestedWildcard := false
	for _, dns := range serverCert.DNSNames {
		if dns == "*.*.localhost" {
			hasNestedWildcard = true
		}
	}

	if hasNestedWildcard {
		t.Error("localhost domain should not have *.*.localhost DNS name")
	}
}
