package cmd

import (
	"github.com/spf13/cobra"
)

var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Manage CA certificate",
	Long: `Manage the roji Certificate Authority (CA) certificate.

The CA certificate is used to sign server certificates for HTTPS.
Installing it into your system's trust store allows browsers to
trust roji-generated certificates without warnings.

Commands:
  status    - Check CA certificate installation status
  install   - Install CA certificate to system trust store
  uninstall - Remove CA certificate from system trust store
  export    - Export CA certificate to a file`,
}

func init() {
	rootCmd.AddCommand(caCmd)
}
