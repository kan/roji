---
title: "Mock Routes"
description: "Define mock API responses for frontend development."
weight: 4
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

roji can return mock responses without a real backend, which is useful for frontend development when the API isn't ready yet.

## How It Works

Define mock responses using Docker labels on any container. The container doesn't need to run a web server — it just needs to be connected to the roji network.

## Label Syntax

| Label | Description |
|-------|-------------|
| `roji.mock.{METHOD}.{PATH}` | Response body (JSON string) |
| `roji.mock.status.{METHOD}.{PATH}` | HTTP status code (default: `200`) |

## Example: Mock API

```yaml
services:
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

networks:
  roji:
    external: true
```

This creates:

- `GET https://api.dev.localhost/api/users` → `200` with user list JSON
- `GET https://api.dev.localhost/api/health` → `200` with health status
- `POST https://api.dev.localhost/api/users` → `201` (empty body)

## Use Cases

- **Frontend development** — Work on the UI while the backend team builds the real API
- **API prototyping** — Quickly define endpoints to test integrations
- **Error testing** — Mock error responses (4xx, 5xx) to test error handling

```yaml
labels:
  - "roji.mock.GET./api/error={\"error\":\"Internal Server Error\"}"
  - "roji.mock.status.GET./api/error=500"
```
