---
title: "Flexible planning"
description: "Types, statuses and hierarchy presets that adapt to how you work."
---

## Purpose
Shows how types, ranks, status, priorities and parents model the structure of a project and which rules Lif enforces on write. Contains the how-to guide for Acceptance criteria and states honestly where the rules end.

## Audience
People and agents who choose or adapt a planning structure (simple to-do list up to deep hierarchy). They want to know which presets exist, what is enforced and what is not.

## Required sections

### What
- An item has `type` (with rank), `status`, `priority`, `tags` and at most one `parent`; a parent is valid if the child's rank is strictly greater than the parent's.
- Built-in types: `milestone` (rank 1), `epic` (2), `feature` (3), `bug` and `task` (4); status: `in-progress`, `todo`, `draft`, `completed`, `scrapped`; priorities: `critical`, `high`, `normal`, `low`, `deferred`.
- Custom types, status and priorities in `.lif.yml` (`types`, `statuses`, `priorities`) override built-in entries or extend them; `types_exclusive` replaces the type table completely.
- Four presets via `lif init --profile`: `classic`, `todo`, `simple`, `complex`; the preset is written out into the configuration on creation, not referenced by name.
- Planning commands: `lif roadmap`, `lif milestones`, `lif progress`, `lif order` (manual ordering via `order`).
- Source: FACTS.md §2.2, §2.3, §3, §4.3, §9(b)

### Why
- Lif adapts to different project structures and enforces the configured rules on write.
- Instead of a fixed template: presets are examples, the structure belongs to the project (D39).
- Examples for different models: single-level list, milestone down to task, release model.
- Source: CONCEPT.md §4.2; DESIGN.md D39; FACTS.md §3.1, §9(c)

### How it looks
- Mermaid diagrams of the four presets from the definitions: `classic` (milestone 1, epic 2, feature 3, bug/task 4), `todo` (only `task`), `simple` (milestone, bucket, epic, feature, bug, task), `complex` (release, bucket, feature, improvement, chore, story, bug, task).
- Note: `bucket` is hidden from the roadmap and the milestone list with `roadmap: false`.
- Output of `lif roadmap` (for example with `--tags`, `--depth`) as a real excerpt; set it up in phase 5 with `lif init --profile` and record it (TODO: verify).
- Show a rule violation: task under task is rejected with `task items can only have milestone, epic, or feature as parent, not task`.
- Rebuild the preset diagrams in phase 5 with `lif init --profile <name>` and check them (TODO: verify)
- Source: FACTS.md §3, §3.1, §9(b); (TODO: verify) for roadmap output and diagrams: run in phase 5 against `lif init --profile` and `lif roadmap`

### Acceptance criteria
- How-to for adding: criteria as a checklist in the item body, each criterion observable; format and template per type (leaf versus container) (TODO: verify)
- How-to for checking: read the criteria and tick them off before `lif complete` runs; evidence in the item or under `attachments/<item-id>/`.
- What Lif enforces on completion: status-dependent required fields via `lif.require_fields_on` (status with a list of extra keys); if a value is missing, create and update fail with a policy violation.
- `lif check` reports missing required fields as a warning; `lif check --strict` counts them as a problem (exit 1).
- `lif complete --summary` appends a "Summary of Changes" section; `--commit` records the SHA in the configured field (default `commit`).
- To clarify: checklists in the body are a convention; FACTS shows no code that evaluates them (TODO: verify)
- Source: FACTS.md §2.3, §6.5, §1.1 (`lif complete`); (TODO: verify) for checklist format and "convention or feature": check `internal/commands/prompt.tmpl`, `complete.go` and `lif prime`

### Limits
- The rank rule applies on write through Lif (CLI, GraphQL, TUI, Web UI), not on manual edits of the file.
- `lif check` checks neither the rank of parent and child nor status values, priority values or tags; it only reports unknown type, broken links, self-references, cycles, missing required fields and orphaned attachment directories.
- Known bug: Changing only `type` (`lif update -t`, GraphQL `updateItem(type)`) bypasses the parent rank check; `lif check` does not report it afterwards either.
- No fixed depth limit: built-in types and `classic` use up to four ranks, custom types with rank 5 or higher are accepted.
- Types with the same rank cannot be nested in each other; ranks may have gaps.
- `lif next` does not exclude containers; the note on type `epic` ("do not work on directly") is not enforced.
- Source: FACTS.md §3, §3.1, §3.2, §5.2, §6.9, §9(a), §9(c), §9(d), §9(e); DESIGN.md D39, D40, D43

### Reference links
- Configuration (`reference/configuration`): types, ranks, status, priorities, presets, `require_fields_on`.
- `lif init` (`reference/commands/lif-init`): option `--profile`.
- `lif roadmap`, `lif milestones`, `lif progress`, `lif order` (`reference/commands/lif-roadmap`, `reference/commands/lif-milestones`, `reference/commands/lif-progress`, `reference/commands/lif-order`): planning views and ordering.
- `lif complete` (`reference/commands/lif-complete`) and `lif check` (`reference/commands/lif-check`): completion and checking.
- Source: DESIGN.md D24, D25 and Open (page cut phase 4)
