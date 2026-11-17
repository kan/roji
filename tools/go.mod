// Development tools (govulncheck). A module of their own, so they do not
// share a build list with roji: in the root go.mod, minimal version selection
// could let their dependencies raise the versions of the modules linked into
// the released binary. The release tools are in tools/release/go.mod.
//
// Run them with `go tool -modfile=tools/go.mod <tool>` from the repository
// root. The go line follows the root go.mod.
module github.com/kan/roji/tools

go 1.27.4

tool golang.org/x/vuln/cmd/govulncheck

require (
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
)
