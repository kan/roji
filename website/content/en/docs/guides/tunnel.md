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

Sites declared in the config file opt in with a `tunnel` key of their own,
which takes `true` or `false`:

```yaml
# ~/.config/roji/config.yaml
proxy_sites:
  - host: hook
    target: http://127.0.0.1:8787
    tunnel: true
static_sites:
  - host: docs
    root: ~/projects/docs/build
    tunnel: true
```

A declared site holds its hostname ahead of any container, and the tunnel
follows that: when a site without `tunnel: true` shares a hostname with a
labelled container, the hostname is not published.

A third way is to list hostnames under `tunnel.routes`. The routes on them are
published without a label or a `tunnel` key, which is what the next section
uses.

## Using several machines

A tunnel's credentials (`~/.cloudflared/<tunnel ID>.json`) exist only on the
machine that created it, and Cloudflare does not choose between two machines
running the same tunnel. Each machine needs a tunnel of its own, and a public
name's DNS record points at one of them, so only one machine publishes a given
name at a time.

When `config.yaml` is shared between machines (from a dotfiles repository, for
example), put what differs in `local.yaml` next to it. roji reads it on top of
`config.yaml`:

```yaml
# ~/.config/roji/config.yaml (shared)
tunnel:
  domain: example.com
  name: auto
  auto_start: true
```

```yaml
# ~/.config/roji/local.yaml (this machine only)
tunnel:
  routes: [api]   # publish api.dev.localhost as api.example.com from here
```

- `name: auto` names the tunnel `roji-<hostname>`: the first label of the
  machine's hostname, lowercased, with anything but letters, digits and hyphens
  replaced by a hyphen. `roji doctor` prints the name and the
  `cloudflared tunnel create` command for it. A name written out still works
- `routes` takes names written like a site's `host`: `api` stands for
  `api.{domain}`, and a name with a dot is taken as written. It covers
  containers and sites from the config file, alongside their own label or key.
  It does not cover routes registered by `roji run`
- Mappings in the two files merge key by key, so `local.yaml` can set one key
  of `tunnel` and keep the rest. A list replaces the shared one whole, and
  `null` removes a key: `tunnel: null` turns the tunnel off on that machine
- `roji config reload` applies a change to `routes` without a restart. The rest
  of `tunnel` needs one
- The per-machine file is read from the directory of the config file in use.
  It is `local.yaml` next to `config.yaml`, and named after the config file
  otherwise (`--config roji.yaml` reads `roji.local.yaml`). `roji config path`
  shows where

Moving a public name to another machine is a DNS change, which stays yours to
make:

```bash
cloudflared tunnel route dns --overwrite-dns roji-laptop api.example.com
```

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
- Static sites and proxy sites without `tunnel: true`

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
- Restarting cloudflared after a crash. roji is not a process manager, so it
  reports the exit and leaves the tunnel down — better than a tunnel that keeps
  failing to authenticate and retrying in silence
- Creating the DNS record through a Cloudflare API token
