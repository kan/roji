# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-01-07

### Added

- **Live Dashboard**: Real-time route updates via Server-Sent Events (SSE)
  - Automatic updates when containers start/stop without page refresh
  - Connection status indicator (Live/Disconnected)
  - Auto-reconnection on connection loss
- **Browser Notifications**: Optional desktop notifications for route changes
  - Notification settings persisted in localStorage
  - "Route added" and "Route removed" notifications
- **Project History**: Track active and recent Docker Compose projects
  - Automatically detects Docker Compose projects
  - Shows project name, working directory, and service count
  - Displays last active time for stopped projects
  - One-click copy of restart commands
  - Data persisted in `/data/projects.json`
- **Dashboard Redirect**: Base domain now redirects to dashboard host
  - `https://dev.localhost/` → `https://roji.dev.localhost/`
  - Preserves path and query strings
  - Ensures consistent bookmarks and URL bar history
- **Version Info**: Hover tooltip on dashboard showing build metadata
  - Commit hash, build date, and built by information
- **Data Persistence**: New `/data` volume for storing project history
- **API Endpoints**:
  - `/_api/events` - Server-Sent Events stream for real-time updates
  - `/_api/projects` - Active and inactive project listing

### Changed

- Dashboard now accessible at `https://roji.dev.localhost` (primary)
- Base domain `https://dev.localhost` redirects to dashboard host
- Environment variable `ROJI_DASHBOARD` default changed to `roji.{domain}`
- Improved dashboard UI with Petite Vue framework (~6KB)
- Enhanced version display with build information tooltip
- Updated install.sh to:
  - Create `/data` directory for project history
  - Mount `/data` volume in generated docker-compose.yml
  - Set `ROJI_DASHBOARD` to `roji.dev.localhost`
  - Add `ROJI_DATA_DIR` environment variable

### Removed

- `examples/` directory (install.sh now generates docker-compose.yml)

### Fixed

- SSE test race condition that occasionally caused test failures
- Dashboard styling improvements for better visual consistency

### Technical Details

- Petite Vue (~6KB) for reactive UI without build step
- SSE Pub/Sub pattern in Router for efficient event broadcasting
- Graceful SSE reconnection handling
- CSS transitions for smooth route add/remove animations
- Project metadata collection from Docker Compose labels

## [0.3.0] - 2026-01-05

### Added

- GoReleaser integration for automated releases
- Multi-platform binary builds (Linux/macOS/Windows, amd64/arm64)
- Multi-architecture Docker images (amd64/arm64)
- Build metadata embedding (version, commit, date, built by)
- `roji version` command with accurate version information
- GitHub Release automation
- Semantic versioning tags (v1.2.3 → 1.2.3, 1.2, 1, latest)

### Changed

- Switched from manual Docker builds to GoReleaser
- Improved release workflow automation

## [0.2.0] - 2026-01-04

### Added

- Security enhancements:
  - Distroless base image (gcr.io/distroless/static:nonroot)
  - X-Forwarded header spoofing protection
  - Path traversal prevention for roji.path label
  - GitHub Actions security scanning (govulncheck, Trivy, hadolint)
- Performance improvements:
  - SSE support (FlushInterval = -1)
  - Shared Transport for connection pooling
  - Docker Events auto-reconnection
- Robustness improvements:
  - Explicit server timeouts
  - Docker API timeout handling
  - `roji health` command for Distroless compatibility

### Changed

- Updated Docker client library to v28.5.2
- Migrated to Distroless image for better security

## [0.1.0] - 2026-01-03

### Added

- Initial release
- Auto-discovery of Docker containers on shared network
- TLS certificate auto-generation
- Label-based configuration (roji.host, roji.port, roji.path)
- Dynamic route updates via Docker Events
- Dashboard for viewing routes
- CLI commands: routes, version, health
- Health check endpoints (/_api/health, /healthz)
- Status API (/_api/status) with certificate expiration monitoring
- One-liner installation script (install.sh)
- HTTP to HTTPS redirection
- Path-based routing support
- Cobra-based CLI structure

[0.4.0]: https://github.com/kan/roji/releases/tag/v0.4.0
[0.3.0]: https://github.com/kan/roji/releases/tag/v0.3.0
[0.2.0]: https://github.com/kan/roji/releases/tag/v0.2.0
[0.1.0]: https://github.com/kan/roji/releases/tag/v0.1.0
