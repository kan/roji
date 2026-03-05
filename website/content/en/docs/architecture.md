---
title: "Architecture"
description: "How roji works internally."
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Overview

roji is a single Go binary that runs on your host machine, watching Docker events and proxying HTTP/HTTPS requests to your containers.

```
┌─────────────────────────────────────────────────────────────┐
│                         roji                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Docker     │  │    Route     │  │   HTTP/HTTPS     │  │
│  │   Watcher    │→ │   Manager    │→ │  Reverse Proxy   │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│         ↓                ↓                   ↑              │
│   Docker Socket    SSE Broadcast        :80 / :443          │
└─────────────────────────────────────────────────────────────┘
```

## Components

### Docker Watcher

Connects to the Docker socket and listens for container events (start, stop, die). When a container on a monitored network starts, the watcher extracts:

- Container name and service name
- Network membership
- Exposed ports
- Docker labels (`roji.*`)
- Docker Compose project info (working directory, config files)

### Route Manager

Maintains the routing table and broadcasts changes:

- Maps hostnames + path prefixes to container backends
- Supports host-based and path-based routing
- Notifies connected dashboard clients via Server-Sent Events (SSE)
- Tracks Docker Compose projects (active and inactive)

### HTTP/HTTPS Reverse Proxy

Built on Go's `net/http/httputil.ReverseProxy`:

- **HTTP (port 80)**: Redirects all requests to HTTPS
- **HTTPS (port 443)**: Terminates TLS and proxies to container backends
- **WebSocket**: Detected via `Upgrade: websocket` header, proxied bidirectionally
- **gRPC**: Detected via `Content-Type: application/grpc`, proxied over HTTP/2
- **Mock routes**: Returns configured responses without hitting a backend

### Certificate Generator

Manages TLS certificates using Go's `crypto/x509` and `crypto/tls` standard libraries:

- **CA certificate**: Generated once and stored in the certs directory
- **Server certificate**: Wildcard cert for `*.{domain}`, regenerated when expired
- **System installation**: CA installed to OS trust store (macOS Keychain, Linux CA store, Windows certutil)

### Dashboard

A single-page application built with [Petite Vue](https://github.com/vuejs/petite-vue) (~6KB, no build step):

- Real-time updates via SSE (no polling)
- Docker Compose controls (start/stop/restart)
- Request log viewer with filtering
- Dark mode (follows system preferences)
- Embedded in the Go binary via `embed.FS`

## Request Flow

1. Browser sends HTTPS request to `https://myapp.dev.localhost`
2. roji's HTTPS server terminates TLS using the wildcard certificate
3. Route Manager looks up `myapp.dev.localhost` in the routing table
4. If found, `httputil.ReverseProxy` forwards the request to the container's internal IP and port
5. Response is returned to the browser

## Certificate Chain

```
roji CA (self-signed)
  └── *.dev.localhost (server certificate)
        └── Used by HTTPS server for all *.dev.localhost requests
```

The CA is generated once and persists across restarts. The server certificate is a wildcard covering all subdomains of the configured domain.

## Technology Choices

| Component | Technology | Why |
|-----------|-----------|-----|
| Language | Go | Single binary, fast startup, excellent stdlib |
| Reverse proxy | `net/http/httputil` | Standard library, battle-tested |
| TLS | `crypto/x509`, `crypto/tls` | No external dependencies |
| Frontend | Petite Vue | ~6KB, no build step, reactive |
| Real-time | Server-Sent Events | Simpler than WebSocket for one-way updates |
| Docker | Docker Engine API | Direct API access, no CLI dependency for core features |
