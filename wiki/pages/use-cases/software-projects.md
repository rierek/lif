---
title: "Software project"
description: "Plan milestones, epics and features beside the code."
---

## Purpose
Shows how a software project is planned and tracked with the preset `classic`, with items in the same repository as the code.

## Audience
Developers, teams and agents who want to keep roadmap, dependencies and progress beside the code.

## Required sections

### Problem
- Describes the situation: planning lives in an external tracker, code and plan drift apart, agents cannot see the plan.
- Names requirements: hierarchy from milestone to task, dependencies, progress at a glance.
- Stays scenario text; no claims about Lif behavior.
- Source: CONCEPT.md §5.4 (Content)

### Solution
- Explains: preset `classic` models milestone > epic > feature > bug/task; every element is an item in the store.
- Explains: `blocked_by`/`blocking` model ordering; `lif next` returns the next ready item.
- Explains: `lif roadmap`, `lif milestones` and `lif progress` show plan and status.
- Source: FACTS.md §9(b), §5.1, §5.2, §1.1 (roadmap, milestones, progress)

### Structure
- Shows preset `classic`: milestone (rank 1), epic (2), feature (3), bug and task (4); parents must have a lower rank than children.
- Hierarchy diagram (Mermaid, D38) with valid parents: feature under epic or milestone, bug/task under milestone, epic or feature.
- Explains: four levels are a property of the built-in rank scale, not an enforced maximum; custom types can go deeper (D39).
- Shows multiple Git worktrees with `lif.anchor: repo-root`: all share one store.
- Example output of `lif roadmap`: build in phase 5 with `lif init --profile classic`. (TODO: verify)
- Source: FACTS.md §3, §9(a), §9(b), §4.2; example output `TODO: verify` — generate in phase 5

### Workflow
- Plan: `lif create "…" -t milestone`, `lif create "…" -t epic --parent <id>`, `lif create "…" -t feature --parent <id>`.
- Dependencies: `lif update <id> --blocked-by <id>`; display with `lif graph --format mermaid`.
- Work: `lif next --type task` → `lif start <id>` → `lif complete <id> --summary "…"`.
- Overview: `lif roadmap --depth 2`, `lif milestones`, `lif progress <milestone-id>`; ordering with `lif order <id> --after <id>`.
- Acceptance criteria as a checklist in the item body: see feature Flexible planning (`features/project-planning`). (TODO: verify)
- Source: FACTS.md §1.1 (create, update, graph, next, start, complete, roadmap, milestones, progress, order); acceptance criteria `TODO: verify` — no FACTS section, check phase 5 against `wiki/` and `lif prime`

### Benefits
- Plan and code live in the same repository and travel with branches and commits.
- Progress per container: percent done = completed / (total − scrapped).
- Ordering is derived from dependencies; blocked children count as blocked when the parent is blocked.
- Agents read the same plan (`lif prime`, `--json`).
- Source: FACTS.md §1.1 (progress, prime), §5.1, §2.4, §4.2

### Limits
- `lif check` does not check parent/child ranks, status values, priority values or tags; manually edited files bypass the hierarchy check (D40).
- Changing the type alone does not trigger a hierarchy check (known finding).
- `lif next` can also return containers (milestone, epic, feature).
- No pre-commit hook is shipped; the Git-native feature page only shows a snippet (D41).
- Source: FACTS.md §3.1, §3.2, §5.2, §6.9, §9(f); DESIGN.md D40, D41, D43

### References
- Features: Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Files in your repo (`features/files-in-your-repo`).
- Reference: Configuration (`reference/configuration`), `lif roadmap`, `lif milestones`, `lif progress`, `lif order`, `lif graph` (`reference/commands`).
- Use case: Release management (`use-cases/release-management`).
- Source: DESIGN.md D32, D39
