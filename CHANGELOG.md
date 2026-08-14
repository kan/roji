# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.0] - 2026-08-14

Feature release. v1.1.0 confined roji to loopback, which left nothing for the
times something outside has to reach the machine. A Cloudflare Tunnel fills
that in, one container at a time.

### Added

- Publish selected routes to the internet through a Cloudflare Tunnel, with a
  `tunnel:` block in the configuration file and a `roji.tunnel` label on each
  container that opts in. roji picks up every container on its network without
  being asked, so publishing is never the default.

  The tunnel gets a listener of its own on `127.0.0.1:{tunnel.port}` (8080
  unless set). cloudflared connects from 127.0.0.1 like any local browser, so
  nothing in a request distinguishes the two; the port it arrived on does, and
  that is what makes the guard possible. Requests reaching it are refused
  unless they name a route that opted in. The dashboard, the base domain,
  `/_api/*` and `/_assets/*` are refused outright — `/_api/projects/{name}/up`
  starts containers without credentials — and static sites have no opt-in
  spelling, so none are published. Every refusal answers with the same bare
  404, since wording them apart would say which hostnames exist.

  A route published as `web.dev.localhost` answers to `web.example.com`. Only
  the suffix changes: the routing key stays local, and so does the `Host` the
  backend receives, which is what a dev server's allowed-hosts list expects.

  With `auto_start: true` roji runs cloudflared as a child process and stops it
  on shutdown. It is not restarted after a crash — roji is not a process
  manager, and a tunnel that keeps failing to authenticate should not retry in
  silence. Pointing `*.{domain}` at the tunnel stays manual, since roji holds
  no Cloudflare credentials. See
  [Public Access](https://roji-proxy.dev/docs/guides/tunnel/).

- `roji doctor` checks that cloudflared is installed, logged in, and that the
  named tunnel exists, and prints the DNS command it cannot run for you.

- The dashboard marks published routes with a 🌐 badge and their public URL.

## [1.1.1] - 2026-08-07

Bug fix release. Three routing defects that share a shape: a route was stored
but could not be reached, and nothing said so.

### Fixed

- Two backends claiming the same hostname *and* the same path prefix both
  joined the routing table without a warning. One answered by sort order and
  the other was unreachable while still looking healthy on the dashboard. The
  collision check asked whether a backend had a prefix rather than whether its
  routing key — hostname and prefix together — was already taken. The same pair
  without a prefix has always been reported.

- `Router.AddBackend` did not update a route carrying a path prefix. Prefixed
  routes were appended rather than replaced, so re-adding a container left a
  second, stale entry behind (three adds gave three entries). Prefix-less routes
  replaced correctly.

- `roji config reload` silently dropped a static site whose hostname a container
  had since taken, and it stayed gone until roji restarted, though nothing about
  the configuration had changed. Reload clears the static sites and registers
  them again, and registration refused a hostname a Docker route held.

  A static site declared in the config file now keeps its hostname, which is
  what request handling and startup ordering already did — only registration
  disagreed. A Docker route hidden that way is reported in the log. See
  [Static Sites](https://roji-proxy.dev/docs/guides/static-sites/).

## [1.1.0] - 2026-08-07

Security and correctness release. roji now listens on loopback only, which is a
breaking change for anyone reaching it from another device. Alongside that, six
routing and WebSocket defects found by review, and the linting that would have
caught several of them.

### Changed

- **Breaking:** roji listens on `127.0.0.1` and `::1` instead of every
  interface. Until now anyone on the same network segment — a shared office LAN,
  a cafe network, a guest VLAN — could reach ports 80 and 443 on the developer's
  machine, and through them every container on the roji network plus the
  dashboard. The Docker Compose endpoints (`/_api/projects/{name}/up|down|restart`)
  have no authentication, so that was container control rather than information
  disclosure.

  To go back to the previous behavior, or to reach roji from a phone on the same
  network, set `bind` to the addresses you want or to an empty string for all of
  them. See [Configuration](https://roji-proxy.dev/docs/guides/configuration/).

- WebSocket upgrades go through `httputil.ReverseProxy` rather than a
  hand-rolled dialer. Two consequences beyond the fixes below: `permessage-deflate`
  is negotiated end to end, where roji previously stripped the offer and left
  every connection uncompressed; and a backend that refuses the upgrade now has
  its own response — status, headers and body — reach the client instead of a
  plain-text error.

### Added

- `bind` setting, as a comma-separated list, with `ROJI_BIND` and `--bind`.
  A loopback address that cannot be bound is skipped with a warning, so a
  machine with IPv6 disabled still starts on `127.0.0.1`; any other configured
  address is required, since nothing substitutes for it.
- `roji doctor` reports which addresses roji listens on.

### Fixed

- Path prefix routes matched without a segment boundary, so a `/api` route also
  captured `/apifoo` and `/api-docs`. The request reached the backend with a
  relative path and was rejected with 400; a prefix producing a valid-looking
  remainder would have been misrouted silently.
- Stripping a path prefix rewrote only the decoded path, so `/api/files/a%2Fb`
  reached the backend as `/files/a/b` — one path segment turned into two. Only
  routes carrying a `roji.path` label were affected.
- WebSocket upgrades reached the backend with the backend's own address in
  `Host`, while ordinary requests on the same route carried the hostname the
  client used. Vite and webpack check the upgrade request's `Host` against their
  allowed-hosts configuration, so HMR could be refused on a page that loaded
  fine.
- WebSocket upgrades dropped the client's headers, `Cookie` and `Authorization`
  among them. A WebSocket behind session or token authentication was refused the
  upgrade while the page itself loaded.
- `roji.path=/` produced a path prefix of `/` rather than no prefix at all,
  which left a prefix-less route on the same hostname unreachable and
  unannounced by the hostname collision check.
- `roji.path` was normalized with `filepath.Clean`, which rewrites slashes to
  the OS separator. On Windows `/api` became `\api` and matched nothing, so
  path-based routing failed entirely.
- `roji log` did not notice a log rotation. The server rotates by renaming, so
  the handle a follower holds keeps pointing at the renamed file; roji watched
  its size, which never shrinks. The follower silently tailed a detached file
  and printed nothing more.
- `roji log -n 0` printed no history in follow mode, though the flag documents
  `0` as "show every line", and `roji log -n` with a negative count panicked.
- Directory listing links were HTML-escaped but not URL-escaped, so a file whose
  name contained `?` or `#` could not be opened from the listing.

### Internal

- `go fix` modernizers applied, `gofmt` run over the files that had drifted, and
  `golangci-lint` added to CI along with a `gofmt` check. The linter runs once
  per `GOOS`, because the `service` and `certgen` packages are split by build
  tag and a single Linux run never compiles the macOS and Windows files.

## [1.0.8] - 2026-08-01

Bug fix release. The `roji routes` and `roji health` commands ignored the
resolved configuration when calling the local API, so both failed on any setup
that did not use the built-in defaults.

### Fixed

- `roji routes` and `roji health` now honor the configured `domain`, `https_port`
  and `dashboard` values from the config file and `ROJI_*` environment variables.
  Neither command called `config.Load()`, and instead read globals that are only
  populated when the root command parses its own flags:
  - `roji routes` requested `https://roji.:0/_api/routes` and always failed to connect
  - `roji health` requested a hardcoded `https://localhost/_api/health`, which returns
    404 whenever `domain` is not `localhost`, because the API is only served on the
    dashboard virtual host

### Changed

- Extract the local API client into a new `apiclient` package, shared by the CLI
  commands and the `roji doctor` port check. Requests connect to localhost and
  carry the dashboard hostname in the `Host` header, so no `*.localhost` DNS
  resolution is required.

## [1.0.7] - 2026-07-13

Dependency and security maintenance release. Clears all open security alerts
(2 code scanning) and the failing govulncheck / Trivy checks, all rooted in the
Go 1.26.4 standard library.

### Security

- Upgrade Go to 1.26.5, fixing 2 standard library vulnerabilities:
  - [CVE-2026-39822](https://nvd.nist.gov/vuln/detail/CVE-2026-39822) (HIGH): Symlink-following in `os.Root` allows directory traversal
  - [CVE-2026-42505](https://nvd.nist.gov/vuln/detail/CVE-2026-42505) (MEDIUM): Information disclosure in `crypto/tls` Encrypted Client Hello
- Upgrade the `golang` builder image 1.26.4-alpine → 1.26.5-alpine (#57), so the released binary is built with the patched standard library

### Dependencies

- Upgrade `golang.org/x/net` 0.56.0 → 0.57.0 (#58), which also pulls `golang.org/x/text` 0.38.0 → 0.40.0

## [1.0.6] - 2026-06-25

Dependency and security maintenance release. Resolves all 14 open security
alerts (Dependabot + code scanning). The fixed issues are in build/website
tooling (Hugo, vite, esbuild, Babel) rather than the roji proxy runtime.

### Security

- Upgrade `github.com/gohugoio/hugo` 0.161.0 → 0.163.3, fixing 5 advisories:
  - [CVE-2026-50133](https://nvd.nist.gov/vuln/detail/CVE-2026-50133): XSS via unescaped code-fence language in the default code block renderer
  - [CVE-2026-50134](https://nvd.nist.gov/vuln/detail/CVE-2026-50134): XSS via `text/html` content files
  - [CVE-2026-50135](https://nvd.nist.gov/vuln/detail/CVE-2026-50135): `security.http.urls` allow-list bypass via HTTP redirects
  - [GHSA-q76j-gcg9-vxc6](https://github.com/advisories/GHSA-q76j-gcg9-vxc6): Symlink confinement bypass in `os.ReadFile`
  - [GHSA-c3wq-j5vh-68rc](https://github.com/advisories/GHSA-c3wq-j5vh-68rc): Symlink confinement bypass in `resources.Get`
- Upgrade `vite` and `esbuild` (website), fixing:
  - [GHSA-jqfw-vq24-v9c3](https://github.com/advisories/GHSA-jqfw-vq24-v9c3) (HIGH): `vite` `server.fs.deny` bypass on Windows alternate paths
  - launch-editor: NTLMv2 hash disclosure via UNC path handling on Windows
  - `esbuild` arbitrary file read when running the dev server on Windows
- Upgrade `@babel/core` 7.29.0 → 7.29.7 (website), fixing [CVE-2026-49356](https://nvd.nist.gov/vuln/detail/CVE-2026-49356): arbitrary file read via `sourceMappingURL` comment
- Remove unused `@changesets/cli` / `@changesets/changelog-github` dev dependencies (website). They were inherited from the Doks theme starter and are not used (this project uses GoReleaser + a hand-maintained CHANGELOG); removing them drops the only paths to the vulnerable `js-yaml` ([GHSA-h67p-54hq-rp68](https://github.com/advisories/GHSA-h67p-54hq-rp68): quadratic-complexity DoS in merge key handling)

### Dependencies

- `golang.org/x/net` 0.55.0 → 0.56.0
- `golang.org/x/text` 0.37.0 → 0.38.0
- `github.com/moby/moby/api` 1.54.2 → 1.55.0
- `github.com/moby/moby/client` 0.4.1 → 0.5.0

### CI

- `actions/checkout` v6 → v7

## [1.0.5] - 2026-06-08

### Security

- Upgrade Go to 1.26.4, fixing 2 standard library vulnerabilities:
  - [GO-2026-5039](https://pkg.go.dev/vuln/GO-2026-5039) / [CVE-2026-42504](https://nvd.nist.gov/vuln/detail/CVE-2026-42504) (HIGH): Decoding a maliciously-crafted MIME header containing many invalid encoded-words in `net/textproto`
  - [GO-2026-5037](https://pkg.go.dev/vuln/GO-2026-5037): Issue in `crypto/x509`

### CI

- `codecov/codecov-action` v6 → v7

## [1.0.4] - 2026-05-26

### Security

- Upgrade `golang.org/x/net` 0.54.0 → 0.55.0, fixing [GO-2026-5026](https://pkg.go.dev/vuln/GO-2026-5026): Punycode label decoding bypass in the `idna` package. `ToASCII`/`ToUnicode` incorrectly accept Punycode-encoded labels that decode to ASCII-only labels, enabling a hostname privilege-escalation bypass. Reachable in roji because the `httputil.ReverseProxy` in `proxy/handler.go` validates upstream hosts through `golang.org/x/net/http2` → `httpguts` → `idna.ToASCII`.

### Dependencies

- `brace-expansion` 5.0.5 → 5.0.6 (website)

## [1.0.3] - 2026-05-18

### Security

- Upgrade Go to 1.26.3, fixing 5 standard library vulnerabilities:
  - [GO-2026-4982](https://pkg.go.dev/vuln/GO-2026-4982): Bypass of meta content URL escaping causes XSS in `html/template`
  - [GO-2026-4980](https://pkg.go.dev/vuln/GO-2026-4980): Escaper bypass leads to XSS in `html/template`
  - [GO-2026-4976](https://pkg.go.dev/vuln/GO-2026-4976): Issue in `net/http/httputil`
  - [GO-2026-4971](https://pkg.go.dev/vuln/GO-2026-4971): Panic in `Dial` and `LookupPort` when handling NUL byte on Windows in `net`
  - [GO-2026-4918](https://pkg.go.dev/vuln/GO-2026-4918): Issue in `net/http/internal/http2` via `golang.org/x/net`
- Upgrade `go.opentelemetry.io/otel` 1.40.0 → 1.41.0, fixing [CVE-2026-29181](https://nvd.nist.gov/vuln/detail/CVE-2026-29181) (HIGH): multi-value `baggage` header extraction causes excessive allocations (remote DoS amplification)

### Dependencies

- `golang.org/x/net` 0.53.0 → 0.54.0
- `github.com/moby/moby/client` 0.4.0 → 0.4.1
- `github.com/gohugoio/hugo` 0.159.2 → 0.161.0 (website)
- `@babel/plugin-transform-modules-systemjs` 7.29.0 → 7.29.4 (website)
- `postcss` 8.5.8 → 8.5.10 (website)

### CI

- `actions/upload-pages-artifact` v4 → v5

## [1.0.2] - 2026-04-13

### Security

- Migrate Docker client from `github.com/docker/docker` to `github.com/moby/moby/client v0.4.0`, fixing [CVE-2026-34040](https://www.cve.org/CVERecord?id=CVE-2026-34040) (AuthZ bypass in Docker daemon)
- Upgrade Go to 1.26.2, fixing 5 standard library vulnerabilities:
  - [GO-2026-4947](https://pkg.go.dev/vuln/GO-2026-4947): Unexpected work during chain building in `crypto/x509`
  - [GO-2026-4946](https://pkg.go.dev/vuln/GO-2026-4946): Inefficient policy validation in `crypto/x509`
  - [GO-2026-4870](https://pkg.go.dev/vuln/GO-2026-4870): TLS 1.3 KeyUpdate DoS in `crypto/tls`
  - [GO-2026-4866](https://pkg.go.dev/vuln/GO-2026-4866): Case-sensitive name constraints Auth Bypass in `crypto/x509`
  - [GO-2026-4865](https://pkg.go.dev/vuln/GO-2026-4865): XSS in `html/template`

### Dependencies

- `golang.org/x/net` 0.52.0 → 0.53.0
- `golang.org/x/text` 0.35.0 → 0.36.0
- `github.com/fatih/color` 1.18.0 → 1.19.0
- `github.com/gohugoio/hugo` 0.149.1 → 0.159.2 (website)

### CI

- `actions/configure-pages` v5 → v6
- `codecov/codecov-action` v5 → v6
- `actions/deploy-pages` v4 → v5
- `docker/login-action` v3 → v4
- `docker/metadata-action` v5 → v6

## [1.0.1] - 2026-03-16

### Security

- Upgrade Go to 1.26.1, fixing 3 standard library vulnerabilities:
  - [GO-2026-4601](https://pkg.go.dev/vuln/GO-2026-4601): Incorrect parsing of IPv6 host literals in `net/url`
  - [GO-2026-4602](https://pkg.go.dev/vuln/GO-2026-4602): FileInfo can escape from a Root in `os`
  - [GO-2026-4603](https://pkg.go.dev/vuln/GO-2026-4603): URLs in meta content attribute actions are not escaped in `html/template`

### Dependencies

- `golang.org/x/net` 0.51.0 → 0.52.0
- `golang.org/x/text` 0.34.0 → 0.35.0
- `golang.org/x/sys` 0.41.0 → 0.42.0

### CI

- `docker/build-push-action` v6 → v7
- `docker/setup-buildx-action` v3 → v4
- `actions/checkout`, `actions/setup-node`, `actions/upload-pages-artifact` updated to latest versions

## [1.0.0] - 2026-02-17

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
- **Internationalization (i18n)**: Japanese language support
  - CLI commands: All user-facing messages translated via `i18n.T()` / `i18n.Tf()`
  - Dashboard: Client-side translation with `Accept-Language` header detection
  - Not Found / Warning pages: Server-side Go template translation
  - Static file directory listing: Translated UI elements
  - install.sh: Language detection via `LANG` / `LC_ALL` / `LC_MESSAGES`
  - Doctor checks: All check names, messages, and details translated
  - Lightweight i18n package (`i18n/`) with JSON message files embedded via `embed.FS`
  - Language auto-detection: CLI uses `LANG` env var, HTTP uses `Accept-Language` header
  - Fallback chain: current language → English → key itself
- **README.md overhaul**: Complete rewrite for v1.0.0
  - New Quick Start section for fastest path to first service
  - New Getting Started tutorial (multi-service project walkthrough)
  - New API Reference with all 16 endpoints
  - Expanded Troubleshooting with platform-specific guides (WSL, macOS, Linux)
  - Expanded CLI Reference with all commands and flags
  - Consolidated Configuration section with full config file example
  - Added missing `ROJI_HTTP_PORT` / `ROJI_HTTPS_PORT` environment variables to docs

### Changed

- `install.sh` now shows an error and migration instructions when Docker Mode is detected, instead of offering automatic migration
- `docker-compose.yml` header updated to clarify it is for development and testing only
- Documented `roji.self` as a reserved internal label

### Removed

- `install-docker.sh` (Docker Mode installer)
- Docker Mode (Legacy) section from README.md
- TLS Certificates manual installation section (replaced by `roji ca install`)
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

[1.2.0]: https://github.com/kan/roji/releases/tag/v1.2.0
[1.1.1]: https://github.com/kan/roji/releases/tag/v1.1.1
[1.1.0]: https://github.com/kan/roji/releases/tag/v1.1.0
[1.0.8]: https://github.com/kan/roji/releases/tag/v1.0.8
[1.0.7]: https://github.com/kan/roji/releases/tag/v1.0.7
[1.0.6]: https://github.com/kan/roji/releases/tag/v1.0.6
[1.0.5]: https://github.com/kan/roji/releases/tag/v1.0.5
[1.0.4]: https://github.com/kan/roji/releases/tag/v1.0.4
[1.0.3]: https://github.com/kan/roji/releases/tag/v1.0.3
[1.0.2]: https://github.com/kan/roji/releases/tag/v1.0.2
[1.0.1]: https://github.com/kan/roji/releases/tag/v1.0.1
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
