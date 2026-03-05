---
title: "Docker Labels"
slug: "labels"
description: "All roji Docker labels for container configuration."
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Label Reference

| Label | Description | Default |
|-------|-------------|---------|
| `roji.host` | Custom hostname | `{service}.dev.localhost` |
| `roji.port` | Target port | First EXPOSE'd port |
| `roji.path` | Path prefix | none |
| `roji.mock.{METHOD}.{PATH}` | Mock response body | none |
| `roji.mock.status.{METHOD}.{PATH}` | Mock response status code | `200` |
| `roji.auth.basic.user` | Basic auth username | none |
| `roji.auth.basic.pass` | Basic auth password | none |
| `roji.auth.basic.realm` | Basic auth realm | `Restricted` |
| `roji.self` | Reserved: excludes container from routing (internal use) | none |

## `roji.host`

Override the default hostname (`{service}.dev.localhost`):

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
```

You can use any hostname. If it doesn't contain a dot, it's expanded with the configured domain.

## `roji.port`

Specify which port roji should proxy to when a container exposes multiple ports:

```yaml
expose:
  - "3000"
  - "9229"   # debugger
labels:
  - "roji.port=3000"
```

If not set, roji uses the first exposed port.

## `roji.path`

Route requests matching a path prefix to this service:

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
  - "roji.path=/api"
```

Requests to `https://myapp.dev.localhost/api/*` will be routed to this service.

## `roji.mock.*`

Define mock responses without a real backend:

```yaml
labels:
  - "roji.host=api.dev.localhost"
  - 'roji.mock.GET./api/users=[{"id":1,"name":"Alice"}]'
  - 'roji.mock.GET./api/health={"status":"ok"}'
  - "roji.mock.status.POST./api/users=201"
```

See the [Mock Routes](/docs/guides/mock-routes/) guide for details.

## `roji.auth.basic.*`

Protect a route with HTTP Basic Authentication:

```yaml
labels:
  - "roji.auth.basic.user=admin"
  - "roji.auth.basic.pass=secret"
  - "roji.auth.basic.realm=Admin Area"   # optional
```

See the [Basic Authentication](/docs/guides/basic-auth/) guide for details.

## `roji.self`

Reserved internal label. Marks a container to be excluded from routing. Used by roji itself when running in Docker mode. Do not set this on your own containers.
