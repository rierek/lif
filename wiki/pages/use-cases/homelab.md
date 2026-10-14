---
title: "Home lab"
description: "Track servers, services and chores."
---

## Purpose
Shows how a home server setup (servers, services, maintenance, expansion plans) is managed with a lean custom Lif configuration.

## Audience
Home lab operators and self-hosters who want to track maintenance and expansion in files; agents that suggest such a setup.

## Required sections

### Problem
- Describes the situation: many systems, scattered notes, forgotten maintenance, unclear order (backup first, then update).
- Names requirements: filter per system, visible dependencies, find an item quickly.
- Stays scenario text; no claims about Lif behavior.
- Source: CONCEPT.md §5.3 (Content)

### Solution
- Explains: A small custom `types` table (suggested types, e.g. project and task) instead of a ready-made preset; tags name the systems.
- Explains `blocked_by` for ordering: an item waits until its blocker is `completed` or `scrapped`.
- Explains `lif next` and `lif list --ready` as the answer to what is due today.
- Source: FACTS.md §4.3 (`types`, `types_exclusive`), §5.1, §5.2; type names are a suggestion → `TODO: verify`

### Structure
- Shows an example `.lif.yml` with `types_exclusive: true` and two to three custom types; choose ranks so that parents have a lower rank than children.
- Explains: A new type without `rank` gets rank 4; ranks may have gaps.
- Hierarchy sketch (Mermaid, D38): project (e.g. network rebuild) > task; tags per system (`nas`, `router`).
- Example configuration and output of `lif check`: build and verify with `lif init` in phase 5. (TODO: verify)
- Optional: `require_fields_on` requires an extra key for a status (e.g. a note on the maintenance window). (TODO: verify)
- Source: FACTS.md §4.3, §3, §2.3; concrete example config `TODO: verify` — build with `lif init` and `lif check` in phase 5

### Workflow
- Loop: `lif create "…" -t <type> --tag <system>` → `lif create … --blocked-by <id>` → `lif next --tag <system>` → `lif start` → `lif complete --summary "…"`.
- Shows `lif list --tag <system>` and `lif list --is-blocked` for the backlog of waiting items.
- Shows `lif pick` and the zsh widget (Alt+i) to put item IDs on the command line via fuzzy selection.
- Shows `lif graph --format mermaid` for the dependency overview.
- Source: FACTS.md §1.1 (create, next, list, pick, graph, complete), §8.3, §5.5

### Benefits
- Maintenance knowledge lives as text in the repository or directory and can be versioned with Git.
- Ordering becomes visible via `blocked_by`; `lif next` skips blocked items.
- Systems can be filtered by tag, without a fixed hierarchy.
- Source: FACTS.md §5.1, §5.2, §2.2 (Tags)

### Limits
- Recurring duties (monthly updates, certificate renewal): no documented feature. (TODO: verify)
- Lif does not monitor systems and does not trigger reminders. (TODO: verify)
- A link is stored on one side only; `lif check` does not report asymmetry between `blocking` and `blocked_by`.
- Source: FACTS.md §5.1, §5.6; recurrence/monitoring `TODO: verify` — check `lif --help` and config keys in phase 5

### References
- Feature: Dependencies (`features/dependencies`), Flexible planning (`features/project-planning`).
- Reference: Configuration (`reference/configuration`), `lif next`, `lif list`, `lif graph`, `lif pick` (`reference/commands`).
- Source: DESIGN.md D32, D33
