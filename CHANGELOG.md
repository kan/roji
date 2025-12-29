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

## [Unreleased]

### Planned
- Integration tests
- Performance optimizations
- Additional monitoring capabilities

---

[0.1.0]: https://github.com/kan/roji/releases/tag/v0.1.0
[Unreleased]: https://github.com/kan/roji/compare/v0.1.0...HEAD
