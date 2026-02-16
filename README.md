# roji

<p align="center">
  <img src="proxy/templates/favicon.svg" alt="roji" width="120" height="120">
</p>

[![CI](https://github.com/kan/roji/actions/workflows/ci.yml/badge.svg)](https://github.com/kan/roji/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kan/roji)](https://github.com/kan/roji/blob/main/go.mod)
[![GoDoc](https://pkg.go.dev/badge/github.com/kan/roji.svg)](https://pkg.go.dev/github.com/kan/roji)
[![License](https://img.shields.io/github/license/kan/roji)](https://github.com/kan/roji/blob/main/LICENSE)

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
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.0/install.sh | bash
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

For a step-by-step walkthrough, see [Getting Started](#getting-started).

## Installation

### Homebrew (macOS)

```bash
brew install kan/roji/roji
```

### One-liner Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.0/install.sh | bash
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
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.0/install.sh | bash
```

### Manual Installation

Download from [GitHub Releases](https://github.com/kan/roji/releases) or build from source:

```bash
git clone https://github.com/kan/roji.git && cd roji && make build
sudo ./bin/roji doctor --fix    # Set up environment
sudo ./bin/roji ca install      # Install CA certificate
sudo ./bin/roji service install && sudo ./bin/roji service start
```

## Getting Started

### Prerequisites

- **Docker** with Docker Compose v2
- **roji** installed and running (see [Installation](#installation))

### Step 1: Verify your setup

```bash
roji doctor
```

All checks should pass. If not, run `sudo roji doctor --fix` to auto-repair.

### Step 2: Create your first project

Create a project with a web frontend and an API backend:

```yaml
# my-project/docker-compose.yml
services:
  web:
    image: nginx:alpine
    expose:
      - "80"
    networks:
      - roji

  api:
    image: node:alpine
    working_dir: /app
    command: ["node", "server.js"]
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

### Step 3: Start and access

```bash
cd my-project
docker compose up -d
```

Your services are now available at:
- `https://web.dev.localhost` — the nginx frontend
- `https://api.dev.localhost` — the Node.js API

### Step 4: Explore the dashboard

Open `https://roji.dev.localhost` to see the live dashboard with all your routes, request logs, and project controls.

### Common Patterns

**Custom hostname:**

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
```

**Path-based routing** (multiple services on one host):

```yaml
# https://myapp.dev.localhost/api/* -> api service
labels:
  - "roji.host=myapp.dev.localhost"
  - "roji.path=/api"
```

**Specific port** (when multiple ports are exposed):

```yaml
expose:
  - "3000"
  - "9229"   # debugger
labels:
  - "roji.port=3000"
```

**Multiple projects** — just connect each to the `roji` network. Each service gets its own `*.dev.localhost` subdomain automatically.

## Configuration

### Config File

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

static_sites:                          # Static file hosting (see below)
  - host: docs
    root: ~/projects/docs/build
```

Manage with `roji config show | path | init | edit`.

### Environment Variables

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

### Settings Priority

Highest to lowest: **CLI flags** > **Environment variables** > **Config file** > **Defaults**

### Docker Labels

| Label | Description | Default |
|-------|-------------|---------|
| `roji.host` | Custom hostname | `{service}.dev.localhost` |
| `roji.port` | Target port | First EXPOSE'd port |
| `roji.path` | Path prefix | none |
| `roji.mock.{METHOD}.{PATH}` | Mock response body | none |
| `roji.mock.status.{METHOD}.{PATH}` | Mock response status code | `200` |
| `roji.auth.basic.user` | Basic auth username | none |
| `roji.auth.basic.pass` | Basic auth password | none |
| `roji.auth.basic.realm` | Basic auth realm | `Restricted` |
| `roji.self` | Reserved: excludes container from routing (used internally) | none |

#### Label Examples

```yaml
services:
  # Custom hostname
  api:
    image: my-api
    labels:
      - "roji.host=api.dev.localhost"
    networks:
      - roji

  # Path-based routing: https://myapp.dev.localhost/api/* -> this service
  api-service:
    image: my-api
    labels:
      - "roji.host=myapp.dev.localhost"
      - "roji.path=/api"
    networks:
      - roji

  # Request mocking (returns mock JSON without a real backend)
  mock-api:
    image: alpine
    command: ["sleep", "infinity"]
    labels:
      - "roji.host=api.dev.localhost"
      - 'roji.mock.GET./api/users=[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]'
      - 'roji.mock.GET./api/health={"status":"ok"}'
      - "roji.mock.status.POST./api/users=201"
    networks:
      - roji
```

### Static File Hosting

Serve static files without Docker containers. Configure in `~/.config/roji/config.yaml`:

```yaml
static_sites:
  - host: docs                    # -> docs.dev.localhost
    root: ~/projects/docs/build
    # index: true                 # Directory listing (default: enabled)
  - host: private.example.com     # FQDN (dot in hostname)
    root: /var/www/private
    index: false                  # Disable directory listing
    auth:
      basic:
        user: admin
        pass: secret
        realm: Private Area
```

Use `roji config reload` or the dashboard's "Reload Config" button to apply changes without restarting.

### Basic Authentication

Protect routes with username/password. Via Docker labels:

```yaml
labels:
  - "roji.auth.basic.user=admin"
  - "roji.auth.basic.pass=secret"
  - "roji.auth.basic.realm=Admin Area"   # optional
```

Or via config file for static sites (see [Static File Hosting](#static-file-hosting) above).

## Dashboard

Access the dashboard at:
- `https://roji.dev.localhost` (dashboard host)
- `https://dev.localhost` (redirects to dashboard host)

The dashboard provides:

- **Live Route Updates**: Real-time updates via Server-Sent Events when containers start/stop
- **Browser Notifications**: Optional desktop notifications for route changes
- **Active Projects**: Currently running Docker Compose projects with start/stop/restart controls
- **Project History**: Recently stopped projects with one-click start or copy of restart commands
- **Project Auto-start**: Not Found page detects inactive projects and offers a start button
- **Request Log Viewer**: Real-time request logging with filtering by host and path
- **Log Export**: Download request logs as JSON or CSV
- **Container Restart**: Restart containers directly from the dashboard
- **Route Management**: Stop projects directly from the routes list
- **Dark Mode**: Toggle between light/dark themes or follow system preferences
- **System Status**: Build version, uptime, connection status

The dashboard automatically updates without page refresh.

## CLI Reference

### `roji`

Start the reverse proxy server:

```bash
sudo roji                           # Run in foreground
sudo roji --network web,api         # Watch multiple networks
sudo roji --domain test.localhost   # Custom domain
sudo roji --http-port 8080 --https-port 8443  # Custom ports
sudo roji --log-level debug         # Verbose logging
```

**Flags:** `--network`, `--domain`, `--http-port`, `--https-port`, `--certs-dir`, `--data-dir`, `--dashboard`, `--log-level`, `--auto-cert`, `--config`

### `roji doctor`

Check environment and fix common issues:

```bash
roji doctor          # Run all checks
roji doctor --fix    # Auto-fix issues where possible
roji doctor --json   # Output as JSON
```

Checks: Docker daemon, socket access, network existence, port availability, CA certificate, server certificate validity, DNS resolution, config file validation.

### `roji config`

Manage configuration:

```bash
roji config show     # Display current settings
roji config path     # Show config file locations
roji config init     # Create default config file
roji config edit     # Open in $EDITOR
```

### `roji ca`

Manage CA certificate:

```bash
roji ca status              # Check installation status
roji ca install             # Install to system trust store
roji ca install --user      # Install to user store (no sudo)
roji ca install --windows   # Install to Windows (from WSL)
roji ca install --firefox   # Also install to Firefox (Linux)
roji ca uninstall           # Remove from trust store
roji ca export [path]       # Export CA certificate (.pem or .crt)
```

### `roji service`

Manage roji as a system service:

```bash
roji service install    # Register as system service
roji service uninstall  # Remove service registration
roji service start      # Start the service
roji service stop       # Stop the service
roji service restart    # Restart the service
roji service status     # Show service status
```

Platform support:
- **Linux**: systemd (`/etc/systemd/system/roji.service`)
- **macOS**: launchd (`~/Library/LaunchAgents/com.roji.agent.plist`)
- **Windows**: NSSM-based Windows Service

### `roji log`

View server logs:

```bash
roji log               # Follow logs in real-time (like tail -f)
roji log -n 100        # Show last 100 lines, then follow
roji log --no-follow   # Print current logs and exit
```

Log file location:
- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

### `roji routes`

List all registered routes from the running server.

### `roji version`

Show version, commit hash, build date, and Go version.

### `roji health`

Check if the roji server is healthy. Exits with code 0 if healthy, 1 otherwise.

## API Reference

All API endpoints are served on the dashboard host (e.g., `https://roji.dev.localhost`).

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_api/health` | GET | Health check (`{"status":"healthy","routes":N}`) |
| `/healthz` | GET | Alias for `/_api/health` |
| `/_api/status` | GET | Detailed status (version, uptime, certificates, Docker) |
| `/_api/routes` | GET | List all registered routes |
| `/_api/projects` | GET | List active and inactive projects |
| `/_api/events` | GET | SSE stream for real-time route updates |
| `/_api/logs` | GET | Recent request logs (last 100) |
| `/_api/logs/events` | GET | SSE stream for real-time log updates |
| `/_api/logs/export` | GET | Export logs as JSON or CSV (query: `format`, `service`, `host`, `method`, `from`, `to`) |
| `/_api/containers/{id}/restart` | POST | Restart a container |
| `/_api/projects/{name}/up` | POST | Start project (`docker compose up -d`) |
| `/_api/projects/{name}/down` | POST | Stop project (`docker compose down`) |
| `/_api/projects/{name}/restart` | POST | Restart all services in project |
| `/_api/projects/{name}/logs` | GET | Stream project logs (SSE) |
| `/_api/projects/{name}/delete` | DELETE | Remove project from history |
| `/_api/config/reload` | POST | Reload config file (updates static sites) |

**Health status values:** `healthy` (all systems operational), `degraded` (certificates expiring within 30 days), `unhealthy` (Docker connection lost).

## Troubleshooting

### First Step: Run `roji doctor`

Always start with diagnostics:

```bash
roji doctor            # Check for issues
sudo roji doctor --fix # Auto-fix where possible
```

This checks Docker, networking, certificates, ports, and DNS — and can fix most common problems automatically.

### `.localhost` domain does not resolve

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

### Certificate errors (ERR_CERT_AUTHORITY_INVALID)

The CA certificate is not trusted by your browser or OS. Run:

```bash
sudo roji ca install           # Install to system trust store
```

Then **restart your browser completely** (close all windows).

**Firefox** uses its own certificate store. Either:
- Run `sudo roji ca install --firefox` (Linux, requires `nss-tools`)
- Or manually import: Firefox → Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import → select the CA certificate file

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

**Using mkcert** (alternative): If you prefer [mkcert](https://github.com/FiloSottile/mkcert), generate certificates to the roji certs directory before starting. roji will use existing certificates and skip auto-generation.

### Container not detected

1. Verify the container is connected to the `roji` network:
   ```bash
   docker network inspect roji
   ```

2. Check if the port is exposed (use `expose` in docker-compose.yml or `EXPOSE` in Dockerfile):
   ```bash
   docker inspect <container> | jq '.[0].Config.ExposedPorts'
   ```

3. Check the dashboard at `https://roji.dev.localhost` for configuration warnings.

### Ports 80 or 443 already in use

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

Or via environment variables: `ROJI_HTTP_PORT=8080 ROJI_HTTPS_PORT=8443`

### Platform: WSL

**Certificates**: Install in both Linux and Windows for browsers to trust HTTPS:

```bash
sudo roji ca install             # Linux
sudo roji ca install --windows   # Windows (for Chrome/Edge on Windows)
```

**PATH**: If installed to `~/.local/bin`, ensure it's in your PATH:

```bash
export PATH="$HOME/.local/bin:$PATH"  # Add to ~/.bashrc or ~/.zshrc
```

**Docker**: WSL2 with Docker Desktop works out of the box. Ensure Docker Desktop's WSL integration is enabled.

### Platform: macOS

**Keychain**: `roji ca install` adds the CA to the System Keychain (requires password). Use `--user` for the login keychain (no sudo).

**Docker Desktop**: Containers run in a Linux VM. roji connects via the Docker socket, which works transparently.

**Port permissions**: Ports below 1024 require `sudo`. The installer sets this up via `launchd`. For manual runs, use `sudo roji`.

### Platform: Linux

**Certificate stores**: `roji ca install` supports Debian/Ubuntu (`update-ca-certificates`) and RHEL/Fedora (`update-ca-trust`).

**Chrome/Chromium NSS**: Chrome uses its own certificate store on Linux. If you see certificate errors only in Chrome:

```bash
# Install nss-tools
sudo apt install libnss3-tools   # Debian/Ubuntu
sudo dnf install nss-tools       # Fedora

# Add CA to Chrome's store
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "roji CA" -i ~/.local/share/roji/certs/ca.pem
```

**Port permissions**: Ports below 1024 require root. Use `sudo roji` or the service installer (`roji service install` uses systemd with proper privileges).

### Viewing logs

```bash
roji log                # Follow logs in real-time
roji log -n 50          # Show last 50 lines
roji log --no-follow    # Print and exit
```

Log file locations:
- Linux/WSL: `~/.local/share/roji/roji.log`
- macOS: `~/Library/Logs/roji.log`

Logs are automatically rotated when they exceed 10MB.

## Name Origin

**roji** means "back alley" or "narrow lane" in Japanese. The concept is to use the highway (Traefik) for production and casually take the back alley (roji) for local development.

## License

MIT
