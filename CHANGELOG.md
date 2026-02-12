# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-02-12

### Breaking Changes

- **Docker Mode removed**: `install-docker.sh` has been deleted. roji now only supports Native Mode (standalone binary). Existing Docker Mode users should migrate manually by stopping the container, removing the installation, and re-running the native installer.
- **`ROJI_DOMAIN` default unified**: `docker-compose.yml` now defaults to `dev.localhost` (was `localhost`), matching Native Mode behavior.

### Added

- **Homebrew tap support**: Install on macOS with `brew install kan/roji/roji`
  - GoReleaser auto-generates and pushes Formula to `kan/homebrew-roji` on release
- **Recent Projects delete**: Remove projects from history via dashboard or API
  - `DELETE /_api/projects/{name}/delete` endpoint
  - Remove button on inactive projects in dashboard
  - Confirmation dialog before deletion

### Changed

- `install.sh` now shows an error and migration instructions when Docker Mode is detected, instead of offering automatic migration
- `docker-compose.yml` header updated to clarify it is for development and testing only

### Removed

- `install-docker.sh` (Docker Mode installer)
- Docker Mode (Legacy) section from README.md
- `--migrate` flag from `install.sh`

## [0.9.1] - 2026-02-06

### Security

- **Go Update**: Update to Go 1.25.7 to address crypto/tls vulnerability (CVE fix)

## [0.9.0] - 2026-02-06

### Added

- **Docker Compose Operations**: Control projects from dashboard and API
  - `POST /_api/projects/{name}/up` - Start project (`docker compose up -d`)
  - `POST /_api/projects/{name}/down` - Stop project (`docker compose down`)
  - `POST /_api/projects/{name}/restart` - Restart all services
  - `GET /_api/projects/{name}/logs` - Stream logs via SSE
  - Dashboard buttons: Start/Stop/Restart on active and inactive projects
  - Stop button on individual routes in the routes list
  - Confirmation dialog before destructive operations
  - CORS support for cross-subdomain API calls
- **Not Found Page Project Detection**: Auto-start inactive projects
  - Detects hostname matching an inactive project
  - Shows "Start" button to bring up the project directly
  - Supports both `service.domain` and `service-project.domain` hostname patterns
- **Basic Authentication**: Protect routes with username/password
  - Docker labels: `roji.auth.basic.user`, `roji.auth.basic.pass`, `roji.auth.basic.realm`
  - Config file support for static sites
  - CORS preflight (OPTIONS) requests bypass authentication
  - Dashboard shows auth badge (🔒) for protected routes
  - Config validation warns on unknown auth keys
- **Static File Hosting**: Serve static files without Docker
  - Configure via `static_sites` in config.yaml
  - Directory listing with dark mode support
  - `index.html` auto-serving
  - Directory traversal protection
  - Basic authentication support
  - Dashboard "Reload Config" button for hot-reload
- **Service Management**: Run roji as system service
  - `roji service install/uninstall/start/stop/restart/status`
  - Linux: systemd service (`/etc/systemd/system/roji.service`)
  - macOS: launchd agent (`~/Library/LaunchAgents/com.roji.agent.plist`)
  - Windows: NSSM-based Windows Service
- **Log File Output**: Service mode logs to file
  - Linux/WSL: `~/.local/share/roji/roji.log`
  - macOS: `~/Library/Logs/roji.log`
  - Auto-rotation at 10MB
- **Config File Validation**: Comprehensive validation on load and in doctor
  - Unknown keys warning (typo detection)
  - Type validation for all fields
  - Missing required fields check
  - `roji doctor` shows config validation results
- **New Native Mode Installer**: `install.sh` now installs native binary
  - Downloads pre-built binary from GitHub Releases
  - Interactive install location selection (`~/.local/bin` or `/usr/local/bin`)
  - Auto-detects and migrates from Docker Mode
  - Runs `doctor --fix`, `ca install`, and `service install` automatically
  - Legacy Docker installer available as `install-docker.sh`

### Changed

- Dashboard now displays authentication status for each route
- Dashboard active projects now have Start/Stop/Restart operation buttons
- Dashboard inactive projects now have a Start button alongside copy command
- Dashboard routes list now includes Stop button for Docker Compose projects
- Static site index status shown in dashboard (📋 enabled / 🔒 disabled)
- `roji doctor` port check now detects if roji itself is using the ports
- Not Found page and Warning page now use absolute URLs for dashboard assets

