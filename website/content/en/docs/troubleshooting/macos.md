---
title: "macOS"
description: "macOS-specific troubleshooting."
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Keychain

`roji ca install` adds the CA to the System Keychain (requires password). Use `--user` for the login keychain (no sudo needed):

```bash
roji ca install --user    # Login keychain (no sudo)
sudo roji ca install      # System keychain
```

## Docker Desktop

Containers run in a Linux VM. roji connects via the Docker socket, which works transparently. No special configuration needed.

## Port Permissions

Ports below 1024 require root. The installer sets this up via `launchd`. For manual runs:

```bash
sudo roji
```

Or use alternative ports:

```yaml
# ~/.config/roji/config.yaml
http_port: 8080
https_port: 8443
```
