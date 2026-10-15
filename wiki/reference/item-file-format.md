---
title: "Item file format"
description: "Structure of an item file: Markdown with YAML front matter."
---

Canonical description of the data model. Measured on Lif 0.14, 2026-10-14. Hierarchy and status rules: [Configuration](../configuration/).

## What an item file is

An item is stored as one Markdown file. **Liff** (plural **Liffs**) is short for "Lif file": the file that stores an item. Field names, commands, JSON and GraphQL keep the word `item`.

- Layout: `---`, YAML front matter, `---`, Markdown body.
- A UTF-8 byte order mark (BOM) in front of the first `---` is ignored on read. Lif never writes one.
- The first front matter line is a YAML comment `# <id>`. Lif writes it when the ID is known. It is a record, not the ID: `lif check` compares it with the ID from the file name.
- ID and slug are not front matter fields. Lif derives them from the file name. A file renamed by hand therefore changes the ID and breaks every link to the old one.

## File name and ID

- File name: `<id>--<slug>.md` (double hyphen). Without a slug: `<id>.md`. If the ID contains `-` or `.` and there is no slug, the name is `<id>--.md`.
- ID: the configured `prefix` plus a random suffix of `id_length` characters (default 4) from `0-9a-z`. IDs that contain blocked words are regenerated.
- Slug: title in lower case, spaces and underscores to `-`, only letters, digits and `-`, collapsed, at most 50 bytes. A title change regenerates a slug that came from the title; a slug set by hand stays.
- Prefix on `lif create --prefix`: starts with a letter, only letters, digits and single hyphens.
- Accepted on read: `id--slug.md`, `id.slug.md`, legacy `id-slug.md` and `id.md`.
- Rename a slug, an ID or the project prefix with [`lif rename`](../commands/lif-rename/), which also rewrites the links. A rename by hand is reported by `lif check` (since 0.15).

## Front matter fields

Lif writes the fields in this order.

