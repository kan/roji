package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kan/roji/apiclient"
	"github.com/kan/roji/config"
	"github.com/spf13/cobra"
)

// apiGet sends a GET request to the local roji API endpoint at path.
//
// Settings are resolved from the config file, environment variables and
// defaults (plus any CLI overrides the invoking command defines), so a custom
// https_port or dashboard hostname is honored. The caller is responsible for
// closing the response body.
func apiGet(cmd *cobra.Command, path string, timeout time.Duration) (*http.Response, error) {
	settings, err := config.Load(configFile, collectCLIOverrides(cmd))
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return apiclient.Get(settings, path, timeout)
}
