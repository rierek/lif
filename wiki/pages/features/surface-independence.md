---
title: "Surface independence"
description: "CLI, TUI, Web UI and your editor all work on the same store."
---

Four paths lead to one store: the CLI, the TUI, the Web UI and direct editing of the Markdown files. Measured on Lif 0.14, 2026-10-14.

## What

- CLI: `lif`. TUI: `lif tui`. Web UI: embedded in the separate program `lif-serve`. Editor: open the Markdown files.
- All paths use the same store. Lif finds it by a fixed precedence: `--lif-path`, configuration, `LIF_PATH`, current directory. See [`lif path`](../../reference/commands/lif-path/).
- The web server watches the store directory and pushes changes to subscribers. The TUI is a companion to the CLI.
- The server has more capabilities (worktrees, agent sessions, terminal). They exist only in `lif-serve`.

## Why

- No lock-in. The same information is reachable through different tools.
- Everyone works in their own tool. Agents use the CLI or GraphQL. People use an editor, the TUI or a browser.

## How it looks

The same item through two paths. `lif show` in the terminal:

```text
$ lif show ft-957g
---
# ft-957g
title: Send reset mail
status: todo
type: task
priority: normal
tags:
    - backend
created_at: 2026-10-14T19:17:18Z
updated_at: 2026-10-14T19:17:18Z
parent: ft-igeu
---
```

The file in your editor holds the same front matter. If you change `status: todo` to `in-progress` there, the CLI, the TUI and the Web UI show the new status on the next load. The Web UI updates by itself through the file watcher.

In the TUI, the key `e` opens the body in `$EDITOR`. See [TUI views](../../reference/tui/views/) and [Editing items](../../reference/tui/editing-items/).

## Consistency and validation

- The writing paths (CLI commands, GraphQL mutations, TUI, Web UI) go through the same item logic. They enforce the same configured rules: the rank rule for parents and valid values for type, status and priority.
- Direct editing is unrestricted. Lif loads the change without checking it. It watches files only to reload.
- Run `lif check` afterwards. It checks configuration, unknown types, broken links, self-references, cycles, missing required fields (warning), unknown commit SHAs (warning) and orphaned attachment directories.
- It does not check the rank of parent and child, status values, priority values, tags, duplicate IDs or file name against ID. Syntax errors in front matter already abort loading.
- For concurrent changes, `--if-match` and GraphQL `ifMatch` protect with an ETag. The configuration `require_if_match` makes it mandatory. The check does not protect two simultaneous processes [9r10].

## Editors such as Obsidian

- Any Markdown-capable editor can open item files. The format is front matter plus body.
- Obsidian is an experiment. It lives in the blog: [Obsidian experiment](../../blog/obsidian-experiment/).
- Bases compatibility is not proven. Obsidian does not appear in the Lif code, docs or README.
- The first front matter line is a YAML comment `# <id>`. The store lives in a dot directory (`.lif/`). Check how your editor treats both before you rely on it.

## Limits

- Lif does not check manual edits while you edit. At most `lif check` does afterwards, with the gaps listed above.
- The Web UI needs the separate program `lif-serve`. `lif serve` in the main program is a hidden stub.
- `lif-serve` has no authentication or TLS and exposes a terminal [8h4v]. Do not expose its port.
- Without `lif.anchor: repo-root`, each Git worktree uses its own store.
- Concurrent editing without `--if-match` has no protection. Lif does not report an overwritten state.
- GraphQL `createItem` without `status` leaves the status empty. `lif create` sets `todo`.
- The TUI ignores `display.theme` and offers fixed value lists [179g].

See [Known limitations](../../known-limitations/) for [9r10], [8h4v] and [179g].

## Reference links

- TUI: [Start and repositories](../../reference/tui/start-and-repositories/), [Views](../../reference/tui/views/), [Editing items](../../reference/tui/editing-items/).
- [Web server](../../reference/web-server/): `lif-serve`, port, CORS.
- [`lif check`](../../reference/commands/lif-check/): scope of checks.
- [`lif path`](../../reference/commands/lif-path/) and [Configuration](../../reference/configuration/): store resolution, `require_if_match`.
