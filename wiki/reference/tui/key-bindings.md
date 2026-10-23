---
title: "Key bindings"
description: "All TUI key bindings, from the key map."
---

Measured on Lif 0.13, code unchanged in 0.14 (checked 2026-10-14). The in-app help (`?`) is generated from the same key map (`keyMap` in `internal/tui/keymap.go`). This page is written by hand; no generator or test ties it to the key map yet, so it can drift. Press `?` in the TUI for the keys of your version. The help panel is 54 rows high; on a shorter terminal it is cut and cannot be scrolled.

Notation: `ctrl+x` is Ctrl and `x`; several keys for one action are separated by `/`. Navigation uses the `i/j/k/l` diamond, not vim `h/j/k/l`.

## Navigation

| Key | Action |
|---|---|
| `↑` / `i` | Up |
| `↓` / `k` | Down |
| `←` / `j` | Back or out (collapse) |
| `→` / `l` | In (expand) |
| `tab` / `shift+tab` | Switch focus between the list and the detail pane (only in layout roadmap + filter + details; never changes the layout) |
| `↑` / `↓` in the detail pane | Previous or next box |
| `enter` | Open or confirm |
| `esc` | Back; leaves the detail pane, menus |
| `pgup` / `pgdown` | Page the detail body (not listed in `?`) |

## Global actions

| Key | Action |
|---|---|
| `K` | Command palette |
| `p` | Repository picker |
| `b` | Ready |
| `/` | Search |
| `f` | Facet filter menu |
| `X` | Clear all filters |
| `S` | Cycle ready sort |
| `G` | Nested or flat |
| `v` / `shift+v` | Next or previous layout: roadmap, roadmap + filter, roadmap + filter + details, details; both wrap |
| `ctrl+r` | Reload |
| `y` | Copy (yank) item context |
| `?` | Help |
| `q` | Quit dialog |
| `ctrl+c` | Quit at once, no confirmation |

## Item actions

All act on the selected item. Behavior: [Editing items](../editing-items/).

| Key | Action |
|---|---|
| `s` | Status menu |
| `o` | Type menu |
| `u` | Priority menu |
| `a` | Assign parent |
| `r` | Relations (blocking picker) |
| `t` | Tag picker |
| `c` | Create item |
| `d` | Delete (with confirmation, no cascade) |
| `e` / `ctrl+e` | Edit the whole item in the editor |

Title, settings and tag management have no key; use the palette (`K`).

## Pickers and facets

| Key | Action | Where |
|---|---|---|
| `space` / `x` | Toggle a value | Facet menu, relations picker |
| `space` | Toggle a tag | Tag picker |
| `ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g` | Chip: type, status, priority, tags | Parent picker and relations picker only |

## Tag picker and tag management

In the tag picker `n` is ordinary text; create a tag by typing a new name and pressing `enter`. In tag management (palette `go to tags`): `n` new, `e` rename, `d` delete a definition, `esc` back.

## Lobby and palette

In the lobby, `↑`/`↓` move; every other key edits the filter. In the palette, `↑`/`↓` select, `enter` runs, `esc` or `ctrl+c` closes.

## Outside the TUI

The zsh widget `extras/zsh/lif-pick.zsh` binds `Alt+i` (`^[i`) to [`lif pick`](../../commands/lif-pick/). It is not part of the TUI. Load it by sourcing the file from your `.zshrc`; that was not measured here.
