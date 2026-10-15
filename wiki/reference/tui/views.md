---
title: "Views"
description: "Tree, flat, backlog and detail views of the TUI."
---

Measured on Lif 0.13 with a test repository of 17 items; the TUI code is unchanged in 0.14 (checked 2026-10-14). Key names: [Key bindings](../key-bindings/).

## Overview of the views

There are four views. The item detail is a pane inside Browse and Backlog, and fullscreen is a mode, not a view.

| View | Purpose | How to reach it |
|---|---|---|
| Browse, tree | Items nested by `parent`. | Default after start. |
| Browse, flat | All items in one list. | `G` toggles. |
| Backlog | Open leaf items, flat. | `b` |
| Lobby | Repository picker. | `p` |
| Tag management | Tag definitions. | Command palette `K`, entry `go to tags`. |
| Detail pane | Fields, relations, body of the selected item. | `tab` from the tree. |
| Fullscreen | List or detail over the full width. | `v` |

The header reads `> <repo>: <View>`. The focused pane is marked by color only. Hidden by default in tree, flat and backlog: `completed` and `scrapped` items. The filter menu category `Archive` shows them.

## Tree view

- Roots are items without a parent. Children follow `parent`. Every level is sorted by status, priority, type, title.
- Items whose parent does not exist, and members of a cycle, appear under a root named `(orphaned)`, last.
- The tree pane width is the larger of one third of the screen and `layout.tree_width` (default 36, 24 to 60). At 140 columns it is 46.
- Sorting and menus use the status, type and priority lists of `.lif.yml` in configured order. Values the config does not name sort last.

Screenshot follows.

## Flat view

`G` switches between nested and flat. The flat list helps when you search across all items and the hierarchy does not matter. Search and filters apply the same way.

## Backlog view

`b` opens it; `esc` returns to Browse. It shows leaf items (no milestones or epics) in a flat list with the detail pane on the right. `S` cycles the sort: `status`, `priority`, `created`, `updated`, then `status` again. `created` and `updated` sort newest first. Timestamps have one-second resolution. `S` is not in the footer; it appears in the `?` help and as the suffix `· sort <mode>` on the search line.

Screenshot follows.

## Detail view

`tab` from the tree focuses the detail pane. The box form is the default. `LIF_TUI_BOXFORM=0`, `false`, `no` or `off` switches to the classic layout. In the box form the boxes are, from top: `Title · <id>`, `Status (s)`, `Type (o)`, `Priority (u)`, `Parent (a)`, `Tags (t)`, `Body (e)`, `Relations (r)`, `History`. The letter is the key that edits the box.

- While the detail pane has focus, `tab` and `shift+tab` step through the boxes. `esc` returns to the tree. `↑`/`↓` scroll.
- `enter` on a box opens its menu, picker or the editor.
- `v` opens the item fullscreen. Inside fullscreen detail, `[` and `]` go back and forward through relation jumps.
- The classic layout (`LIF_TUI_BOXFORM=0`) has four collapsible sections `META`, `BODY`, `RELATIONS`, `HISTORY`. Digits `1` to `4` jump to them, only in that layout. In the default layout the digits do nothing.

## Moving around

- Movement uses the `i/j/k/l` diamond (`i` up, `k` down, `j` left, `l` right) and the arrow keys, not vim keys.
- `enter` opens, `esc` goes back. `ctrl+r` reloads. `K` opens the command palette.
- The TUI watches the store: a change from another process appears without a key press.
- Mouse: always on. Wheel scrolls the list (or the detail body). A left click selects a row; a second click within 500 ms expands or collapses an item with children. Clicking a detail box opens its editor or picker. A click on the filter chips opens the filter menu. The mouse is ignored in forms, overlays, the lobby, tag management and fullscreen.

## Search and filters

- `/` starts the search in tree, flat and backlog. Each keystroke can trigger a search.
- Text of three or more bytes goes to `lif list --search`, so [search syntax](../../search-syntax/) applies, including its limits (no `AND`/`OR`/`NOT`). Shorter text matches as a substring of title or ID. An ID substring always matches.
- The TUI splits off its own prefixes before the search: `st:`, `ty:`, `pr:`, `tag:` followed by a value become facet filters (status, type, priority, tag). `tag:auth` in the TUI is a facet, not a search field.
- A bad query shows an error toast, for example `lif list: exit status 1: Error: querying items: syntax error`. Incomplete queries such as `title:` flash this while you type.
- `f` opens the facet menu with the categories Status, Type, Priority, Tags, Archive. `space` or `x` toggles a value, `X` clears all. Facets combine with AND across categories and OR within one.
- The item pickers (`a` and `r`) have chips: `ctrl+t` type, `ctrl+n` status, `ctrl+p` priority, `ctrl+g` tags. They exist only in those two pickers.
