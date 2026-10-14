---
title: "About GraphQL"
description: "GraphQL API for items."
sidebar:
  order: 6
---

:::caution[Raw draft]
Source: `internal/graph/schema.graphqls`, `lif graphql --help`, `lif-serve`.
:::

- `lif graphql <query>` (alias `lif query`): runs against local store, no server needed. Same schema as `lif-serve` HTTP.
- No argument: reads query from stdin.
- Flags: `-v/--variables` (JSON string), `-o/--operation`, `--json` (no ANSI), `--schema` (print SDL, exit).
- Output: `data` part only. Error: stderr, non-zero exit, no partial data.
- Use for: richer queries than `list` flags, scripted mutations, web UI backend.
- Use plain `--json` commands for simple reads.
- TODO: generate schema reference from SDL, 3 real examples, CORS note.
