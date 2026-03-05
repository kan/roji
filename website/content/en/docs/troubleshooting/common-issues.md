---
title: "Common Issues"
description: "Frequently encountered problems and solutions."
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## First Step: Run `roji doctor`

Always start with diagnostics:

```bash
roji doctor            # Check for issues
sudo roji doctor --fix # Auto-fix where possible
```

This checks Docker, networking, certificates, ports, and DNS — and can fix most common problems automatically.

## `.localhost` domain does not resolve

**macOS**: `.localhost` resolves to `127.0.0.1` automatically. No action needed.

**Linux (systemd-resolved)**: Most modern distributions resolve `.localhost` automatically. If not:

```bash
# Check if systemd-resolved handles it
resolvectl query myapp.dev.localhost

# If not, add entries to /etc/hosts
echo "127.0.0.1 myapp.dev.localhost roji.dev.localhost dev.localhost" | sudo tee -a /etc/hosts
```

**Firefox**: Firefox may not use the system DNS resolver. Go to `about:config` and set `browser.fixup.domainsuffixwhitelist.localhost` to `true`.

**Alternative**: Use `*.lvh.me` — a public domain that always resolves to `127.0.0.1`:

```yaml
# ~/.config/roji/config.yaml
domain: lvh.me
```

## Certificate errors (ERR_CERT_AUTHORITY_INVALID)

The CA certificate is not trusted by your browser or OS. Run:

```bash
sudo roji ca install           # Install to system trust store
```

Then **restart your browser completely** (close all windows).

**Firefox** uses its own certificate store. Either:

- Run `sudo roji ca install --firefox` (Linux, requires `nss-tools`)
- Or manually import: Firefox Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import

**WSL**: You need certificates in both Linux and Windows:

```bash
sudo roji ca install             # Linux trust store
sudo roji ca install --windows   # Windows trust store (for Windows browsers)
```

**Export CA certificate** for manual installation:

```bash
roji ca export ~/roji-ca.pem    # PEM format (macOS/Linux)
roji ca export ~/roji-ca.crt    # DER format (Windows)
```

## Container not detected

1. Verify the container is connected to the `roji` network:
   ```bash
   docker network inspect roji
   ```

2. Check if the port is exposed (use `expose` in docker-compose.yml or `EXPOSE` in Dockerfile):
   ```bash
   docker inspect <container> | jq '.[0].Config.ExposedPorts'
   ```

3. Check the dashboard at `https://roji.dev.localhost` for configuration warnings.

## Ports 80 or 443 already in use

Check what is using the port:

```bash
sudo lsof -i :80
sudo lsof -i :443
```

Common culprits: Apache, nginx, or another reverse proxy. Stop the conflicting service, or configure roji to use alternative ports:

```yaml
# ~/.config/roji/config.yaml
http_port: 8080
https_port: 8443
```

## Viewing logs

```bash
roji log                # Follow logs in real-time
roji log -n 50          # Show last 50 lines
roji log --no-follow    # Print and exit
```

Log file locations:

- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

Logs are automatically rotated when they exceed 10MB.
