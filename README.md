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

- **Auto-discovery**: Automatically detects and routes containers on the shared network
- **TLS Support**: Auto-generates certificates (no mkcert required) or use your own
- **Label-based Configuration**: Customize hostnames and ports via container labels
- **Dynamic Updates**: Automatically tracks container start/stop events
- **Live Dashboard**: Real-time route updates via Server-Sent Events with optional browser notifications
- **Project History**: Tracks active and recent Docker Compose projects with quick restart commands
- **Dark Mode**: Automatic theme switching based on system preferences with manual toggle
- **Request Logging**: Real-time request log viewer with filtering by host and path
- **Multiple Networks**: Monitor multiple Docker networks simultaneously
- **Container Management**: Restart containers directly from the dashboard
- **Request Mocking**: Define mock responses via labels for frontend development
- **Simple**: Minimal implementation focused on local development

## Installation

### One-liner Install (Recommended)

Install and start roji with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.6.0/install.sh | bash
```

This will:
- Check Docker and Docker Compose prerequisites
- Create the `roji` network
- Install roji to `~/.roji` (customize with `ROJI_INSTALL_DIR`)
- Start roji with default settings
- Generate TLS certificates automatically
- Display CA certificate installation instructions

**Custom installation directory:**

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.6.0/install.sh | ROJI_INSTALL_DIR=/opt/roji bash
```

### Upgrading

The install script automatically detects existing installations and offers to upgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.6.0/install.sh | bash
```

When an existing installation is detected:
- **Interactive mode**: Choose to upgrade, keep current version, or reinstall
- **Piped mode** (`curl | bash`): Automatically upgrades to the latest version

The script backs up your configuration before upgrading and provides rollback instructions.

**Force upgrade (skip prompts):**

```bash
curl -fsSL https://raw.githubusercontent.com/kan/roji/v0.6.0/install.sh | bash -s -- --upgrade
```

### Manual Installation

If you prefer manual setup or want to contribute to development:

#### 1. Clone the repository

```bash
git clone https://github.com/kan/roji.git
cd roji
```

#### 2. Create the shared network

```bash
docker network create roji
```

#### 3. Copy environment file (optional)

```bash
cp .env.example .env
# Edit .env to customize settings if needed
```

#### 4. Start roji

```bash
docker compose up -d
```

The repository includes a production-ready `docker-compose.yml` that uses the latest published image.

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
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ROJI_NETWORK` | Docker network(s) to watch (comma-separated) | `roji` |
| `ROJI_DOMAIN` | Base domain | `dev.localhost` |
| `ROJI_CERTS_DIR` | Certificate directory | `/certs` |
| `ROJI_DATA_DIR` | Data directory (project history) | `/data` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | Log level | `info` |
| `ROJI_AUTO_CERT` | Auto-generate certificates | `true` |

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
- **Active Projects**: Currently running Docker Compose projects with service counts
- **Project History**: Recently stopped projects with one-click copy of restart commands
- **Request Log Viewer**: Real-time request logging with filtering by host and path
- **Container Restart**: Restart containers directly from the dashboard
- **Dark Mode**: Toggle between light/dark themes or follow system preferences
- **System Status**: Build version, uptime, connection status

The dashboard automatically updates without page refresh.

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
