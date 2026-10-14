# Configuration Reference

This page documents every key `.lif.yml` accepts, in the shape the `lif` and `lif-serve` binaries (including `lif tui`) actually read it. It also covers the type/status/priority override system, custom types, display options, and how a Lif store's path is resolved.

## Minimal configuration

A repository does not need a `.lif.yml` at all: without one, every key falls back to its built-in default. `lif init` (with no `--profile`) writes one that spells out the project name, the `lif` section, and the `worktree`, `agent` and `server` defaults; it writes no `types` table, so the built-in types apply. For a project in a directory named `myproject` it writes, with every key preceded by a comment line in the real file:

```yaml
# Lif configuration
# See: https://github.com/xRiErOS/lif
project:
    name: myproject
lif:
    path: .lif
    prefix: myproject-
    id_length: 4
    default_status: todo
    default_type: task
worktree:
    base_ref: main
    setup: ""
    run: ""
    integrate: local
agent:
    enabled: true
    default_mode: act
server:
    port: 8080
```

Every top-level section (`project`, `lif`, `worktree`, `agent`, `server`, `display`) is optional; an absent section, or an absent field within it, falls back to its documented default.

## File location and store path resolution

`lif init` writes `.lif.yml` at the project root, next to the `.lif` data directory it creates. Every command searches upward from the current directory for the nearest `.lif.yml` unless `--config <path>` names one explicitly.

The data directory itself is resolved with the following precedence, from highest to lowest:

1. `--lif-path <path>` on the command line.
2. A `.lif.yml` that was actually found: its `lif.path` value, or the default `.lif`, resolved according to `lif.anchor`.
3. `LIF_PATH`, but only when no `.lif.yml` was found.
4. The default `.lif` directory for the current working directory when neither a config nor environment override exists.

A found `.lif.yml` outranks `LIF_PATH` on purpose: the config file is a repository's own declaration of where its store lives, while `LIF_PATH` is commonly exported by a tool like direnv and inherited into unrelated repositories, where honoring it would silently redirect commands to a foreign store. Run `lif path` in any project to print the path a given invocation actually resolved to.

### Anchoring in git worktrees

`lif.anchor` controls what `lif.path` is relative to:

```yaml
lif:
    path: .lif
    anchor: repo-root
```

- Left empty (the default), `lif.path` is relative to the directory containing `.lif.yml`. In a secondary git worktree this means the worktree gets its own store, separate from the main worktree's.
- Set to `repo-root`, `lif.path` resolves against the main worktree's root instead, so every worktree of one repository shares a single store. From the main worktree, or outside a git repository entirely, `repo-root` has no other worktree to redirect to and behaves the same as leaving `anchor` unset.

Any other value fails config loading immediately with an "unknown lif.anchor" error, rather than silently falling back to the default and resolving a different store than the file asked for.

## `project`

| Key | Type | Default |
| --- | --- | --- |
| `name` | string | the project directory's name, set once by `lif init` |

`project.name` is the human-readable project name shown in the web UI and TUI. It has no effect on item IDs or file paths.

## `lif`

| Key | Type | Default |
| --- | --- | --- |
| `path` | string | `.lif` |
| `anchor` | string | unset (config-file-relative) |
| `prefix` | string | `""` |
| `id_length` | int | `4` |
| `default_status` | string | `todo` |
| `default_type` | string | `task` when no config file exists; otherwise the first merged type when omitted |
| `default_priority` | string | unset |
| `default_tags` | list of strings | unset |
| `require_if_match` | bool | `false` |
| `require_fields_on` | map of status name to list of field names | unset |
| `commit_field` | string | `commit` |

`lif.path` is the directory item files are stored in, resolved as described above.

`lif.prefix` is prepended to every generated item ID, for example `myproject-abc1`. `lif init` sets it to the project directory's name followed by a hyphen; it can be changed later with `lif rename --prefix`, which also rewrites every existing ID and cascades the change through cross-references (see the command reference in [wiki/commands/organization-and-relations.md](commands/organization-and-relations.md)).

`lif.id_length` is the number of random characters in the generated ID suffix.

`lif.default_status` and `lif.default_type` are applied to `lif create` when `--status`/`--type` are not passed. `lif check` fails if `default_status` names a status the merged status list does not contain, or if `default_type` names an unknown type.

