---
title: "Search syntax"
description: "Query syntax for lif list --search."
sidebar:
  order: 3
---

:::caution[Raw draft]
Source: `lif list --help`. Today buried in the `Long` text, renders as one paragraph.
:::

- `lif list --search/-S "<query>"`, Bleve query string syntax.
- Exact term: `login`. Fuzzy: `login~`, `login~2`. Wildcard: `log*`. Phrase: `"user login"`.
- Boolean: `user AND login`, `user OR login`.
- Field scopes: `slug:auth`, `title:login`, `body:auth`.
- Combine with filters: status, type, tag, parent, blocked.
- TODO: verify more operators (negation, boost), combine with `--json`, examples from real work.
