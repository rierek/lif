---
title: "Key bindings"
description: "All TUI key bindings, from the key map."
---

Measured on Lif 0.13, code unchanged in 0.14 (checked 2026-10-14). The in-app help (`?`) is generated from the same key map (`keyMap` in `internal/tui/keymap.go`). This page is written by hand; no generator or test ties it to the key map yet, so it can drift. Press `?` in the TUI for the keys of your version. See [Help](#help) for search and scrolling.

Notation: `ctrl+x` is Ctrl and `x`; several keys for one action are separated by `/`. Navigation uses the `i/j/k/l` diamond, not vim `h/j/k/l`.

## Navigation

| Key | Action |
|---|---|
| `↑` / `i` | Up |
| `↓` / `k` | Down |
| `←` / `j` | Back or out (collapse) |
| `→` / `l` | In (expand) |
| `tab` / `shift+tab` | Move focus to the next or previous target of the layout's focus ring and wrap at the ends (see Views, Focus ring). Never changes the layout; does nothing in layout Roadmap. The footer lists them only when the ring has two or more targets, and `/` only while the list has focus |
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
| `X` | Clear Type, Status, Priority and Tags (not Archive) |
| `S` | Cycle ready sort |
| `ctrl+r` | Reload |
| `y` | Copy (yank) item context |
| `?` | Help ([search and scroll](#help)) |
| `q` | Quit dialog |
| `ctrl+c` | Quit at once from every state (lists, forms, menus, dialogs, lobby), no confirmation, no lobby |

## Help

`?` opens the help: a search field, a Views section that describes every view and layout in one line with the keys that reach it, and the key groups (the Tasks group shows when `tui.actions` exist).

| Key | Action |
|---|---|
| `/` | Focus the search field. Every printable key is text while it has focus, so `q`, `?`, `i` and `k` do not close or scroll |
| `enter` | Keep the term and leave the search field |
| `esc` | Clear the term; a second `esc` closes the help |
| `?` / `q` | Close the help (outside the search field) |
| `↑` / `↓` | Scroll one row (`i` / `k` too outside the search field) |
| `pgup` / `pgdown` | Scroll one page |

The search matches the key, the description and the group name, ignoring case; a group header shows only for groups with matches. No match shows `No matching key bindings`. The help scrolls, so every row is reachable on a 24-row terminal; the footer shows the visible rows.

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
| `b` | Relations (blocking picker) |
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
| `X` | Clear Type, Status, Priority and Tags; Archive, layout and menu stay | Focused filter field, filter value menu, lists |
| `esc` | Close the menu; a second `esc` returns to the list | Filter value menu, focused filter field |
| `space` / `x` | Toggle a value | Relations picker |
| `space` | Toggle a tag | Tag picker |
| `ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g` | Chip: type, status, priority, tags | Parent picker and relations picker only |

## Tag picker and tag management

In the tag picker `n` is ordinary text; create a tag by typing a new name and pressing `enter`. In tag management (palette `view: tags`): `n` new, `e` rename, `d` delete a definition, `esc` back.

## Lobby and palette

In the lobby, `↑`/`↓` (or `i`/`k`) move, `enter` opens the repository, `/` opens the repository filter (the only way to type into it), `q` opens the `Quit?` dialog (`y` quits, `n` or `esc` stays), and `esc` twice leaves the lobby (back to the open repository, or quit when none is open). In the filter `esc` closes it. Any other key between the two `esc` presses cancels the first. In the palette, `↑`/`↓` select, `enter` runs, `esc` closes.

The palette entry `open: documentation` opens this reference (`https://rierek.github.io/lif/reference/`) in the default browser and shows the URL in a toast. If no opener exists, an error toast shows the URL instead.

The palette lists its entries by verb: all `set:` entries first, then all `view:` entries, then the others (`create:`, `delete:`, `focus:`, `register:`, `reload:`, `run:`, `search:`), and `open:` last. Inside each group the entries are alphabetical. The `set:` and `delete:` entries and `create: tag` appear only when an item is selected.

A sticky toast, such as a warning, has no timer. Click it, or press `esc`, to close it. `esc` closes the toast only when it has no other job: with an overlay, form, palette, search, filter, detail focus, flat list or Ready list open, `esc` keeps its own meaning. The footer shows `esc dismiss` while it applies.

## Outside the TUI

The zsh widget `extras/zsh/lif-pick.zsh` binds `Alt+i` (`^[i`) to [`lif pick`](../../commands/lif-pick/). It is not part of the TUI. Load it by sourcing the file from your `.zshrc`; that was not measured here.
