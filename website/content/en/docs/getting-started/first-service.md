---
title: "Your First Service"
slug: "first-service"
description: "Build a multi-service project from scratch with roji."
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

This tutorial walks you through creating a multi-service project from scratch and accessing it through roji.

## Create a New Project

Create a project directory with a web frontend and an API backend:

```bash
mkdir my-project && cd my-project
```

Create a `docker-compose.yml`:

```yaml
services:
  web:
    image: nginx:alpine
    expose:
      - "80"
    networks:
      - roji

  api:
    image: node:alpine
    working_dir: /app
    command: ["node", "server.js"]
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

## Start and Access

```bash
docker compose up -d
```

Your services are now available at:

- `https://web.dev.localhost` — the nginx frontend
- `https://api.dev.localhost` — the Node.js API

## Customize with Labels

### Custom Hostname

Give a service a custom hostname instead of the default `{service}.dev.localhost`:

```yaml
services:
  api:
    image: my-api
    labels:
      - "roji.host=api.dev.localhost"
    networks:
      - roji
```

### Path-based Routing

Route multiple services under the same hostname using path prefixes:

```yaml
services:
  frontend:
    image: nginx:alpine
    labels:
      - "roji.host=myapp.dev.localhost"
    networks:
      - roji

  api:
    image: my-api
    labels:
      - "roji.host=myapp.dev.localhost"
      - "roji.path=/api"
    networks:
      - roji
```

Now `https://myapp.dev.localhost` serves the frontend, and `https://myapp.dev.localhost/api/*` routes to the API.

### Specific Port

When a container exposes multiple ports, specify which one roji should proxy to:

```yaml
services:
  app:
    expose:
      - "3000"
      - "9229"   # debugger
    labels:
      - "roji.port=3000"
    networks:
      - roji
```

## Check the Dashboard

Open `https://roji.dev.localhost` to see all your routes, request logs, and project controls.

## Next Steps

- [Configuration](/docs/guides/configuration/) — Config file and environment variables
- [Docker Compose Patterns](/docs/guides/docker-compose-patterns/) — Common project layouts
- [Mock Routes](/docs/guides/mock-routes/) — Define mock API responses
