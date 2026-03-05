---
title: "Environment Variables"
description: "All ROJI_* environment variables."
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Variable Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `ROJI_NETWORK` | Docker network(s) to watch (comma-separated) | `roji` |
| `ROJI_DOMAIN` | Base domain for service hostnames | `dev.localhost` |
| `ROJI_HTTP_PORT` | HTTP port (redirects to HTTPS) | `80` |
| `ROJI_HTTPS_PORT` | HTTPS port | `443` |
| `ROJI_CERTS_DIR` | Directory for TLS certificates | `~/.local/share/roji/certs` |
| `ROJI_DATA_DIR` | Directory for data (project history, logs) | `~/.local/share/roji` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `ROJI_AUTO_CERT` | Auto-generate TLS certificates | `true` |

## Usage

```bash
# Run with custom settings
ROJI_DOMAIN=test.localhost ROJI_LOG_LEVEL=debug sudo roji

# Watch multiple networks
ROJI_NETWORK=web,api sudo roji

# Use alternative ports
ROJI_HTTP_PORT=8080 ROJI_HTTPS_PORT=8443 sudo roji
```

## Priority

Environment variables override config file values but are overridden by CLI flags:

**CLI flags** > **Environment variables** > **Config file** > **Defaults**

See [Configuration](/docs/guides/configuration/) for full details.
