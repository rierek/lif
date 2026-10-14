---
title: "lif-serve"
description: "Web UI and GraphQL API server."
sidebar:
  order: 7
---

:::caution[Raw draft]
Source: `lif-serve --help`, `wiki/web-ui-and-api.md`.
:::

**Intro**
- Separate binary. Starts web UI + GraphQL HTTP API. `lif-serve` == `lif-serve serve`.
- Aliases: `serve`, `server`, `s`.

**Reference**
- `-p, --port int` (default 8080)
- `--cors-origin strings` (default localhost/127.0.0.1)
- Global: `--config`, `--lif-path`
- Routes, network binding: TODO
- No `version`/`check` on this binary.
- TODO: screenshot, tailnet/remote example, generate flags like command pages.
