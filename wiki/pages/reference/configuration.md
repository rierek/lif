---
title: "Configuration"
description: "The .lif.yml file: discovery, keys, defaults, types, statuses, priorities and presets."
---

## Purpose
Canonical page for the `.lif.yml` configuration file, for types, ranks and hierarchy rules, and for the shipped presets. It states exactly what Lif enforces on write and what `lif check` does not verify.

## Audience
Users who set up or adapt a project (custom types, statuses, priorities, store path), and agents that need to know which rules reliably hold. Overview and Features link here instead of repeating the rules.

## Required sections

### Config file and discovery
- File name `.lif.yml`; a legacy `.beans.yml` is read in place and never renamed.
- Only one of the two files may exist per directory; otherwise the error `both .lif.yml and .beans.yml exist in <dir>; keep one of them`.
- Discovery: `--config <path>` loads exactly that file; otherwise the search walks up directory by directory from the current directory to the filesystem root.
- No file found: built-in defaults, anchored in the current directory.
- Format: YAML; item settings live under the key `lif:` (legacy `beans:`; both in one file is an error); the other sections sit at top level.
- `lif init` writes a file with commented keys (`project.name`, `lif.path`, `lif.prefix`, `lif.id_length`, `lif.default_status`, `lif.default_type`, `worktree.base_ref`/`setup`/`run`/`integrate`, `agent.enabled`/`default_mode`, `server.port`).
- Source: FACTS.md §4.1, §1.1 (`lif init`)

### Store resolution
- Precedence: (1) `--lif-path` (2) config file (its `lif.path` or the default store directory next to it) (3) `LIF_PATH` (4) default store directory in the current directory.
- A discovered config file, and only a discovered one, demotes `LIF_PATH`; reason in the code: variables exported via direnv must not redirect unrelated repositories.
- `LIF_PATH` and legacy `BEANS_PATH` both set: error; if only the legacy one is set, it is used.
- A relative `lif.path` is resolved against the config directory; an absolute one is used unchanged.
- `lif.anchor: repo-root`: all Git worktrees of a repository share one store; without `anchor` a worktree uses its own local store; any other value is a config load error.
- Error messages for a missing directory: `no .lif directory found at <path> (run 'lif init' to create one)`; with an explicit path `store path does not exist or is not a directory: <path>`. `lif path` prints the result.
- Task-oriented view of the same rules: Store management (`overview/store-management`), including worktrees, a shared store and the ID prefix; this section stays the canonical precedence table.
- Source: FACTS.md §4.2, §1.1 (`lif path`); DESIGN.md D56

### Keys and defaults
- One table per group: key, type, default, effect; `lif check` reports invalid values for `agent.default_effort` and `display.theme`. `project`: `name` (default empty; `lif init` writes the directory name).
- `lif`: `path` (empty = `.lif`), `anchor` (empty; only `repo-root`), `prefix` (empty; `lif init` writes `<directory name>-`), `id_length` (4), `default_status` (`todo`), `default_type` (`task`; without `task` in the type table, the first type), `require_if_match` (false), `require_fields_on` (none), `commit_field` (`commit`).
- `worktree`: `base_ref` (`main`), `path` (`~/.lif/worktrees/<project>/`), `setup`, `run`, `integrate` (`local` or `pr`; default `local`), `fetch_timeout` (10 seconds, 0 disables the fetch).
- `agent`: `enabled` (true), `default_mode` (`act` or `plan`; default `act`), `default_effort` (`low`, `medium`, `high`, `max`; default empty = no override).
- `server`: `port` (8080), `cors_origins` (default `http://localhost:*`, `http://127.0.0.1:*`; `*` allows all).
- `display`: `theme` (`latte`, `frappe`, `macchiato`, `mocha`; default `mocha`), `max_width` (0 = 110; -1 removes the limit; `--max-width` overrides per command).
- Source: FACTS.md §4.3

### Statuses, types and priorities
- Built-in tables: status (`in-progress`, `todo`, `draft`, `completed`, `scrapped`), types (`milestone`, `epic`, `feature`, `bug`, `task`), priorities (`critical`, `high`, `normal`, `low`, `deferred`); details on `reference/item-file-format` (Allowed values and defaults).
- Custom entries under `statuses:`, `types:`, `priorities:` override built-in entries by name or are appended.
- Fields: `statuses`: `name`, `color`, `archive`, `short`, `description`; `types`: `name`, `color`, `rank`, `emphasis`, `roadmap`, `short`, `description`; `priorities`: `name`, `color`, `symbol`, `description`. Omitting `archive` keeps the default of the built-in entry; `lif archive` moves items with an archive status to `archive/`.
- `types_exclusive: true`: `types` is the complete table, built-in types are not added; `lif init --profile` sets it.
- Caution, blocking logic: "resolved" is fixed to `completed` or `scrapped`; the flag `archive: true` does not count there, so a custom archive status keeps blocking. Ready logic (`ready`), in contrast, excludes every status with `archive: true`.
- Accepted color values (TODO: verify)
- Source: FACTS.md §2.3, §4.3, §5.1 ("Resolved" status), §5.2; `TODO: verify` for the color format: see `pkg/config/config.go` and the color check in `internal/commands/check.go`

