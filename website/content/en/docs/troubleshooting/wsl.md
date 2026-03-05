---
title: "WSL"
description: "WSL-specific troubleshooting."
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Certificates

You need to install the CA certificate in **both** Linux and Windows for browsers to trust HTTPS:

```bash
sudo roji ca install             # Linux trust store
sudo roji ca install --windows   # Windows trust store (for Chrome/Edge on Windows)
```

The `--windows` flag uses `certutil.exe` to install the certificate in the Windows user certificate store (CurrentUser\ROOT).

## PATH

If installed to `~/.local/bin`, ensure it's in your PATH:

```bash
export PATH="$HOME/.local/bin:$PATH"  # Add to ~/.bashrc or ~/.zshrc
```

## Docker

WSL2 with Docker Desktop works out of the box. Ensure Docker Desktop's WSL integration is enabled:

1. Docker Desktop → Settings → Resources → WSL Integration
2. Enable for your WSL distribution

The Docker socket is shared between WSL and Docker Desktop automatically.