| Field | Type | Default | Validation |
|---|---|---|---|
| `title` | string | `Untitled` on `lif create` without a title | Required, not empty, at most 1000 characters on write. |
| `status` | string | `todo` | A name from the status table. |
| `type` | string | `task` | A name from the type table. Empty becomes `task` on load. |
| `priority` | string | none written | A name from the priority table. Empty counts as `normal`. |
| `tags` | list | empty | Each tag lower case, trimmed, pattern `^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`. |
| `created_at`, `updated_at` | RFC 3339, UTC, seconds | now | If missing on load: the other field, else the file modification time. |
| `order` | string | none | Fractional index in base62, last digit never `0`. Written by `lif order`, `lif create --order`, GraphQL `updateItem(order)`. |
| `parent` | ID | none | At most one. |
| `blocking`, `blocked_by` | list of IDs | empty | See [Relationships](#relationships). |

## Allowed values and defaults

| Table | Values (sort order) |
|---|---|
| Status | `in-progress`, `todo`, `draft`, `completed`, `scrapped`. Archive statuses: `completed`, `scrapped`. |
| Type (rank) | `milestone` (1), `epic` (2), `feature` (3), `bug` (4), `task` (4) |
| Priority | `critical`, `high`, `normal`, `low`, `deferred` (highest urgency first) |

Defaults: status `todo`, type `task`, no priority written. Custom entries and the hierarchy rules: [Configuration](../configuration/#statuses-types-and-priorities). Lif checks the values against the tables when it writes (CLI, GraphQL). `update` checks only the fields it sets.

## Relationships

- `parent`: a single parent. On write, the rank rule applies ([Configuration](../configuration/#types-ranks-and-hierarchy-rules)).
- `blocking` and `blocked_by`: two separate lists. A link is stored only in the file of the item you changed; the other side is not written. On read, Lif merges both directions. `lif check` does not report a difference between the two sides.
- `lif delete` removes incoming references (`parent`, `blocking`, `blocked_by`) from other files.

Meaning of blocked and ready: [Dependencies](../../features/dependencies/). Commands: [`lif next`](../commands/lif-next/), [`lif list`](../commands/lif-list/), [`lif graph`](../commands/lif-graph/).

## Extra keys

Every other front matter key is kept and written alphabetically after the fields above. The reserved keys are `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`; `--set` and `--unset` reject them.

`summary:` is an ordinary extra key with no special handling. `lif list --where key=value` filters on it; repeat the flag for AND. Extra keys are not searchable ([Search syntax](../search-syntax/)). `require_fields_on` and `commit_field` use extra keys: [Configuration](../configuration/#status-dependent-required-fields).

An extra key must not hold `.nan`, `.inf` or `-.inf`, alone or inside a list or map: JSON cannot carry them. Loading stops with `loading <path>: parsing front matter: key "<key>" holds NaN: ...`. Quote the value (`".nan"`) to keep it as text.

## Body

- The body is the Markdown after the front matter. A trailing line break is not part of it. At most 1 MiB on write.
- The ETag is a hash of the file bytes. It supports optimistic locking with `--if-match` and GraphQL `ifMatch`. `require_if_match: true` makes it mandatory. Since 0.15 the update holds a lock on the item file from the check to the write, so two simultaneous writers with the same ETag cannot both succeed. On a file system without working file locks (some NFS or FUSE mounts) an update with `--if-match` fails with a locking error.
- `lif complete --summary` appends a section `## Summary of Changes`. `lif scrap --reason` appends `## Reason for Scrapping`.

## Where files live

- The store directory is `.lif/` next to `.lif.yml`. A legacy `.beans/` is used if it exists. `lif.path` overrides both.
- New items go flat into the store root.
- On load Lif reads all `*.md` files recursively. It skips directories with a leading dot and `attachments/`. `archive/` is loaded: archived items stay addressable.
- `attachments/<item-id>/` holds attachments per item. `lif init` writes `<store>/.gitignore` with `.conversations/`.
- An unreadable `.md` file stops loading with `loading <path>: ...`.

## Editing by hand

Editing files by hand is allowed. Lif loads and reloads without validating hierarchy, status or priority. `lif check` finds many problems afterwards (since 0.14).

| `lif check` finds | `lif check` does not find |
|---|---|
| Unknown type | Duplicate IDs |
| Unknown status or priority | Front matter syntax errors beyond loading |
| Invalid tags | |
| File name that gives another ID than the file records in `# <id>` (a hand rename, since 0.15) | |
| Parent of the wrong type rank | |
| Broken links, self-references, cycles | |
| Missing required fields (warning), unknown commit SHAs (warning) | |
| Orphaned `attachments/<id>` | |

Example, status `bogus` written by hand:

```text
✗ item <id> carries unknown status 'bogus'
1 issue found
```

Example, a file renamed by hand from `ex-zyo0--write-docs.md` to `ex-new1--write-docs.md`, with the item `ex-3ip9` blocked by `ex-zyo0`:

```text
✗ item ex-new1: file name ex-new1--write-docs.md gives ID ex-new1 but the file records ID ex-zyo0 (renamed by hand? rename it back; lif rename changes an ID together with its links); links to ex-zyo0 dangle: ex-3ip9 blocked_by
✗ ex-3ip9: broken link blocked_by:ex-zyo0
2 issues found
```

Do not remove that link by hand or with `lif check --fix`: `--fix` keeps it and prints `kept broken link blocked_by:ex-zyo0`. Rename the file back, or run `lif rename ex-new1 ex-zyo0`, and the link works again.

:::caution
Duplicate IDs silently overwrite each other on load. A parse error stops loading. Run `lif check` after hand edits.
:::

## Legacy names

| Old | New |
|---|---|
| `.beans/` | `.lif/` |
| `.beans.yml` | `.lif.yml` |
| config key `beans:` | `lif:` |
| file name `id-slug.md` | still read |

Lif reads old names in place and never renames them by itself. `lif check` lists them under "Legacy names"; they never count as a finding. In JSON the fields `bean_id` and `bean_issues` are now `item_id` and `item_issues`.

## Annotated example

A file made with `lif create "Write notes" -t task --parent <epic> -p high --tag docs`:

```markdown
---
# f014j-ix1n                      # ID comment, derived from the file name
title: Write notes                # required
status: todo                      # from the status table
type: task                        # from the type table
priority: high
tags:
    - docs                        # lower case, hyphenated
created_at: 2026-10-14T18:54:05Z  # UTC
updated_at: 2026-10-14T18:54:05Z
parent: f014j-uzgy                # one parent, rank must be higher
---

Body in Markdown.
```

(The trailing `#` comments were added for this page; the body line too.) File name: `f014j-ix1n--write-notes.md`. The same item from `lif show f014j-ix1n --json`; see [JSON output](../json-output/):

```json
{
  "id": "f014j-ix1n",
  "slug": "write-notes",
  "path": "f014j-ix1n--write-notes.md",
  "title": "Write notes",
  "status": "todo",
  "type": "task",
  "priority": "high",
  "tags": ["docs"],
  "created_at": "2026-10-14T18:54:05Z",
  "updated_at": "2026-10-14T18:54:05Z",
  "parent": "f014j-uzgy",
  "etag": "3c75a4149f169b2a"
}
```
