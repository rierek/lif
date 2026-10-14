---
title: "Web server"
description: "lif-serve: the web UI and GraphQL HTTP server."
---

`lif-serve` is a separate binary. It serves the web interface, the GraphQL HTTP API and a terminal WebSocket for one Lif store. Measured on Lif 0.14, 2026-10-14. The web UI itself was read from the code, not run (frontend not built in the test checkout).

:::danger[No authentication, no TLS, exposed terminal [8h4v]]
`lif-serve` has **no authentication** and **no TLS**. It listens on **all network interfaces**; there is no flag or key to choose a bind address. Anyone who can reach the port can read and change every item, and can open a **shell as the user running `lif-serve`** through `/api/terminal`. The origin list protects only browsers; `curl` and scripts are not checked.

Run it only on a machine you trust and a network you trust. Convention, not enforced by Lif: keep the port blocked by a firewall and reach it through an SSH tunnel (`ssh -L 8080:localhost:8080 host`) or a VPN. Tracked as [8h4v] on the [Known limitations](../../known-limitations/) page.
:::

## What lif-serve is

- `lif` itself contains no server. `lif serve` in the `lif` binary is a hidden placeholder that points to `lif-serve` and exits with 1.
- `lif-serve` knows one command, `serve`. Without arguments, or when the first argument starts with `-`, `serve` is added. Aliases: `server`, `s`.
- It loads the store like the CLI and accepts the global flags `--config` and `--lif-path`.

See [Installation](../../overview/installation/) for the binaries and [GraphQL](../graphql/) for the API.

## Starting the server

```sh
lif-serve
lif-serve --port 9090 --cors-origin 'http://192.168.1.50:*'
```

| Option | Config key | Default |
|---|---|---|
| `--port`, `-p` | `server.port` | `8080` |
| `--cors-origin` (list; comma-separated or repeated) | `server.cors_origins` | `http://localhost:*`, `http://127.0.0.1:*` |

A flag overrides the config value, which overrides the default. A given `--cors-origin` **replaces** the default list; it does not extend it. See [Configuration](../configuration/).

Startup output:

```text
[lif] Starting server at http://localhost:18550/
[lif] GraphQL Playground: http://localhost:18550/playground
[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*
```

The text says `localhost`, but the server listens on `:<port>`, all interfaces [9a66]. SIGINT or SIGTERM prints `Shutting down...` and `Server stopped`.

## Network and CORS

- Pattern `scheme://host:*` matches any port. `*` allows everything. Other entries match exactly.
- The origin list controls the CORS response header and the WebSocket upgrade (`/api/graphql`, `/api/terminal`). A disallowed WebSocket origin gets 403 `origin not allowed`.
- HTTP requests are **never** rejected because of their origin. A `POST /api/graphql` from a foreign origin runs, mutations included; the browser only refuses to hand the response to the page.
- A request without an `Origin` header (curl, scripts) always passes.
- `--cors-origin '*'` allows every WebSocket origin, including the terminal.

To open the UI from another machine, add that machine's address as an origin, for example `--cors-origin 'http://192.168.1.50:*'`. Without it, the page loads but the WebSocket (live updates) fails with 403. Think about the no-authentication warning above before you do this.

## Routes

| Path | Content |
|---|---|
| `/api/graphql` | GraphQL over WebSocket, OPTIONS, GET, POST. See [GraphQL](../graphql/). |
| `/playground` | GraphQL Playground. Loads GraphiQL from a CDN, so it needs internet access. |
| `/api/attachments/:beanId/:filename` | Image attachments of the agent chat. `:beanId` is the item ID. |
| `/api/terminal` | WebSocket terminal. Starts `$SHELL` (else `/bin/sh`). |
| everything else | The embedded single-page app (falls back to `index.html`). |

Limits: request body 10 MB, 100 concurrent WebSocket upgrades (then 503), read and write timeout 15 s.

## Web UI

The frontend (SvelteKit) is embedded in the binary. No second server is needed.

- `/planning` is the **Backlog**: two sections, Todo and Draft, with top-level items and their children nested.
- `/planning/board` is the **Board**: three fixed columns Todo, In Progress, Completed. Dragging a card sets `status` and `order`.
- `/workspace/<id>` is the workspace of a Git worktree: agent chat, changes, terminal. It appears only when `agent.enabled` is true. A **Run** button appears when `worktree.run` is set.
- The UI has a filter field (substring over title, type, status, ID, tags). It does not use [search syntax](../search-syntax/).
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

These calls need no credentials either. Config keys: `agent.enabled`, `agent.default_mode`, `agent.default_effort`, `worktree.base_ref`, `worktree.path`, `worktree.setup`, `worktree.run`, `worktree.integrate`, `worktree.fetch_timeout`; see [Configuration](../configuration/). `agent.enabled: false` only hides the agent parts of the UI; the server-side agent code stays reachable.

Shells in workspaces get the environment variable `LIF_WORKSPACE_PORT` (legacy `BEANS_WORKSPACE_PORT`).

## Limits

- No authentication, no TLS, no bind-address option, exposed terminal: [8h4v].
- `lif serve` in the `lif` binary is only a placeholder.
