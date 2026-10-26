---
title: "Editing items"
description: "Menus, pickers and dialogs for changing items in the TUI."
---

On a type change the CLI the TUI calls re-checks the hierarchy. Keys: [Key bindings](../key-bindings/).

## Principles

The TUI has no write logic of its own. Every change goes through the same item core as `lif update`, `lif create` and `lif delete`, so the same validation applies: rank rule and cycle check for `parent` ([Configuration](../../configuration/#types-ranks-and-hierarchy-rules)). Updates send the item's ETag as the `If-Match` precondition. If the item changed meanwhile, a sticky toast shows `Conflict: item changed externally`. After every change the TUI reloads. All actions work on the selected item.

## Create an item

`c` opens the form `New item`. Fields in order: Title (required), Type, Priority, Status, Parent (optional), Tags (optional, space or comma separated), Body (optional). `enter` moves to the next field; `enter` in Body submits. `esc` discards. Then a dialog `Create?` shows `Item (<type>): <title>`: `enter` creates, `esc` or `n` goes back to the form with your values.

- Prefill: type `task`, priority `normal`, status `todo`. These are fixed, not `lif.default_type` or `lif.default_status`.
- Parent is prefilled with the ID of the selected item, even if its type cannot be a parent. The form checks only that the ID exists; the CLI then applies the rank rule and the form reopens with an error toast if it fails.
- After success the cursor moves to the new item.

CLI equivalent: [`lif create`](../../commands/lif-create/).

## Quick changes

- `s` status menu, `t` type menu, `p` priority menu. `enter` applies at once; `esc` closes.
- The menus list the statuses, types and priorities of the project: the built-in ones plus those in `.lif.yml`, in configured order.
- Title: there is no key. Use the command palette (`K`, `set: title`). In the classic detail layout `enter` on `title:` also works. The form saves on `enter` without confirmation.

## Pickers

- `a` parent picker: first row `(No parent)`, then candidates. A type may hang under every type of a lower rank, using the ranks of the project config, the same rule the CLI enforces.
- `b` blocking picker: toggles `blocking` for the selected item. `space` toggles, `enter` saves. It does not exclude cycles; the check happens on save.
- `g` tag picker: type to filter, `space` toggles, `enter` saves. To create a tag, type a name that matches nothing and press `enter`, then `enter` again to save. Tag names are lowercase, hyphen-separated.
- The parent and relations pickers offer the chips `ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g`.

## Edit the whole item

`e` or `ctrl+e` opens the **whole item** (front matter and body, not only the body) as a temporary file in an editor. The editor is chosen by `editor` in the [settings file](../settings-and-themes/#choosing-the-editor), then `$VISUAL`, then `$EDITOR`, then `vi`.

- When the editor exits, Lif compares the file with the original. Unchanged: nothing happens. Changed: the keys `title`, `status`, `type`, `priority`, `tags`, `parent`, `blocking`, `blocked_by` and the body are written with one `lif update`. Edits to `created_at`, `updated_at` and the `# <id>` line are dropped without notice.
- This is the only way to set a status or type that is not in the fixed menu lists.
- If saving fails (conflict, invalid value), a toast shows the error and `Version saved: <path>` names a recovery file `lif-tui-conflict-*.md` with your text.
- A GUI editor must block until closed (`code -w`), or the late write is lost.

## Delete

`d` opens `Delete <type>` with the title. The dialog counts direct children (`N children lose their parent — become their own roots`) and items that link to it, and says `Irreversible.` `enter` deletes permanently; `esc` or `n` cancels. Children are not deleted. Incoming `parent`, `blocking` and `blocked_by` references are removed from other items, like `lif delete`.

## Tag management

Open it with the command palette entry `view: tags`; it has no key. It lists tag definitions from the repository file `.lif-tags.yml` and tags in use. Keys: `n` new definition (or register a free tag), `e` rename, `d` delete a definition, `esc` back.

- Rename rewrites the definition and runs one `lif update` for every item with the old tag. It continues after a failure and shows a summary toast.
- Delete removes only the definition. Items keep the tag.

## Limits

- No multi-select and no bulk change over several items. The tag rename is the only multi-item write.
