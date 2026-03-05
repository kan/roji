---
title: "API Reference"
slug: "api"
description: "Dashboard API endpoints."
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

All API endpoints are served on the dashboard host (default: `https://roji.dev.localhost`).

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_api/health` | GET | Health check |
| `/healthz` | GET | Alias for `/_api/health` |
| `/_api/status` | GET | Detailed status (version, uptime, certificates, Docker) |
| `/_api/routes` | GET | List all registered routes |
| `/_api/projects` | GET | List active and inactive projects |
| `/_api/events` | GET | SSE stream for real-time route updates |
| `/_api/logs` | GET | Recent request logs (last 100) |
| `/_api/logs/events` | GET | SSE stream for real-time log updates |
| `/_api/logs/export` | GET | Export logs as JSON or CSV |
| `/_api/containers/{id}/restart` | POST | Restart a container |
| `/_api/projects/{name}/up` | POST | Start project (`docker compose up -d`) |
| `/_api/projects/{name}/down` | POST | Stop project (`docker compose down`) |
| `/_api/projects/{name}/restart` | POST | Restart all services in project |
| `/_api/projects/{name}/logs` | GET | Stream project logs (SSE) |
| `/_api/projects/{name}/delete` | DELETE | Remove project from history |
| `/_api/config/reload` | POST | Reload config file |

## Health Check

```bash
curl -s https://roji.dev.localhost/_api/health
```

```json
{"status": "healthy", "routes": 5}
```

**Status values:**

| Status | Description |
|--------|-------------|
| `healthy` | All systems operational |
| `degraded` | Certificates expiring within 30 days |
| `unhealthy` | Docker connection lost |

## Log Export

Export request logs with optional filters:

```bash
# JSON format
curl -s "https://roji.dev.localhost/_api/logs/export?format=json"

# CSV format with filters
curl -s "https://roji.dev.localhost/_api/logs/export?format=csv&service=myapp&method=GET"
```

**Query parameters:**

| Parameter | Description |
|-----------|-------------|
| `format` | `json` (default) or `csv` |
| `service` | Filter by service name |
| `host` | Filter by hostname |
| `method` | Filter by HTTP method |
| `from` | Start time (RFC 3339) |
| `to` | End time (RFC 3339) |

## SSE Streams

### Route Events (`/_api/events`)

Sends events when routes are added, updated, or removed:

```javascript
const es = new EventSource("https://roji.dev.localhost/_api/events");
es.onmessage = (event) => {
  const routes = JSON.parse(event.data);
  console.log("Routes updated:", routes);
};
```

### Log Events (`/_api/logs/events`)

Streams request log entries in real-time:

```javascript
const es = new EventSource("https://roji.dev.localhost/_api/logs/events");
es.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(`${log.method} ${log.url} -> ${log.status}`);
};
```

## CORS

API endpoints support cross-origin requests from any `*.dev.localhost` subdomain, allowing API calls from services proxied through roji.
