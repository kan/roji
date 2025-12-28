# roji

A simple reverse proxy for local development environments. Automatically discovers Docker Compose services and provides HTTPS access via `*.localhost`.

> "Use the highway (Traefik) for production, take the back alley (roji) for development"

## Features

- **Auto-discovery**: Automatically detects and routes containers on the shared network
- **TLS Support**: Auto-generates certificates (or integrates with mkcert)
- **Label-based Configuration**: Customize hostnames and ports via container labels
- **Dynamic Updates**: Automatically tracks container start/stop events
- **Dashboard**: View current routes in your browser
- **Simple**: Minimal implementation focused on local development

## Quick Start

### 1. Create the shared network

```bash
docker network create roji
```

### 2. Generate certificates (mkcert)

```bash
# Install mkcert if not already installed
# macOS: brew install mkcert
# Linux: https://github.com/FiloSottile/mkcert#installation

# Install the root CA
mkcert -install

# Generate certificates
mkdir -p certs
mkcert -cert-file certs/cert.pem -key-file certs/key.pem \
  "*.localhost" localhost 127.0.0.1
```

### 3. Start roji

```bash
# Copy the example compose file
cp examples/docker-compose.yml docker-compose.yml

# Start roji
docker compose up -d
```

### 4. Start your application

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

Your app is now accessible at `https://myapp.localhost`!

## Configuration

### How Auto-discovery Works

1. Detects containers connected to the `roji` network
2. Uses the `EXPOSE`d port (first one if multiple)
3. Generates hostname as `{service}.{domain}` from the service name

### Customizing with Labels

| Label | Description | Default |
|-------|-------------|---------|
| `roji.host` | Custom hostname | `{service}.localhost` |
| `roji.port` | Target port | First EXPOSE'd port |
| `roji.path` | Path prefix | none |

#### Examples

```yaml
services:
  # Custom hostname
  api:
    image: my-api
    labels:
      - "roji.host=api.myproject.localhost"
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
  # https://myapp.localhost/api/* -> this service
  api-service:
    image: my-api
    labels:
      - "roji.host=myapp.localhost"
      - "roji.path=/api"
    networks:
      - roji
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ROJI_NETWORK` | Docker network to watch | `roji` |
| `ROJI_DOMAIN` | Base domain | `localhost` |
| `ROJI_CERTS_DIR` | Certificate directory | `/certs` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.{domain}` |
| `ROJI_LOG_LEVEL` | Log level | `info` |

### Custom Domain Example

```yaml
environment:
  - ROJI_DOMAIN=dev.localhost  # Use *.dev.localhost
  - ROJI_DASHBOARD=roji.dev.localhost
```

## Dashboard

Access `https://roji.localhost` (or your custom configured host) to view a list of currently registered routes.

## Troubleshooting

### `.localhost` domain doesn't resolve

**macOS**: `.localhost` automatically resolves to `127.0.0.1`.

**Linux**: Add to `/etc/hosts` or configure dnsmasq:

```bash
echo "127.0.0.1 myapp.localhost" | sudo tee -a /etc/hosts
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

### Certificate errors

Ensure the CA certificate is installed in your browser/OS.

## Name Origin

**roji** means "back alley" or "narrow lane" in Japanese. The concept is to use the highway (Traefik) for production and casually take the back alley (roji) for local development.

## License

MIT
