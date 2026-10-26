---
title: "Configuration"
description: "The .lif.yml file: discovery, keys, defaults, types, statuses, priorities and presets."
---

Canonical page for the `.lif.yml` file, for types, ranks and hierarchy rules, and for the shipped presets. It states what Lif enforces on write and what `lif check` verifies.

## Config file and discovery

- File name `.lif.yml`. A legacy `.beans.yml` is read in place and never renamed.
- Only one of the two files may exist per directory. Otherwise: `both .lif.yml and .beans.yml exist in <dir>; keep one of them`.
- `--config <path>` loads exactly that file. Otherwise Lif searches from the current directory upward to the filesystem root.
- No file found: built-in defaults, anchored in the current directory.
- Format: YAML. Item settings live under `lif:` (legacy `beans:`; both in one file is an error). The other sections sit at top level.

## Store resolution

Where Lif looks for the store directory, highest priority first:

| # | Source |
|---|---|
| 1 | `--lif-path` |
| 2 | The config file: its `lif.path`, or the default store directory next to it |
| 3 | `LIF_PATH` |
| 4 | The default store directory in the current directory |

- A discovered config file, and only a discovered one, demotes `LIF_PATH`. Reason in the code: variables exported with direnv must not redirect unrelated repositories.
- `LIF_PATH` and legacy `BEANS_PATH` both set is an error. If only the legacy one is set, it is used.
- A relative `lif.path` is resolved against the config directory. An absolute one is used as is.
- `lif.anchor: repo-root`: all Git worktrees of a repository share one store. Without `anchor` a worktree uses its own store. Any other value is a config load error: `unknown lif.anchor "<value>" (expected "repo-root" or an empty value)`.
- Missing directory: `no .lif directory found at <path> (run 'lif init' to create one)`. With an explicit path: `store path does not exist or is not a directory: <path>`. `lif path` prints the result.

The task-oriented view, with worktrees, a shared store and the ID prefix, is [Store management](../../overview/store-management/). This section stays the canonical precedence table.

## Keys and defaults

`lif init` writes a file like this (excerpt):

```yaml
project:
    # Human-readable project name (displayed in the UI)
    name: myproject
lif:
    # Directory where item files are stored
    path: .lif
    # Prefix for item IDs (e.g., "myproject-abc1")
    prefix: myproject-
    # Length of the random ID suffix
    id_length: 4
    default_status: todo
    default_type: task
worktree:
    base_ref: main
agent:
    enabled: true
    default_mode: act
server:
    port: 8080
```

| Group | Key | Default |
|---|---|---|
| `project` | `name` | empty (`lif init` writes the directory name) |
| `lif` | `path` | empty = `.lif` |
| | `anchor` | empty; only `repo-root` |
| | `prefix` | empty (`lif init` writes `<directory>-`) |
| | `id_length` | 4 |
| | `default_status` | `todo` |
| | `default_type` | `task` (without `task` in the type table: the first type) |
| | `default_priority` | unset; read only by `lif create -i` |
| | `default_tags` | unset (list of strings); read only by `lif create -i` |
| | `require_if_match` | false |
| | `require_fields_on` | none |
| | `commit_field` | `commit` |
| `worktree` | `base_ref` | `main` |
| | `path` | `~/.lif/worktrees/<project>/`; a leading `~` is expanded. An existing legacy `~/.beans/worktrees/<project>/` is used instead. |
| | `setup`, `run` | empty; `setup` is a shell command run in each new worktree (for example `pnpm install`). Both run with `sh -c` and are outside the [hook trust](hooks/#trust): read them in a cloned repository before you create a worktree or start a run in `lif-serve`. |
| | `integrate` | `local` (or `pr`). `local` squash-merges in the local repository and hides the PR buttons; `pr` pushes the branch and opens a PR. |
| | `fetch_timeout` | 10 seconds; 0 disables the fetch |
| `agent` | `enabled` | true |
| | `default_mode` | `act` (or `plan`). `act` runs autonomously, `plan` is read-only. `yolo` is accepted as an alias for `act`; any other value falls back to `act`. |
| | `default_effort` | empty = the tool's own default (`low`, `medium`, `high`, `max`) |
| `server` | `port` | 8080 |
| | `cors_origins` | none: only the server's own origin. Entries are added to it. `*` adds the CORS header for every origin and allows nothing else (pages can read, not change). A port wildcard such as `http://localhost:*` lets a page on any local port change items (project hooks) and open the terminal |
| `display` | `theme` | `mocha` (`latte`, `frappe`, `macchiato`, `mocha`). Colors the CLI output. In the web UI it picks the flavour of the dark scheme (`latte` leaves dark on `mocha`; the light scheme is always `latte`); see [Theme](../web-server/#theme). The TUI uses a fixed dark palette. |
| | `max_width` | 0 = 110; -1 removes the limit; `--max-width` overrides per command. The width never exceeds the terminal width and never falls below 80. |
| `tasks` | `<name>` | none; see [Tasks](tasks/) |
| `hooks` | `<name>` | none; see [Hooks](hooks/) |

`lif.default_priority` and `lif.default_tags` are read only by the guided `lif create -i` form, which pre-fills them. Plain `lif create`, the GraphQL `createItem` mutation and the TUI create form ignore them. `lif check` fails if `default_priority` names an unknown priority (built-in or configured), or if a `default_tags` entry is empty or not a valid tag. An empty list behaves like an unset key.

`lif check` does not validate `lif.default_status`: it prints "Default status exists" without a real test, because the status table is built in. `lif check` reports invalid values for `agent.default_effort` and `display.theme`. The `agent`, `worktree` and `server` keys matter only for [`lif-serve`](../web-server/). Every option that opens the server has no key on purpose: the bind address, the remote terminal and the remote agent, the token and TLS options are flags (`--host`, `--allow-remote-terminal`, `--allow-remote-agent`, `--require-token`, `--public-url`, `--token-file`, `--tls-cert`, `--tls-key`). See [Remote access](../remote-access/).

New worktrees get the branch `lif/<id>`. Worktrees on legacy `beans/` branches are still listed and used.

## Topics

- [Tasks and TUI actions](tasks/)
- [Hooks and gates](hooks/)
- [Statuses, types and hierarchy](types-and-statuses/)
