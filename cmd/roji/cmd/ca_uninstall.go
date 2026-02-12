package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/kan/roji/certgen"
	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
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
		fmt.Printf("%s %s\n", yellow("⚠"), i18n.Tf("cmd.ca.uninstall.not_found", caCertPath))
		fmt.Println(i18n.T("cmd.ca.uninstall.no_file"))
		return nil
	}

	// Get the appropriate installer
	var installer certgen.CAInstaller
	if caUninstallWindows {
		// Uninstall from Windows via WSL
		if !certgen.IsWSL() {
			fmt.Printf("%s %s\n", red("✗"), i18n.T("cmd.ca.uninstall.wsl_only"))
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
		fmt.Printf("%s %s\n", yellow("⚠"), i18n.T("cmd.ca.uninstall.not_installed"))
		fmt.Println()
		return nil
	}

	// Show what we're about to do
	fmt.Println(i18n.Tf("cmd.ca.uninstall.removing", installer.Description()))
	if installer.NeedsSudo() {
		fmt.Println(i18n.T("cmd.ca.uninstall.needs_sudo"))
	}
	fmt.Println()

	// Uninstall the certificate
	if err := installer.Uninstall(caCertPath); err != nil {
		return fmt.Errorf("failed to uninstall CA certificate: %w", err)
	}

	fmt.Printf("%s %s\n", green("✓"), i18n.T("cmd.ca.uninstall.success"))
	fmt.Println()

	// Optionally uninstall from Firefox (Linux only)
	if caUninstallFirefox {
		fmt.Println(i18n.T("cmd.ca.uninstall.firefox_removing"))
		ffInstaller := certgen.NewFirefoxInstaller()
		if err := ffInstaller.Uninstall(caCertPath); err != nil {
			fmt.Printf("%s %s\n", red("✗"), i18n.Tf("cmd.ca.uninstall.firefox_failed", err))
		} else {
			fmt.Printf("%s %s\n", green("✓"), i18n.T("cmd.ca.uninstall.firefox_success"))
		}
		fmt.Println()
	}

	fmt.Println(i18n.T("cmd.ca.uninstall.restart_hint"))
	fmt.Println()

	return nil
}
