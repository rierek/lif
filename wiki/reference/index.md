---
title: "Reference"
description: "Look-up pages for commands, formats, configuration and interfaces."
---

This chapter is for exact answers: a flag, a field, a configuration key, an error code. It looks things up; it does not tell stories. Every rule has exactly one canonical page, and other chapters link here instead of repeating it.

## How the Reference is organised

- [Commands](#commands)
- [Data and formats](#data-and-formats)
- [Configuration](#configuration)
- [Interfaces](#interfaces)
- [Glossary](#glossary)

There is no separate Data model, Hierarchy, Presets or Dependencies page. The data model is in [Item file format](item-file-format/). Hierarchy and presets are in [Configuration](configuration/). Dependencies are described in [Dependencies](../features/dependencies/) and on the generated pages for [`lif next`](commands/lif-next/), [`lif list`](commands/lif-list/) and [`lif graph`](commands/lif-graph/).

## Commands

The [Command overview](commands/) is the hand-written introduction. Every visible command has its own generated page. The four `completion` shells share one page. Hidden commands have no page, and Cobra's built-in `help` has none either.

The pages are alphabetical, without categories, and are generated from the command tree of the `lif` binary; do not edit them by hand. The overview shows how the commands relate to their parents. `lif --help` lists 28 commands, 27 of them have a page (Lif 0.14).

## Data and formats

- [Item file format](item-file-format/): use it when you change a file by hand. Structure of the Markdown file, fields, file names.
- [JSON output](json-output/): use it when a script reads the output. Payloads of `--json`, the error object, exit codes.
- [Search syntax](search-syntax/): use it for full-text search with `lif list --search` and the GraphQL filter `ItemFilter.search`.

## Configuration

[Configuration](configuration/) covers `.lif.yml`, how Lif finds it, keys and defaults. It is also the canonical page for types, ranks, hierarchy rules and the shipped presets (`classic`, `todo`, `simple`, `complex`). It answers: Which types exist? What does `lif check` check? What does Lif enforce on writes?

For the task-oriented view (store location, `LIF_PATH`, `--lif-path`, anchor, several projects) see [Store management](../overview/store-management/). The precedence table stays on the Configuration page.

## Interfaces

- [GraphQL](graphql/): schema, queries, mutations. Call it with `lif graphql` or over HTTP.
- [Web server](web-server/): the separate binary `lif-serve`. Read its security notes before you start it.
- TUI: five pages, start with [Start and repositories](tui/start-and-repositories/). Then [Views](tui/views/), [Editing items](tui/editing-items/), [Key bindings](tui/key-bindings/) and [Settings and themes](tui/settings-and-themes/).

The web server is its own binary. `lif serve` in the `lif` binary is a hidden placeholder and is not documented.

## Glossary

The [Glossary](glossary/) gives short definitions and, for each term, the page with the details. The terms follow the code: item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset. "Level" is only a prose synonym for rank. The Markdown file of an item is called a Liff; see [Item file format](item-file-format/) and the glossary.

## Conventions

- **Item IDs:** the full ID or the short ID without the configured prefix, for example `ix1n` for `f014j-ix1n`.
- **`--json`:** many commands have this flag; see [JSON output](json-output/). `lif graph` uses `--format json`.
- **Exit codes:** 0 on success, 1 on error. `lif check` returns 1 on at least one finding. There is no exit code 2, except from `lif run`, which returns the exit code of the failing step.
- **Global flags:** `--config` and `--lif-path`. See the [Command overview](commands/) and [Store resolution](configuration/#store-resolution).
- **Formatting:** commands and field names are set in code formatting and stay English.

## Where narrative lives

Features explain the benefit and the flow. Use cases show scenarios. Overview explains terms in plain words. Overview and Features link here; they do not repeat the rules. Start with [Concepts](../overview/concepts/), [Features](../features/) and [Use cases](../use-cases/).
