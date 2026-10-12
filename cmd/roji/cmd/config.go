package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/kan/roji/apiclient"
	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor/checks"
	"github.com/kan/roji/i18n"
	"github.com/kan/roji/proxy"
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
		if err := runCommand(editor, path); err != nil {
			return err
		}

		// Saving the file changes nothing a running roji is doing. Reloading is
		// left to the user rather than done here: an edit is not always meant
		// to take effect at once.
		fmt.Println(i18n.T("cmd.config.edit.reload_hint"))
		return nil
	},
}

var configReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Apply config file changes to the running roji",
	Long: `Ask the running roji to read the config file again.

Only static_sites and proxy_sites take effect this way. Changes to anything
else, such as domain, bind or tunnel, need roji to be restarted.`,
	Args: cobra.NoArgs,
	RunE: runConfigReload,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configReloadCmd)
}

func runConfigReload(cmd *cobra.Command, args []string) error {
	// Nothing below is about how the command was typed.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	settings, err := config.Load(configFile, collectCLIOverrides(cmd))
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	resp, err := apiclient.SendJSON(cmd.Context(), apiclient.NewClient(10*time.Second), settings,
		http.MethodPost, "/_api/config/reload", nil)
	if err != nil {
		return errors.New(i18n.Tf("cmd.config.reload.unreachable", err))
	}
	defer resp.Body.Close()

	// The answer is small. Whatever else is at this address is not trusted to
	// keep it that way.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.New(i18n.Tf("cmd.config.reload.unreachable", err))
	}

	if resp.StatusCode != http.StatusOK {
		var failed struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &failed); err == nil && failed.Error != "" {
			// roji could not read the file at all: bad YAML, most likely.
			// Nothing was changed, so this is a failure.
			return errors.New(i18n.Tf("cmd.config.reload.failed", failed.Error))
		}
		// Not the reload endpoint's answer, so not the config file's fault.
		// The request went where the config file as it is now says roji is,
		// and a roji started before `domain`, `dashboard` or `https_port` was
		// edited is somewhere else.
		return errors.New(i18n.Tf("cmd.config.reload.unexpected", resp.StatusCode))
	}

	var reloaded proxy.ConfigReloadResponse
	if err := json.Unmarshal(body, &reloaded); err != nil {
		return errors.New(i18n.Tf("cmd.config.reload.unexpected", resp.StatusCode))
	}

	// The running roji's base domain is the one the sites were judged against.
	// One from before it said so is assumed to agree with the config file.
	if reloaded.Domain == "" {
		reloaded.Domain = settings.Domain
	}

	printReloadReport(cmd.OutOrStdout(), reloaded)
	return nil
}

// printReloadReport says what the reload did with each declared site.
//
// A site that was skipped does not make the command fail. The others were
// applied, and a script that reloads after editing should not stop because of
// a site that was already broken before the edit. It is printed where it
// cannot be missed instead.
func printReloadReport(w io.Writer, reloaded proxy.ConfigReloadResponse) {
	// Said last and every time: "config reload" reads as if it reloaded the
	// config, and most of the config is not something a reload applies.
	defer fmt.Fprintln(w, i18n.T("cmd.config.reload.scope"))

	// nil means the running roji is from before it reported sites.
	sites := reloaded.Sites
	if sites == nil {
		fmt.Fprintln(w, i18n.T("cmd.config.reload.done"))
		return
	}

	// Which file, since it is roji's and not necessarily the one just edited:
	// a roji started with --config, or as another user, reads its own.
	if reloaded.ConfigPath != "" {
		defer fmt.Fprintln(w, i18n.Tf("cmd.config.reload.file", reloaded.ConfigPath))
	}

	skipped := 0
	for _, site := range sites {
		if site.Skipped != "" {
			skipped++
		}
	}

	switch {
	case len(sites) == 0:
		fmt.Fprintln(w, i18n.T("cmd.config.reload.no_sites"))
	case skipped == 0:
		fmt.Fprintln(w, i18n.Tf("cmd.config.reload.summary", len(sites)))
	default:
		fmt.Fprintln(w, i18n.Tf("cmd.config.reload.summary_skipped", len(sites)-skipped, skipped))
	}

	// The wording for what is wrong with a site is roji doctor's, which
	// reports the same findings from the same source.
	for _, site := range sites {
		if site.Skipped != "" {
			fmt.Fprintf(w, "  ✗ %s\n", checks.SiteSkipMessage(site.Skipped, site.Hostname, site.Target, site.Detail))
			continue
		}

		fmt.Fprintf(w, "  ✓ %s -> %s\n", site.Hostname, site.Target)
		if site.CertUncovered {
			fmt.Fprintf(w, "  ⚠ %s\n", i18n.Tf("doctor.check.sites.cert", site.Hostname, reloaded.Domain))
		}
		if site.Unpublishable {
			fmt.Fprintf(w, "  ⚠ %s\n", i18n.Tf("doctor.check.sites.unpublishable", site.Hostname, reloaded.Domain))
		}
	}
}

// runCommand executes a command with arguments
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
