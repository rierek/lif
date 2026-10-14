---
title: "Key bindings"
description: "All TUI key bindings, generated from the key map."
---

## Purpose
Complete list of all TUI key bindings. The final page is generated from `internal/tui/keymap.go` and is not a hand-maintained table.

## Audience
TUI users who look up a key, and documentation authors who maintain the generation.

## Required sections

### How to read this page
- Explains: the source is `keyMap` in `internal/tui/keymap.go`; the in-app help (`?`) is generated from the same table, so page and help match.
- Explains the notation (`ctrl+x`, `shift+tab`, several keys per action).
- Note: navigation uses the `i/j/k/l` diamond, not vim `h/j/k/l`.
- Table layout: key, action, context (where it applies).
- Source: FACTS.md §8.3; DESIGN.md D44

### Navigation
- Up `↑`/`i`, down `↓`/`k`, left (back/out) `←`/`j`, right (in/expand) `→`/`l`.
- `tab` focus in (tree to detail), `shift+tab` focus out.
- `enter` open/confirm, `esc` back, `[`/`ctrl+left` history back, `]`/`ctrl+right` history forward.
- Source: FACTS.md §8.3

### Global actions
- `K` command palette, `p` repository picker, `b` backlog, `/` search, `f` facet filter, `X` clear filters, `S` change backlog sort order, `G` nested/flat, `ctrl+r` reload.
- `v` fullscreen, `1`..`9` jump to accordion section, `y` copy context (OSC52 and native).
- `q`/`ctrl+c` quit (with confirmation), `?` help.
- Source: FACTS.md §8.3

### Item actions
- `s` status menu, `o` type menu, `u` priority menu, `a` assign parent, `r` relations (blocking picker), `c` create, `d` delete (confirmation, no cascading), `t` tag picker, `e`/`ctrl+e` body in `$EDITOR`.
- Links to `reference/tui/editing-items` for the behavior.
- Source: FACTS.md §8.3

### Pickers and facets
- `space`/`x` toggle facet; chips in the item picker: `ctrl+t` type, `ctrl+n` status, `ctrl+p` priority, `ctrl+g` tags.
- Shows in which pickers the chips are available. (TODO: verify)
- Source: FACTS.md §8.3; availability per picker — `TODO: verify` (Phase 5: check `internal/tui/keymap.go` and picker files)

### Tag picker and tag management
- Tag picker: `n` new tag, `space` toggle tag.
- Tag management: `e` rename tag.
- Source: FACTS.md §8.3

### Outside the TUI
- The zsh widget `extras/zsh/lif-pick.zsh` binds `Alt+i` (`^[i`) to `lif pick`; it is not part of the TUI.
- Shows how to load the widget. (TODO: verify)
- Links to the command page `lif pick` (`reference/commands/lif-pick`).
- Source: FACTS.md §8.3, §1.1 (`lif pick`); loading the widget — `TODO: verify` (Phase 5: read `extras/zsh/lif-pick.zsh`)

### Generation
- Describes: the generator (`cmd/lif-docgen` or an extension) reads `keyMap` and writes the tables into this page; no hand-copied table in the final page.
- Note for Phase 5: the generator extension is still to be built; until then the page remains a contract.
- Describes the check: a test or CI step ensures the page does not deviate from `keymap.go`. (TODO: verify)
- Source: DESIGN.md D24, D44, FACTS.md §8.3; generator and check — `TODO: verify` (Phase 5: inspect the generator under `cmd/lif-docgen`)
