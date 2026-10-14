---
title: "Glossary"
description: "Short definitions of the terms Lif uses."
---

## Purpose
Short definitions of the terms Lif uses in commands, files, JSON and documentation. Every entry names the page that owns the detail. Together with `reference/item-file-format`, this page introduces the term Liff.

## Audience
New users who look up a term, and agents that should use the same words as the code. Anyone coming from earlier versions (`beans`) finds the mapping from old to new names.

## Required sections

### How to read the glossary
- Explains: the terms follow the code (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset, store).
- Explains: "level" is only a synonym for rank in running text, never a field name.
- Explains Liff: the name for the Markdown file that stores an item; field names, commands, JSON and GraphQL keep `item`.
- Explains the entry format: term, one line of definition, page with details in backticks; groups: Core terms, Store and files, Planning terms, Interface terms, Legacy terms.
- Source: DESIGN.md D45, D47

### Core terms
- item, Liff: item = unit of work (task, feature, epic, milestone ...); Liff = the Markdown file for it. Details: `reference/item-file-format`.
- type, rank: type names the kind of item; `rank` is an integer that decides which type may sit under which (child rank strictly greater than parent rank). Details: `reference/configuration` (Types, ranks and hierarchy rules).
- status, priority: status = state of the work (`in-progress`, `todo`, `draft`, `completed`, `scrapped`); priority = urgency (`critical` to `deferred`). Details: `reference/item-file-format` (Allowed values and defaults), `reference/configuration`.
- tag, parent: tag = label in lower case with hyphens; `parent` = the ID of the one item above. Details: `reference/item-file-format`.
- blocking, blocked_by: two lists of IDs; `blocking` names what this item blocks, `blocked_by` names what blocks it; stored on one side only. Details: `features/dependencies`, `reference/item-file-format` (Relationships).
- preset: a type table written out once into `.lif.yml` by `lif init --profile` (`classic`, `todo`, `simple`, `complex`). Details: `reference/configuration` (Presets).
- Source: FACTS.md §2.1, §2.2, §2.3, §3, §5.1, §9(b); DESIGN.md D45, D47

### Store and files
- store, archive: store = the directory with the Liffs (default `.lif/`); `archive/` = subdirectory for items with an archive status, they stay loaded and addressable. Details: `reference/item-file-format` (Where files live), `overview/store-management`.
- store location: where Lif looks for the store directory: `--lif-path`, then the config file (`lif.path` or the default directory next to it), then `LIF_PATH`, then `.lif/` in the current directory; `lif path` prints the result. Details: `reference/configuration` (Store resolution), `overview/store-management`.
- attachments: `attachments/<item-id>/` with evidence per item. Details: `reference/item-file-format` (Where files live).
- ID, prefix, slug: ID = prefix plus random suffix; slug = short form of the title in the file name `<id>--<slug>.md`; short ID = ID without the prefix. Details: `reference/item-file-format` (File name and ID), `overview/store-management` (ID prefix).
- anchor: `lif.anchor: repo-root` lets all Git worktrees of a repository share one store. Details: `reference/configuration` (Store resolution), `overview/store-management`.
- ETag: hash of the file bytes for `--if-match` and GraphQL `ifMatch`. Details: `reference/item-file-format` (Body).
- Source: FACTS.md §2.1, §2.4, §4.2, §5.4; DESIGN.md D56

### Planning terms
- container, leaf: container = item on rank 1 to 3, leaf = item on rank 4 (built-in scale); custom types can have other ranks. Details: `reference/configuration` (Types, ranks and hierarchy rules).
- order: the manual order key (`order` in the front matter, a base62 fractional index) that places an item among its siblings; set with `lif order`. Details: `overview/concepts` (Order), `reference/item-file-format`.
- ready: not blocked (directly or through an ancestor), status not `in-progress`, `draft` or an archive status, no ancestor `completed` or `scrapped`. Details: `features/dependencies`, `reference/commands/index` (generated pages `lif next`, `lif list`).
- blocked, explicit and implicit: explicit = has active blockers itself; implicit = an ancestor has active blockers; active means status not `completed` or `scrapped`. Details: `features/dependencies`.
- roadmap: tree view of milestones, epics and children; types with `roadmap: false` do not appear. Details: `lif roadmap` (generated page), `reference/configuration` (Presets).
- Source: FACTS.md §2.2, §3, §5.1, §5.2; DESIGN.md D52

### Interface terms
- CLI, `lif prime`: CLI = the binary `lif`; `lif prime` prints a prompt that introduces a coding agent to the use of Lif. Details: `reference/commands/index`, `overview/agent-onboarding`.
- TUI: terminal interface, started with `lif tui`. Details: `reference/tui/*`.
- `lif-serve`: separate binary for the web server. Details: `reference/web-server`.
- GraphQL: query and mutation interface; in process with `lif graphql` (alias `lif query`) or over HTTP through `lif-serve`. Details: `reference/graphql`.
- Source: FACTS.md §1.0, §1.1 (`lif prime`, `lif tui`, `lif-serve`, `lif graphql`), §7.5

### Legacy terms
- bean: former name for item; today item or Liff (D47). The JSON fields `bean_id` and `bean_issues` are now `item_id` and `item_issues`. Details: `reference/json-schemas`.
- `.beans/`, `.beans.yml`, `beans:`: old store folder, old config file, old config key; they are read in place and never renamed; `lif check` reports them under "Legacy names". Details: `reference/configuration`.
- `BEANS_PATH`: old environment variable; read when `LIF_PATH` is not set; setting both is an error. Details: `reference/configuration` (Store resolution), `overview/store-management`.
- `beans-` prefix: old ID prefix; mapping to `lif rename --prefix` (TODO: verify)
- Source: FACTS.md §2.4, §4.1, §4.2, §6.6, §6.8; `TODO: verify` for the `beans-` prefix: not backed by FACTS, check `lif rename --help` and the migration notes in the repository
