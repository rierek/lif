---
title: "Editing items"
description: "Menus, pickers and dialogs for changing items in the TUI."
---

## Purpose
Explains how to create, change and delete items in the TUI, which menus, pickers and dialogs are involved and where the limits are.

## Audience
TUI users who maintain items without the CLI, and readers who want to know whether the TUI applies the same rules as the CLI.

## Required sections

### Principles
- Explains: write access in the TUI goes through the same item operations as CLI and GraphQL; example: the parent picker calls `SetParent` with the ETag.
- Consequence: the same rules (rank rule, cycle check) apply as with `lif update --parent`.
- All actions work on the selected item; for shortcuts see `reference/tui/key-bindings`.
- Source: FACTS.md §8.2, §3.1

### Create an item
- `c` opens the create form and afterwards a confirmation dialog.
- Shows the fields of the form. (TODO: verify)
- Explains which status and priority are prefilled and whether a parent is taken from the selection. (TODO: verify)
- Links to the CLI equivalent `lif create` (`reference/commands/lif-create`).
- Source: FACTS.md §8.2, §8.3; form fields and prefill — `TODO: verify` (Phase 5: check `internal/tui/form_create_bean.go` and a running TUI)

### Quick changes
- `s` opens the status menu, `o` the type menu, `u` the priority menu.
- Menus show the configured values (`reference/configuration`). (TODO: verify)
- Change the title through a separate form (key: TODO: verify).
- Source: FACTS.md §8.3, §8.2; menu values and key for the title dialog — `TODO: verify` (Phase 5: check `internal/tui/keymap.go` and menu code)

### Pickers
- Parent picker `a`: selects the parent; applies the hierarchy rules.
- Relations picker `r`: sets and removes blocking relations (blocking / blocked_by).
- Tag picker `t`: toggles tags with `space`; `n` creates a new tag.
- Item pickers offer facet chips for narrowing (`ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g`).
- Source: FACTS.md §8.2, §8.3

### Edit the body
- `e` or `ctrl+e` opens the body in the editor.
- Explains how the editor is chosen (`editor` in the settings or `$EDITOR`) with a link to `reference/tui/settings-and-themes`. (Order: TODO: verify)
- The body is saved after closing. (TODO: verify)
- Source: FACTS.md §8.2, §8.3; editor order and saving — `TODO: verify` (Phase 5: check `internal/tui/editor.go`)

### Delete
- `d` deletes after confirmation; no cascading (children are not deleted with it).
- Deleting removes incoming links automatically (documented for GraphQL `deleteItem`; presumably the same for the TUI). (TODO: verify)
- Source: FACTS.md §8.3, §7.2; behavior in the TUI — `TODO: verify` (Phase 5: check `internal/tui/box_confirm_delete.go`)

### Tag management
- Separate view for tag management; `e` renames a tag.
- How the view is opened and what renaming does to items. (TODO: verify)
- Source: FACTS.md §8.2, §8.3; access and effect — `TODO: verify` (Phase 5: check `internal/tui/view_tag_management.go`)

### Limits
- A change of only the `type` does not re-check parent/child ranks; same behavior as `lif update -t` (known bug).
- The TUI does not write bulk changes across several items. (TODO: verify)
- Source: FACTS.md §3.1; DESIGN.md D43; bulk changes — `TODO: verify` (Phase 5: check the TUI)
