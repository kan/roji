package cmd

import (
	"fmt"
	"runtime"

	"github.com/kan/roji/service"
	"github.com/spf13/cobra"
)

var serviceUser string

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage roji as a system service",
	Long: `Manage roji as a system service.

Supported platforms:
  - Linux: systemd (/etc/systemd/system/roji.service)
  - macOS: launchd (~/Library/LaunchAgents/com.roji.agent.plist)
  - Windows: Windows Service (coming soon)

Linux requires root privileges (use sudo).
macOS runs as a user-level service (no sudo required).`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install roji as a system service",
	Long: `Install roji as a system service.

On Linux, this creates a systemd service unit and enables it to start on boot.
Requires root privileges (use sudo).`,
	RunE: runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall roji system service",
	Long: `Uninstall roji system service.

This stops the service (if running), disables auto-start, and removes the service configuration.
Requires root privileges (use sudo).`,
	RunE: runServiceUninstall,
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the roji service",
	RunE:  runServiceStart,
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the roji service",
	RunE:  runServiceStop,
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the roji service",
	RunE:  runServiceRestart,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show roji service status",
	RunE:  runServiceStatus,
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceStatusCmd)

	// Add --user flag to install command
	serviceInstallCmd.Flags().StringVar(&serviceUser, "user", "", "User to run the service as (default: current user)")
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManagerWithOptions(service.Options{
		User: serviceUser,
	})
	if err != nil {
		return err
	}

	fmt.Println("Installing roji service...")
	if err := mgr.Install(); err != nil {
		return err
	}

	fmt.Println("✓ Service installed successfully")
	fmt.Println()
	fmt.Println("To start the service:")
	switch runtime.GOOS {
	case "linux":
		fmt.Println("  sudo roji service start")
	case "darwin":
		fmt.Println("  roji service start")
	default:
		fmt.Println("  roji service start")
	}
	fmt.Println()
	fmt.Println("To view logs:")
	switch runtime.GOOS {
	case "linux":
		fmt.Println("  sudo journalctl -u roji -f")
	case "darwin":
		fmt.Println("  tail -f ~/Library/Logs/roji.log")
	}
	fmt.Println("  roji log")

	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println("Uninstalling roji service...")
	if err := mgr.Uninstall(); err != nil {
		return err
	}

	fmt.Println("✓ Service uninstalled successfully")

	return nil
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println("Starting roji service...")
	if err := mgr.Start(); err != nil {
		return err
	}

	fmt.Println("✓ Service started")

	return nil
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println("Stopping roji service...")
	if err := mgr.Stop(); err != nil {
		return err
	}

	fmt.Println("✓ Service stopped")

	return nil
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println("Restarting roji service...")
	if err := mgr.Restart(); err != nil {
		return err
	}

	fmt.Println("✓ Service restarted")

	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	status, err := mgr.Status()
	if err != nil {
		return err
	}

	fmt.Println("roji service status")
	fmt.Println("───────────────────")

	if !status.Installed {
		fmt.Println("Status: not installed")
		fmt.Println()
		fmt.Println("Run 'sudo roji service install' to install the service.")
		return nil
	}

	fmt.Printf("Installed: yes\n")

	if status.Running {
		fmt.Printf("Running:   yes\n")
	} else {
		fmt.Printf("Running:   no\n")
	}

	if status.Enabled {
		fmt.Printf("Enabled:   yes (auto-start on boot)\n")
	} else {
		fmt.Printf("Enabled:   no\n")
	}

	if status.Description != "" {
		fmt.Printf("\n%s\n", status.Description)
	}

	return nil
}
