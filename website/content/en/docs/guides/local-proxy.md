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
nothing discovers it. roji can still give such a server a hostname, HTTPS
included, next to your containers.

There are two ways to do it:

- **`roji run`** starts the dev server for you on a free port. Nothing to
  configure, and no port to pick
- **`proxy_sites`** in the config file names a port you chose. The hostname is
  there whether or not the server is running

## `roji run`: no port to pick

```bash
roji run app -- npx vite --port {port}
# roji: https://app.dev.localhost -> localhost:41873
```

`roji run <name> -- <command>` picks a free port, routes
`https://<name>.<domain>` to it, and runs the command. The route lasts as long
as the command does.

Every project gets a port nothing else is using, so two dev servers that both
default to 5173 or 8787 stop colliding, and no project needs a port written
into its config.

### Passing the port to the command

The command has to listen on the port roji chose. It gets it two ways:

- `{port}` anywhere in the arguments is replaced with the port
- The `PORT` environment variable is set to it

Which one a tool reads is the tool's business:

| Tool | How |
|---|---|
| Vite | `roji run app -- npx vite --port {port}` |
| Wrangler | `roji run api -- npx wrangler dev --port {port}` |
| Next.js, Rails, most others that read `PORT` | `roji run web -- npm run dev` |

Vite and Wrangler ignore `PORT`. Started without `--port {port}` they open
their usual port, the URL roji printed shows an error page, and after 20
seconds `roji run` says that nothing is listening on the port it chose.

The `--` is required. Everything after it belongs to the command, which is
what keeps its flags from being read as roji's.

### What `roji run` does and does not do

- roji has to be running. `roji run` registers the route with it first, and
  does not start the command if it cannot
- The name follows the rule for `host` below. A name the config file declares
  under `proxy_sites` or `static_sites` is refused, as is one another
  `roji run` is using
- A container routed to the same hostname is hidden while the command runs,
  and `roji run` says so
- The command's exit code is `roji run`'s; a command stopped by a signal is
  reported as 128 plus the signal number, as a shell does (130 for Ctrl-C).
  Ctrl-C and `kill` reach the command and everything it started, so `npx`
  wrappers do not leave a server behind
- The route can only be registered from the machine roji runs on, whatever
  `bind` is set to
- If `roji run` itself is killed outright (`kill -9`), the route goes away
  within 30 seconds. The dev server it started is not stopped
- The route is not published through the [tunnel](/docs/guides/tunnel/). Use
  `proxy_sites` with `tunnel: true` for that

## `proxy_sites`: a port you chose

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

No restart needed. Either:

```bash
roji config reload
```

Or click **Reload Config** in the dashboard. This is also how to point a site
at a different port while roji is running.

The command lists every declared site and says which were registered. A site
that was left out — a target that is not a URL, a hostname that is the
dashboard's — is marked with the reason; the reload still applies the rest.

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
