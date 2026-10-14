---
title: "Dependencies"
description: "Blocking and blocked-by links, ready work, and the dependency graph."
---

Items can block each other. Lif works out what is blocked, what is ready and what to start next. Measured on Lif 0.14, 2026-10-14.

## What

- Two fields: `blocking` (items this item blocks) and `blocked_by` (items that block this one). Lif stores an edge on one side only: on the item you just changed.
- On read, Lif merges both directions. Deleting an item removes all references to it.
- An item is **blocked** if a direct blocker is still active, or if an ancestor in the parent chain is blocked.
- An item is **ready** if it is not blocked, its status is not `in-progress`, `draft` or an archive status, and no ancestor is `completed` or `scrapped`.
- Commands: `lif next` (highest priority among the ready items), `lif list --ready`, `--is-blocked`, `--unblocked`, and `lif graph` (formats `dot`, `ascii`, `mermaid`, `json`).

## Why

- Relations become explicit. You see blockers before you start.
- `lif next` answers "What now?" for people and for agents without their own planning logic.
- You get structured workflows without a workflow engine.

## How it looks

Task `ft-o87g` ("Reset page") waits for `ft-957g` ("Send reset mail"):

```text
$ lif update ft-o87g --blocked-by ft-957g
Updated ft-o87g ft-o87g--reset-page.md
$ lif list --is-blocked -q
ft-o87g
$ lif list --ready -q
ft-igeu
ft-2gae
ft-txrv
ft-957g
```

`lif graph --format mermaid --relation blocks` draws the edge (node styling left out):

```text
ft-957g -->|blocks| ft-o87g
```

`--ready` and `--is-blocked` exclude each other. `lif graph` takes `--depth` and `--relation parent|blocks`.

A cycle is refused when you add the link:

```text
$ lif update ft-957g --blocked-by ft-o87g
Error: adding blocked-by relationship would create cycle: [ft-957g ft-o87g ft-957g]
```

After you complete the blocker with `lif complete ft-957g`, `ft-o87g` is ready:

```text
$ lif list --ready -q
ft-igeu
ft-2gae
ft-txrv
ft-o87g
```

Note that the ready list contains containers (the feature `ft-igeu`, the epic, the milestone). `lif next` can return a container too. In this store it returned the feature, because it has priority `high`.

## Limits

- No symmetry check. `A.blocking=[B]` and `B.blocked_by=[A]` can both be present, or only one side. `lif check` does not report a mismatch.
- "Done" is fixed in the code. Only `completed` and `scrapped` release a blocker. `draft`, `todo`, `in-progress` and custom archive status keep blocking.
- Cycle detection follows one field at a time. A cycle of three or more items whose path mixes `blocking` and `blocked_by` is accepted without a message, and `lif check` stays green. All items in it stay blocked [et2a]. Cycles of two items, and cycles whose edges all use one field, are refused.
- `lif check` reports cycles that come from manual edits. `lif check --fix` removes broken links and self-references, never cycles.
- `lif next` ignores dependency order among ready items. `lif roadmap` uses it.
- A blocker that does not exist in the store is ignored. A blocker blocks by its own status only, not by its parents.

See [Known limitations](../../known-limitations/) for [et2a].

## Reference links

- [`lif next`](../../reference/commands/lif-next/): filters, sorting, behavior without matches.
- [`lif list`](../../reference/commands/lif-list/): `--ready`, `--is-blocked`, `--unblocked`, `--has-blocking`.
- [`lif graph`](../../reference/commands/lif-graph/): formats and depth.
- [`lif check`](../../reference/commands/lif-check/): links and cycles.
- [`lif create`](../../reference/commands/lif-create/) and [`lif update`](../../reference/commands/lif-update/): `--blocking`, `--blocked-by`.
- [GraphQL](../../reference/graphql/): `addBlocking`, `addBlockedBy`, `ItemFilter`.