`lif.default_priority` and `lif.default_tags` are read only by the guided `lif create -i` form, which pre-fills them. Plain `lif create`, the GraphQL `createItem` mutation and the TUI create form ignore them. `lif check` fails if `default_priority` names an unknown priority (built-in or configured), or if a `default_tags` entry is empty or not a valid tag. An empty list behaves like an unset key.

`lif.require_if_match` requires an `If-Match` header (an ETag) on updates made through the API layer, for optimistic concurrency control.

`lif.require_fields_on` declares a completion policy: it maps a target status name to a list of front matter keys that must carry a non-empty value whenever an item is written into that status.

```yaml
lif:
    require_fields_on:
        completed:
            - commit
```

With the example above, `lif complete <id>` fails with an error naming the missing field and the flag that supplies it (for example `--commit HEAD`), unless the field is already set or supplied in the same call via `--commit` or `--set key=value`. The write is rejected before anything is saved. Config loading itself rejects a `require_fields_on` entry that names an unknown status, an empty field name, or a field items already manages natively (`title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`) — those are written through their own flags, not through `require_fields_on`.

`lif.commit_field` renames the front matter key `require_fields_on` and `lif complete --commit` use to record a git commit SHA. Only this key is git-verified (the value must resolve to a real commit) when written or checked; every other field named in `require_fields_on` only needs to be non-empty.

## `worktree`

| Key | Type | Default |
| --- | --- | --- |
| `base_ref` | string | `main` |
| `path` | string | `~/.lif/worktrees/<project-name>/` |
| `setup` | string | `""` |
| `run` | string | `""` |
| `integrate` | string (`local` or `pr`) | `local` |
| `fetch_timeout` | int (seconds) | `10` |

These settings govern the git worktree management surfaced by the web UI (`lif-serve`). `base_ref` is the git ref new worktree branches start from. `path` is where worktrees are created; it supports a leading `~` for the home directory. `setup` is a shell command run inside a fresh worktree (for example `pnpm install`). `run` is a shell command that, when set, adds a "Run" button to the workspace toolbar. `integrate` chooses the worktree integration strategy: `local` squash-merges locally and hides the PR-related buttons, `pr` pushes the branch and opens a pull request instead. `fetch_timeout` bounds the `git fetch` that refreshes `base_ref` before a new worktree is created; set it to `0` to skip that fetch entirely, for example in an airgapped environment.

Worktrees live under `worktree.path`, by default `~/.lif/worktrees/<project-name>/`, and every new worktree branch is named `lif/<id>`. `lif-serve` exports the workspace's port to the shell and run sessions of each worktree as `LIF_WORKSPACE_PORT`; a legacy port variable is exported beside it, see [Legacy names](#legacy-names).

## `agent`

| Key | Type | Default |
| --- | --- | --- |
| `enabled` | bool | `true` |
| `default_mode` | string (`act` or `plan`) | `act` |
| `default_effort` | string (`low`, `medium`, `high`, `max`) | unset |

`agent.enabled` toggles whether the web UI exposes agent chats, status panes, and worktree features at all. `agent.default_mode` sets the default permission mode for new agent sessions: `act` runs autonomously, `plan` is read-only. `yolo` is still accepted as a backwards-compatible alias for `act`. `agent.default_effort`, when set, is the thinking-effort level new agent sessions start with; left unset, a new session starts with no effort override and uses the invoking tool's own default. See [wiki/agent-integration.md](agent-integration.md) for the session lifecycle these settings apply to.

## `server`

| Key | Type | Default |
| --- | --- | --- |
| `port` | int | `8080` |
| `cors_origins` | list of strings | `["http://localhost:*", "http://127.0.0.1:*"]` |

Both keys apply to `lif-serve`. `cors_origins` accepts exact origins and port wildcards (`http://localhost:*`); `"*"` allows every origin and is not recommended outside local development. See [wiki/web-ui-and-api.md](web-ui-and-api.md) for the API this server exposes.

## `display`

| Key | Type | Default |
| --- | --- | --- |
| `theme` | string | `mocha` |
| `max_width` | int | `110` |

```yaml
display:
    theme: latte
    max_width: 100
```

`display.theme` selects one of the four bundled Catppuccin flavors: `latte`, `frappe`, `macchiato`, or `mocha`. It governs terminal rendering in the `lif` CLI and `lif tui`; an unrecognized name is ignored and the default applies (`lif check` flags it as invalid). `display.max_width` caps the rendered width in terminal cells; `0` (unset) yields the default of `110`, and `-1` disables the cap entirely so output uses the full terminal width.

