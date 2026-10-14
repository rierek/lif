---
title: "Surface independence"
description: "Use the CLI, TUI, web UI or any editor on the same files."
---

## Purpose
Shows that the CLI, TUI, Web UI and direct editing of the Markdown files all use the same store. Clarifies which paths enforce rules and what `lif check` covers afterwards.

## Audience
People who want to choose and switch tools by preference, and teams with mixed usage (terminal, browser, editor, agent). They want to understand whether the data stays consistent on every path.

## Required sections

### What
- Four paths to one Store: CLI (`lif`), TUI (`lif tui`), Web UI (embedded in the separate program `lif-serve`) and direct editing of the Markdown files.
- All use the same store; the store path is determined by a fixed precedence (`--lif-path`, configuration, `LIF_PATH`, current directory).
- The web server watches the store directory and pushes changes to subscribers; the TUI is a companion to the CLI.
- The server has further capabilities (worktrees, agent sessions, terminal) that are only available through `lif-serve`.
- Source: FACTS.md §4.2, §8.1, §8.2

### Why
- No lock-in: the same information is reachable through different tools.
- Everyone works in their own tool; agents use the CLI or GraphQL, people use an editor, the TUI or a browser.
- Source: CONCEPT.md §4.4

### How it looks
- Four-way comparison of the same item: file in the editor, `lif show`, TUI detail view, Web UI view.
- Media plan: VHS GIFs for CLI and TUI, screenshots (webp) for the Web UI.
- Shows the same change (for example status) through two paths and the effect on the file.
- TUI: key `e` opens the body in `$EDITOR`.
- Contents of the Web UI view (columns, board): FACTS says nothing about this (TODO: verify)
- Source: DESIGN.md D38; FACTS.md §8.2, §8.3; (TODO: verify) for the Web UI view: take a screenshot in phase 5, check `frontend/` in the checkout

### Consistency and validation
- Writing paths (CLI commands, GraphQL mutations, TUI, Web UI) go through the same item logic and enforce the same configured rules (rank rule for parents, valid values for type, status, priority).
- Direct editing stays unrestricted; Lif loads the change without checking it and watches files only to reload.
- Then run `lif check`. It checks: configuration, unknown types, broken links, self-references, cycles, missing required fields (warning), unknown commit SHAs (warning), orphaned attachment directories.
- It does not check: rank of parent and child, status values, priority values, tags, duplicate IDs, file name against ID, syntax errors in the front matter (those already abort loading).
- Concurrent changes: `--if-match` and GraphQL `ifMatch` protect via ETag; the configuration `require_if_match` makes it mandatory.
- Extending the check is planned as an improvement; the page follows once it ships (D40).
- Source: FACTS.md §2.1, §3.1, §3.2, §6.8, §6.9, §8.2; DESIGN.md D40

### Editors such as Obsidian
- Any Markdown-capable editor can open item files; the format is front matter plus body.
- Obsidian is an experiment and lives in the blog (`blog/obsidian-experiment`), no longer as a use case.
- Bases compatibility is not proven: Obsidian does not appear in the code, the docs or the README.
- Note: the first front matter line is a YAML comment `# <id>`; the store lives in a dot directory (`.lif/`) (effect on Obsidian: TODO: verify)
- Source: FACTS.md §2.1, §2.4, §9(g); DESIGN.md D42; (TODO: verify) for Obsidian behavior: check in the experiment (blog)

### Limits
- Manual edits are not checked while editing but at most afterwards with `lif check`; its gaps are listed above.
- The Web UI needs the separate program `lif-serve`; `lif serve` in the main program is a hidden stub.
- A store applies to one path only: Without `lif.anchor: repo-root`, each Git worktree uses its own local store.
- Concurrent editing without `--if-match` has no protection; an overwritten state is not reported (TODO: verify)
- GraphQL `createItem` without `status` leaves the status empty (known bug); `lif create` sets `todo`.
- Source: FACTS.md §4.2, §7.5, §8.1; DESIGN.md D43; (TODO: verify) for write conflicts: check the behavior of `Core.Update` without ETag

### Reference links
- TUI: Start and repositories (`reference/tui/start-and-repositories`), Views (`reference/tui/views`), Editing items (`reference/tui/editing-items`).
- Web server (`reference/web-server`): `lif-serve`, port, CORS.
- `lif check` (`reference/commands/lif-check`): scope of checks.
- `lif path` (`reference/commands/lif-path`) and Configuration (`reference/configuration`): Store resolution, `require_if_match`.
- Source: DESIGN.md D28, D29, D44
