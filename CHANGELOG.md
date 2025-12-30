# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-12-29

First stable release of roji - a simple reverse proxy for local development.

### Added

#### Proxy & Routing
- Auto-discovery of Docker Compose services on shared network
- Dynamic routing based on hostname and path
- Container label support for customization:
  - `roji.host` - Custom hostname
  - `roji.port` - Target port selection
  - `roji.path` - Path-based routing
- Automatic route updates on container start/stop
- HTTP to HTTPS automatic redirect

#### TLS Certificates
- Automatic certificate generation (CA + server certificates)
- Support for wildcard certificates (*.localhost)
- Certificate expiration tracking
- OS-specific installation guides (Windows/macOS/Linux)
- Support for external certificates (mkcert compatible)

#### Dashboard & Monitoring
- Web dashboard for viewing registered routes
- Real-time status endpoint with:
  - Certificate expiration information
  - Docker connection status
  - Proxy configuration details
  - System uptime
  - Overall health status
- Health check endpoints (`/_api/health`, `/healthz`)
- JSON API for route listing

#### CLI
- Command-line interface built with Cobra
- Commands:
  - `roji` - Start the proxy server
  - `roji routes` - List all registered routes
  - `roji version` - Display version information
  - `roji --help` - Show help and available commands

#### Installation & Distribution
- One-liner installation script:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash
  ```
- Automated prerequisite checks (Docker, Docker Compose)
- Default installation to `~/.roji`
- Automatic Docker image publishing to ghcr.io
- Multi-tag strategy (semver + latest)

#### Developer Experience
- Hot reload development environment (air)
- Comprehensive test suite (48.2% coverage)
- GitHub Actions CI/CD
- Docker health checks
- Graceful shutdown handling

### Documentation
- Installation guide with one-liner setup
- TLS certificate installation instructions
- Configuration reference (environment variables & labels)
- Development setup guide (CONTRIBUTING.md)
- Troubleshooting section
- API documentation

## [0.2.0] - 2025-12-30

Security and performance improvements release.

### Security

- **Distroless image migration**: Production image now uses `gcr.io/distroless/static:nonroot` for minimal attack surface
- **X-Forwarded header protection**: Strip client-provided X-Forwarded-* headers to prevent spoofing
- **Path traversal prevention**: Reject paths containing `..` in `roji.path` labels
- **Dependency updates**: Docker client library updated to v28.5.2
- **Security scanning**: Added govulncheck, Trivy, and hadolint to CI pipeline
- **Non-root execution**: Container runs as nonroot user (UID 65532)

### Added

- `roji health` command for container healthcheck (replaces curl-based healthcheck)
- GitHub Actions security scan workflow (weekly + on PR/push)
- Trivy vulnerability scanning before Docker image release

### Improved

- **SSE support**: Added `FlushInterval = -1` for Server-Sent Events compatibility
- **Connection pooling**: Shared HTTP transport for better performance
- **Docker Events reconnection**: Automatic reconnection on Docker daemon restart
- **Explicit server timeouts**: Configured timeouts for HTTP/HTTPS servers
- **Docker API timeouts**: Added timeouts to prevent indefinite blocking

### Changed

- Docker healthcheck now uses `/roji health` instead of `curl`
- install.sh updated with healthcheck configuration

## [0.3.0] - 2024-12-30

Major release with GoReleaser integration, improved domain structure, and bug fixes.

### Added

#### Release Automation
- **GoReleaser integration** for automated multi-platform releases
  - Binary builds for Linux, macOS, Windows (amd64/arm64)
  - Multi-arch Docker images with manifest lists
  - Automated changelog generation from git history
  - GitHub Release creation with artifacts
- **Enhanced version command** with detailed build metadata:
  - Version number from git tag
  - Git commit hash
  - Build timestamp
  - Builder identification (goreleaser/github-actions/docker/manual)
  - Go version and platform information

#### Installation Improvements
- Simplified installation script with better error handling
- Direct certificate mounting (removed Docker volume complexity)
- Root user execution in container for proper permissions
- Automatic certificate export from container to host

### Changed

#### BREAKING: Domain Structure
- **Default domain changed from `localhost` to `dev.localhost`**
  - Dashboard: `dev.localhost` (was `roji.localhost`)
  - Services: `*.dev.localhost` (was `*.localhost`)
  - Provides better browser certificate validation
  - Migration: Update ROJI_DOMAIN environment variable and reinstall CA certificate

#### Certificate Improvements
- Fixed duplicate DNS names in certificate SAN entries
- Improved wildcard certificate support for nested subdomains
- Better compatibility with browser certificate validation

### Fixed

- **Certificate validation errors** (ERR_CERT_COMMON_NAME_INVALID) in browsers
- **Version ldflags** not being applied due to hardcoded override in main.go
- **Docker socket permission issues** by running container as root
- **Installation script** certificate handling and error recovery

### Technical Improvements

- Added build metadata to all build methods (Docker, GoReleaser, manual)
- Improved CI/CD pipeline with version injection
- Better test coverage for certificate generation
- Cleaner code structure with removed version override

## [Unreleased]

### Planned
- Integration tests
- Additional cloud provider support

---

[0.1.0]: https://github.com/kan/roji/releases/tag/v0.1.0
[0.2.0]: https://github.com/kan/roji/releases/tag/v0.2.0
[0.3.0]: https://github.com/kan/roji/releases/tag/v0.3.0
[Unreleased]: https://github.com/kan/roji/compare/v0.3.0...HEAD
