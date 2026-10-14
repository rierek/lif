---
title: "Features"
description: "What Lif can do, with a page for each capability."
---

## Purpose
Entry point to the Features chapter: shows the six capabilities of Lif at a glance and explains how they fit together. The page describes capabilities, not individual commands; details live on the feature pages and in the Reference.

## Audience
People evaluating or recommending Lif, and coding agents that use web search to find out what Lif can do. They want to know in a few lines which capability answers their question, and jump straight to the matching page.

## Required sections

### What Lif can do
- One line per feature with the slug in backticks: Markdown-native (`features/markdown-native`), Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Surface independence (`features/surface-independence`), Automation and agents (`features/automation`), Git-native (`features/git-native`).
- Principle up front: A feature is a capability, not a command; commands only show capabilities (CONCEPT §4).
- Note on the fixed page pattern: What, Why, How it looks, Limits, Reference links; how-to guides moved into the matching feature page as collapsed sections.
- Six pages, not five: Git-native was added as the sixth feature.
- Source: DESIGN.md D30, D31, D32; CONCEPT.md §4

### How the features fit together
- Explains the shared foundation: An item is a Markdown file in the store (`.lif/` next to `.lif.yml`), and all other features build on it.
- Link: Planning (types, ranks, parents) and dependencies (`blocking` / `blocked_by`) describe relations between files; `lif next` and `lif roadmap` read both.
- Link: All writing paths (CLI, GraphQL, TUI, Web UI) go through the same item logic and enforce the same configured rules; editing the files directly is free and is checked afterwards with `lif check`.
- Link: Because the store lives in the repository, items travel with the code (Git-native); automation and agents work on the same files.
- Optional: Mermaid diagram that shows the store in the middle and groups the six features around it (D38).
- Source: FACTS.md §2.1, §2.4, §3.1, §4.2, §5.2, §7.5; DESIGN.md D38

### Which feature answers which question
- Table sketch with columns question, feature page, entry command.
- Row: "Can I read and change my data without Lif?" with `features/markdown-native`.
- Row: "How do I model milestones, epics and tasks?" with `features/project-planning` and `lif init --profile`.
- Row: "What can I start now, what blocks me?" with `features/dependencies` and `lif next`.
- Row: "Can I mix CLI, TUI, Web UI and my editor?" with `features/surface-independence`.
- Row: "How do I connect scripts or coding agents?" with `features/automation`, `lif prime`, `--json`.
- Row: "How does this work with branches, worktrees and commits?" with `features/git-native`.
- Source: FACTS.md §1.1 (`lif init`, `lif next`, `lif prime`), §5.2; DESIGN.md D30

### Where to go next
- Use cases (`use-cases/index`): the same features in five worked scenarios.
- Reference (`reference/index`): commands, item file format, configuration, GraphQL.
- Entry point for coding agents: Agent onboarding (`overview/agent-onboarding`).
- Note on limits: Every feature page has a Limits section; the page points out that it states honestly there what Lif does not check (D39 to D41).
- Source: DESIGN.md D32, D39, D40, D41
