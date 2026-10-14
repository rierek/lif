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
- Short description: lightweight, Markdown-native work management for humans, AI agents and automation.
- Three entry points to one truth: Markdown files, CLI with `lif prime` and help, JSON and GraphQL.
- Links to Lif in brief (`overview`) and Features (`features`) for details.
- Source: FACTS.md §2.1, §1.1 (prime), §7; self-description and positioning `TODO: verify` — check `README.md` in the checkout

### Philosophy
- Adapts to existing ways of working instead of prescribing one; terminology is configurable.
- Local, human-readable, diff-friendly, LLM-friendly.
- Presets are examples, not a workflow engine; presets use up to four ranks, custom types can go deeper (D39). The trial sentence "four fixed levels" is outdated.
- Files are the source of truth; several surfaces (CLI, TUI, web) work on the same files.
- Source: FACTS.md §9(a), §9(b), §3; DESIGN.md D39; attitude and design intent `TODO: verify` — no FACTS; check texts from `README.md` and `wiki/` in the checkout

### Origin
- Explains: Lif is the continuation of the fork `xRiErOS/beans` of `hmans/beans` by Hendrik Mans. (TODO: verify)
- Names what comes from the upstream: idea, Markdown-plus-YAML model, first commands, first GraphQL engine. (TODO: verify)
- Names the license Apache-2.0 and attribution; the fork is independent and does not speak for the upstream project. (TODO: verify)
- Explains why `bean` became `item` and `beans` became Lif. (TODO: verify)
- Source: `TODO: verify` — no FACTS; check `wiki/fork-lineage.md`, `LICENSE` and `README.md` in the checkout

### Legacy compatibility
- Shows what keeps working from the Beans era: config `.beans.yml` (read-only, never renamed), store `.beans/`, prefix `beans-`, environment variable `BEANS_PATH`.
- Explains: `.lif.yml` and `.beans.yml` in the same directory is an error; `LIF_PATH` and `BEANS_PATH` at the same time is an error.
- Explains: `lif check` reports legacy names as a note (`Legacy names`), they never count as errors.
- Links to the releases pages for the migration (`releases`).
- Source: FACTS.md §2.4, §4.1, §4.2, §6.6

### Where to go next
- Links to Releases (`releases`), Contribute (`contribute`) and Lif in brief (`overview`).
- Source: DESIGN.md D35, D46, D48
