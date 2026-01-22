package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/kan/roji/certgen"
	"github.com/kan/roji/config"
	"github.com/spf13/cobra"
)

var (
	caUninstallUser    bool
	caUninstallFirefox bool
	caUninstallWindows bool
)

var caUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove CA certificate from system trust store",
	Long: `Remove the roji CA certificate from the system trust store.

This revokes the trust for roji-generated HTTPS certificates.`,
	RunE: runCAUninstall,
}

func init() {
	caCmd.AddCommand(caUninstallCmd)
	caUninstallCmd.Flags().BoolVar(&caUninstallUser, "user", false,
		"Remove from user store instead of system store (macOS/Windows only)")
	caUninstallCmd.Flags().BoolVar(&caUninstallFirefox, "firefox", false,
		"Also remove from Firefox's certificate store (Linux only)")
	caUninstallCmd.Flags().BoolVar(&caUninstallWindows, "windows", false,
		"Remove from Windows certificate store (WSL only)")
}

func runCAUninstall(cmd *cobra.Command, args []string) error {
	// Load configuration
	overrides := collectCLIOverrides(cmd)
	settings, err := config.Load(configFile, overrides)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	certsDir := settings.CertsDir
	caCertPath := filepath.Join(certsDir, "ca.pem")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	// Check if CA certificate exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		fmt.Printf("%s CA certificate not found at: %s\n", yellow("⚠"), caCertPath)
		fmt.Println("Cannot uninstall without the original certificate file.")
		return nil
	}

	// Get the appropriate installer
	var installer certgen.CAInstaller
	if caUninstallWindows {
		// Uninstall from Windows via WSL
		if !certgen.IsWSL() {
			fmt.Printf("%s --windows flag is only available on WSL\n", red("✗"))
			return nil
		}
		// WSL always uses user store (cannot elevate to admin)
		installer = certgen.NewWSLInstaller()
	} else if caUninstallUser {
		installer = certgen.NewUserInstaller()
	} else {
		installer = certgen.NewSystemInstaller()
	}

	// Check if installed
	installed, _ := installer.IsInstalled(caCertPath)
	if !installed {
		fmt.Printf("%s CA certificate is not installed in system trust store\n", yellow("⚠"))
		fmt.Println()
		return nil
	}

	// Show what we're about to do
	fmt.Printf("Removing CA certificate from: %s\n", installer.Description())
	if installer.NeedsSudo() {
		fmt.Println("This operation requires elevated privileges.")
	}
	fmt.Println()

	// Uninstall the certificate
	if err := installer.Uninstall(caCertPath); err != nil {
		return fmt.Errorf("failed to uninstall CA certificate: %w", err)
	}

	fmt.Printf("%s CA certificate removed successfully\n", green("✓"))
	fmt.Println()

	// Optionally uninstall from Firefox (Linux only)
	if caUninstallFirefox {
		fmt.Println("Removing from Firefox...")
		ffInstaller := certgen.NewFirefoxInstaller()
		if err := ffInstaller.Uninstall(caCertPath); err != nil {
			fmt.Printf("%s Failed to remove from Firefox: %v\n", red("✗"), err)
		} else {
			fmt.Printf("%s Removed from Firefox\n", green("✓"))
		}
		fmt.Println()
	}

	fmt.Println("You may need to restart your browser for changes to take effect.")
	fmt.Println()

	return nil
}
