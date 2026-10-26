---
title: "Statuses, types and hierarchy"
description: "Built-in statuses and priorities, custom types and ranks, hierarchy rules, presets and status-dependent required fields."
---

## Statuses, types and priorities

- Built-in statuses: `in-progress`, `todo`, `draft`, `completed`, `scrapped`. Types: `milestone`, `epic`, `feature`, `bug`, `task`. Priorities: `critical`, `high`, `normal`, `low`, `deferred`. See [Item file format](../../item-file-format/#allowed-values-and-defaults).
- Entries under `statuses:`, `types:` and `priorities:` override a built-in entry by name or are appended.
- Fields. `statuses`: `name`, `color`, `archive`, `short`, `description`. `types`: `name`, `color`, `rank`, `emphasis`, `roadmap`, `short`, `description`. `priorities`: `name`, `color`, `symbol`, `description`. Omitting `archive` keeps the default of a built-in entry. `lif archive` moves items with an archive status to `archive/`.
- `types_exclusive: true`: `types` is the complete table; built-in types are not added. `lif init --profile` sets it.
- Colors: a Catppuccin tone name or a hex value (`#rgb` or `#rrggbb`, for example `#ff8800`). Tones: `rosewater`, `flamingo`, `pink`, `mauve`, `red`, `maroon`, `peach`, `yellow`, `green`, `teal`, `sky`, `sapphire`, `blue`, `lavender`, `text`, `subtext1`, `subtext0`, `overlay2`, `overlay1`, `overlay0`, `surface2`, `surface1`, `surface0`, `base`. The legacy names `gray`, `grey`, `purple` and `cyan` still work. An empty color is valid. `lif check` reports an invalid one: `✗ invalid color 'notacolor' for type 'spike'`. The web UI applies these colors to badges and borders too, see [Web UI](../../web-server/#web-ui).
- Blocking logic: "resolved" means `completed` or `scrapped`, fixed. The flag `archive: true` does not count there, so an item in a custom archive status keeps blocking. The ready logic, in contrast, excludes every status with `archive: true`.

### Recipes

Add a status and a type, set colors. Verified with `lif check` and `lif create`:

```yaml
statuses:
    - name: review
      color: "#ff8800"
      description: Waiting for review
    - name: shipped
      color: green
      archive: true
types:
    - name: spike
      rank: 3
      color: teal
      description: Time-boxed research
```

```sh
lif check
lif create "Try it" -t spike -s review
```

To rename a type, add the new type, then change the old items: `lif update <id> -t <new-type>`. Lif does not rename types in existing files.

## Types, ranks and hierarchy rules

- Every type has an integer `rank`. A parent is valid if the child's rank is **strictly greater** than the parent's. Types of equal rank do not nest. "Level" is only a prose synonym for rank.
- Ranks 1 to 3 are container ranks, rank 4 is the leaf rank. A type without `rank` gets 4, an unknown type counts as 4. Gaps are allowed. Container ranks also decide what is ready: `lif next` and `lif list --ready` never return an item whose type sits on rank 1 to 3.
- Depth: the built-in types and the preset `classic` have four levels (`milestone` > `epic` > `feature` > `bug`/`task`). Custom types can go deeper; there is no upper limit (ranks 5 to 7 were accepted). "Four levels" describes the built-in scale, not a maximum.
- Enforced on write: on `lif create --parent`, `lif update --parent`, `lif update --type` and the GraphQL mutations `createItem`, `updateItem`, `setParent`. The CLI, GraphQL and the TUI go through the same checks. Errors:

```text
parent item not found: <id>
<type> items cannot have a parent
<type> items can only have <a, b, or c> as parent, not <type>
would create cycle: [...]
```

- A type change checks both directions (since 0.14): the parent must allow the new type, and every child must be allowed under it. `lif update` checks every target before it writes the first one. Example: `Error: <id>: task items cannot be a child of task items (child <id>)`.
- Manual edits (`parent:`, `type:`, `status:`, `blocking:`) are accepted on load. `lif check` finds them afterwards.

### What `lif check` verifies

| Verified | Not verified |
|---|---|
| Configuration: default type, default priority and tags, colors, theme, effort, prefix consistency | Duplicate IDs |
| Type of every item against the type table | Front matter syntax beyond loading |
| Status and priority values, tag format | |
| File name against the ID the file records in its `# <id>` line, with the links that dangle (a hand rename, since 0.15) | |
| Parent of the wrong type rank: `✗ item <id> (epic) cannot sit under parent <id> (task): parent must rank higher` | |
| Broken links, self-references, cycles in `parent`, `blocking`, `blocked_by` | |
| Orphaned `attachments/<id>` directories | |
| Required fields and commit SHAs (warnings) | |

## Presets

A preset is a type table that `lif init --profile <name>` writes once into `.lif.yml`. The file does not refer to the preset name afterwards. `lif init --profile` sets `types_exclusive: true` and the default type `task`. A preset changes only the type table; statuses and priorities stay the built-in ones. An unknown name gives `unknown profile "<x>" (must be classic, complex, simple, todo)`.

`roadmap: false` on a type hides it from `lif roadmap` and `lif milestones`. On a container rank the whole subtree disappears.

| Preset | Types (rank) | Levels |
|---|---|---|
| `classic` | `milestone` (1), `epic` (2), `feature` (3), `bug` (4), `task` (4) | 4 |
| `todo` | `task` (4) | 1 |
| `simple` | `milestone` (1), `bucket` (1, `roadmap: false`), `epic` (2), `feature` (2), `bug` (4), `task` (4) | 3 |
| `complex` | `release` (1), `bucket` (1, `roadmap: false`), `feature` (2), `improvement` (2), `chore` (2), `story` (4), `bug` (4), `task` (4) | 3 |

```mermaid
flowchart TD
  subgraph classic
    m1[milestone 1] --> e1[epic 2] --> f1[feature 3] --> t1[bug / task 4]
  end
  subgraph complex
    r2[release 1] --> f2[feature / improvement / chore 2] --> s2[story / bug / task 4]
  end
```

## Status-dependent required fields

- `lif.require_fields_on` maps a status to a list of extra keys. An unknown status, an empty field name or a reserved field name is a config load error. Reserved: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`.
- Writing an item into such a status without a non-empty value for every key fails on `create` and `update` with error code `POLICY_VIOLATION`.
- `lif complete` into such a status fails and names the missing field. Satisfy it in the same call with `--commit <ref>` or `--set key=value`. The check runs before anything is saved.
- `lif check` reports it as a warning in the section "Policy". With `--strict` the warning counts as a finding (exit code 1).
- "Missing" means: key absent, null, or only whitespace.
- `lif.commit_field` (default `commit`) is the extra key that holds a Git SHA. `lif complete --commit <ref>` writes it. `lif check` verifies that the repository knows it (otherwise a warning; without a Git repository the check is skipped).

```yaml
lif:
  require_fields_on:
    completed: [commit]
```
