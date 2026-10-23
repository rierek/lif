---
title: "Views"
description: "Roadmap tree, flat, ready and detail views of the TUI."
---

Measured on Lif 0.13 with a test repository of 17 items; the TUI code is unchanged in 0.14 (checked 2026-10-14). Key names: [Key bindings](../key-bindings/).

## Overview of the views

There are four views. The item detail is a pane inside Browse and Ready; which panes show is a layout, not a view.

| View | Purpose | How to reach it |
|---|---|---|
| Browse, tree | The roadmap: milestones, epics, features, tasks. | Default after start; `M` |
| Browse, flat | All items in one list. | `L` |
| Ready | Items `lif list --ready` lists, flat. | `R` |
| Lobby | Repository picker. | `P` |
| Tag management | Tag definitions. | Command palette `K`, entry `view: tags`. |
| Detail pane | Fields, relations, body of the selected item. Not shown at start. | `v` / `shift+v` change the [layout](#layouts). |

The header reads `> <repo>: <View>`. The focused pane is marked by color only. Hidden by default in tree, flat and ready: `completed` and `scrapped` items. The filter menu category `Archive` shows them.

## Tree view

- The tree is the roadmap (`lif roadmap`). At start only the milestones and the roots `Unscheduled` and `Not on roadmap` show, all collapsed. Every level is sorted like `lif roadmap`.
- `Unscheduled` holds epics, features and items without a milestone. `Not on roadmap` holds every item the roadmap places nowhere: milestones without children, types with `roadmap: false`, items whose parent does not exist, and members of a parent cycle. Both roots come last.
- The tree pane width is the larger of one third of the screen and `layout.tree_width` (default 36, 24 to 60). At 140 columns it is 46.
- Sorting and menus use the status, type and priority lists of `.lif.yml` in configured order. Values the config does not name sort last.

Screenshot follows.

## Flat view

`L` shows the flat list; `M` returns to the Roadmap (nested), and so does `esc` once no search or filter is active. The flat list helps when you search across all items and the hierarchy does not matter. Search and filters apply the same way.

## Ready view

`R` opens it; `esc` clears an active search or filter and then returns to the Roadmap (`M` returns at once). It shows the items `lif list --ready` lists (not blocked, no container types, not `in-progress` or `draft`, not archived, no archived ancestor), flat. `v` and `shift+v` cycle the same [layouts](#layouts) as in Browse. `S` cycles the sort: `status`, `priority`, `created`, `updated`, then `status` again. `created` and `updated` sort newest first. Timestamps have one-second resolution. `S` is not in the footer; it appears in the `?` help and as the suffix `· sort <mode>` on the search line.

Screenshot follows.

## Layouts

`v` and `shift+v` cycle the layout of Browse (tree and flat) and Ready. `v` goes forward, `shift+v` backward, both wrap. In Ready the Ready list takes the place of the roadmap. The order is:

| Layout | Shows |
|---|---|
| Roadmap | The list at full width. No filter bar, no detail pane. This is the start layout. |
| Roadmap + filter | The filter bar above the list. |
| Roadmap + filter + details | The filter bar above the list, the detail pane on the right. |
| Details | The detail of the selected item at full width. No filter bar. |

When a layout change leaves the focused region out, focus moves to the list, or to the detail in layout Details. `tab` and `shift+tab` never change the layout.

## Detail view

The detail is a box form. The boxes are, from top: `Title · <id>`, `Status (s)`, `Type (t)`, `Priority (p)`, `Parent (a)`, `Tags (g)`, `Body (e)`, `Relations (b)`, `History`. The letter is the key that edits the box.

- In layout Roadmap + filter + details, `tab` and `shift+tab` switch focus between the list and the detail pane, in both directions. In the other layouts there is only one region and they do nothing.
- While the detail has focus, `↑`/`↓` step through the boxes. `esc` returns to the list (not in layout Details, which has no list).
- `enter` on a box opens its menu, picker or the editor.

## Moving around

- Movement uses the `i/j/k/l` diamond (`i` up, `k` down, `j` left, `l` right) and the arrow keys, not vim keys.
- `enter` opens, `esc` goes back. `ctrl+r` reloads. `K` opens the command palette.
- The TUI watches the store: a change from another process appears without a key press.
- Mouse: always on. Wheel scrolls the list (or the detail body). A left click selects a row; a second click within 500 ms expands or collapses an item with children. Clicking a detail box opens its editor or picker. A click on the filter chips opens the filter menu. The mouse is ignored in forms, overlays, the lobby and tag management. Without a detail pane, a click does not reach the former detail area.

## Search and filters

- `/` starts the search in tree, flat and ready. Each keystroke can trigger a search.
- Text of three or more bytes goes to `lif list --search`, so [search syntax](../../search-syntax/) applies, including its limits (no `AND`/`OR`/`NOT`). Shorter text matches as a substring of title or ID. An ID substring always matches.
- The TUI splits off its own prefixes before the search: `st:`, `ty:`, `pr:`, `tag:` followed by a value become facet filters (status, type, priority, tag). `tag:auth` in the TUI is a facet, not a search field.
- A bad query shows an error toast, for example `lif list: exit status 1: Error: querying items: syntax error`. Incomplete queries such as `title:` flash this while you type.
- `f` opens the facet menu with the categories Status, Type, Priority, Tags, Archive. `space` or `x` toggles a value, `X` clears all. Facets combine with AND across categories and OR within one.
- The item pickers (`a` and `b`) have chips: `ctrl+t` type, `ctrl+n` status, `ctrl+p` priority, `ctrl+g` tags. They exist only in those two pickers.
