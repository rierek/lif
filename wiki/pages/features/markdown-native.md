---
title: "Markdown-native"
description: "Items are plain Markdown files you can read, diff and edit anywhere."
---

An item is one Markdown file.

## What

- Layout: `---`, YAML front matter, `---`, then the Markdown body.
- File name: `<id>--<slug>.md`. ID and slug are not front matter fields. Lif derives them from the file name.
- Lif writes the fields in a fixed order: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`. The first front matter line is a YAML comment `# <id>`.
- Lif keeps unknown keys and writes them in alphabetical order after the known fields.
- A **Liff** is the Markdown file of an item. See the [glossary](../../reference/glossary/) and the [item file format](../../reference/item-file-format/).

## Why

- You keep ownership of your information. No database, no export step.
- The files are readable, diffable and editable in any editor.
- The format is open. Scripts and agents read the files without Lif.

## How it looks

`lif complete --commit HEAD --summary "Mail sent"` changed this item. Excerpt of the real `git diff` (context lines left out):

```diff
-status: todo
+status: completed
+priority: normal
-updated_at: 2026-10-14T19:17:18Z
+updated_at: 2026-10-14T19:17:23Z
+commit: 22b4b97e63ce8e8bdf836c6aa509adfeb1cca41b
+## Summary of Changes
+
+Mail sent
```

Reading the diff:

- `status` and `updated_at` are known fields. Lif rewrites them in place.
- `commit` is an extra key. Lif writes it after the known fields.
- The body gains a `## Summary of Changes` section.

`lif show <id>` prints the same data in the terminal.

<details>
<summary>How to use attachments</summary>

Each item can have a directory `attachments/<item-id>/` in the store for evidence and artifacts, such as a design note or a screenshot.

- On load, Lif skips the `attachments/` directory. It never looks for items there.
- `lif check` reports a directory without a matching item as a problem (`attachments/<id> has no item <id>`, exit 1). An item in the archive counts as present.
- Lif does not create the directory for you. Create it yourself and mention the file in the item body. This is a convention, not enforced by Lif.

</details>

<details>
<summary>How to link items from a README</summary>

Keep the README as the entry point and let the items hold the work state. Do not copy status into the README. Generate a roadmap instead:

```bash
lif roadmap --format markdown
```

Real output (excerpt):

```markdown
# Roadmap

## Milestone: Ship v1 ([ft-2gae](.lif/ft-2gae--ship-v1.md))

### Epic: Login flow ([ft-txrv](.lif/ft-txrv--login-flow.md))
```

Useful options: `--no-links`, `--link-prefix`, `--depth`, `--tags`, `--include-done`. See [`lif roadmap`](../../reference/commands/lif-roadmap/).

</details>

## Limits

- Lif does not check manual edits on load: not the hierarchy, not status, not priority.
- Limits apply only when you write through Lif: body at most 1 MiB, title at most 1000 characters, tags in lower case, starting with a letter, hyphens only between characters.
- One unparsable `.md` file aborts the whole load (`loading <path>: ...`). Lif then returns no items.
- `lif check` reports neither a file name that differs from the ID nor duplicate IDs. With a duplicate ID, the file loaded later silently overwrites the other.
- Obsidian Bases compatibility is unclear. Nothing in the code or the docs says anything about it. See [Surface independence](../surface-independence/#editors-such-as-obsidian).

## Reference links

- [Item file format](../../reference/item-file-format/): all fields, values, file names, manual edits.
- [Configuration](../../reference/configuration/): store path, ID prefix, types, status.
- [`lif check`](../../reference/commands/lif-check/): what is checked and what is not.
- [`lif roadmap`](../../reference/commands/lif-roadmap/): Markdown output and link options.
