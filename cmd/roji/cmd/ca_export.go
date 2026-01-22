package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/kan/roji/certgen"
	"github.com/kan/roji/config"
	"github.com/spf13/cobra"
)

var caExportCmd = &cobra.Command{
	Use:   "export [path]",
	Short: "Export CA certificate to a file",
	Long: `Export the roji CA certificate to a file.

Supported formats (determined by file extension):
  .pem  - PEM format (default, used by most Unix systems)
  .crt  - DER format (used by Windows)
  .cer  - DER format (alternative extension)

If no path is specified, exports to the current directory as roji-ca.pem.

Examples:
  roji ca export                    # Export as ./roji-ca.pem
  roji ca export ~/roji-ca.pem     # Export to home directory
  roji ca export ca.crt            # Export in DER format for Windows`,
	RunE: runCAExport,
}

func init() {
	caCmd.AddCommand(caExportCmd)
}

func runCAExport(cmd *cobra.Command, args []string) error {
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

	// Check if CA certificate exists
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		fmt.Printf("%s CA certificate not found at: %s\n", red("✗"), caCertPath)
		fmt.Println()
		fmt.Println("Run 'roji' to auto-generate certificates first.")
		return nil
	}

	// Determine output path
	outputPath := "roji-ca.pem"
	if len(args) > 0 {
		outputPath = args[0]
	}

	// Expand ~ in path
	if strings.HasPrefix(outputPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			outputPath = filepath.Join(home, outputPath[2:])
		}
	}

	// Check if output file already exists
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("output file already exists: %s\nUse a different path or remove the existing file", outputPath)
	}

	// Determine format based on extension
	ext := strings.ToLower(filepath.Ext(outputPath))
	var exportErr error

	switch ext {
	case ".crt", ".cer":
		// DER format
		exportErr = certgen.ExportToDER(caCertPath, outputPath)
	default:
		// PEM format (default)
		exportErr = certgen.ExportToPEM(caCertPath, outputPath)
	}

	if exportErr != nil {
		return fmt.Errorf("failed to export certificate: %w", exportErr)
	}

	fmt.Printf("%s CA certificate exported to: %s\n", green("✓"), outputPath)

	switch ext {
	case ".crt", ".cer":
		fmt.Println()
		fmt.Println("This is a DER-encoded certificate (binary format).")
		fmt.Println("On Windows, double-click to install, or use:")
		fmt.Println("  certutil -addstore ROOT " + outputPath)
	default:
		fmt.Println()
		fmt.Println("This is a PEM-encoded certificate (text format).")
		fmt.Println("Import this file into your browser or OS trust store.")
	}

	fmt.Println()
	return nil
}