Every `color` field described below (on statuses, types, and priorities) must name one of the Catppuccin tones the active theme defines: `rosewater`, `flamingo`, `pink`, `mauve`, `red`, `maroon`, `peach`, `yellow`, `green`, `teal`, `sky`, `sapphire`, `blue`, `lavender`, `text`, `subtext1`, `subtext0`, `overlay2`, `overlay1`, `overlay0`, `surface2`, `surface1`, `surface0`, or `base`. An empty `color` field is valid and renders with no explicit color.

## Statuses, types, and priorities

Lif ships built-in tables for statuses, types, and priorities. `.lif.yml` can override entries in these tables field by field, or add new entries, through the top-level `statuses`, `types`, and `priorities` lists. A list entry is matched to a built-in entry by `name`; an entry naming an unrecognized name is appended as a new one instead.

### Built-in statuses

| Name | Color | Archive | Short | Description |
| --- | --- | --- | --- | --- |
| `in-progress` | `peach` | no | `I` | Currently being worked on |
| `todo` | `green` | no | `T` | Ready to be worked on |
| `draft` | `overlay2` | no | `D` | Needs refinement before it can be worked on |
| `completed` | `overlay1` | yes | `C` | Finished successfully |
| `scrapped` | `surface2` | yes | `S` | Will not be done |

`short` is the single-character code narrow terminal views and legends render; when a status has none, the status name's first letter is upper-cased instead, and only an unknown status renders `?`. An entry in `statuses` overrides `color`, `description`, `archive`, and `short` on the named status, or defines a new status if the name is not one of the five above:

```yaml
statuses:
    - name: completed
      color: green
    - name: blocked
      color: red
      description: Waiting on an external dependency
```

`archive` is only touched by an explicit `archive: true` or `archive: false`; a color- or description-only override on `completed` cannot accidentally flip it back to non-archiving. Archived statuses are excluded from the default working views and are what `lif archive` moves into cold storage.

### Built-in types

| Name | Rank | Short | Emphasis | Description |
| --- | --- | --- | --- | --- |
| `milestone` | 1 | `M` | yes | A target release or checkpoint; group work that should ship together |
| `epic` | 2 | `E` | yes | A thematic container for related work; should have child items, not be worked on directly |
| `feature` | 3 | `F` | no | A user-facing capability or enhancement |
| `bug` | 4 | `B` | no | Something that is broken and needs fixing |
| `task` | 4 | `T` | no | A concrete piece of work to complete (e.g. a chore, or a sub-task for a feature) |

`rank` carries the parent/child hierarchy: an item can be the parent of a child item only when the child's rank is strictly greater than the parent's. Ranks 1 through 3 are container ranks; rank 4 is the leaf rank, and any type without an explicit `rank` (including a newly appended one) lands there. `short` is the single-character code the narrow list view renders; when empty it defaults to the type name's first letter, upper-cased. `emphasis` renders the type bold across the type word, ID, and title in list views — the mechanism that visually distinguishes a container type from a leaf type, since the Catppuccin palette is uniformly pastel and color alone would not separate them. `roadmap`, when explicitly set to `false`, hides a type from its own section in `lif roadmap` and from `lif milestones`; on a container rank this also hides its whole subtree. Leaving `roadmap` unset keeps the type visible.

```yaml
types:
    - name: bug
      color: maroon
      short: B
    - name: chore
      rank: 4
      short: C
      description: Internal work without customer benefit
```

`types_exclusive: true` switches off the merge with the built-in table entirely: `types` then becomes the complete type table on its own, with nothing from the built-in defaults layered underneath it. This is what `lif init --profile <name>` sets, since a profile is meant to give a project exactly its own types. A hand-written `.lif.yml` that never sets `types_exclusive` keeps the default entry-by-entry override behavior, where an override list naming only `bug` leaves the other four built-in types untouched.

```yaml
types_exclusive: true
types:
    - name: task
      short: T
      description: A concrete piece of work to complete
```

See [wiki/project-profiles.md](project-profiles.md) for the bundled profiles (`classic`, `todo`, `simple`, `complex`) `lif init --profile` can expand into a `types_exclusive` type table like this one.

### Built-in priorities

