# roji

<p align="center">
  <img src="proxy/templates/favicon.svg" alt="roji" width="120" height="120">
</p>

[![CI](https://github.com/kan/roji/actions/workflows/ci.yml/badge.svg)](https://github.com/kan/roji/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kan/roji)](https://github.com/kan/roji/blob/main/go.mod)
[![GoDoc](https://pkg.go.dev/badge/github.com/kan/roji.svg)](https://pkg.go.dev/github.com/kan/roji)
[![License](https://img.shields.io/github/license/kan/roji)](https://github.com/kan/roji/blob/main/LICENSE)
[![Docs](https://img.shields.io/badge/docs-roji--proxy.dev-blue)](https://roji-proxy.dev)

A simple reverse proxy for local development environments. Automatically discovers Docker Compose services and provides HTTPS access via `*.dev.localhost`.

> "Use the highway (Traefik) for production, take the back alley (roji) for development"

## Features

- **Auto-discovery**: Automatically detects and routes containers on the shared network
- **Auto HTTPS**: Generates and installs TLS certificates with zero configuration
- **Live Dashboard**: Real-time route updates, request logging, and project management
- **Docker Compose Operations**: Start/stop/restart projects from dashboard or API
- **Label-based Configuration**: Customize hostnames, ports, and paths via container labels
- **WebSocket & gRPC**: Full bidirectional WebSocket and HTTP/2 gRPC proxying
- **Request Mocking**: Define mock responses via labels for frontend development
- **Basic Authentication**: Protect routes with username/password via labels or config
- **Static File Hosting**: Serve static files with directory listing
- **Service Management**: Run as system service (systemd/launchd/Windows Service)
- **Environment Diagnostics**: `roji doctor` checks and fixes common issues

## Quick Start

Install roji:

```bash
# macOS
brew install kan/roji/roji

# Linux / macOS (one-liner)
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.8/install.sh | bash
```

Add your app to the `roji` network:

```yaml
# your-app/docker-compose.yml
services:
  myapp:
    image: your-app
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

Start your app and open `https://myapp.dev.localhost` — that's it!

For a step-by-step walkthrough, see the [Getting Started guide](https://roji-proxy.dev/docs/getting-started/installation/).

## Installation

### Homebrew (macOS)

```bash
brew install kan/roji/roji
```

### One-liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.8/install.sh | bash
```

This will:
- Download the roji binary for your platform (Linux/macOS, x86_64/arm64)
- Install to `~/.local/bin` by default (interactive prompt for location)
- Run `roji doctor --fix` to set up the environment
- Install CA certificate to system trust store
- Register and start roji as a system service

**Options:**

```bash
curl -fsSL ... | bash -s -- --global       # Install to /usr/local/bin
curl -fsSL ... | bash -s -- --local        # Install to ~/.local/bin (default)
curl -fsSL ... | bash -s -- --no-service   # Skip service registration
curl -fsSL ... | bash -s -- --upgrade      # Skip upgrade prompts
```

### Upgrading

Re-run the install script. It detects existing installations and upgrades automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.8/install.sh | bash
```

### Manual Installation

Download from [GitHub Releases](https://github.com/kan/roji/releases) or build from source:

```bash
git clone https://github.com/kan/roji.git && cd roji && make build
sudo ./bin/roji doctor --fix    # Set up environment
sudo ./bin/roji ca install      # Install CA certificate
sudo ./bin/roji service install && sudo ./bin/roji service start
```

## Configuration

Configuration file: `~/.config/roji/config.yaml`

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

static_sites:                          # Static file hosting
  - host: docs
    root: ~/projects/docs/build
```

Manage with `roji config show | path | init | edit`.

Settings priority: **CLI flags** > **Environment variables** > **Config file** > **Defaults**

For full details on environment variables, Docker labels, static sites, and authentication, see the [Configuration guide](https://roji-proxy.dev/docs/guide/configuration/).

## Dashboard

Access at `https://roji.dev.localhost` — a live dashboard with real-time route updates, request logging, Docker Compose controls, and project management. See the [Dashboard documentation](https://roji-proxy.dev/docs/guide/configuration/#dashboard) for details.

## CLI Reference

| Command | Description |
|---------|-------------|
| `roji` | Start the reverse proxy server |
| `roji doctor` | Check environment and fix issues (`--fix`, `--json`) |
| `roji config` | Manage configuration (`show`, `path`, `init`, `edit`) |
| `roji ca` | Manage CA certificate (`status`, `install`, `uninstall`, `export`) |
| `roji service` | Manage system service (`install`, `uninstall`, `start`, `stop`, `restart`, `status`) |
| `roji log` | View server logs (`-n`, `--no-follow`) |
| `roji routes` | List registered routes |
| `roji version` | Show version info |
| `roji health` | Check server health |

For full command reference with all flags, see the [CLI Reference](https://roji-proxy.dev/docs/reference/cli/).

## API Reference

All API endpoints are served on the dashboard host. See the [API Reference](https://roji-proxy.dev/docs/reference/api/) for the complete list.

## Troubleshooting

Start with diagnostics:

```bash
roji doctor            # Check for issues
sudo roji doctor --fix # Auto-fix where possible
```

This checks Docker, networking, certificates, ports, and DNS — and can fix most common problems automatically.

For platform-specific issues (WSL, macOS, Linux) and detailed solutions, see the [Troubleshooting guide](https://roji-proxy.dev/docs/troubleshooting/common-issues/).

## Documentation

Full documentation is available at **[roji-proxy.dev](https://roji-proxy.dev)**:

- [Getting Started](https://roji-proxy.dev/docs/getting-started/installation/) — Installation and first service
- [Configuration Guide](https://roji-proxy.dev/docs/guide/configuration/) — Config file, labels, static sites, auth
- [CLI Reference](https://roji-proxy.dev/docs/reference/cli/) — All commands and flags
- [API Reference](https://roji-proxy.dev/docs/reference/api/) — Dashboard API endpoints
- [Troubleshooting](https://roji-proxy.dev/docs/troubleshooting/common-issues/) — Common issues and platform guides

## Name Origin

**roji** means "back alley" or "narrow lane" in Japanese. The concept is to use the highway (Traefik) for production and casually take the back alley (roji) for local development.

## License

MIT
