---
title: "Dev Servers on the Host"
slug: "local-proxy"
description: "Give a hostname to a server running outside Docker."
weight: 2
date: 2026-10-12T00:00:00+00:00
lastmod: 2026-10-12T00:00:00+00:00
draft: false
toc: true
---

roji discovers containers on its own. A dev server you start on the host
itself — `vite dev`, `wrangler dev`, `rails s` — is not a container, so
nothing discovers it. The `proxy_sites` section of the config file gives such
a server a hostname, HTTPS included, next to your containers.

## Configuration

Add entries to `~/.config/roji/config.yaml`:

```yaml
proxy_sites:
  - host: app                       # -> app.dev.localhost
    target: http://localhost:5173   # where the dev server listens
  - host: api
    target: http://127.0.0.1:8787
```

Start the dev server as usual and open `https://app.dev.localhost`.

### Hostname

`host` follows the same rule as [static sites](/docs/guides/static-sites/):

- **No dot**: expanded to `{host}.{domain}` (`app` → `app.dev.localhost`)
- **With dot**: used as written

Keep the hostname one level under the base domain. roji's certificate covers
`*.{domain}` and nothing deeper, so a browser rejects `a.b.dev.localhost` as
it does a name on another domain. `roji doctor` reports the ones that fall
outside.

The dashboard's hostname (`roji.dev.localhost` by default) and the base domain
itself cannot be used: the dashboard answers on them first, so a site declared
there is skipped with a warning.

### Target

`target` is a URL, scheme included. `http` is the only scheme: roji terminates
TLS itself.

- `target: 8787` and `target: 127.0.0.1:8787` are not accepted. There is no
  `port:` shorthand either, so that a site has one spelling
- A path in the target is put in front of every request:
  `target: http://localhost:3000/base` sends `/items` to `/base/items`
- Prefer `http://localhost:PORT` when you are unsure which address the server
  binds. Depending on the tool and the Node.js version, "localhost" means
  `127.0.0.1` or `::1`, and roji tries both
- A target on another machine works, with a validation warning: requests to it
  leave this machine unencrypted

### What the dev server receives

The same as a container does:

- The `Host` header is the hostname the browser asked for
  (`app.dev.localhost`), not the target's. Dev servers that check the host,
  such as Vite's `server.allowedHosts`, see that name. Vite allows
  `.localhost` names by default
- `X-Forwarded-Proto: https`, `X-Forwarded-For` and `X-Real-IP`
- WebSocket (Vite's HMR), Server-Sent Events and gRPC pass through

## When the server is not running

A proxy site whose dev server has not been started is the normal state of
things, so roji registers the site without checking the target. A request then
gets a `502` page naming the target address and saying what to start. roji
cannot start the server for you: unlike a Compose project, it knows nothing
about the process.

`roji doctor` lists the proxy sites nothing is listening behind, as a warning.

## Precedence over Docker routes

A proxy site holds its hostname, like a static site does. If a container is
routed to the same hostname, the proxy site answers and roji logs which Docker
route it hides. A hostname declared twice in the config file — under
`proxy_sites`, `static_sites`, or one of each — goes to the later declaration,
`proxy_sites` being read after `static_sites`.

A proxy site takes the whole hostname. There is no path prefix for it.

## Applying changes

No restart needed. Click **Reload Config** in the dashboard, or call the API
behind the button:

```bash
curl -X POST https://roji.dev.localhost/_api/config/reload
```

This is also how to point a site at a different port while roji is running.

## Authentication

```yaml
proxy_sites:
  - host: api
    target: http://127.0.0.1:8787
    auth:
      basic:
        user: admin
        pass: secret
        realm: API   # optional
```

See the [Basic Authentication](/docs/guides/basic-auth/) guide.

## Publishing through the tunnel

Receiving a webhook on a dev server is what the
[tunnel](/docs/guides/tunnel/) is for. Opt the site in with `tunnel: true`:

```yaml
proxy_sites:
  - host: hook
    target: http://127.0.0.1:8787
    tunnel: true        # reachable as https://hook.example.com
```

The public name is the local one with the base domain swapped for the
tunnel's, so only a hostname under the base domain can be published.

A site without the key stays private. When the dev server is not running, a
request through the tunnel gets the same bare 404 as every other refusal
there, not the page described above.
