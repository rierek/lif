---
title: "Web server"
description: "lif-serve: the web UI and GraphQL HTTP server."
---

`lif-serve` is a separate binary. It serves the web interface, the GraphQL HTTP API and a terminal WebSocket for one Lif store. Measured on Lif 0.14, 2026-10-14. The web UI itself was read from the code, not run (frontend not built in the test checkout). The network behaviour (bind address, access token, host and origin checks, terminal) changed in 0.15; those parts were checked on a built 0.15 development binary, 2026-10-15.

:::caution[Network access, TLS and the terminal [8h4v]]
Since 0.15 `lif-serve` listens on `127.0.0.1` only, so only this machine can reach it. There is **no authentication on loopback**: every local user of the machine can use the port. A web page cannot: the server refuses a request whose `Host` names a host outside the origin list, and a request that changes something when its `Origin` is neither in the list nor the server's own address.

`--host` with another address opens the server to the network. The server then generates an **access token**, prints it at startup and asks every client for it. The terminal, a **shell as the user running `lif-serve`**, stays off unless you add `--allow-remote-terminal`. A client that has the token can still start agent sessions, and those run commands. There is **no TLS**: the token travels in clear text. For access from another machine prefer an SSH tunnel (`ssh -L 8080:localhost:8080 host`) or a VPN to the default loopback address. Tracked as [8h4v] on the [Known limitations](../../known-limitations/) page.
:::

## What lif-serve is

- `lif` itself contains no server. `lif serve` in the `lif` binary is a hidden placeholder that points to `lif-serve` and exits with 1.
- `lif-serve` knows one command, `serve`. Without arguments, or when the first argument starts with `-`, `serve` is added. Aliases: `server`, `s`.
- It loads the store like the CLI and accepts the global flags `--config` and `--lif-path`.

See [Installation](../../overview/installation/) for the binaries and [GraphQL](../graphql/) for the API.

## Starting the server

```sh
lif-serve
lif-serve --port 9090
lif-serve --host 0.0.0.0
```

| Option | Config key | Default |
|---|---|---|
| `--host` | none | `127.0.0.1` |
| `--port`, `-p` | `server.port` | `8080` |
| `--cors-origin` (list; comma-separated or repeated) | `server.cors_origins` | `http://localhost:*`, `http://127.0.0.1:*` |
| `--allow-remote-terminal` | none | off |

`--host` takes an IP address or a host name. The server binds the address first and then looks at what it really got. A loopback address (`127.0.0.1`, `::1`, `127.0.0.2`) keeps the safe defaults, and the server adds that address to the origin list, so its own URL always works. Anything else counts as the network: `0.0.0.0`, `::`, an empty value, a LAN address, a name that resolves to one. `--host` and `--allow-remote-terminal` have no config key on purpose: a committed `.lif.yml` cannot open the server to the network for everyone who clones the repository.

