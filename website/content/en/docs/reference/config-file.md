---
title: "Config File"
description: "Complete reference for config.yaml."
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## File Location

- Default: `~/.config/roji/config.yaml`
- Follows the XDG Base Directory specification
- Override with `--config` flag or check with `roji config path`

## Full Reference

```yaml
# ~/.config/roji/config.yaml

# Docker network(s) to watch (comma-separated for multiple)
network: roji

# Base domain for service hostnames
# Services get {service}.{domain} hostnames
domain: dev.localhost

# HTTP port (all requests redirect to HTTPS)
http_port: 80

# HTTPS port
https_port: 443

# Directory for TLS certificates (CA + server certs)
certs_dir: ~/.local/share/roji/certs

# Directory for data (project history, logs)
data_dir: ~/.local/share/roji

# Dashboard hostname
dashboard: roji.dev.localhost

# Log level: debug, info, warn, error
log_level: info

# Auto-generate TLS certificates on startup
auto_cert: true

# Static file hosting
static_sites:
  - host: docs                    # Short name -> docs.dev.localhost
    root: ~/projects/docs/build   # Directory to serve
    index: true                   # Directory listing (default: true)
  - host: private.example.com    # FQDN (contains dot)
    root: /var/www/private
    index: false
    auth:                         # Basic authentication
      basic:
        user: admin
        pass: secret
        realm: Private Area       # Optional, defaults to "Restricted"
```

## Managing Config

```bash
roji config init     # Create default config file
roji config show     # Display current effective settings
roji config path     # Show config file location
roji config edit     # Open in $EDITOR
```

## Validation

roji validates the config file on startup:

- **Unknown keys** are reported as warnings (catches typos)
- **Type errors** are reported (e.g., string where int expected)
- **Missing required fields** in `static_sites` (host, root) are flagged

Run `roji doctor` for a full validation report.