| Name | Color | Symbol | Description |
| --- | --- | --- | --- |
| `critical` | `red` | `‼` | Urgent, blocking work. When possible, address immediately |
| `high` | `yellow` | `!` | Important, should be done before normal work |
| `normal` | `""` | none | Standard priority |
| `low` | `overlay0` | `↓` | Less important, can be delayed |
| `deferred` | `overlay0` | `→` | Explicitly pushed back, avoid doing unless necessary |

Priorities are ordered from highest to lowest urgency, and `symbol` is the compact glyph narrow views and legends render for a priority (a priority without one renders no glyph). An item file may omit `priority`; the loader then materializes `normal` in memory without rewriting the file. `priorities` overrides follow the same name-matched, field-by-field rule as `statuses` and `types`, including `symbol`.

```yaml
priorities:
    - name: critical
      color: pink
```

## Validation

`lif check` validates a project's configuration alongside its item files: that the merged status and type tables are well-formed, that `default_status` and `default_type` name entries that actually exist, that every item carries a known type, that every configured `color` names a valid theme tone, that `lif.prefix` matches the IDs actually on disk, and that any `require_fields_on` policy is satisfiable. A misconfigured `.lif.yml` — an unknown `lif.anchor`, an invalid `require_fields_on` entry, or a config file that fails to parse — is rejected at load time, before any command runs, rather than surfacing as a downstream validation warning. See [wiki/commands/validation-and-maintenance.md](commands/validation-and-maintenance.md) for the full `lif check` output.

## Environment variables

| Variable | Meaning |
| --- | --- |
| `LIF_PATH` | Data directory, used only when no `.lif.yml` was found; `--lif-path` and a found `.lif.yml` outrank it. |
| `LIF_WORKSPACE_PORT` | Set by `lif-serve` in the shell and run sessions of a workspace that has an allocated port. |

## Legacy names

Projects created by an earlier release keep working without any change. These legacy names are still read, in place, and never renamed by Lif:

| Level | Current name | Legacy name still read |
| --- | --- | --- |
| Config file | `.lif.yml` | `.beans.yml` |
| Section key in the config file | `lif:` | `beans:` |
| Store directory | `.lif` | `.beans` |
| Store path variable | `LIF_PATH` | `BEANS_PATH` |
| Worktree root | `~/.lif/worktrees/<project-name>/` | `~/.beans/worktrees/<project-name>/` |
| Worktree branch prefix | `lif/` | `beans/` |
| Port variable in workspace sessions | `LIF_WORKSPACE_PORT` | `BEANS_WORKSPACE_PORT` (exported together with the current one) |

- **Both at one level is an error.** The first four levels refuse to guess: if a project holds both a legacy name and its current name at the same level, the command fails with an error that names both. The error appears wherever that level is consulted: `both .lif.yml and .beans.yml exist in <dir>; keep one of them`; `<path>: both sections "lif:" and "beans:" are present; keep one of them`; `both .lif and .beans exist in <dir>; keep one of them`, when the store directory is the default (the config file sets no `path:`); and `both LIF_PATH and BEANS_PATH are set; unset one of them`, only when the variables are consulted at all, that is when no config file was found and no `--lif-path` was given.
- **The worktree levels have no such error.** The legacy worktree root and legacy-branch worktrees work beside the current ones; when both worktree roots exist, the legacy root wins. Worktrees already on a `beans/` branch are still listed and used; new ones are created on `lif/<id>`.
- **`lif init` keeps legacy names.** In a project that already uses `.beans` or `.beans.yml`, it does not add the current names next to them.
- **`lif check` warns.** It reports every legacy name in use as a warning in a `Legacy names` section, with the step that ends it; see [Validation and maintenance](commands/validation-and-maintenance.md) for the output. The warnings never fail the check.

Renaming is optional. The [upgrade guide](compatibility-and-upgrading.md#optional-rename-of-legacy-data-names) lists each warning with its concrete command. Item files and ID prefixes do not change either way.

## Related documentation

- [wiki/project-profiles.md](project-profiles.md)
- [wiki/commands/project-setup.md](commands/project-setup.md)
- [wiki/commands/organization-and-relations.md](commands/organization-and-relations.md)
- [wiki/commands/validation-and-maintenance.md](commands/validation-and-maintenance.md)
- [wiki/agent-integration.md](agent-integration.md)
- [wiki/web-ui-and-api.md](web-ui-and-api.md)
- [wiki/commands/separate-binaries.md](commands/separate-binaries.md)
- [wiki/tui-companion.md](tui-companion.md)
- [wiki/fork-lineage.md](fork-lineage.md)