A flag overrides the config value, which overrides the default. A given `--cors-origin` **replaces** the default list; it does not extend it, so name `http://localhost:*` yourself if you still open the UI as `localhost`. The page that `lif-serve` itself serves needs no entry, under whatever address you open it. A dev server of the UI and every other page do. The entry `*` is special, see [Network and CORS](#network-and-cors). See [Configuration](../configuration/).

Startup output with the defaults:

```text
[lif] Listening on 127.0.0.1:8080 (loopback only: reachable from this machine only)
[lif] Starting server at http://127.0.0.1:8080/
[lif] GraphQL Playground: http://127.0.0.1:8080/playground
[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*
[lif] Terminal: enabled
```

The first line is the address the listener really got [9a66]. With `--host 0.0.0.0` the output is:

```text
[lif] Listening on [::]:8080 (ALL network interfaces: reachable from the network)
[lif] WARNING: anyone who can reach this port and has the access token can read and change items and run agent sessions as the user running lif-serve. There is no TLS, so the token travels in clear text. Prefer the default loopback address with an SSH tunnel or a VPN.
[lif] Access token: <token>
[lif] Open the UI: http://192.168.1.50:8080/?token=<token>
[lif] GraphQL Playground: http://192.168.1.50:8080/playground?token=<token>
[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*
[lif] Terminal: disabled (the server is reachable from the network). Add --allow-remote-terminal to enable it.
```

A listener on all interfaces reports itself as `[::]` (IPv4 and IPv6) where the system has IPv6, and as `0.0.0.0` otherwise. The two address lines name the first LAN IPv4 address of the machine (a VPN address counts; `localhost` when there is none), and with `--host <address>` that address. Open the first one in the browser: it carries the token. The line `Starting server at …` is printed for a loopback listener only, because the address it would name answers 401 without the token. Every `WARNING` line marks a setting that exposes more than the defaults: a listener beyond loopback, a remote terminal, `*` in the origin list. A port that is already taken stops the server at once with `server error: listen tcp 127.0.0.1:8080: bind: address already in use`. SIGINT or SIGTERM prints `Shutting down...` and `Server stopped`. Running requests get 5 s to finish. After a hard deadline of 10 s the process exits anyway with `Shutdown deadline exceeded, forcing exit` on stderr. The access log lines (`[GIN] …`) show the path without the query string, so the token never reaches the log.

## Network and CORS

What may use the server depends on the address it listens on.

**On loopback (default)** two checks stand in front of every route. Both read the origin list.

- **`Host` header.** A request whose `Host` names a host outside the list gets 403 `host "…" is not allowed`. This stops DNS rebinding: a page of another site makes its name resolve to `127.0.0.1`, so the browser treats the server as that site's own origin. The list gives the host names (`localhost` and `127.0.0.1` by default, and the loopback address the server is bound to); the port is not compared.
- **`Origin` header.** A request that can change state, that is any method but GET, HEAD and OPTIONS and every WebSocket upgrade, gets 403 `origin "…" is not allowed` when its `Origin` is neither in the list nor the server's own origin. The server's own origin is `http://` plus the `Host` the request was sent to: the page that `lif-serve` serves comes from there, so the built-in UI needs no `--cors-origin`. `Origin: null` (sandboxed pages) is refused. This stops a page of another site from sending `POST /api/graphql` or opening a WebSocket. Reads are not refused, but the page cannot read the answer: the CORS header goes only to listed origins.
- **No `Origin` at all** passes, so `curl` and scripts keep working, as in the example on [GraphQL](../graphql/). The two checks keep web pages out; they are no protection against a program, which can send any header. The exception is `/api/terminal`: only the web UI uses it and a browser always sends `Origin` on a WebSocket upgrade, so a client without one gets 403 there.
- A `POST /api/graphql` needs `Content-Type: application/json`. Form and `text/plain` posts, the kinds a page can send without a CORS preflight, answer `transport not supported`, and a mutation over GET answers `GET requests only allow query operations`.
- Pattern `scheme://host:*` matches any port. Other entries match exactly. With the default list, a page served from any other local port (`http://localhost:*`) passes: another local web app can use the server.
- `--cors-origin '*'` sends `Access-Control-Allow-Origin: *`, so any page can **read** what the server answers: items, workspaces, agent sessions. That is all it does. The host check stays on, a page of another site still cannot change anything or open a WebSocket or the terminal, and the default local origins stay allowed as if they were listed. Name the origins that may change things in the list. The server prints a `WARNING` for `*` that names the shell. In 0.14 `*` also let every origin open a WebSocket, the terminal included.

**Beyond loopback** a script can send any header, so the checks above prove nothing against it. The server asks for the access token instead:

- Every request needs the token, the UI and the playground included: as `Authorization: Bearer <token>` for scripts, or in the cookie `lif_token_<port>` for the browser. Without it the answer is 401.
- Open the address printed at startup once (`http://<address>:<port>/?token=<token>`). The server sets the cookie (`HttpOnly`, `SameSite=Lax`, until the browser closes) and redirects to the same address without the token. The token is new on every start and is in the server log, so protect the log like a password.
- The `Host` header does not matter there: the token is the protection.
- The origin check still applies to requests that change state, and the UI the server serves passes it as its own origin: open `http://192.168.1.50:8080/?token=<token>` and no `--cors-origin` is needed.
- A page of another origin cannot use a server on the network, and that is by design. Its preflight carries no credentials and gets 401, and a browser sends neither the cookie nor an `Authorization` header across origins. Only the UI the server serves, and scripts that send the Bearer header, work.
- `/api/terminal` answers 403 `the terminal is disabled…`, and the GraphQL calls `writeTerminalInput`, `startRun` and `stopRun` fail with `terminal support not available`. `--allow-remote-terminal` turns the terminal on; the token is still required.
- There is no TLS. A reverse proxy in front of a loopback server also puts it on the network without the token: add authentication at the proxy and list the proxy's host name in `--cors-origin`.

## Routes

| Path | Content |
|---|---|
| `/api/graphql` | GraphQL over WebSocket, OPTIONS, GET, POST. See [GraphQL](../graphql/). |
| `/playground` | GraphQL Playground. Loads GraphiQL from a CDN, so it needs internet access. |
| `/api/attachments/:beanId/:filename` | Image attachments of the agent chat. `:beanId` is the item ID. |
| `/api/terminal` | WebSocket terminal. Starts `$SHELL` (else `/bin/sh`). Needs an `Origin` header. Off when the server listens beyond loopback, unless `--allow-remote-terminal` is given. |
| everything else | The embedded single-page app. A path that matches no file and has no `.` gets `index.html`. A path with a `.` that matches no file (a missing `.js` or `.css`) gets a plain 404. |

Limits: request body 10 MB, 100 concurrent WebSocket upgrades (then 503), read and write timeout 15 s, idle timeout 60 s. `/api/graphql` has no write timeout: hooks and gates run synchronously inside a mutation, and an after hook can take longer than 15 s. `/api/graphql` subscriptions and `/api/terminal` share the 100 WebSocket slots.

## Web UI

The frontend (SvelteKit) is embedded in the binary. No second server is needed.

- `/planning` is the **Backlog**: two sections, Todo and Draft, with top-level items and their children nested.
- `/planning/board` is the **Board**: three fixed columns Todo, In Progress, Completed. Dragging a card sets `status` and `order`.
- `/workspace/<id>` is the workspace of a Git worktree: agent chat, changes, terminal. It appears only when `agent.enabled` is true. A **Run** button appears when `worktree.run` is set.
- The UI has a filter field (substring over title, type, status, ID, tags). It does not use [search syntax](../search-syntax/).
- The UI keeps layout in `localStorage` under `lif-planning-chat`, `lif-changes-pane`, `lif-filter-text` and `lif-split-<panel>`. The selected item is mirrored as `?item=<id>`, so `http://localhost:8080/?item=<id>` is a deep link.
- There is no settings page and no theme switch; the colour scheme follows the operating system. `display.theme` is not read by the UI.
- The status list is hard-coded (`draft`, `todo`, `in-progress`, `completed`, `scrapped`). Custom statuses are not shown on the board; `scrapped` is shown in neither view.

Screenshot follows.

## Live updates

The server watches the store directory and pushes changes through the `itemChanged` subscription. The source is the file system, so `lif update` in another process and a manual edit of a file both appear without reloading. Measured: a change reached the subscriber about one second later, as `UPDATED`, `CREATED` or `DELETED`. Directories of worktree stores are watched too. The subscription is described on [GraphQL](../graphql/#subscriptions).

## Server-only capabilities

Only `lif-serve` provides worktrees, agent sessions, terminal sessions and starting or stopping the project run command. In `lif graphql` the same mutations fail:

| Call | `lif graphql` | `lif-serve` |
|---|---|---|
| `createWorktree(name:"x")` | `Error: graphql: worktree support not available` | creates a Git worktree |
| `stopAgent(itemId:"...")` | `Error: graphql: agent manager not available` | `true` |
| `writeTerminalInput(...)` | `Error: graphql: terminal support not available` | `true` |

On loopback these calls need no credentials. Beyond loopback they need the access token like every other call, and with the terminal off `writeTerminalInput` fails with `terminal support not available`. Config keys: `agent.enabled`, `agent.default_mode`, `agent.default_effort`, `worktree.base_ref`, `worktree.path`, `worktree.setup`, `worktree.run`, `worktree.integrate`, `worktree.fetch_timeout`; see [Configuration](../configuration/). `agent.enabled: false` only hides the agent parts of the UI; the server-side agent code stays reachable. An agent session in act mode, the default, runs commands without asking, so a client that can send an agent message can run commands even when the terminal is off.

Shells in workspaces get the environment variable `LIF_WORKSPACE_PORT` (legacy `BEANS_WORKSPACE_PORT`).

## Limits

- No TLS: beyond loopback the access token travels in clear text [8h4v].
- No authentication on loopback: every local user of the machine, and every program that runs as you, can use the port [8h4v].
- With the default origin list a page served from any other local port can use the server (see [Network and CORS](#network-and-cors)).
- Beyond loopback the terminal pane of the UI cannot connect while the terminal is off and retries every 500 ms.
- `lif serve` in the `lif` binary is only a placeholder.
