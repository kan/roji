---
title: "Linux"
description: "Linux-specific troubleshooting."
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Certificate Stores

`roji ca install` supports both major Linux families:

- **Debian/Ubuntu**: Uses `update-ca-certificates` (`/usr/local/share/ca-certificates/`)
- **RHEL/Fedora**: Uses `update-ca-trust` (`/etc/pki/ca-trust/source/anchors/`)

## Chrome/Chromium NSS

Chrome uses its own certificate store on Linux. If you see certificate errors only in Chrome:

```bash
# Install nss-tools
sudo apt install libnss3-tools   # Debian/Ubuntu
sudo dnf install nss-tools       # Fedora

# Add CA to Chrome's store
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "roji CA" -i ~/.local/share/roji/certs/ca.pem
```

Or use the built-in flag:

```bash
sudo roji ca install --firefox   # Also works for Chrome on Linux
```

## Port Permissions

Ports below 1024 require root. Use `sudo roji` or the service installer:

```bash
sudo roji service install  # Uses systemd with proper privileges
sudo roji service start
```

## systemd Issues

Check service status:

```bash
roji service status
systemctl status roji
journalctl -u roji -f    # View systemd logs
```

If the service fails to start, check the log file:

```bash
roji log --no-follow
```