### Types, ranks and hierarchy rules
- Rank scale: every type has an integer `rank`; a parent is valid if the child's rank is strictly greater than the parent's; types of equal rank do not nest ("level" is only a prose synonym for rank). Ranks 1 to 3 are container ranks, rank 4 is the leaf rank; a type without `rank` gets 4, an unknown type counts as 4; gaps are allowed.
- Depth: with the built-in types and the preset `classic` there are four levels (`milestone` > `epic` > `feature` > `bug`/`task`); custom types can go deeper, there is no fixed upper limit (ranks 5 to 7 were accepted). The "four levels" are a property of the built-in scale, not an enforced maximum.
- Where enforced: `Core.ValidateParent` checks on create with `--parent`, on `lif update --parent` and on the GraphQL mutations `createItem`, `updateItem`, `setParent`; all CLI write commands, the GraphQL mutations and the TUI parent picker go through the same resolver. Errors: `parent item not found: <id>`, `<type> items cannot have a parent`, `<type> items can only have <a, b, or c> as parent, not <type>`. Additionally a cycle check for `parent` (`would create cycle: [...]`).
- Enforcement gap: changing only the `type` (`lif update -t`, `updateItem(type)`) does not re-trigger the rank check; an item can stay under a parent with an unsuitable rank and `lif check` stays green. Tracked as a bug (D43).
- What is only validated: `lif check` verifies configuration (default type, colors, theme, effort, prefix consistency), the type of every item against the type table, broken links, self-references, cycles in `parent`, `blocking` and `blocked_by`, orphaned `attachments/<id>` directories, and required fields and commit SHAs as warnings.
- What `lif check` does NOT verify (canonical statement for D40): rank rule of parent and child, status and priority values of items, tag format, duplicate IDs, mismatch between file name and ID, front matter syntax beyond loading; manual edits (`parent:`, `type:`, `status:`, `blocking:`) are accepted on load. Extending `lif check` is planned; this page follows once it ships.
- Source: FACTS.md §3, §3.1, §3.2, §5.3, §6.9, §9(a), §9(c), §9(d), §9(e); DESIGN.md D39, D40, D43, D45

### Presets
- Explains: a preset is a type table that `lif init --profile <name>` writes out once into `.lif.yml`; the file no longer refers to the preset name afterwards. `lif init --profile` sets `types_exclusive: true` and the default type `task`.
- Table `classic`: `milestone` (1), `epic` (2), `feature` (3), `bug` (4), `task` (4); four levels. Table `todo`: only `task` (4); one level.
- Table `simple`: `milestone` (1), `bucket` (1, `roadmap: false`), `epic` (2), `feature` (2), `bug` (4), `task` (4); three levels.
- Table `complex`: `release` (1), `bucket` (1, `roadmap: false`), `feature` (2), `improvement` (2), `chore` (2), `story` (4), `bug` (4), `task` (4); three levels.
- Explains `roadmap: false` on a type: hides the type from `lif roadmap` and `lif milestones`; on a container rank the whole subtree disappears. Mermaid diagram per preset (hierarchy; D38); an unknown profile gives `unknown profile "<x>" (must be classic, complex, simple, todo)`.
- Open: whether a preset also changes statuses or priorities (TODO: verify)
- Source: FACTS.md §9(b), §3, §1.1 (`lif init`); DESIGN.md D38, D39; `TODO: verify` for statuses and priorities of the presets: see `pkg/config/profiles.go`

### Status-dependent required fields
- `lif.require_fields_on`: map of status to list of extra keys; an unknown status, an empty field name or a reserved field name is a config load error.
- Writing an item into such a status without a non-empty value for every key fails on `create` and `update` with a policy violation (error code `POLICY_VIOLATION`).
- `lif check` reports it as a warning (section "Policy"); with `--strict` the warning counts as a finding (exit code 1).
- "Missing" means: key absent, null, or whitespace only.
- `lif.commit_field` (default `commit`): extra key holding a Git SHA; `lif complete --commit <ref>` writes it, `lif check` verifies that the repository knows it (otherwise a warning; without a Git repository the check is skipped).
- Source: FACTS.md §2.3, §4.3, §6.5, §1.0

### Annotated example and recipes
- Shows a commented `.lif.yml` as `lif init` generates it (TODO: verify)
- Recipe: rename a type or add a custom type with a rank (TODO: verify)
- Recipe: add a custom status, including `archive` (TODO: verify)
- Recipe: set custom colors (TODO: verify)
- Source: `TODO: verify` — build the examples in phase 5 with `lif init` and verify them with `lif check`; FACTS contains no example configuration
