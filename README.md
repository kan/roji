# roji

<p align="center">
  <img src="proxy/templates/favicon.svg" alt="roji" width="120" height="120">
</p>

[![CI](https://github.com/kan/roji/actions/workflows/ci.yml/badge.svg)](https://github.com/kan/roji/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kan/roji)](https://github.com/kan/roji/blob/main/go.mod)
[![GoDoc](https://pkg.go.dev/badge/github.com/kan/roji.svg)](https://pkg.go.dev/github.com/kan/roji)
[![License](https://img.shields.io/github/license/kan/roji)](https://github.com/kan/roji/blob/main/LICENSE)
[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fkan%2Froji-blue?logo=docker)](https://github.com/kan/roji/pkgs/container/roji)

A simple reverse proxy for local development environments. Automatically discovers Docker Compose services and provides HTTPS access via `*.dev.localhost`.

> "Use the highway (Traefik) for production, take the back alley (roji) for development"

## Features

- **Native Mode**: Run as a standalone binary without Docker (v0.8.0+)
- **Auto-discovery**: Automatically detects and routes containers on the shared network
- **TLS Support**: Auto-generates certificates (no mkcert required) or use your own
- **Label-based Configuration**: Customize hostnames and ports via container labels
- **Dynamic Updates**: Automatically tracks container start/stop events
- **Live Dashboard**: Real-time route updates via Server-Sent Events with optional browser notifications
- **Project History**: Tracks active and recent Docker Compose projects with quick restart commands
- **Dark Mode**: Automatic theme switching based on system preferences with manual toggle
- **Request Logging**: Real-time request log viewer with filtering and JSON/CSV export
- **WebSocket Support**: Full bidirectional WebSocket proxying
- **gRPC Support**: HTTP/2 based gRPC proxying with streaming support
- **Multiple Networks**: Monitor multiple Docker networks simultaneously
- **Container Management**: Restart containers directly from the dashboard
- **Docker Compose Operations**: Start/stop/restart projects from dashboard or API
- **Request Mocking**: Define mock responses via labels for frontend development
- **Basic Authentication**: Protect routes with username/password via labels or config
- **Static File Hosting**: Serve static files with directory listing
- **Service Management**: Run as system service (systemd/launchd/Windows Service)
- **Environment Diagnostics**: `roji doctor` checks and fixes common issues
- **Simple**: Minimal implementation focused on local development

## Installation

### One-liner Install (Recommended)

Install and start roji with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.9.0/install.sh | bash
```

This will:
- Download the roji binary for your platform (Linux/macOS, x86_64/arm64)
- Install to `~/.local/bin` by default (interactive prompt for location)
- Run `roji doctor --fix` to set up the environment
- Install CA certificate to system trust store
- Register and start roji as a system service

**Installation options:**

```bash
# Install to /usr/local/bin (system-wide)
curl -fsSL ... | bash -s -- --global

# Install to ~/.local/bin (default, no sudo for install)
curl -fsSL ... | bash -s -- --local

# Skip service installation (manual start with `sudo roji`)
curl -fsSL ... | bash -s -- --no-service
```

### Upgrading

The install script automatically detects existing installations:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.9.0/install.sh | bash
```

- **Same version**: Shows "already up to date" and exits
- **Newer available**: Prompts to upgrade (auto-upgrades in piped mode)
- **Docker Mode detected**: Offers migration to Native Mode

**Force upgrade (skip prompts):**

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.9.0/install.sh | bash -s -- --upgrade
```

### Manual Installation

Download from [GitHub Releases](https://github.com/kan/roji/releases) or build from source:

```bash
# Build from source
git clone https://github.com/kan/roji.git
cd roji
make build

# Run diagnostics and fix issues
sudo ./bin/roji doctor --fix

# Install CA certificate to system trust store
sudo ./bin/roji ca install

# Option 1: Run as service (recommended)
sudo ./bin/roji service install
sudo ./bin/roji service start

# Option 2: Run in foreground
sudo ./bin/roji
```

Configuration is stored in `~/.config/roji/config.yaml`. See [CLI Commands](#cli-commands) for more details.

### Docker Mode (Legacy)

For Docker-based installation, use the legacy installer:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.9.0/install-docker.sh | bash
```

This installs roji as a Docker container with docker-compose. Note that Native Mode (above) is now the recommended approach.

**Manual Docker setup:**

```bash
# Clone repository
git clone https://github.com/kan/roji.git
cd roji

# Create network and start
docker network create roji
docker compose up -d
```

**For development**: Use `docker compose -f docker-compose.dev.yml up` for hot-reloading with [Air](https://github.com/cosmtrek/air).

Certificates are **automatically generated** on first startup. See [TLS Certificates](#tls-certificates) for how to trust them.

#### 5. Start your application

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

Your app is now accessible at `https://myapp.dev.localhost`!

## TLS Certificates

### Auto-generated Certificates (Default)

roji automatically generates TLS certificates on first startup:

```
certs/
├── ca.crt      # CA certificate (Windows)
├── ca.pem      # CA certificate (macOS/Linux)
├── ca-key.pem  # CA private key
├── cert.pem    # Server certificate
└── key.pem     # Server private key
```

To trust HTTPS connections, install the CA certificate in your OS/browser:

#### Windows

1. Double-click `certs/ca.crt`
2. Click **"Install Certificate"**
3. Select **"Local Machine"** (requires admin) or "Current User"
4. Select **"Place all certificates in the following store"**
5. Click **"Browse"** → Select **"Trusted Root Certification Authorities"**
6. Click "Next" → "Finish"
7. **Restart your browser**

#### macOS

```bash
# Add to system keychain (requires password)
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain certs/ca.pem

# Or open in Keychain Access and set "Always Trust"
open certs/ca.pem
```

#### Linux (Chrome/Chromium)

```bash
# Install certutil if needed
# Debian/Ubuntu: sudo apt install libnss3-tools
# Fedora: sudo dnf install nss-tools

# Add to Chrome/Chromium certificate store
certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "roji CA" -i certs/ca.pem
```

#### Firefox (All platforms)

Firefox uses its own certificate store:

1. Open Firefox → Settings → Privacy & Security
2. Scroll to "Certificates" → Click "View Certificates"
3. Go to "Authorities" tab → Click "Import"
4. Select `certs/ca.pem` (or `ca.crt` on Windows)
5. Check "Trust this CA to identify websites"
6. Click OK

### Using mkcert (Alternative)

If you prefer [mkcert](https://github.com/FiloSottile/mkcert), generate certificates before starting roji:

```bash
mkcert -install
mkdir -p certs
mkcert -cert-file certs/cert.pem -key-file certs/key.pem \
  "*.dev.localhost" "*.yourproject.localhost" localhost 127.0.0.1
```

roji will use existing certificates and skip auto-generation.

## Configuration

### How Auto-discovery Works

1. Detects containers connected to the `roji` network
2. Uses the `EXPOSE`d port (first one if multiple)
3. Generates hostname as `{service}.{domain}` from the service name

### Customizing with Labels

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

#### Examples

```yaml
services:
  # Custom hostname
  api:
    image: my-api
    labels:
      - "roji.host=api.dev.localhost"
    networks:
      - roji

  # Port specification (when multiple ports are exposed)
  app:
    image: my-app
    expose:
      - "3000"
      - "9229"
    labels:
      - "roji.port=3000"
    networks:
      - roji

  # Path-based routing
  # https://myapp.dev.localhost/api/* -> this service
  api-service:
    image: my-api
    labels:
      - "roji.host=myapp.dev.localhost"
      - "roji.path=/api"
    networks:
      - roji

  # Request mocking (for frontend development)
  # Returns mock JSON responses without a real backend
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

  # Basic authentication
  # Requires username/password to access
  admin:
    image: my-admin-panel
    labels:
      - "roji.host=admin.dev.localhost"
      - "roji.auth.basic.user=admin"
      - "roji.auth.basic.pass=secret"
      - "roji.auth.basic.realm=Admin Area"
    networks:
      - roji
```

### Static File Hosting

Host static files directly from roji without Docker containers. Configure in `~/.config/roji/config.yaml`:

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
        realm: Private Area       # Optional
```

## Environment Variables

| Variable | Description | Default (Native) | Default (Docker) |
|----------|-------------|------------------|------------------|
| `ROJI_NETWORK` | Docker network(s) to watch (comma-separated) | `roji` | `roji` |
| `ROJI_DOMAIN` | Base domain | `dev.localhost` | `dev.localhost` |
| `ROJI_CERTS_DIR` | Certificate directory | `~/.local/share/roji/certs` | `/certs` |
| `ROJI_DATA_DIR` | Data directory (project history) | `~/.local/share/roji` | `/data` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.{domain}` | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | Log level | `info` | `info` |
| `ROJI_AUTO_CERT` | Auto-generate certificates | `true` | `true` |

Settings priority (highest to lowest): CLI flags > Environment variables > Config file > Defaults

### Custom Domain Example

```yaml
environment:
  - ROJI_DOMAIN=dev.localhost  # Use *.dev.localhost
  - ROJI_DASHBOARD=dev.localhost
```

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
- **Container Restart**: Restart containers directly from the dashboard
- **Route Management**: Stop projects directly from the routes list
- **Dark Mode**: Toggle between light/dark themes or follow system preferences
- **System Status**: Build version, uptime, connection status

The dashboard automatically updates without page refresh.

## CLI Commands

### `roji doctor`

Check environment and fix common issues:

```bash
roji doctor          # Run all checks
roji doctor --fix    # Auto-fix issues where possible
roji doctor --json   # Output as JSON
```

Checks include:
- Docker daemon and socket accessibility
- Network existence
- Port availability (80, 443)
- CA certificate existence and installation
- Server certificate validity and domain matching
- DNS resolution

### `roji config`

Manage configuration:

```bash
roji config show     # Display current settings
roji config path     # Show config file locations
roji config init     # Create default config file
roji config edit     # Open in $EDITOR
```

Configuration file location: `~/.config/roji/config.yaml`

### `roji ca`

Manage CA certificate:

```bash
roji ca status              # Check installation status
roji ca install             # Install to system trust store
roji ca install --user      # Install to user store (no sudo)
roji ca install --windows   # Install to Windows (from WSL)
roji ca uninstall           # Remove from trust store
roji ca export [path]       # Export CA certificate
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

## Health Check

roji provides health check endpoints for monitoring and container orchestration:

- `/_api/health` - JSON health status (consistent with API pattern)
- `/healthz` - Kubernetes/Docker standard health check

Both endpoints return the same response:

```json
{
  "status": "healthy",
  "routes": 3
}
```

**Docker health check**: Automatically configured in the production image (checks every 30 seconds).

## Status API

roji provides a comprehensive status endpoint at `/_api/status` that shows the current state of the proxy:

```json
{
  "version": "0.1.0",
  "uptime_seconds": 3600,
  "certificates": {
    "auto_generated": true,
    "directory": "/certs",
    "ca": {
      "exists": true,
      "valid_until": "2035-01-15T12:00:00Z",
      "days_remaining": 3650,
      "subject": "CN=roji CA,O=roji Dev CA"
    },
    "server": {
      "exists": true,
      "valid_until": "2026-01-15T12:00:00Z",
      "days_remaining": 365,
      "subject": "CN=*.dev.localhost",
      "dns_names": ["*.dev.localhost", "dev.localhost", "localhost"]
    }
  },
  "docker": {
    "connected": true,
    "network": "roji"
  },
  "proxy": {
    "routes_count": 3,
    "dashboard_host": "dev.localhost",
    "base_domain": "localhost",
    "http_port": 80,
    "https_port": 443
  },
  "health": "healthy"
}
```

### Health Status

The `health` field indicates the overall system health:

- `healthy` - All systems operational
- `degraded` - Certificates expiring within 30 days or missing
- `unhealthy` - Docker connection lost

## Troubleshooting

### `.localhost` domain doesn't resolve

**macOS**: `.localhost` automatically resolves to `127.0.0.1`.

**Linux**: Add to `/etc/hosts` or configure dnsmasq:

```bash
echo "127.0.0.1 myapp.dev.localhost dev.localhost" | sudo tee -a /etc/hosts
```

Or use `*.lvh.me` (a public domain that always resolves to 127.0.0.1)

### Container not detected

1. Verify the container is connected to the `roji` network:
   ```bash
   docker network inspect roji
   ```

2. Check if the port is exposed:
   ```bash
   docker inspect <container> | jq '.[0].Config.ExposedPorts'
   ```

### Certificate errors (ERR_CERT_AUTHORITY_INVALID)

The CA certificate is not trusted. See [TLS Certificates](#tls-certificates) for installation instructions.

**Important**: On Windows, make sure to install the certificate in the **"Trusted Root Certification Authorities"** store, not the default store.

After installing, **restart your browser completely** (close all windows).

## Name Origin

**roji** means "back alley" or "narrow lane" in Japanese. The concept is to use the highway (Traefik) for production and casually take the back alley (roji) for local development.

## License

MIT
