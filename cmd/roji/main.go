// roji is a simple reverse proxy for local development environments.
//
// It automatically discovers Docker Compose services and provides HTTPS access
// via *.dev.localhost domains.
//
// Usage:
//
//	roji [command]
//
// Available Commands:
//
//	help        Help about any command
//	routes      Display registered routes
//	version     Show version information
//	health      Check health status
//
// For more information, visit: https://github.com/kan/roji
package main

import (
	"fmt"
	"os"

	"github.com/kan/roji/cmd/roji/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
