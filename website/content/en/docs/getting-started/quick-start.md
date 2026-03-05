---
title: "Quick Start"
description: "Get your first service running in 5 minutes."
weight: 2
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

## Prerequisites

- **Docker** with Docker Compose v2
- **roji** installed and running (see [Installation](/docs/getting-started/installation/))

## Verify Setup

```bash
roji doctor
```

All checks should pass. If not, run `sudo roji doctor --fix` to auto-repair.

## Add roji to an Existing Project

Add the `roji` network to your existing `docker-compose.yml`:

```yaml
services:
  myapp:
    image: your-app
    expose:
      - "3000"
    networks:
      - roji      # Add this

networks:
  roji:           # Add this
    external: true
```

Start your project:

```bash
docker compose up -d
```

Open `https://myapp.dev.localhost` — that's it!

## Explore the Dashboard

Open `https://roji.dev.localhost` to see the live dashboard with:

- All registered routes
- Real-time request logs
- Docker Compose project controls (start/stop/restart)
- Project history with quick-start buttons

The dashboard automatically updates without page refresh via Server-Sent Events.

## Common Patterns

**Custom hostname:**

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
```

**Path-based routing** (multiple services on one host):

```yaml
labels:
  - "roji.host=myapp.dev.localhost"
  - "roji.path=/api"
```

**Specific port** (when multiple ports are exposed):

```yaml
labels:
  - "roji.port=3000"
```

**Multiple projects** — just connect each to the `roji` network. Each service gets its own `*.dev.localhost` subdomain automatically.

## Next Steps

- [Your First Service](/docs/getting-started/first-service/) — Build a multi-service project from scratch
- [Docker Labels](/docs/reference/labels/) — All label options
- [Configuration](/docs/guides/configuration/) — Config file and environment variables
