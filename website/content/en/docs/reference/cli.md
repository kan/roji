---
title: "CLI Reference"
slug: "cli"
description: "All roji commands and flags."
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## `roji`

Start the reverse proxy server.

```bash
sudo roji                                          # Run in foreground
sudo roji --network web,api                        # Watch multiple networks
sudo roji --domain test.localhost                   # Custom domain
sudo roji --http-port 8080 --https-port 8443       # Custom ports
sudo roji --log-level debug                        # Verbose logging
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--network` | Docker network(s) to watch | `roji` |
| `--domain` | Base domain | `dev.localhost` |
| `--http-port` | HTTP port | `80` |
| `--https-port` | HTTPS port | `443` |
| `--certs-dir` | Certificate directory | `~/.local/share/roji/certs` |
| `--data-dir` | Data directory | `~/.local/share/roji` |
| `--dashboard` | Dashboard hostname | `roji.{domain}` |
| `--log-level` | Log level (debug, info, warn, error) | `info` |
| `--auto-cert` | Auto-generate certificates | `true` |
| `--config` | Config file path | `~/.config/roji/config.yaml` |

## `roji doctor`

Check environment and fix common issues.

```bash
roji doctor          # Run all checks
roji doctor --fix    # Auto-fix issues where possible
roji doctor --json   # Output as JSON
```

**Checks performed:**

- Docker daemon running
- Docker socket accessible
- Docker network(s) exist
- Ports 80/443 available
- CA certificate exists and is valid
- CA certificate installed in system trust store
- Server certificate validity
- DNS resolution (*.localhost)
- Config file validation
- Tunnel (when configured: cloudflared installed, logged in, named tunnel exists)

## `roji config`

Manage configuration.

```bash
roji config show     # Display current settings
roji config path     # Show config file locations
roji config init     # Create default config file
roji config edit     # Open in $EDITOR
```

## `roji ca`

Manage the CA certificate.

```bash
roji ca status              # Check installation status
roji ca install             # Install to system trust store
roji ca install --user      # Install to user store (no sudo)
roji ca install --windows   # Install to Windows (from WSL)
roji ca install --firefox   # Also install to Firefox (Linux)
roji ca install --force     # Force reinstall
roji ca uninstall           # Remove from trust store
roji ca export [path]       # Export CA certificate (.pem or .crt)
```

**Platform support:**

| Platform | Method |
|----------|--------|
| macOS | Keychain (`security add-trusted-cert`) |
| Linux (Debian/Ubuntu) | `update-ca-certificates` |
| Linux (RHEL/Fedora) | `update-ca-trust` |
| Windows | `certutil -addstore` |
| WSL → Windows | `certutil.exe` (user store) |

## `roji service`

Manage roji as a system service.

```bash
roji service install    # Register as system service
roji service uninstall  # Remove service registration
roji service start      # Start the service
roji service stop       # Stop the service
roji service restart    # Restart the service
roji service status     # Show service status
```

**Platform support:**

| Platform | Service Manager | Config Location |
|----------|----------------|-----------------|
| Linux | systemd | `/etc/systemd/system/roji.service` |
| macOS | launchd | `~/Library/LaunchAgents/com.roji.agent.plist` |
| Windows | NSSM | Windows Service |

## `roji log`

View server logs.

```bash
roji log               # Follow logs in real-time (like tail -f)
roji log -n 100        # Show last 100 lines, then follow
roji log --no-follow   # Print current logs and exit
```

**Log file locations:**

- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

Logs are automatically rotated when they exceed 10MB.

## `roji routes`

List all registered routes from the running server.

## `roji version`

Show version, commit hash, build date, and Go version.

## `roji health`

Check if the roji server is healthy. Exits with code 0 if healthy, 1 otherwise.
