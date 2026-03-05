---
title: "Configuration"
description: "Configure roji via config file, environment variables, or CLI flags."
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

roji can be configured through a config file, environment variables, or CLI flags.

## Config File

Location: `~/.config/roji/config.yaml`

```yaml
network: roji                          # Docker network(s) to watch (comma-separated)
domain: dev.localhost                  # Base domain
http_port: 80                          # HTTP port (redirect to HTTPS)
https_port: 443                        # HTTPS port
certs_dir: ~/.local/share/roji/certs   # Certificate directory
data_dir: ~/.local/share/roji          # Data directory
dashboard: roji.dev.localhost          # Dashboard hostname
log_level: info                        # Log level (debug, info, warn, error)
auto_cert: true                        # Auto-generate certificates

static_sites:                          # Static file hosting (see Static Sites guide)
  - host: docs
    root: ~/projects/docs/build
```

### Managing the Config File

```bash
roji config show     # Display current settings
roji config path     # Show config file locations
roji config init     # Create default config file
roji config edit     # Open in $EDITOR
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ROJI_NETWORK` | Docker network(s) to watch (comma-separated) | `roji` |
| `ROJI_DOMAIN` | Base domain | `dev.localhost` |
| `ROJI_HTTP_PORT` | HTTP port | `80` |
| `ROJI_HTTPS_PORT` | HTTPS port | `443` |
| `ROJI_CERTS_DIR` | Certificate directory | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | Data directory (project history) | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | Log level | `info` |
| `ROJI_AUTO_CERT` | Auto-generate certificates | `true` |

## Settings Priority

Settings are applied in this order (highest priority first):

1. **CLI flags** (`--network`, `--domain`, etc.)
2. **Environment variables** (`ROJI_NETWORK`, `ROJI_DOMAIN`, etc.)
3. **Config file** (`~/.config/roji/config.yaml`)
4. **Defaults**

## Config File Validation

roji validates the config file on startup and via `roji doctor`:

- **Unknown keys**: Detects typos in key names
- **Type checking**: Validates value types (string, int, bool, array, object)
- **Required fields**: Checks for missing required fields in `static_sites`

Warnings are logged during startup. Run `roji doctor` for a full validation report.
