package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()

	if dir == "" {
		t.Error("ConfigDir should not be empty")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(dir, "roji") {
			t.Errorf("ConfigDir should contain 'roji', got %s", dir)
		}
	} else {
		if !strings.HasSuffix(dir, "/roji") {
			t.Errorf("ConfigDir should end with '/roji', got %s", dir)
		}
	}
}

func TestDataDir(t *testing.T) {
	dir := DataDir()

	if dir == "" {
		t.Error("DataDir should not be empty")
	}

	if runtime.GOOS == "windows" {
		if !strings.Contains(dir, "roji") {
			t.Errorf("DataDir should contain 'roji', got %s", dir)
		}
	} else {
		if !strings.HasSuffix(dir, "/roji") {
			t.Errorf("DataDir should end with '/roji', got %s", dir)
		}
	}
}

func TestCertsDir(t *testing.T) {
	dir := CertsDir()

	if dir == "" {
		t.Error("CertsDir should not be empty")
	}

	if !strings.HasSuffix(dir, "certs") {
		t.Errorf("CertsDir should end with 'certs', got %s", dir)
	}
}

func TestConfigFilePath(t *testing.T) {
	path := ConfigFilePath()

	if path == "" {
		t.Error("ConfigFilePath should not be empty")
	}

	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("ConfigFilePath should end with 'config.yaml', got %s", path)
	}
}

func TestDefaultPaths(t *testing.T) {
	paths := DefaultPaths()

	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}
	if paths.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if paths.CertsDir == "" {
		t.Error("CertsDir should not be empty")
	}
}

func TestXDGOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG not applicable on Windows")
	}

	// Test XDG_CONFIG_HOME override
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	dir := ConfigDir()
	expected := filepath.Join(tmpDir, AppName)
	if dir != expected {
		t.Errorf("expected ConfigDir %s with XDG_CONFIG_HOME, got %s", expected, dir)
	}
}

func TestXDGDataOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG not applicable on Windows")
	}

	// Test XDG_DATA_HOME override
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	dir := DataDir()
	expected := filepath.Join(tmpDir, AppName)
	if dir != expected {
		t.Errorf("expected DataDir %s with XDG_DATA_HOME, got %s", expected, dir)
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "a", "b", "c")

	if err := EnsureDir(newDir); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	if !Exists(newDir) {
		t.Error("EnsureDir did not create directory")
	}

	// Calling again should not fail
	if err := EnsureDir(newDir); err != nil {
		t.Fatalf("EnsureDir failed on existing dir: %v", err)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	if !Exists(tmpDir) {
		t.Error("Exists should return true for existing directory")
	}

	nonExistent := filepath.Join(tmpDir, "nonexistent")
	if Exists(nonExistent) {
		t.Error("Exists should return false for non-existent path")
	}

	// Test with file
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if !Exists(tmpFile) {
		t.Error("Exists should return true for existing file")
	}
}
