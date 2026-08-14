---
title: "Public Access (Tunnel)"
slug: "tunnel"
description: "Reach selected routes from the internet through a Cloudflare Tunnel."
weight: 7
date: 2026-08-14T00:00:00+00:00
lastmod: 2026-08-14T00:00:00+00:00
draft: false
toc: true
---

roji listens on loopback only by default (see [`bind` in the configuration
guide](/docs/guides/configuration/)). For the times something outside has to
reach your machine — receiving a webhook, showing someone a work in progress —
a Cloudflare Tunnel can publish **selected routes**.

Unlike widening `bind`, publishing is per container, and the dashboard and its
API are never published.

## Requirements

- A Cloudflare account with a zone on it (`example.com`, say)
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)
  on your PATH. roji runs it as a child process

```bash
cloudflared tunnel login          # opens a browser, writes ~/.cloudflared/cert.pem
cloudflared tunnel create roji    # creates the named tunnel
```

## Configuration

Add a `tunnel:` block to `~/.config/roji/config.yaml`.

```yaml
tunnel:
  domain: example.com   # a zone on your Cloudflare account
  name: roji            # the named tunnel from `cloudflared tunnel create`
  port: 8080            # the tunnel's own listener on 127.0.0.1 (default 8080)
  auto_start: false     # whether roji starts cloudflared for you
```

The tunnel turns on only when both `domain` and `name` are set. There are no
environment variables for it; it is config-file only.

With `auto_start: true` roji starts cloudflared and stops it on shutdown. Left
`false`, only the listener opens and you run `cloudflared tunnel run roji`
yourself.

## Choosing what to publish

Label the container. Only labelled containers are reachable from outside.

```yaml
# docker-compose.yml
services:
  api:
    labels:
      - "roji.tunnel=true"
```

`true`, `1`, `yes` and `on` count as affirmative; anything else — no label, an
empty one, a typo — leaves the route private. roji picks up every container on
its network without being asked, so publishing by default would put all of them
on the internet. Hence the explicit opt-in.

## The DNS record

Pointing `*.{domain}` at the tunnel is yours to do. roji deliberately holds no
Cloudflare API token, so this is the one step it leaves to you.

```bash
cloudflared tunnel route dns roji "*.example.com"
```

A CNAME to `<uuid>.cfargotunnel.com` made in the Cloudflare dashboard does the
same thing.

### Keep the domain to one level

Cloudflare's Universal SSL covers `example.com` and `*.example.com`, nothing
deeper. A two-level name like `tunnel.example.com` needs a certificate for
`*.tunnel.example.com`, which means the paid Advanced Certificate Manager.
Configuration validation warns about it.

## Hostname mapping

The public name is the local one with its suffix swapped.

| Local | Public |
|---|---|
| `web.dev.localhost` | `web.example.com` |
| `api.dev.localhost` | `api.example.com` |

The `Host` your backend receives stays the local name, so a dev server's
allowed-hosts setting (Vite's `server.allowedHosts`, for instance) keeps
listing `*.dev.localhost` as it always did.

## What is never published

Tunnelled requests arrive on a port of their own, separate from the normal
listeners. cloudflared connects from 127.0.0.1 like any local browser, so
nothing in the request tells the two apart — only the port it reached does.
That separation is what makes the following refusals possible.

- `/_api/*` and `/_assets/*`, refused on the path alone and not openable by
  configuration. `/_api/projects/{name}/up` starts containers without
  credentials, and `/_api/logs` reads request logs
- The dashboard hostname and the base domain
- Docker routes without `roji.tunnel`
- Static sites (`static_sites:` has no opt-in spelling, so none are published)

Every refusal answers with the same bare 404. Wording them differently would
tell whoever is probing which hostnames exist.

## Checking it

```bash
roji doctor
```

This verifies that cloudflared is installed, logged in, and that the named
tunnel exists. The DNS record is the one thing roji cannot check, so it only
tells you what to run.

On the dashboard, published routes carry a 🌐 badge and their public URL. A
label with no `tunnel:` block behind it publishes nothing, and shows no badge.

## Deliberately absent

- A `roji tunnel start/stop/status` command. With `auto_start: false` you run
  `cloudflared tunnel run` yourself
- Publishing static sites
- Restarting cloudflared after a crash. roji is not a process manager, so it
  reports the exit and leaves the tunnel down — better than a tunnel that keeps
  failing to authenticate and retrying in silence
- Creating the DNS record through a Cloudflare API token
