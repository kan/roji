package cmd

import (
	"fmt"
	"runtime"

	"github.com/kan/roji/i18n"
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
  - Windows: NSSM (requires NSSM from https://nssm.cc/)

Linux requires root privileges (use sudo).
macOS runs as a user-level service (no sudo required).
Windows requires Administrator privileges.`,
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

	fmt.Println(i18n.T("cmd.service.install.installing"))
	if err := mgr.Install(); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.install.success"))
	fmt.Println()
	fmt.Println(i18n.T("cmd.service.install.start_hint"))
	switch runtime.GOOS {
	case "linux":
		fmt.Println(i18n.T("cmd.service.install.start_linux"))
	case "darwin":
		fmt.Println(i18n.T("cmd.service.install.start_macos"))
	case "windows":
		fmt.Println(i18n.T("cmd.service.install.start_windows"))
	default:
		fmt.Println(i18n.T("cmd.service.install.start_macos"))
	}
	fmt.Println()
	fmt.Println(i18n.T("cmd.service.install.log_hint"))
	switch runtime.GOOS {
	case "linux":
		fmt.Println(i18n.T("cmd.service.install.log_linux"))
	case "darwin":
		fmt.Println(i18n.T("cmd.service.install.log_macos"))
	case "windows":
		fmt.Println(i18n.T("cmd.service.install.log_windows"))
	}
	fmt.Println(i18n.T("cmd.service.install.log_roji"))

	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.uninstall.uninstalling"))
	if err := mgr.Uninstall(); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.uninstall.success"))

	return nil
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.start.starting"))
	if err := mgr.Start(); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.start.success"))

	return nil
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.stop.stopping"))
	if err := mgr.Stop(); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.stop.success"))

	return nil
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.restart.restarting"))
	if err := mgr.Restart(); err != nil {
		return err
	}

	fmt.Println(i18n.T("cmd.service.restart.success"))

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

	fmt.Println(i18n.T("cmd.service.status.title"))
	fmt.Println("───────────────────")

	if !status.Installed {
		fmt.Println(i18n.T("cmd.service.status.not_installed"))
		fmt.Println()
		fmt.Println(i18n.T("cmd.service.status.install_hint"))
		return nil
	}

	fmt.Println(i18n.T("cmd.service.status.installed"))

	if status.Running {
		fmt.Println(i18n.T("cmd.service.status.running_yes"))
	} else {
		fmt.Println(i18n.T("cmd.service.status.running_no"))
	}

	if status.Enabled {
		fmt.Println(i18n.T("cmd.service.status.enabled_yes"))
	} else {
		fmt.Println(i18n.T("cmd.service.status.enabled_no"))
	}

	if status.Description != "" {
		fmt.Printf("\n%s\n", status.Description)
	}

	return nil
}