### Fixed

- `certutil.exe` not found on some WSL configurations

## [0.8.0] - 2026-01-24

### Added

- **Native Mode**: Run roji as a standalone binary without Docker
  - Single binary execution on host system
  - Direct access to Docker socket from host
  - Simplified deployment for local development
- **Configuration File**: YAML-based configuration support
  - Default location: `~/.config/roji/config.yaml`
  - XDG Base Directory compliant paths
  - Settings priority: CLI > Environment > Config file > Defaults
- **`roji config` Command**: Manage configuration files
  - `roji config show` - Display current configuration
  - `roji config path` - Show configuration file paths
  - `roji config init` - Create default configuration file
  - `roji config edit` - Open configuration in editor
- **`roji doctor` Command**: Environment diagnostics with auto-fix
  - Docker daemon and socket checks
  - Network existence verification
  - Port availability checks
  - CA certificate existence and installation status
  - Server certificate validity and domain matching
  - DNS resolution checks
  - `--fix` flag for automatic remediation
  - `--json` flag for machine-readable output
- **`roji ca` Command**: CA certificate management
  - `roji ca status` - Check installation status
  - `roji ca install` - Install CA to system trust store
  - `roji ca uninstall` - Remove CA from trust store
  - `roji ca export` - Export CA certificate (PEM/DER)
  - Platform support: macOS (Keychain), Linux (system CA store), Windows (certutil), WSL (Windows user store)
  - `--user` flag for user-level installation (no sudo)
  - `--windows` flag for WSL to Windows installation
- **Makefile**: Common development tasks
  - `make build` - Build binary to ./bin/roji
  - `make test` - Run all tests
  - `make doctor` - Build and run doctor

### Changed

- Dashboard host default corrected to `roji.{domain}` (was incorrectly `{domain}`)
- `*.localhost` DNS resolution failure now passes in doctor (Chrome auto-resolves per RFC 6761)
- CA certificate names now include unique identifier for easier management

### Fixed

- Certificate domain mismatch detection in `roji doctor`
- Auto-regeneration of server certificate when domain changes
- Server startup now detects domain mismatch and regenerates certificate with helpful message

### Technical Details

- XDG paths: config in `~/.config/roji/`, data in `~/.local/share/roji/`
- CA installer interface with platform-specific implementations
- Doctor check interface for extensible diagnostics
- Certificate domain validation using x509 DNSNames

## [0.7.0] - 2026-01-15

### Added

- **WebSocket Proxy Support**: Full bidirectional WebSocket proxying
  - Automatic detection via `Upgrade: websocket` header
  - Bidirectional message forwarding between client and backend
  - Support for text and binary messages
  - Proper connection cleanup on disconnect
- **gRPC Proxy Support**: HTTP/2 based gRPC proxying
  - Automatic detection via `Content-Type: application/grpc`
  - HTTP/2 transport for backend connections (h2c)
  - gRPC-compatible error responses with `Grpc-Status` header
  - Streaming support with immediate flush
- **Log Export**: Export request logs from dashboard
  - JSON and CSV download formats
  - Filter by host, service, method, and time range
  - Export buttons added to dashboard UI
  - `/_api/logs/export` endpoint

### Changed

- HTTPS server now explicitly configured with HTTP/2 support (required for gRPC)
- Request log panel now includes JSON/CSV export buttons

### Technical Details

- WebSocket proxy using `gorilla/websocket` library
- HTTP/2 transport via `golang.org/x/net/http2` package
- LogFilter type for flexible log querying
- CSV escaping for safe export of paths with special characters

## [0.6.1] - 2026-01-10

### Fixed

- **Dashboard crash when no containers connected**: Fixed JavaScript error "Cannot read properties of null (reading 'forEach')" that occurred when accessing the dashboard with no containers in the roji network. Routes are now properly initialized as an empty array instead of null.

## [0.6.0] - 2026-01-10

### Added

