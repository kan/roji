# Contributing

Guide for setting up a local development environment for roji.

## Prerequisites

- Docker & Docker Compose
- Go 1.25+ (for local builds without Docker)
- [mkcert](https://github.com/FiloSottile/mkcert) (for certificate generation)

## Quick Start

### 1. Create the roji network

```bash
docker network create roji
```

### 2. Generate TLS certificates

```bash
# Install mkcert (first time only)
# macOS: brew install mkcert
# Linux: sudo apt install mkcert

# Install root CA to system (first time only)
mkcert -install

# Generate certificates
mkdir -p certs
mkcert -cert-file certs/cert.pem -key-file certs/key.pem \
  "*.localhost" localhost 127.0.0.1
```

### 3. Configure environment (optional)

```bash
# Copy example env file
cp .env.example .env

# Edit .env to customize settings
```

### 4. Start development server

```bash
# Start with hot reload
docker compose up

# Or run in background
docker compose up -d
docker compose logs -f
```

The development server uses [air](https://github.com/air-verse/air) for hot reload. Any changes to `.go` files will automatically trigger a rebuild.

### 5. Start test services

```bash
mkdir -p test
cat > test/docker-compose.yml << 'EOF'
services:
  web:
    image: nginx:alpine
    networks:
      - roji

  api:
    image: nginx:alpine
    labels:
      - "roji.host=api.myapp.localhost"
    networks:
      - roji

networks:
  roji:
    external: true
EOF

cd test && docker compose up -d
```

### 6. Verify

```bash
# Check routes in logs
docker compose logs roji-dev

# Access via browser or curl
curl -k https://web.localhost
curl -k https://api.myapp.localhost

# View dashboard
open https://roji.localhost
```

## Development Workflow

### Hot reload

The development container mounts the source code and uses air for automatic rebuilding:

```bash
# Watch logs while developing
docker compose logs -f

# Rebuild container if Dockerfile or go.mod changes
docker compose up --build
```

### Running tests

```bash
# Run tests locally
go test ./...

# Run tests in container
docker compose exec roji-dev go test ./...
```

### Building production image

```bash
# Build production image
docker build --target production -t roji:latest .

# Run production image
docker run -d \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./certs:/certs \
  --network roji \
  roji:latest
```

### Environment variables

Configure via `.env` file (gitignored) or directly in shell:

| Variable | Description | Default |
|----------|-------------|---------|
| `ROJI_NETWORK` | Docker network to watch | `roji` |
| `ROJI_DOMAIN` | Base domain | `localhost` |
| `ROJI_HTTP_PORT` | HTTP port | `80` |
| `ROJI_HTTPS_PORT` | HTTPS port | `443` |
| `ROJI_CERTS_DIR` | Certificate directory | `/certs` |
| `ROJI_DASHBOARD` | Dashboard hostname | `roji.localhost` |
| `ROJI_LOG_LEVEL` | Log level (debug/info/warn/error) | `debug` |

## Project Structure

```
roji/
├── main.go                 # Entry point
├── internal/
│   ├── docker/
│   │   ├── client.go       # Docker API wrapper
│   │   └── watcher.go      # Events watcher
│   ├── proxy/
│   │   ├── handler.go      # ReverseProxy implementation
│   │   └── router.go       # Routing
│   └── config/
│       └── labels.go       # Label parser
├── certs/                  # Certificates (gitignored)
├── examples/
│   └── docker-compose.yml  # Example for users
├── test/                   # Test services (gitignored)
├── Dockerfile              # Multi-stage (development + production)
├── docker-compose.yml      # Development setup
├── .air.toml               # Hot reload configuration
├── .env.example            # Environment template
├── go.mod
└── go.sum
```

## Troubleshooting

### `.localhost` domain doesn't resolve

| OS | Solution |
|----|----------|
| macOS | Automatically resolves to `127.0.0.1` |
| Linux | Add to `/etc/hosts` or configure `systemd-resolved` |
| Windows (WSL2) | Add to Windows hosts file |

```bash
# Linux: add to /etc/hosts
echo "127.0.0.1 web.localhost api.localhost roji.localhost" | sudo tee -a /etc/hosts
```

### Ports 80/443 already in use

Edit `.env` to use different ports:

```bash
ROJI_HTTP_PORT=8080
ROJI_HTTPS_PORT=8443
```

### Container not detected

1. Check network connection:
   ```bash
   docker network inspect roji
   ```

2. Check container port configuration:
   ```bash
   docker inspect <container> | jq '.[0].Config.ExposedPorts'
   ```

3. Ensure container doesn't have `roji.self=true` label

### Certificate errors

```bash
# Reinstall root CA
mkcert -install

# Regenerate certificates
mkcert -cert-file certs/cert.pem -key-file certs/key.pem \
  "*.localhost" localhost 127.0.0.1

# Restart browser
```

### Hot reload not working

```bash
# Check air logs
docker compose logs -f

# Rebuild container
docker compose up --build
```
