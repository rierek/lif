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
| `tab` / `shift+tab` | Switch focus between the list and the detail pane (only in layout roadmap + filter + details; never changes the layout). From a filter field, `tab` returns to the list. The footer lists `tab` only in that layout and `/` only while the list has focus |
| `↑` / `↓` in the detail pane | Previous or next box |
| `enter` | Open or confirm |
| `esc` | Back; leaves the detail pane and menus; in the flat list and Ready, see Views |
| `pgup` / `pgdown` | Page the detail body (not listed in `?`) |

## Views

| Key | Action |
|---|---|
| `M` | Roadmap |
| `L` | Flat list |
| `R` | Ready |
| `P` | Repository picker (lobby) |
| `v` / `shift+v` | Next or previous layout: roadmap, roadmap + filter, roadmap + filter + details, details; both wrap |

The Board view (`B`) is planned and not bound yet. `esc` in the flat list or Ready first clears an active search or filter, then returns to the Roadmap.

## Global actions

| Key | Action |
|---|---|
| `K` / `:` | Command palette |
| `/` | Search |
| `f` | Jump to the filter bar, Type field; from layout roadmap or details it first switches to roadmap + filter |
| `X` | Clear all filters |
| `S` | Cycle ready sort |
| `ctrl+r` | Reload |
| `y` | Copy (yank) item context |
| `?` | Help |
| `q` | Quit dialog |
| `ctrl+c` | Quit at once from every state (lists, forms, menus, dialogs, lobby), no confirmation, no lobby |

## Dialogs

| Key | Action |
|---|---|
| `y` | Confirm (quit, create, delete, tag delete); `enter` confirms too |
| `n` | Cancel; `esc` cancels too |

In a dialog `y` confirms; everywhere else `y` copies the item context.

## Item actions

All act on the selected item. Behavior: [Editing items](../editing-items/).

| Key | Action |
|---|---|
| `s` | Status menu |
| `t` | Type menu |
| `p` | Priority menu |
| `a` | Assign parent |
| `b` | Blocking (blocking picker) |
| `g` | Tag picker |
| `c` | Create item |
| `d` | Delete (with confirmation, no cascade) |
| `e` / `ctrl+e` | Edit the whole item in the editor |

Title, settings and tag management have no key; use the palette (`K` or `:`).

Moved or removed keys: type was `o`, priority was `u`, tags were `t`, blocking was `r`, Ready was `b`, repositories were `p`, nested or flat was `G`. The old keys do nothing now.

## Pickers and filter fields

| Key | Action | Where |
|---|---|---|
| `←` / `→` | Previous or next filter field (Type, Status, Priority, Tags, Archive) | Focused filter field |
| `enter` / `space` / `↓` | Open the value menu of the field | Focused filter field |
| `↑` / `↓` | Move within the values | Filter value menu |
| `space` / `x` / `enter` | Toggle a value; the list filters at once and the menu stays open | Filter value menu |
| `X` | Clear all filters; layout and menu stay | Focused filter field, filter value menu, lists |
| `esc` | Close the menu; a second `esc` returns to the list | Filter value menu, focused filter field |
| `space` / `x` | Toggle a value | Relations picker |
| `space` | Toggle a tag | Tag picker |
| `ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g` | Chip: type, status, priority, tags | Parent picker and relations picker only |

## Tag picker and tag management

In the tag picker `n` is ordinary text; create a tag by typing a new name and pressing `enter`. In tag management (palette `view: tags`): `n` new, `e` rename, `d` delete a definition, `esc` back.

## Lobby and palette

In the lobby, `↑`/`↓` (or `i`/`k`) move, `enter` opens the repository, `/` opens the repository filter (the only way to type into it), `q` quits at once, and `esc` twice leaves the lobby (back to the open repository, or quit when none is open). In the filter `esc` closes it. Any other key between the two `esc` presses cancels the first. In the palette, `↑`/`↓` select, `enter` runs, `esc` closes.

The palette entry `open: documentation` opens this reference (`https://rierek.github.io/lif/reference/tui/views/`) in the default browser and shows the URL in a toast. If no opener exists, an error toast shows the URL instead.

## Outside the TUI

The zsh widget `extras/zsh/lif-pick.zsh` binds `Alt+i` (`^[i`) to [`lif pick`](../../commands/lif-pick/). It is not part of the TUI. Load it by sourcing the file from your `.zshrc`; that was not measured here.