- **Dark Mode**: Automatic theme switching based on system preferences
  - Manual toggle button (sun/moon icon) in dashboard
  - Settings persisted in localStorage
  - FOUC prevention (theme applied before page render)
  - Applied to all pages (dashboard, not found, warning)
- **Request Log Viewer**: Real-time request logging in dashboard
  - Ring buffer storing last 100 requests
  - Real-time streaming via SSE
  - Filtering by hostname (dropdown) and path (text input)
  - Shows method, path, status code, latency, and hostname
- **Multiple Network Support**: Monitor multiple Docker networks simultaneously
  - Comma-separated network names in `ROJI_NETWORK`
  - Network badge on each route in dashboard
  - Click network badge to filter routes
- **Container Restart Button**: Restart containers directly from dashboard
  - Restart button on each route card
  - Confirmation dialog before restart
  - `/_api/containers/{id}/restart` API endpoint
- **Request Mocking**: Define mock responses via container labels
  - `roji.mock.{METHOD}.{PATH}` for response body
  - `roji.mock.status.{METHOD}.{PATH}` for status code
  - Supports JSON and text responses
  - Useful for frontend development without backend
- **Upgrade Support in install.sh**: Seamless upgrades via one-liner
  - Automatic detection of existing installation
  - Version comparison with GitHub API
  - Interactive upgrade menu (upgrade/keep/reinstall)
  - Automatic upgrade when running non-interactively
  - Configuration backup before upgrade
  - Configuration migration for new settings
  - Rollback instructions
- **Integration Tests**: Docker Compose based testing
  - Real container lifecycle testing
  - Route detection and proxy verification
  - Warning case testing (no port, hostname conflicts)
  - GitHub Actions integration
- **E2E Tests**: Full HTTP/HTTPS server testing
  - Real HTTP request verification
  - TLS certificate validation
  - SSE connection testing
  - Dashboard accessibility testing

### Changed

- `ROJI_NETWORK` now supports comma-separated values for multiple networks
- Dashboard now shows network badges for each route
- Request log panel added below routes in dashboard

### Technical Details

- CSS variables for theme management (`theme.css`)
- LogBuffer ring buffer for efficient request log storage
- SSE endpoint `/_api/logs` for real-time log streaming
- Network-aware container discovery and routing
- Docker API `ContainerRestart` integration

## [0.5.0] - 2026-01-08

### Added

- **Misconfigured Container Warnings**: Dashboard now shows containers with configuration issues
  - Containers without exposed ports are displayed with warning badge
  - Warning page with context-aware fix suggestions
  - Hostname conflict detection when multiple services use the same hostname
- **Warning Page**: Dedicated error page for misconfigured routes
  - Shows specific issue and affected service
  - Provides tailored fix instructions based on warning type
  - Direct link back to dashboard
- **GitHub Link**: Added GitHub repository link with Octocat icon to dashboard header

### Fixed

- **Multi-service Hostname Format**: Changed from `service.project.localhost` to `service-project.localhost`
  - Keeps hostname at single subdomain level
  - Ensures wildcard SSL certificate (`*.localhost`) compatibility
  - Prevents certificate errors for multi-service projects

### Technical Details

- Warning type detection in proxy handler for template switching
- Hostname conflict check in Router.AddBackend with warning propagation
- Inline SVG for GitHub Octocat icon (no external dependencies)

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

[1.0.0]: https://github.com/kan/roji/releases/tag/v1.0.0
[0.9.1]: https://github.com/kan/roji/releases/tag/v0.9.1
[0.9.0]: https://github.com/kan/roji/releases/tag/v0.9.0
[0.8.0]: https://github.com/kan/roji/releases/tag/v0.8.0
[0.7.0]: https://github.com/kan/roji/releases/tag/v0.7.0
[0.6.1]: https://github.com/kan/roji/releases/tag/v0.6.1
[0.6.0]: https://github.com/kan/roji/releases/tag/v0.6.0
[0.5.0]: https://github.com/kan/roji/releases/tag/v0.5.0
[0.4.0]: https://github.com/kan/roji/releases/tag/v0.4.0
[0.3.0]: https://github.com/kan/roji/releases/tag/v0.3.0
[0.2.0]: https://github.com/kan/roji/releases/tag/v0.2.0
[0.1.0]: https://github.com/kan/roji/releases/tag/v0.1.0
