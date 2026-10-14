---
title: "Flexible planning"
description: "Types, statuses and hierarchy presets that adapt to how you work."
---

Types, ranks, status, priority and parents model the structure of a project. Lif enforces the configured rules when it writes. Measured on Lif 0.14, 2026-10-14.

## What

- An item has a `type` (with a rank), a `status`, a `priority`, `tags` and at most one `parent`. A parent is valid if the child's rank is strictly greater than the parent's.
- Built-in types: `milestone` (rank 1), `epic` (2), `feature` (3), `bug` and `task` (4).
- Built-in status: `in-progress`, `todo`, `draft`, `completed`, `scrapped`. Built-in priorities: `critical`, `high`, `normal`, `low`, `deferred`.
- Custom types, status and priorities go in `.lif.yml` (`types`, `statuses`, `priorities`). They override or extend the built-in entries. `types_exclusive` replaces the type table completely.
- Four presets via `lif init --profile`: `classic`, `todo`, `simple`, `complex`. Lif writes the preset into the configuration once. The file does not refer to the preset name afterwards.
- Planning commands: `lif roadmap`, `lif milestones`, `lif progress`, `lif order` (manual order in the field `order`).

## Why

- The structure belongs to the project. Presets are examples, not a fixed template.
- One tool covers a flat to-do list (`todo`), a milestone-to-task tree (`classic`) or a release model (`complex`).
- Lif checks the configured rules on every write through Lif.

## How it looks

| Preset | Types (rank) |
|---|---|
| `classic` | `milestone` (1), `epic` (2), `feature` (3), `bug` (4), `task` (4) |
| `todo` | `task` |
| `simple` | `milestone`, `bucket`, `epic`, `feature`, `bug`, `task` |
| `complex` | `release`, `bucket`, `feature`, `improvement`, `chore`, `story`, `bug`, `task` |

`bucket` has `roadmap: false`. It does not appear in `lif roadmap` or `lif milestones`. Full definitions with all ranks: [Configuration, Presets](../../reference/configuration/#presets).

Rank rule on write. A task cannot be the parent of a task:

```text
$ lif create "Nested" -t task --parent ft-957g
Error: failed to create item: task items can only have milestone, epic, or feature as parent, not task
```

Roadmap of a small `classic` store, as Markdown (excerpt, `--depth 2`):

```markdown
# Roadmap

## Milestone: Ship v1 ([ft-2gae](.lif/ft-2gae--ship-v1.md))

### Epic: Login flow ([ft-txrv](.lif/ft-txrv--login-flow.md))
```

`lif update --status foo` is refused too: `invalid status: foo (must be in-progress, todo, draft, completed, scrapped)`.

<details>
<summary>How to use acceptance criteria</summary>

Lif has no special field for acceptance criteria. Write them as a checklist in the item body, one observable criterion per line:

```markdown
## Acceptance
- [ ] A reset mail arrives within one minute
- [ ] The link in the mail works once
```

Tick the boxes before you run `lif complete`. Put evidence in the item or under `attachments/<item-id>/`. A checklist is a convention, not enforced by Lif. Lif does not read the boxes.

What Lif does enforce is a required field per status. `lif.require_fields_on` maps a status to extra keys:

```yaml
lif:
  require_fields_on:
    completed: [commit]
```

If a value is missing, `lif create` and `lif update` fail with a policy violation. `lif check` reports it as a warning. `lif check --strict` counts it as a problem (exit 1).

`lif complete --summary "..."` appends a "Summary of Changes" section. `lif complete --commit <ref>` writes the SHA to the configured field (default `commit`).

</details>

## Limits

- The rank rule applies when you write through Lif (CLI, GraphQL, TUI, Web UI). It does not apply to manual edits of the file.
- `lif check` does not check the rank of parent and child, status values, priority values or tags. It reports unknown types, broken links, self-references, cycles, missing required fields and orphaned attachment directories.
- Changing only `type` (`lif update -t`, GraphQL `updateItem(type)`) bypasses the parent rank check. `lif check` does not report it afterwards. TODO: finding (no tracking item yet).
- There is no fixed depth limit. Built-in types and `classic` use up to four ranks. Custom types with rank 5 or higher are accepted.
- Types with the same rank cannot nest. Ranks may have gaps.
- `lif next` does not exclude containers. The hint on type `epic` ("do not work on directly") is not enforced.
- The TUI offers only the built-in status, type and priority lists [179g]. See [Known limitations](../../known-limitations/).

## Reference links

- [Configuration](../../reference/configuration/): types, ranks, status, priorities, presets, `require_fields_on`.
- [`lif init`](../../reference/commands/lif-init/): option `--profile`.
- [`lif roadmap`](../../reference/commands/lif-roadmap/), [`lif milestones`](../../reference/commands/lif-milestones/), [`lif progress`](../../reference/commands/lif-progress/), [`lif order`](../../reference/commands/lif-order/): planning views and order.
- [`lif complete`](../../reference/commands/lif-complete/) and [`lif check`](../../reference/commands/lif-check/): completion and checking.
