//go:build linux

package certgen

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// IsWSL returns true if running under Windows Subsystem for Linux
func IsWSL() bool {
	// Check for WSL-specific indicators
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

// WSLInstaller installs CA certificates to Windows from WSL
// Note: WSL cannot run Windows commands with admin privileges,
// so this always installs to the current user's certificate store.
type WSLInstaller struct{}

// NewWSLInstaller returns a Windows CA installer for use from WSL
// Note: Always installs to user store since WSL cannot elevate to admin
func NewWSLInstaller() *WSLInstaller {
	return &WSLInstaller{}
}

// NewWSLUserInstaller returns a Windows user-level CA installer for use from WSL
// Deprecated: Use NewWSLInstaller instead. WSL always uses user store.
func NewWSLUserInstaller() *WSLInstaller {
	return &WSLInstaller{}
}

func (i *WSLInstaller) Install(caCertPath string) error {
	// Verify the certificate file exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found: %s", caCertPath)
	}

	// Copy certificate to Windows temp directory (avoids path conversion issues)
	tempDir := getWindowsTempDir()
	if tempDir == "" {
		return fmt.Errorf("cannot find Windows temp directory")
	}

	tempCertPath := filepath.Join(tempDir, "roji-ca-install.crt")

	// Copy cert to temp location
	if err := copyFile(caCertPath, tempCertPath); err != nil {
		return fmt.Errorf("failed to copy certificate to temp: %w", err)
	}
	defer os.Remove(tempCertPath)

	// Convert temp path to Windows path
	winPath, err := wslToWindowsPath(tempCertPath)
	if err != nil {
		return fmt.Errorf("failed to convert path: %w", err)
	}

	// Always use user store since WSL cannot elevate to admin
	args := []string{"-addstore", "-user", "-f", "ROOT", winPath}

	// Run certutil.exe directly
	cmd := exec.Command("certutil.exe", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Convert Shift-JIS output to UTF-8 for display
		errMsg := decodeShiftJIS(stderr.Bytes())
		if errMsg == "" {
			errMsg = decodeShiftJIS(stdout.Bytes())
		}
		return fmt.Errorf("failed to install certificate: %w\n%s", err, errMsg)
	}

	// Print success output (converted from Shift-JIS)
	fmt.Print(decodeShiftJIS(stdout.Bytes()))

	return nil
}

func (i *WSLInstaller) Uninstall(caCertPath string) error {
	// Get the certificate's subject name
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to get certificate subject: %w", err)
	}

	// Always use user store since WSL cannot elevate to admin
	args := []string{"-delstore", "-user", "ROOT", subject}

	cmd := exec.Command("certutil.exe", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := decodeShiftJIS(stderr.Bytes())
		if errMsg == "" {
			errMsg = decodeShiftJIS(stdout.Bytes())
		}
		return fmt.Errorf("failed to uninstall certificate: %w\n%s", err, errMsg)
	}

	fmt.Print(decodeShiftJIS(stdout.Bytes()))

	return nil
}

func (i *WSLInstaller) IsInstalled(caCertPath string) (bool, error) {
	subject, err := GetCASubject(caCertPath)
	if err != nil {
		return false, fmt.Errorf("failed to get certificate subject: %w", err)
	}

	// Always use user store since WSL cannot elevate to admin
	args := []string{"-store", "-user", "ROOT", subject}

	cmd := exec.Command("certutil.exe", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false, nil
	}

	return stdout.Len() > 0, nil
}

func (i *WSLInstaller) NeedsSudo() bool {
	// WSL installs to user store, no elevation needed
	return false
}

func (i *WSLInstaller) Description() string {
	return "Windows User Certificate Store via WSL (CurrentUser\\ROOT)"
}

// wslToWindowsPath converts a WSL path to a Windows path
func wslToWindowsPath(wslPath string) (string, error) {
	absPath, err := filepath.Abs(wslPath)
	if err != nil {
		return "", err
	}

	// For /mnt/c/... paths, convert directly to C:\...
	// This is more reliable than wslpath for certutil
	if strings.HasPrefix(absPath, "/mnt/") && len(absPath) > 6 {
		driveLetter := strings.ToUpper(string(absPath[5]))
		rest := absPath[6:] // e.g., /Users/xxx/...
		winPath := driveLetter + ":" + strings.ReplaceAll(rest, "/", "\\")
		return winPath, nil
	}

	// For other paths, try wslpath
	cmd := exec.Command("wslpath", "-w", absPath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Fallback: use \\wsl$\ path
		distro := getWSLDistroName()
		if distro != "" {
			return fmt.Sprintf(`\\wsl$\%s%s`, distro, strings.ReplaceAll(absPath, "/", `\`)), nil
		}
		return "", fmt.Errorf("cannot convert path: %s", absPath)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// getWSLDistroName returns the current WSL distribution name
func getWSLDistroName() string {
	// WSL_DISTRO_NAME environment variable is set in WSL2
	return os.Getenv("WSL_DISTRO_NAME")
}

// getWindowsTempDir returns a Windows temp directory accessible from WSL
func getWindowsTempDir() string {
	// Try common Windows temp locations via /mnt/c
	candidates := []string{
		"/mnt/c/Windows/Temp",
		"/mnt/c/Temp",
	}

	// Also try user's temp directory
	if user := os.Getenv("USER"); user != "" {
		candidates = append([]string{
			fmt.Sprintf("/mnt/c/Users/%s/AppData/Local/Temp", user),
		}, candidates...)
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	return ""
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// decodeShiftJIS converts Shift-JIS encoded bytes to UTF-8 string
// This is needed because Windows certutil outputs in Shift-JIS on Japanese systems
func decodeShiftJIS(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	decoder := japanese.ShiftJIS.NewDecoder()
	result, _, err := transform.Bytes(decoder, b)
	if err != nil {
		// If decoding fails, return as-is
		return string(b)
	}
	return string(result)
}
