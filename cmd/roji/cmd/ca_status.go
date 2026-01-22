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
	fmt.Println(bold("CA Certificate Status"))
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()

	// Check if CA certificate exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		fmt.Printf("%s CA certificate not found\n", red("✗"))
		fmt.Printf("  Expected at: %s\n", caCertPath)
		fmt.Println()
		fmt.Println("Run 'roji' to auto-generate certificates, or generate manually.")
		fmt.Println()
		return nil
	}

	fmt.Printf("%s CA certificate exists\n", green("✓"))
	fmt.Printf("  Location: %s\n", caCertPath)

	// Get certificate subject
	subject, err := certgen.GetCASubject(caCertPath)
	if err == nil {
		fmt.Printf("  Subject: %s\n", subject)
	}

	fmt.Println()

	// Check system installation status
	installer := certgen.NewSystemInstaller()
	installed, err := installer.IsInstalled(caCertPath)
	if err != nil {
		fmt.Printf("%s Cannot check installation status: %v\n", yellow("⚠"), err)
	} else if installed {
		fmt.Printf("%s Installed in system trust store\n", green("✓"))
		fmt.Printf("  %s\n", installer.Description())
	} else {
		fmt.Printf("%s Not installed in system trust store\n", red("✗"))
		fmt.Printf("  %s\n", installer.Description())
		fmt.Println()
		if installer.NeedsSudo() {
			fmt.Println("Run 'roji ca install' (requires elevated privileges)")
		} else {
			fmt.Println("Run 'roji ca install' to install")
		}
	}

	// If running on WSL, also check Windows installation status
	if certgen.IsWSL() {
		fmt.Println()
		wslInstaller := certgen.NewWSLInstaller()
		wslInstalled, err := wslInstaller.IsInstalled(caCertPath)
		if err != nil {
			fmt.Printf("%s Cannot check Windows installation status: %v\n", yellow("⚠"), err)
		} else if wslInstalled {
			fmt.Printf("%s Installed in Windows user trust store\n", green("✓"))
			fmt.Printf("  %s\n", wslInstaller.Description())
		} else {
			fmt.Printf("%s Not installed in Windows user trust store\n", red("✗"))
			fmt.Printf("  %s\n", wslInstaller.Description())
			fmt.Println()
			fmt.Println("Run 'roji ca install --windows' to install to Windows")
		}
	}

	fmt.Println()
	return nil
}
