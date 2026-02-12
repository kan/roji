package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage roji configuration",
	Long:  `Manage roji configuration files and settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Show the current configuration values after merging defaults, config file, environment variables, and CLI flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		overrides := collectCLIOverrides(cmd)
		settings, err := config.Load(configFile, overrides)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		yaml, err := settings.ToYAML()
		if err != nil {
			return fmt.Errorf("failed to format configuration: %w", err)
		}

		fmt.Print(yaml)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file paths",
	Long:  `Show the paths where roji looks for configuration files and stores data.`,
	Run: func(cmd *cobra.Command, args []string) {
		paths := config.DefaultPaths()
		fmt.Println(i18n.Tf("cmd.config.path.config_file", config.ConfigFilePath()))
		fmt.Println(i18n.Tf("cmd.config.path.config_dir", paths.ConfigDir))
		fmt.Println(i18n.Tf("cmd.config.path.data_dir", paths.DataDir))
		fmt.Println(i18n.Tf("cmd.config.path.certs_dir", paths.CertsDir))
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	Long:  `Create a default configuration file at ~/.config/roji/config.yaml (or the path specified by --config).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configFile
		if path == "" {
			path = config.ConfigFilePath()
		}

		// Check if file already exists
		if config.Exists(path) {
			return fmt.Errorf("configuration file already exists: %s\nUse 'roji config show' to view current settings", path)
		}

		// Create default settings and save
		settings := config.Defaults()
		if err := settings.SaveToFile(path); err != nil {
			return err
		}

		fmt.Println(i18n.Tf("cmd.config.init.success", path))
		fmt.Println(i18n.T("cmd.config.init.edit_hint"))
		fmt.Println(i18n.T("cmd.config.init.show_hint"))
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration file in editor",
	Long:  `Open the configuration file in your default editor ($EDITOR or vi).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configFile
		if path == "" {
			path = config.ConfigFilePath()
		}

		// Check if file exists, create if not
		if !config.Exists(path) {
			fmt.Println(i18n.Tf("cmd.config.edit.creating", path))
			settings := config.Defaults()
			if err := settings.SaveToFile(path); err != nil {
				return err
			}
		}

		// Get editor from environment
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			editor = "vi"
		}

		// Open in editor
		return runCommand(editor, path)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configEditCmd)
}

// runCommand executes a command with arguments
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
