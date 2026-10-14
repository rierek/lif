---
title: "Item file format"
description: "Structure of a Lif item: Markdown with YAML front matter."
sidebar:
  order: 1
---

:::caution[Raw draft]
Topic outline with bullet points. Source: `wiki/data-model.md`. Not final prose.
:::

- One item = one Markdown file. YAML front matter + Markdown body.
- Filename is the only source of truth for ID and slug: `<id>--<slug>.md`.
- Front matter fields: `title` (required), `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`, extra keys.
- Defaults: statuses `draft/todo/in-progress/completed/scrapped`; priorities `critical/high/normal/low/deferred`.
- IDs: NanoID, lowercase alphanumeric, length configurable, optional project prefix. Short suffix accepted when prefix set.
- Relationships: `parent`, `blocking`, `blocked_by`. Cycles and self-references caught by `lif check`. A `parent` must be of a higher-ranked type than the item; `lif check` reports a violation.
- Manual ordering: fractional-index string in `order`; `lif order` places siblings.
- Hand-editing: allowed for fields and body. Never rename IDs by hand, use `lif rename`. Run `lif check` after edits: it reports unknown `status`/`priority`, invalid tags and wrong-rank parents (exit 1).
- Legacy: `.beans` directories and `beans-` prefixes still read (see upgrade guide).
- TODO: annotated example file, JSON mirror of the same item.
