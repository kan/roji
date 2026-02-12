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

var caStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check CA certificate installation status",
	Long:  `Check whether the roji CA certificate exists and is installed in the system trust store.`,
	RunE:  runCAStatus,
}

func init() {
	caCmd.AddCommand(caStatusCmd)
}

func runCAStatus(cmd *cobra.Command, args []string) error {
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
	bold := color.New(color.Bold).SprintFunc()

	fmt.Println()
	fmt.Println(bold(i18n.T("cmd.ca.status.title")))
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()

	// Check if CA certificate exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		fmt.Printf("%s %s\n", red("✗"), i18n.T("cmd.ca.status.not_found"))
		fmt.Printf("  %s\n", i18n.Tf("cmd.ca.status.expected_at", caCertPath))
		fmt.Println()
		fmt.Println(i18n.T("cmd.ca.status.generate_hint"))
		fmt.Println()
		return nil
	}

	fmt.Printf("%s %s\n", green("✓"), i18n.T("cmd.ca.status.exists"))
	fmt.Printf("  %s\n", i18n.Tf("cmd.ca.status.location", caCertPath))

	// Get certificate subject
	subject, err := certgen.GetCASubject(caCertPath)
	if err == nil {
		fmt.Printf("  %s\n", i18n.Tf("cmd.ca.status.subject", subject))
	}

	fmt.Println()

	// Check system installation status
	installer := certgen.NewSystemInstaller()
	installed, err := installer.IsInstalled(caCertPath)
	if err != nil {
		fmt.Printf("%s %s\n", yellow("⚠"), i18n.Tf("cmd.ca.status.check_error", err))
	} else if installed {
		fmt.Printf("%s %s\n", green("✓"), i18n.T("cmd.ca.status.installed"))
		fmt.Printf("  %s\n", installer.Description())
	} else {
		fmt.Printf("%s %s\n", red("✗"), i18n.T("cmd.ca.status.not_installed"))
		fmt.Printf("  %s\n", installer.Description())
		fmt.Println()
		if installer.NeedsSudo() {
			fmt.Println(i18n.T("cmd.ca.status.install_hint_sudo"))
		} else {
			fmt.Println(i18n.T("cmd.ca.status.install_hint"))
		}
	}

	// If running on WSL, also check Windows installation status
	if certgen.IsWSL() {
		fmt.Println()
		wslInstaller := certgen.NewWSLInstaller()
		wslInstalled, err := wslInstaller.IsInstalled(caCertPath)
		if err != nil {
			fmt.Printf("%s %s\n", yellow("⚠"), i18n.Tf("cmd.ca.status.win_check_error", err))
		} else if wslInstalled {
			fmt.Printf("%s %s\n", green("✓"), i18n.T("cmd.ca.status.win_installed"))
			fmt.Printf("  %s\n", wslInstaller.Description())
		} else {
			fmt.Printf("%s %s\n", red("✗"), i18n.T("cmd.ca.status.win_not_installed"))
			fmt.Printf("  %s\n", wslInstaller.Description())
			fmt.Println()
			fmt.Println(i18n.T("cmd.ca.status.win_install_hint"))
		}
	}

	fmt.Println()
	return nil
}
