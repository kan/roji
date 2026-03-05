---
title: "Basic Authentication"
slug: "basic-auth"
description: "Protect routes with username and password."
weight: 3
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

roji supports HTTP Basic Authentication to protect routes in your local development environment.

## Via Docker Labels

Add authentication to any Docker-based route:

```yaml
services:
  admin:
    image: my-admin-panel
    labels:
      - "roji.auth.basic.user=admin"
      - "roji.auth.basic.pass=secret"
      - "roji.auth.basic.realm=Admin Area"   # optional, defaults to "Restricted"
    networks:
      - roji
```

## Via Config File

Protect static sites in `~/.config/roji/config.yaml`:

```yaml
static_sites:
  - host: docs
    root: ~/projects/docs/build
    auth:
      basic:
        user: admin
        pass: secret
        realm: Documentation   # optional
```

## Behavior

- Returns `401 Unauthorized` with `WWW-Authenticate` header when credentials are missing or wrong
- CORS preflight requests (`OPTIONS`) bypass authentication
- The dashboard shows a lock badge on protected routes
- Passwords are stored in plaintext (suitable for local development only)
