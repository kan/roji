---
title: "roji"
description: "A simple reverse proxy for local development. Auto-discovers Docker services, provides HTTPS via *.dev.localhost."
lead: "Use the highway for production, take the back alley for development."
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
params:
  seo:
    title: "roji — Simple Reverse Proxy for Local Development"
    description: "Auto-discovers Docker Compose services and provides HTTPS access via *.dev.localhost. Zero configuration required."
---

## Install in seconds

```bash
# macOS
brew install kan/roji/roji

# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/kan/roji/v1.0.0/install.sh | bash
```

## Try it out

```yaml
# docker-compose.yml
services:
  myapp:
    image: your-app
    expose:
      - "3000"
    networks:
      - roji

networks:
  roji:
    external: true
```

```bash
docker compose up -d
# Open https://myapp.dev.localhost
```
