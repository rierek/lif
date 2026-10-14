---
title: "Dependencies"
description: "Model blockers between items and see what is ready next."
---

## Purpose
Explains how blockers between items are modeled, how Lif determines "blocked" and "ready", and how to view dependencies. Describes exactly where the model has gaps (no symmetry check, fixed definition of "done").

## Audience
People and agents who want to decide what to start next, and planners who maintain dependencies between epics and tasks. They need the exact semantics before relying on `lif next`.

## Required sections

### What
- Two fields: `blocking` (items that this item blocks) and `blocked_by` (items that block this one); an edge is stored on one side only, namely on the item that was just changed.
- On read, Lif merges both directions; deleting an item removes all incoming references.
- Blocked means: a direct active blocker or a blocked ancestor in the parent chain (explicit versus implicit).
- "Ready" = not blocked, status not `in-progress`, `draft` or an archive status, and no ancestor with status `completed` or `scrapped`.
- Commands: `lif next` (highest priority among the ready items), `lif list --ready`, `--is-blocked`, `--unblocked`, `lif graph` (formats dot, ascii, mermaid, json).
- Source: FACTS.md §5.1, §5.2, §5.4, §5.5

### Why
- Complex relations become explicit and manageable; blockers are visible before anyone starts.
- `lif next` answers "What now?" even for agents without their own planning logic.
- Supports structured workflows without being a workflow engine.
- Source: CONCEPT.md §4.3; FACTS.md §5.2

### How it looks
- Command loop: `lif create --blocked-by <id>` or `lif update A --blocking B`; afterwards `lif next` returns the blocker A, not B.
- Observed behavior: `lif update B --blocking A` is rejected with a cycle message (`adding blocking relationship would create cycle: [B A B]`).
- Comparison of `lif list --ready`, `--is-blocked`, `--unblocked`; `--ready` and `--is-blocked` are mutually exclusive.
- A blocked parent makes all children implicitly blocked; generate example output in phase 5 (TODO: verify).
- `lif graph --format mermaid` with example; option `--depth` and `--relation parent|blocks`.
- Source: FACTS.md §5.2, §5.3, §5.5, §5.6; (TODO: verify) for concrete outputs: run in phase 5 with `lif list --ready` and `lif graph --format mermaid`

### Limits
- No symmetry check: `A.blocking=[B]` and `B.blocked_by=[A]` can both be present or only one side; `lif check` does not report the mismatch.
- "Done" is hard-coded: only `completed` and `scrapped` resolve a blocker; `draft`, `todo`, `in-progress` and custom archive status keep blocking.
- Cycle detection follows one link type at a time; it may not detect mixed cycles across `blocking` and `blocked_by` (TODO: verify)
- `lif check` reports cycles from manual edits; `lif check --fix` removes only broken links and self-references, never cycles.
- `lif next` ignores dependency order, `lif roadmap` uses it; containers can be returned as ready.
- Blockers that do not exist in the store are ignored; a blocker blocks only by its own status, not by its parents.
- Source: FACTS.md §5.1, §5.2, §5.3, §5.6, §6.7; (TODO: verify) for mixed cycles: FACTS gives this only as a conclusion (INFERENCE), test in phase 5

### Reference links
- `lif next` (`reference/commands/lif-next`): filters, sorting, behavior without matches.
- `lif list` (`reference/commands/lif-list`): `--ready`, `--is-blocked`, `--unblocked`, `--has-blocking`.
- `lif graph` (`reference/commands/lif-graph`): formats and depth.
- `lif check` (`reference/commands/lif-check`): check links and cycles.
- `lif create` and `lif update` (`reference/commands/lif-create`, `reference/commands/lif-update`): options `--blocking`, `--blocked-by`.
- GraphQL (`reference/graphql`): `addBlocking`, `addBlockedBy`, `ItemFilter`.
- Source: DESIGN.md D24, D25, D27 and Open (page cut phase 4: dependency semantics canonical here plus generated command pages)
