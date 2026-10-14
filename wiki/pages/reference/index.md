---
title: "Reference"
description: "Look-up pages for commands, formats, configuration and interfaces."
---

## Purpose
Entry page of the Reference chapter. It shows which look-up pages exist, where each rule is canonical and which conventions hold for all pages.

## Audience
People and agents who look for an exact answer (flag, field, configuration key, error code) and want to know which page holds it. Anyone looking for explanatory text is sent to Features and Use cases.

## Required sections

### How the Reference is organised
- Explains in two sentences: Reference = look up, not narrate; every rule has exactly one canonical page.
- Shows the structure as a list: Commands, Data and formats, Configuration, Interfaces, Glossary.
- Names the page-cut decision: no separate Data model, Hierarchy, Presets or Dependencies reference; the data model is in `reference/item-file-format`, hierarchy and presets are in `reference/configuration`.
- Names where dependencies are described canonically: `features/dependencies` plus the generated pages for `lif next`, `lif list`, `lif graph`.
- Source: DESIGN.md D26, D45 and Open (page cut, phase 4)

### Commands
- Links to `reference/commands/index` (Command overview) as the hand-written introduction.
- Explains: every visible command has its own generated page; the four `completion` shells share one page; hidden commands are not documented; Cobra's built-in `help` has no page (D57).
- Explains: pages are ordered alphabetically, without categories; pages are generated from the Cobra tree and not edited by hand; the overview page shows how the commands relate to their parents.
- Names the number of commands of the `lif` binary (28 per `lif --help`, 27 with a page; check the numbers against the generator output in phase 5).
- Source: DESIGN.md D24, D25, D55, D57; FACTS.md §1.1 (Command count)

### Data and formats
- Links to `reference/item-file-format` (Item file format): structure of the Markdown file, fields, file names, manual editing.
- Links to `reference/json-output` (JSON output): output of the commands with `--json`, error payload, exit codes.
- Links to `reference/search-syntax` (Search syntax): query syntax for `lif list --search` and GraphQL `ItemFilter.search`.
- One sentence per page on when to use it (change a file by hand, read the output by machine, full-text search).
- Source: DESIGN.md D26; FACTS.md §1.1 (`lif list`), §2.1, §6.8

### Configuration
- Links to `reference/configuration` (Configuration): `.lif.yml`, discovery, keys and defaults.
- States explicitly: this is also the canonical page for types, ranks, hierarchy rules and the shipped presets (`classic`, `todo`, `simple`, `complex`).
- Shows which question it answers there (Which types exist? What does `lif check` check? What does Lif enforce on writes?).
- Points to Store management (`overview/store-management`) for the task-oriented view: store location, `LIF_PATH`, `--lif-path`, anchor and several projects. The page lives in Overview (D56); the precedence table stays here.
- Source: DESIGN.md D39, D40, D56 and Open (page cut, phase 4); FACTS.md §4.1, §9(b)

### Interfaces
- Links to `reference/graphql` (GraphQL): schema, queries, mutations; call it with `lif graphql` or over HTTP.
- Links to `reference/web-server` (Web server): separate binary `lif-serve`; low priority.
- Links to the TUI subsection `reference/tui/*`: five pages, start with `lif tui`.
- Note: the web server is its own binary `lif-serve`; `lif serve` in `lif` is a hidden placeholder and is not documented.
- Source: DESIGN.md D27, D28, D29, D44; FACTS.md §1.1 (`lif serve` stub, `lif-serve`, `lif tui`)

### Glossary
- Links to `reference/glossary` (Glossary): short definitions, and for each term the page with the details.
- Explains that the terms follow the code (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset) and that "level" is only a prose synonym for rank.
- Mentions the name Liff for the Markdown file of an item; it is introduced on `reference/item-file-format` and `reference/glossary`.
- Source: DESIGN.md D45, D47

### Conventions
- Item IDs: full ID or short ID without the configured prefix (point to an example in phase 5).
- `--json`: many commands have the flag; details on `reference/json-output`; `lif graph` uses `--format json` instead.
- Exit codes: 0 on success, 1 on error; `lif check` returns 1 on at least one finding; there is no exit code 2.
- Global flags `--config` and `--lif-path`; link to `reference/commands/index` and `reference/configuration` (Store resolution).
- Examples in the Reference chapter: commands and field names are set in code formatting and stay English.
- Source: FACTS.md §1.0, §1.1 (`lif graph`), §6.11

### Where narrative lives
- Explains the division of labour: Features explain benefit and flow (pattern What, Why, How it looks, Limits, Reference links); Use cases show scenarios; Overview explains terms in plain words.
- Links to `overview/concepts` (Concepts), `features/index` (Features) and `use-cases/index` (Use cases).
- Says: Overview and Features link here, they do not repeat the rules.
- Source: DESIGN.md D30, D32, D33 and Open (page cut, phase 4)
