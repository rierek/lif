---
title: "Markdown-native"
description: "Items are plain Markdown files you can read, diff and edit anywhere."
---

## Purpose
Explains that every item is a plain Markdown file and what that means for ownership, readability and editing. Contains two collapsed how-to guides (attachments, links from READMEs) and states honestly what stays unchecked on manual edits.

## Audience
People who want to know whether their data stays open and tool-independent, and agents that read or write item files directly. The page answers "What is in the file?" and "What may I change by hand?".

## Required sections

### What
- An item is exactly one Markdown file: `---`, YAML front matter, `---`, then the Markdown body.
- File name `<id>--<slug>.md`; ID and slug are not in the front matter but derived from the file name.
- Fields written by Lif in fixed order: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`; the first front matter line is a YAML comment `# <id>`.
- Unknown keys are preserved (extra keys) and written alphabetically after the known fields.
- First use of the term: Liff = the Markdown file of an item; the definition lives on `reference/item-file-format` and `reference/glossary`, name it briefly here and link.
- Source: FACTS.md §2.1, §2.2, §2.4

### Why
- Users keep ownership and control of their information (core benefit according to the concept).
- Human-readable, diffable, editable in any editor; no database export needed.
- Open format: scripts and agents read the files without Lif.
- Source: CONCEPT.md §4.1

### How it looks
- Annotated example file: file name, front matter, body; arrows or numbers explain each field.
- Shows an extra key (for example `commit`) and its place after the known fields.
- Generate the example in phase 5 with `lif create` and paste the file unchanged (TODO: verify).
- Optional link to `lif show` for the same view in the terminal.
- Source: FACTS.md §2.1, §2.2, §2.4; (TODO: verify) for the concrete example: build it in phase 5 with `lif init` and `lif create`

### Attachments
- How-to: Each item has a directory `attachments/<item-id>/` in the store for evidence and artifacts.
- On load, Lif skips the `attachments/` directory; no items are searched there.
- `lif check` reports directories without a matching item as a problem (`attachments/<id> has no item <id>`, exit 1); items in the archive still count as present.
- When and how to reference an attachment from the body (design doc, decision, review artifact, screenshot): convention from the draft (TODO: verify)
- How attachments are linked in the item file and whether `lif` creates them or only checks them (TODO: verify)
- Source: FACTS.md §2.4, §6.4; (TODO: verify) for attachment workflow: check the `lif prime` template (`internal/commands/prompt.tmpl`) and `wiki/` in the checkout

### Link items from READMEs
- How-to: README as entry point, items as work state; do not duplicate status in the README.
- Generate the roadmap as Markdown: `lif roadmap --format markdown`, options `--no-links`, `--link-prefix`, `--depth`, `--tags`, `--include-done`.
- Shows a short roadmap snippet from real output (generate in phase 5).
- Example section for the README, badge and link to the docs site or `lif prime`: only hinted at in the draft (TODO: verify)
- Source: FACTS.md §1.1 (`lif roadmap`); (TODO: verify) for example section and badge: check the trial page and README in the checkout, run the snippet for real with `lif roadmap`

### Limits
- Manual edits are not checked on load: no check of hierarchy, status or priority.
- Limits apply when writing through Lif: body at most 1 MiB, title at most 1000 characters, tags follow a pattern (lowercase, starting with a letter, hyphens only between characters).
- One unparsable `.md` file aborts the whole load (`loading <path>: ...`); Lif then returns no items.
- `lif check` reports neither a mismatched file name nor duplicate IDs; with a duplicate ID the file loaded later silently overwrites.
- Obsidian Bases compatibility of the format is unclear; nothing in the code or docs says anything about it.
- Source: FACTS.md §2.1, §2.2, §2.4, §3.2, §6.8, §6.9, §9(g)

### Reference links
- Item file format (`reference/item-file-format`): all fields, values, file names, manual edits.
- Configuration (`reference/configuration`): store path, ID prefix, types, status.
- `lif check` (`reference/commands/lif-check`): what is checked and what is not.
- `lif roadmap` (`reference/commands/lif-roadmap`): Markdown output and link options.
- Glossary (`reference/glossary`): term Liff.
- Source: DESIGN.md D26, D47 and Open (page cut phase 4)
