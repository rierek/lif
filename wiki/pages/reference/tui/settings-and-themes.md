---
title: "Settings and themes"
description: "TUI settings file, editor, accent colour and themes."
---

## Purpose
Describes the TUI settings (settings file, editor, accent color, layout) and the themes that TUI and CLI share.

## Audience
Users who customize the TUI (repositories, editor, colors), and readers who want to know which settings live in `.lif.yml` and which in the TUI file.

## Required sections

### Settings file
- Paths: `~/.config/lif/tui/config.yaml` (settings) and `state.json` in the same directory (state).
- Table of keys: `repos` (list of repository paths), `editor`, `theme.accent` (hex `#rrggbb`, empty = built-in mauve), `layout.tree_width` (default 36, minimum 24, maximum 60), `repo_store_paths` (mapping of repository to store path).
- Scope boundary: this file belongs to the TUI; project settings live in `.lif.yml` (`reference/configuration`).
- There is a settings form in the TUI. (Access and key: TODO: verify)
- Source: FACTS.md §8.2; settings form — `TODO: verify` (Phase 5: check `internal/tui/box_form_settings.go` and the command palette)

### Themes
- `display.theme` in `.lif.yml`: `latte`, `frappe`, `macchiato`, `mocha`; default `mocha`.
- The theme also applies to the CLI colors; an unknown name is reported by `lif check`.
- Shows the four themes side by side (screenshots). (TODO: verify)
- Scope boundary: `display.theme` (global, all interfaces) vs `theme.accent` (TUI only).
- Source: FACTS.md §8.4, §4.3; screenshots — `TODO: verify` (Phase 5: record)

### Display width
- `display.max_width`: 0 = unset = 110, `-1` removes the limit; the flag `--max-width` overrides it per command.
- Clarifies whether `display.max_width` affects the TUI. (TODO: verify)
- Source: FACTS.md §4.3; effect on the TUI — `TODO: verify` (Phase 5: check the code for where `max_width` is read)

### Choosing the editor
- Key `editor` in the settings file; command `e`/`ctrl+e` opens the body in the editor.
- Explains whether `editor` takes precedence over `$EDITOR` and what happens if neither is set. (TODO: verify)
- Shows an example with a terminal editor and a GUI editor. (TODO: verify)
- Source: FACTS.md §8.2, §8.3; order and examples — `TODO: verify` (Phase 5: check `internal/tui/editor.go`)

### Example settings file
- Shows a complete `config.yaml` with `repos`, `editor`, `theme.accent`, `layout.tree_width`.
- The example is built in Phase 5 and checked with `lif tui`, not invented.
- Source: `TODO: verify` — FACTS names keys but gives no example; in Phase 5 create a file and start the TUI

### Limits
- Settings apply per user, not per repository.
- Invalid values (e.g. width outside 24–60, invalid hex color): behavior not documented. (TODO: verify)
- The themes are limited to the four values; custom themes do not exist. (TODO: verify)
- Source: FACTS.md §8.2, §8.4; error behavior and custom themes — `TODO: verify` (Phase 5: check `internal/tui/config/settings.go`)
