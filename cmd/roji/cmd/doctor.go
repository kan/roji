package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/doctor/checks"
	"github.com/spf13/cobra"
)

var (
	doctorFix  bool
	doctorJSON bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system environment for common issues",
	Long: `Check your system environment for common issues that may prevent roji from working correctly.

The doctor command runs a series of diagnostic checks:
  - Docker daemon status
  - Docker socket accessibility
  - Docker network existence
  - Port availability (80, 443)
  - CA certificate status
  - Server certificate validity
  - DNS resolution for *.localhost

Use --fix to automatically fix issues where possible.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Auto-fix issues where possible")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output results as JSON")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	overrides := collectCLIOverrides(cmd)
	settings, err := config.Load(configFile, overrides)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine config file path
	configPath := configFile
	if configPath == "" {
		configPath = config.ConfigFilePath()
	}

	// Create doctor configuration
	cfg := &doctor.Config{
		Settings:   settings,
		CertsDir:   settings.CertsDir,
		ConfigPath: configPath,
	}

	// Create doctor with all checks
	d := doctor.New(cfg)
	d.AddCheck(&checks.Config{})
	d.AddCheck(&checks.DockerDaemon{})
	d.AddCheck(&checks.DockerSocket{})
	d.AddCheck(&checks.Network{})
	d.AddCheck(&checks.Ports{})
	d.AddCheck(&checks.CACert{})
	d.AddCheck(&checks.CAInstall{})
	d.AddCheck(&checks.ServerCert{})
	d.AddCheck(&checks.DNS{})

	// Run checks (or fix)
	var results []doctor.CheckResult
	if doctorFix {
		results, err = d.Fix(ctx)
		if err != nil {
			return fmt.Errorf("failed to run doctor: %w", err)
		}
	} else {
		results = d.RunAll(ctx)
	}

	// Output results
	if doctorJSON {
		return outputJSON(results)
	}
	return outputPretty(results)
}

func outputJSON(results []doctor.CheckResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func outputPretty(results []doctor.CheckResult) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Println()
	fmt.Println(bold("roji doctor"))
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()

	for _, r := range results {
		var icon, status string
		switch r.Status {
		case doctor.Pass:
			icon = green("✓")
			status = green("pass")
		case doctor.Warn:
			icon = yellow("⚠")
			status = yellow("warn")
		case doctor.Fail:
			icon = red("✗")
			status = red("fail")
		}

		fmt.Printf("%s %s [%s]\n", icon, r.Name, status)
		fmt.Printf("  %s\n", r.Message)
		if r.Details != "" {
			fmt.Printf("  %s\n", color.HiBlackString(r.Details))
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("─────────────────────────────────────────")

	failCount := 0
	warnCount := 0
	for _, r := range results {
		switch r.Status {
		case doctor.Fail:
			failCount++
		case doctor.Warn:
			warnCount++
		}
	}

	if failCount > 0 {
		fmt.Printf("%s %d error(s), %d warning(s)\n", red("✗"), failCount, warnCount)
		fmt.Println()
		fmt.Println("Run 'roji doctor --fix' to auto-fix where possible.")
		os.Exit(2)
	} else if warnCount > 0 {
		fmt.Printf("%s %d warning(s)\n", yellow("⚠"), warnCount)
		os.Exit(1)
	} else {
		fmt.Printf("%s All checks passed!\n", green("✓"))
	}

	fmt.Println()
	return nil
}
