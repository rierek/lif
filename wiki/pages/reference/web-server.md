---
title: "Web server"
description: "lif-serve: the web UI and GraphQL HTTP server."
---

## Purpose
Reference for the web server `lif-serve`: start, options, routes, network/CORS behavior and what it offers in addition to the CLI. The page has low priority and follows the implemented scope.

## Audience
Users who start the web interface, and operators who integrate the GraphQL HTTP interface into scripts or other machines.

## Required sections

### What lif-serve is
- Explains: `lif-serve` is a separate binary that provides the web interface and the GraphQL HTTP interface; `lif` itself contains no server.
- `lif serve` in the `lif` binary is a hidden placeholder that points to `lif-serve` and exits with 1 (do not document it, give only this note).
- `lif-serve` knows only the command `serve`; with no argument or when the first argument starts with `-`, `serve` is added.
- Links to `overview/installation` (which binaries exist) and `reference/graphql`.
- Source: FACTS.md §1.0, §1.1 (`lif serve`, `lif-serve`), §8.1

### Starting the server
- Shows `lif-serve` and `lif-serve serve` (equivalent); aliases `server` and `s`.
- Options: `--port`/`-p` (default 8080), `--cors-origin` (list, comma-separated or repeated; default `http://localhost:*` and `http://127.0.0.1:*`; `*` allows all).
- Configuration: `server.port` and `server.cors_origins` in `.lif.yml`; precedence flag over configuration over default.
- Startup message of the server (address, playground, allowed origins) as example output.
- Note that the server loads the store configuration like the CLI (global flags `--config`, `--lif-path`). (TODO: verify)
- Source: FACTS.md §8.1, §4.3, §1.1 (`lif-serve`); `--config`/`--lif-path` on `lif-serve` — `TODO: verify` (Phase 5: run `lif-serve --help`)

### Network and CORS
- The server listens on `:<port>`, that is on all interfaces; warning note for networks.
- CORS allow list: a pattern ending in `:*` matches any port; `*` allows everything.
- The WebSocket upgrade (GraphQL) checks the origin against the same list; disallowed origin → 403 `origin not allowed`.
- Example of access from another machine with an adjusted `--cors-origin`. (TODO: verify)
- Source: FACTS.md §8.1, §4.3; example — `TODO: verify` (Phase 5: check with a second host/tailnet)

### Routes
- Table: `/api/graphql` (GraphQL; WebSocket, OPTIONS, GET, POST), `/playground` (GraphQL Playground), `/api/attachments/:beanId/:filename` (image attachments of the agent chat), `/api/terminal` (WebSocket terminal), all other paths serve the embedded single-page app (fallback to `index.html`).
- Note: the placeholder `:beanId` comes from the code; word the description so it stays clear that it means the item ID.
- Links to `reference/graphql` for `/api/graphql` and `/playground`.
- Source: FACTS.md §8.1

### Web UI
- Explains: the frontend is embedded in the binary (`dist/*`), a SvelteKit/Vite project in `frontend/`; no separate web server needed.
- Shows screenshots of the interface (webp, D38): board, detail view. (TODO: verify)
- Describes views and board columns. (TODO: verify)
- Note: a toolbar button "Run" appears when `worktree.run` is set.
- Source: FACTS.md §8.1, §4.3; DESIGN.md D38; views and board columns — `TODO: verify` (Phase 5: check the code under `frontend/` (BoardView) and the running interface)

### Live updates
- Explains: the server watches the store directory and sends changes to subscribers (`itemChanged` subscription); external changes to files appear without reloading.
- Directories of worktree Stores are also watched and merged into the runtime state.
- Shows a short example: subscription in the playground, change an item with `lif update`, the event appears.
- Source: FACTS.md §8.1, §7.3; example — `TODO: verify` (Phase 5: run it)

### Server-only capabilities
- Names the functions that exist only with `lif-serve`: worktrees, agent sessions (Claude), terminal sessions, file changes and diffs, start/stop of the project, opening in the editor.
- Switches in the configuration: `agent.enabled` (default `true`), `agent.default_mode`, `agent.default_effort`, `worktree.base_ref`, `worktree.path`, `worktree.setup`, `worktree.run`, `worktree.integrate`, `worktree.fetch_timeout`.
- Environment variable `LIF_WORKSPACE_PORT` for shells in workspaces (old: `BEANS_WORKSPACE_PORT`).
- Links to `reference/configuration` for the keys.
- Source: FACTS.md §8.1, §4.3, §7

### Limits
- The facts contain no description of authentication; state honestly what is known: the server listens on all interfaces, protection only through the origin list.
- Do not invent statements about authentication and transport encryption (TLS).
- `lif serve` in the `lif` binary does not work (placeholder).
- Source: FACTS.md §8.1; authentication/TLS — `TODO: verify` (Phase 5: check `internal/commands/serve.go`)
