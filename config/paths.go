package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the application name used for directory paths
	AppName = "roji"
)

// Paths holds the resolved paths for configuration and data directories
type Paths struct {
	ConfigDir string // Directory for configuration files
	DataDir   string // Directory for data files (projects.json, etc.)
	CertsDir  string // Directory for certificates
}

// DefaultPaths returns the default paths following XDG Base Directory specification
func DefaultPaths() *Paths {
	return &Paths{
		ConfigDir: ConfigDir(),
		DataDir:   DataDir(),
		CertsDir:  CertsDir(),
	}
}

// ConfigDir returns the configuration directory following XDG spec
// Linux/macOS: ~/.config/roji
// Windows: %APPDATA%\roji
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, AppName)
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", AppName)
	}

	// XDG_CONFIG_HOME or fallback to ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, AppName)
}

// DataDir returns the data directory following XDG spec
// Linux/macOS: ~/.local/share/roji
// Windows: %LOCALAPPDATA%\roji
func DataDir() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, AppName)
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", AppName)
	}

	// XDG_DATA_HOME or fallback to ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, AppName)
}

// CertsDir returns the certificates directory
// Defaults to DataDir()/certs
func CertsDir() string {
	return filepath.Join(DataDir(), "certs")
}

// ConfigFilePath returns the path to the configuration file
func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Exists checks if a file or directory exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
