---
title: "Multiple Networks"
slug: "multi-network"
description: "Monitor multiple Docker networks simultaneously."
weight: 5
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

roji can monitor multiple Docker networks at the same time, allowing you to organize services into separate network groups.

## Configuration

Specify multiple networks as a comma-separated list:

### Config File

```yaml
# ~/.config/roji/config.yaml
network: web,api,internal
```

### Environment Variable

```bash
ROJI_NETWORK=web,api,internal
```

### CLI Flag

```bash
sudo roji --network web,api,internal
```

## Creating Networks

Create the networks before starting:

```bash
docker network create web
docker network create api
```

Or use `roji doctor --fix` to create missing networks automatically.

## Example Setup

```yaml
# frontend/docker-compose.yml
services:
  app:
    image: nginx:alpine
    networks:
      - web

networks:
  web:
    external: true
```

```yaml
# backend/docker-compose.yml
services:
  api:
    image: my-api
    networks:
      - api

networks:
  api:
    external: true
```

Both services are accessible via roji:

- `https://app.dev.localhost` (web network)
- `https://api.dev.localhost` (api network)

## Dashboard

The dashboard displays a network badge on each route, showing which network the container belongs to.
