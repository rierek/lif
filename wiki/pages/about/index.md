---
title: "About"
description: "What Lif is, the philosophy behind it and where it comes from."
---

## Purpose
One page that explains what Lif is, the attitude behind it and where the project comes from. Philosophy and Origin are sections of this page, not separate pages.

## Audience
Readers who want to understand why Lif is built this way and how it relates to Beans; people who check origin and license; users with legacy data (`.beans`).

## Required sections

### What Lif is
- Short description: a file-based issue tracker for developers, product teams and AI coding agents; lightweight, Markdown-native work management for humans, AI agents and automation.
- Each piece of work is an item: one Markdown file with YAML front matter, stored beside the code and versioned with Git.
- Three entry points to one truth: Markdown files, CLI with `lif prime` and help, JSON and GraphQL.
- Lif is not a hosted project-management platform; it stays a small, inspectable data and workflow layer.
- Links to Lif in brief (`overview`) and Features (`features`) for details.
- Source: `README.md` (intro, sections "Why Lif" and "Feature overview"); FACTS.md §2.1, §1.1 (prime), §7

### Philosophy
- Adapts to existing ways of working instead of prescribing one: types, statuses, priorities, display rules and lifecycle policies are configurable per repository; terminology is configurable.
- Local, human-readable, diff-friendly, LLM-friendly: no service or proprietary export is needed to read active or archived work; task changes are reviewed with the Git tools used for code.
- Presets are examples, not a workflow engine; presets use up to four ranks, custom types can go deeper (D39). The trial sentence "four fixed levels" is outdated.
- Files are the source of truth; several surfaces (CLI, TUI, web) work on the same files.
- Source: `README.md` (sections "Why Lif", "Feature overview", "Choose a project profile"); FACTS.md §9(a), §9(b), §3; DESIGN.md D39

### Origin
- Explains: Lif originated as a fork of `hmans/beans`, the agentic-first issue tracker by Hendrik Mans; it is now maintained independently at `xRiErOS/lif` and is an actively maintained continuation, not a hostile fork.
- Names what comes from the upstream: the original idea, the data model (one Markdown file per item with YAML front matter), the initial command surface and the initial GraphQL query engine.
- Names what Lif added: project profiles, scoped roadmaps, order and rename workflows, completion policies, structured JSON and error output, worktree-aware storage, the integrated terminal UI (`lif tui`, formerly the companion `beans-tui`).
- Names the license Apache-2.0: upstream and Lif use the same license, the `LICENSE` file is unchanged; carried-over work stays attributed to `hmans/beans` and Hendrik Mans.
- Explains why `bean` became `item` and `beans` became Lif: the project was first published as beans like the upstream; the domain object is now an item, the type names `milestone`, `epic`, `feature`, `bug`, `task` are unchanged.
- Tells readers to file issues against `xRiErOS/lif` and to cite `hmans/beans` and Hendrik Mans for the original concept.
- Source: `wiki/fork-lineage.md` (sections "Origin", "Names", "License and attribution", "Why this fork exists", "What has changed so far", "Related projects", "How to reference Lif"); `README.md` (intro, section "Acknowledgements"); `LICENSE`

### Legacy compatibility
- Shows what keeps working from the Beans era: config `.beans.yml` (read in place, never renamed), store `.beans/`, environment variable `BEANS_PATH`. An existing id prefix such as `beans-` is plain data and is not a legacy name in code (FACTS.md §6.6).
- Explains: `.lif.yml` and `.beans.yml` in the same directory is an error; `LIF_PATH` and `BEANS_PATH` at the same time is an error.
- Explains: `lif check` reports legacy names as a note (`Legacy names`), they never count as errors.
- Links to the releases pages for the migration (`releases`).
- Source: FACTS.md §2.4, §4.1, §4.2, §6.6

### Where to go next
- Links to Releases (`releases`), Contribute (`contribute`) and Lif in brief (`overview`).
- Source: DESIGN.md D35, D46, D48
