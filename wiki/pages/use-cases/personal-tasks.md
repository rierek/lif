---
title: "Personal tasks"
description: "Run your own to-do list as plain files."
---

## Purpose
Shows how one person runs a to-do list with Lif: flat with the preset `todo`, or with a small hierarchy via `simple`.

## Audience
Individuals who want a local, text-based task list without a service, and agents that set up such a configuration for users.

## Required sections

### Problem
- Describes the situation: tasks are scattered across note apps, paper slips and emails; priorities and the next step are unclear.
- Names the requirements: local, readable, no account, extensible later.
- Stays scenario text; no claims about Lif behavior.
- Source: CONCEPT.md §5.2 (Content)

### Solution
- Explains: Each task is an item in a Markdown file in the store; status, priority and tags organize it.
- Names the two paths: preset `todo` (one type `task`, no hierarchy) or `simple` (small hierarchy).
- Explains that `lif next` names the most important ready task.
- Source: FACTS.md §9(b), §2.1, §5.2

### Structure
- Shows preset `todo`: single type `task` at rank 4; grouping via tags (e.g. areas like `home`, `work`).
- Shows preset `simple`: milestone (rank 1), bucket (rank 1, `roadmap: false`), epic and feature (rank 2), bug and task (rank 4); three levels.
- Hierarchy diagram (Mermaid, D38) for `simple`; one line is enough for `todo`.
- Explains tag format: lowercase, starts with a letter, hyphens only between alphanumerics.
- Example output of `lif init --profile todo` and `lif check`: build and verify in phase 5. (TODO: verify)
- Source: FACTS.md §9(b), §3, §2.2 (Tags); example output `TODO: verify` — generate with `lif init --profile todo` in phase 5

### Workflow
- Command loop: `lif create "…" --tag <tag> -p <priority>` → `lif next` → `lif start <id>` → `lif complete <id>`.
- Shows `lif list --ready` and `lif list --tag <tag>` for overviews.
- Shows priorities (`critical`, `high`, `normal`, `low`, `deferred`) and ordering with `lif order`.
- Shows `lif scrap <id> --reason "…"` for discarded tasks and `lif archive` for cleanup.
- Shows short IDs: item ID arguments also accept the ID without prefix.
- Source: FACTS.md §1.1 (create, next, start, complete, list, order, scrap, archive), §2.3, §1.0

### Benefits
- Tasks are plain files: readable, searchable, versionable.
- One priority and one status are enough for `lif next`; no extra tool.
- Grows with you: from `todo` to `simple` or custom `types`, without a data format change. (TODO: verify)
- Source: FACTS.md §2.1, §5.2; growth claim `TODO: verify` — try switching the preset with existing items in phase 5

### Limits
- No typed due date: `created_at` and `updated_at` are managed; custom keys like `due` are only extra front matter keys (`--set`, `lif list --where`).
- Recurring tasks and reminders: support not documented. (TODO: verify)
- `lif next` can also return container items (no type exclusion).
- Source: FACTS.md §2.1, §2.2, §5.2; recurrence/reminder `TODO: verify` — check `lif prime`/help and config keys in phase 5

### References
- Feature: Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`).
- Reference: Configuration (`reference/configuration`) for presets; commands `lif create`, `lif next`, `lif start`, `lif complete`, `lif archive` (`reference/commands`).
- Overview: First steps (`overview/first-steps`).
- Source: DESIGN.md D32, D33, D39
