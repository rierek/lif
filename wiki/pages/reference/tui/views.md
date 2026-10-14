---
title: "Views"
description: "Tree, flat, backlog and detail views of the TUI."
---

## Purpose
Describes the views of the TUI, how to switch between them and how to move, search and filter within them.

## Audience
TUI users who want to know which view fits which task and how operation works in general.

## Required sections

### Overview of the views
- Table: repo tree (browse), flat list, backlog, item detail, fullscreen, lobby, tag management; per view one line of purpose and the key to reach it.
- Explains: `tab` switches focus between tree and detail. (Screen layout: TODO: verify)
- Links to `reference/tui/key-bindings` for views with a key.
- Source: FACTS.md §8.2, §8.3; screen layout — `TODO: verify` (Phase 5: check a running TUI)

### Tree view
- Shows the item hierarchy (parent/child) in the repo tree; `→`/`l` goes in or expands, `←`/`j` goes back or out.
- Tree width: setting `layout.tree_width` (default 36, minimum 24, maximum 60).
- Screenshot `browse.png` (planned).
- Source: FACTS.md §8.2, §8.3

### Flat view
- `G` toggles between nested and flat.
- Shows when the flat list helps (search across all items, without hierarchy).
- Source: FACTS.md §8.3, §8.2

### Backlog view
- `b` opens the backlog; `S` changes the sort order (cycles).
- Explains which items the backlog shows and which sort orders exist. (TODO: verify)
- Screenshot `backlog.png` (planned).
- Source: FACTS.md §8.3, §8.2; content and sort orders — `TODO: verify` (Phase 5: check `internal/tui/view_browse_backlog.go` and a running TUI)

### Detail view
- Shows an item with fields, relations and body; `tab` moves focus into the detail view, `shift+tab` back.
- `v` opens the item fullscreen (screenshot `detail-fullscreen.png`, planned).
- `1`..`9` jump to a section (accordion) of the detail view.
- Describe the structure of the sections in the detail view. (TODO: verify)
- Source: FACTS.md §8.3, §8.2; section names — `TODO: verify` (Phase 5: check `internal/tui/view_detail_bean.go`)

### Moving around
- Movement with the `i/j/k/l` diamond (up/left/down/right), not with vim `h/j/k/l`; arrow keys also work.
- `enter` opens, `esc` goes back; `[` / `ctrl+left` back in the history list, `]` / `ctrl+right` forward.
- `K` opens the command palette, `ctrl+r` reloads.
- Mouse is supported. (Extent of mouse support: TODO: verify)
- Source: FACTS.md §8.3, §8.2; mouse extent — `TODO: verify` (Phase 5: check `internal/tui/mouse.go`)

### Search and filters
- `/` starts the search; `f` opens the facet filters (type, status, priority, tags); `space`/`x` toggles a facet; `X` clears all filters.
- The item picker has facet chips: `ctrl+t` type, `ctrl+n` status, `ctrl+p` priority, `ctrl+g` tags.
- Links to `reference/search-syntax` for the search syntax. (Does the syntax also apply in the TUI? TODO: verify)
- Source: FACTS.md §8.3, §8.2; search syntax in the TUI — `TODO: verify` (Phase 5: check `internal/tui/box_filter_bar.go`)

### Screenshots plan
- Existing images: `wiki/screenshots/{browse,backlog,detail-fullscreen,command-center,lobby}.png`; mapping: tree, backlog, detail fullscreen, command palette, lobby.
- New recordings as webp or VHS GIF (D38) if the images no longer match the interface.
- Source: DESIGN.md D38, D44; currency — `TODO: verify` (Phase 5: compare with a running TUI)
