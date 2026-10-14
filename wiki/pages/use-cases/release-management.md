---
title: "Release management"
description: "Plan, track and ship a release."
---

## Purpose
Shows how a release is planned and tracked with the preset `complex` and made traceable through commit SHAs.

## Audience
Release owners, teams with review findings, and agents that manage delivery scope and evidence.

## Required sections

### Problem
- Describes the situation: unclear what belongs in which release, which items are done and with which commit.
- Names requirements: separate value contributions (feature, improvement, chore), take in findings from reviews, evidence per item.
- Stays scenario text; no claims about Lif behavior.
- Source: CONCEPT.md §5.5 (Content)

### Solution
- Explains: preset `complex` separates release containers, value contributions and implementation items.
- Explains: `lif promote` turns review findings into items; `lif complete --commit` stores the SHA on the item.
- Explains: `lif check` verifies stored SHAs against the repository.
- Source: FACTS.md §9(b), §1.1 (promote, complete), §6.5

### Structure
- Shows preset `complex`: release (rank 1), bucket (rank 1, `roadmap: false`), feature, improvement and chore (rank 2), story, bug and task (rank 4).
- Explains: rank 3 is unused, the rule allows gaps; the preset has three levels.
- Explains `bucket` with `roadmap: false`: hidden in `lif roadmap` and `lif milestones`; on a container rank the whole subtree is hidden.
- Hierarchy diagram (Mermaid, D38).
- Optional: `require_fields_on` requires `commit` for `completed`; the key is `commit` by default (`lif.commit_field`).
- Example configuration: build with `lif init --profile complex` in phase 5 and verify with `lif check`. (TODO: verify)
- Source: FACTS.md §9(b), §3, §4.3, §2.3; example `TODO: verify` — generate in phase 5

### Workflow
- Plan: `lif create "…" -t release`, attach items with `--parent <release-id>`.
- Review findings: `lif promote <artifact> <finding-id>…` (artifact format: see command page).
- Implement: `lif start <id>` → `lif complete <id> --commit HEAD --summary "…"`.
- Verify: `lif check --strict` counts policy warnings (missing required fields, unknown SHAs) as errors; exit code 1 on problems.
- Roadmap as Markdown: `lif roadmap --format markdown`; progress `lif progress <release-id>`.
- Setting the release tag and writing release notes: not part of Lif, verify the process. (TODO: verify)
- Source: FACTS.md §1.1 (create, promote, start, complete, check, roadmap, progress), §6.5, §6.8; release tag/notes `TODO: verify` — no FACTS section, check in phase 5 against the repo process (`justfile`, `README.md`)

### Benefits
- Every completed item can carry a commit SHA: traceability from item to code.
- Findings from reviews are taken in after validation; findings already taken in are skipped.
- Release scope is a subtree with a progress number.
- Source: FACTS.md §1.1 (promote, complete, progress), §6.5

### Limits
- Lif does not create Git tags and does not generate release notes from items. (TODO: verify)
- `lif check` only verifies non-empty SHA strings in the commit field and warns (no error without `--strict`); outside a Git repository the check is skipped.
- `lif check` does not check parent/child ranks or status values.
- No pre-commit hook is shipped (D41).
- Source: FACTS.md §6.5, §6.9, §9(f); DESIGN.md D40, D41; tagging/notes `TODO: verify` — check in phase 5 against the repo process

### References
- Features: Dependencies (`features/dependencies`), Git-native (`features/git-native`), Automation and agents (`features/automation`).
- Reference: Configuration (`reference/configuration`), `lif promote`, `lif complete`, `lif check`, `lif roadmap` (`reference/commands`).
- Use case: Software project (`use-cases/software-projects`).
- Source: DESIGN.md D32, D39
