---
title: "Lif in brief"
description: "What Lif is, who it is for and where to go next."
---

## Purpose
Short, convincing orientation: what Lif is, who it is for, how it works and what it is not. The page replaces the seed page "Feature Overview" and does not duplicate feature details.

## Audience
People who evaluate or recommend Lif, and agents that use web search to find out what Lif is. No prior knowledge needed.

## Required sections

### What Lif is
- One-sentence definition: file-based issue tracker for AI-first workflows, items as Markdown files next to the code.
- Second sentence: gives humans and coding agents the same complete view of the project.
- Shows the tagline from `lif --help` verbatim as a quote block.
- Optional: terminal capture `lif roadmap --tags` as hero media (link to media plan, no image in the skeleton).
- Source: FACTS.md §1.0; DESIGN.md D48

### Who it is for
- Humans: CLI, TUI (`lif tui`), web UI (`lif-serve`) or any editor on the same files.
- Agents: `lif prime` and `lif --help` are enough to get started, no website needed.
- Scripts: `--json` output of the commands and GraphQL (`lif graphql`).
- Three access paths as a small table: Markdown files · CLI plus `lif prime` and `--help` · JSON plus GraphQL.
- Link to the feature pages "Surface independence" (`features/surface-independence`) and "Automation and agents" (`features/automation`).
- Source: FACTS.md §1.0, §1.1 (`prime`, `graphql`, `tui`), §7.5, §8.1

### How it works
- Store: directory `.lif/` next to `.lif.yml`; one item = one Markdown file (Liff) with YAML front matter and body.
- Shows a small example of an item file (front matter fields `title`, `status`, `type`, `priority`, `tags`, `parent`, `blocking`, `blocked_by`); generate the example with `lif create` in phase 5.
- `.lif.yml`: configuration, searched upward from the current directory; without a file, built-in defaults apply.
- No server needed: all CLI commands load the files directly; `lif-serve` is optional.
- Note on the legacy names `.beans/` and `.beans.yml` only as a side note that they are still read.
- Source: FACTS.md §2.1, §2.4, §4.1, §1.0 (store is loaded before every command)

### What makes it different
- Six features, one line each plus slug in backticks: Markdown-native (`features/markdown-native`), Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Surface independence (`features/surface-independence`), Automation and agents (`features/automation`), Git-native (`features/git-native`).
- One value promise per line, no details; details are on the feature pages.
- Link to the features index (`features/index`).
- Source: DESIGN.md D30, D32; CONCEPT.md §3.3 (Feature Overview must not duplicate features)

### What Lif is not
- Not a workflow engine: status, types and priorities are configurable names, not enforced transitions. (TODO: verify: check "no transition rules" against the code; FACTS shows only `require_fields_on` per status.)
- Presets (`lif init --profile`: `classic`, `todo`, `simple`, `complex`) are examples, not a fixed requirement.
- Not "four fixed levels": bundled presets use up to four ranks, custom types can go deeper, there is no hard upper limit (D39).
- Not a replacement for hosted trackers with user management (TODO: verify: the scope boundary against other trackers is editorial, FACTS contains nothing on it).
- Source: FACTS.md §3, §9(a), §9(b); DESIGN.md D39. The bullets "no transition rules" and "no hosted tracker" are `TODO: verify` (check: `pkg/config/config.go`, `README.md`).

### Where to go next
- Installation (`overview/installation`) for newcomers.
- First steps (`overview/first-steps`) for the first hour.
- Concepts (`overview/concepts`) for the mental model.
- Agent onboarding (`overview/agent-onboarding`) for coding agents.
- Store management (`overview/store-management`) for several projects, worktrees and a shared store.
- Features (`features/index`) and Use cases (`use-cases/index`) for a deeper look.
- Source: DESIGN.md D35, D37, D56; CONCEPT.md §3
