---
title: "Static File Hosting"
slug: "static-sites"
description: "Serve static files without Docker containers."
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

roji can serve static files directly without Docker containers, using the `static_sites` section in the config file.

## Configuration

Add entries to `~/.config/roji/config.yaml`:

```yaml
static_sites:
  - host: docs                    # -> docs.dev.localhost
    root: ~/projects/docs/build
    # index: true                 # Directory listing (default: enabled)
  - host: private.example.com     # FQDN (dot in hostname)
    root: /var/www/private
    index: false                  # Disable directory listing
```

### Host Name Resolution

- **Without dots**: Expanded to `{host}.{ROJI_DOMAIN}` (e.g., `docs` becomes `docs.dev.localhost`)
- **With dots**: Used as-is as a fully qualified domain name

### Directory Listing

- `index: true` (default) — Shows an Apache/nginx-style directory listing when no `index.html` is found
- `index: false` — Returns 403 Forbidden for directory access without `index.html`

## Applying Changes

No restart needed. Use either:

```bash
roji config reload
```

Or click the **Reload Config** button on the dashboard.

## Adding Authentication

Protect static sites with Basic Authentication:

```yaml
static_sites:
  - host: docs.dev.localhost
    root: ~/projects/docs/build
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation   # optional
```

See the [Basic Authentication](/docs/guides/basic-auth/) guide for more details.

## Dashboard Integration

Static sites appear on the dashboard alongside Docker-based routes. The dashboard shows:

- Directory listing status icon
- Reload Config button to apply changes without restarting
