---
title: "Item file format"
description: "Structure of an item file: Markdown with YAML front matter."
---

## Purpose
Canonical description of the data model: how an item is stored as a Markdown file, which fields and values exist, and what applies when editing by hand. This page introduces the term Liff.

## Audience
Users and agents who read item files, edit them by hand, or process them with their own scripts. Readers looking for hierarchy and status rules are pointed to `reference/configuration`.

## Required sections

### What an item file is
- Introduces **Liff** (plural **Liffs**): short for Lif file, the Markdown file that stores an item; field names, commands, JSON and GraphQL keep the word `item`.
- Explains: one item = one Markdown file; layout `---`, YAML front matter, `---`, Markdown body.
- Explains: the first line in the front matter is a YAML comment `# <id>`, which Lif writes when the ID is known.
- Explains: ID and slug are not in the front matter, they are derived from the file name.
- Source: DESIGN.md D47; FACTS.md §2.1

### File name and ID
- File name `<id>--<slug>.md` (double hyphen); without a slug `<id>.md`; if the ID contains `-` or `.` and there is no slug, the file is named `<id>--.md`.
- ID = configured `prefix` plus a random suffix of `id_length` characters (default 4) from the alphabet `0-9a-z`; IDs containing blocked words are regenerated.
- Slug: title lowercased, spaces and underscores to `-`, only letters, digits and `-`, collapsed, truncated to 50 bytes.
- Custom prefix on create (`lif create --prefix`): starts with a letter, contains only letters, digits and single hyphens.
- Accepted on read: `id--slug.md`, `id.slug.md`, legacy `id-slug.md` and `id.md`.
- Link: rename slug, ID or prefix with `lif rename` (generated command page).
- Source: FACTS.md §2.4, §1.1 (`lif rename`)

### Front matter fields
- Table: field, type, default, validation. Order in which Lif writes: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`.
- `title`: required, non-empty, at most 1000 characters on write; `lif create` without a title uses "Untitled".
- `status`, `type`, `priority`: strings from the configured tables; an empty `type` becomes `task` on load, an empty `priority` becomes `normal`; `priority` is not pre-filled on create.
- `tags`: list; each tag lowercase, trimmed, pattern `^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`.
- `created_at`, `updated_at`: RFC 3339, UTC, seconds; if missing on load, Lif falls back to the other field, otherwise to the file modification time.
- `order`: string, fractional index in base62, last digit never `0`; written by `lif order`, `lif create --order` and GraphQL `updateItem(order)`. `parent`: at most one ID; `blocking` and `blocked_by`: lists of IDs.
- Source: FACTS.md §2.1, §2.2

### Allowed values and defaults
- Status table (sort order): `in-progress`, `todo`, `draft`, `completed`, `scrapped`; archive statuses are `completed` and `scrapped`.
- Type table: `milestone` (rank 1), `epic` (2), `feature` (3), `bug` (4), `task` (4).
- Priority table, highest urgency first: `critical`, `high`, `normal`, `low`, `deferred`.
- Defaults: status `todo`, type `task`, no priority written.
- Link to `reference/configuration` for custom statuses, types and priorities and for the hierarchy rules.
- Note: values are checked against the tables on write through Lif (CLI, GraphQL); `update` checks only the fields it sets.
- Source: FACTS.md §2.3

### Relationships
- Explains `parent` (single-parent relationship, rank rule on write, details on `reference/configuration`).
- Explains `blocking` and `blocked_by`: two separate lists; a link is stored only in the file of the changed item, the opposite side is not written.
- Explains: on read both directions are merged; `lif check` reports no asymmetry between `blocking` and `blocked_by`.
- Explains: `lif delete` cleans up incoming references (parent, blocking, blocked_by) in other files.
- Link to `features/dependencies` for the semantics (blocked, ready) and to the generated pages for `lif next`, `lif list`, `lif graph`.
- Source: FACTS.md §2.2, §5.1, §5.4, §5.6

### Extra keys
- Explains: every additional front matter key is preserved and written alphabetically after the typed fields.
- Reserved keys: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`; `--set` and `--unset` reject them.
- Example: `summary:` is an ordinary extra key without special handling; `lif list --where key=value` filters on it (repeatable, AND).
- Explains the interplay with `require_fields_on` and `commit_field` (link to `reference/configuration`).
- Source: FACTS.md §2.1, §2.3, §1.1 (`lif list`)

### Body
- Body = Markdown after the front matter; a trailing line break is not part of the content.
- Limit: at most 1 MiB on write.
- ETag: hash of the file bytes; for optimistic locking with `--if-match` and GraphQL `ifMatch`; `require_if_match: true` makes it mandatory.
- Note: `lif complete --summary` appends a section "## Summary of Changes", `lif scrap --reason` a section "## Reason for Scrapping".
- Source: FACTS.md §2.1, §1.1 (`lif complete`, `lif scrap`), §4.3

### Where files live
- Store directory `.lif/` next to `.lif.yml`; a legacy `.beans/` is used if it exists; `lif.path` overrides.
- New items are stored flat in the store root directory.
- Load: recursively all `*.md`; directories with a leading dot and `attachments/` are skipped; `archive/` is loaded.
- `archive/`: subdirectory; archived items stay loaded and addressable.
- `attachments/<item-id>/`: attachments per item; `lif init` writes `<store>/.gitignore` with `.conversations/`.
- An unreadable `.md` file aborts loading (`loading <path>: ...`).
- Source: FACTS.md §2.4, §5.4

### Editing by hand
- Explains: editing by hand is allowed; Lif loads and reloads without validating hierarchy, status or priority; `lif check` finds those afterwards (since 0.14, verified against the `lif check --help` text of milestone/0.14).
- Table: what `lif check` finds in manual edits (unknown type, unknown status or priority, invalid tags, parent of the wrong type rank, broken links, self-references, cycles, missing required fields as a warning, unknown commit SHAs as a warning, orphaned `attachments/<id>`).
- Table: what `lif check` does not find (duplicate IDs, mismatch between file name and ID, front matter syntax beyond loading).
- Warns: duplicate IDs silently overwrite each other on load; a parse error aborts loading.
- Link to `reference/configuration` (Types, ranks and hierarchy rules) as the canonical statement for D40.
- Source: FACTS.md §3.2, §6.9, §9(d), §9(e); DESIGN.md D40

### Legacy names
- Table: old to new: `.beans/` to `.lif/`, `.beans.yml` to `.lif.yml`, config key `beans:` to `lif:`, file name `id-slug.md` is still read.
- Explains: Lif reads old names in place and never renames on its own; `lif check` reports them under "Legacy names", they never count as a finding.
- Explains: JSON fields `bean_id` and `bean_issues` are now called `item_id` and `item_issues`.
- Source: FACTS.md §2.4, §4.1, §6.6, §6.8

### Annotated example
- Shows a real Liff file with comments on every field (TODO: verify)
- Shows the JSON equivalent of the same item from `lif show --json` (TODO: verify)
- Source: `TODO: verify` — build the example in phase 5 with `lif init` and `lif create` and take over the output of `lif show --json`; FACTS contains no example file
