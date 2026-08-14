---
title: "Installation"
description: "Install roji on macOS, Linux, or WSL."
weight: 1
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Homebrew (macOS)

```bash
brew install kan/roji/roji
```

## One-liner Install (Recommended)

Works on Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.2.0/install.sh | bash
```

This will:

- Download the roji binary for your platform (Linux/macOS, x86_64/arm64)
- Install to `~/.local/bin` by default (interactive prompt for location)
- Run `roji doctor --fix` to set up the environment
- Install CA certificate to system trust store
- Register and start roji as a system service

### Options

```bash
curl -fsSL ... | bash -s -- --global       # Install to /usr/local/bin
curl -fsSL ... | bash -s -- --local        # Install to ~/.local/bin (default)
curl -fsSL ... | bash -s -- --no-service   # Skip service registration
curl -fsSL ... | bash -s -- --upgrade      # Skip upgrade prompts
```

## Manual Installation

Download from [GitHub Releases](https://github.com/kan/roji/releases) or build from source:

```bash
git clone https://github.com/kan/roji.git && cd roji && make build
sudo ./bin/roji doctor --fix    # Set up environment
sudo ./bin/roji ca install      # Install CA certificate
sudo ./bin/roji service install && sudo ./bin/roji service start
```

## Upgrading

Re-run the install script. It detects existing installations and upgrades automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.2.0/install.sh | bash
```

Or with Homebrew:

```bash
brew upgrade kan/roji/roji
```

## Verifying Installation

```bash
roji version    # Show version info
roji doctor     # Check environment
```

All doctor checks should pass. If not, run `sudo roji doctor --fix` to auto-repair.

## Next Steps

- [Quick Start](/docs/getting-started/quick-start/) — Get your first service running
- [Configuration](/docs/guides/configuration/) — Customize roji settings
