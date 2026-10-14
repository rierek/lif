---
title: "Managing Lif items in Obsidian"
description: "An experiment: browsing and editing Lif items in an Obsidian vault."
---

## Purpose
Placeholder for an experiment post (tag `experiment`): viewing and editing Lif items in an Obsidian vault. The post claims nothing about Obsidian behavior that has not been tested.

## Audience
Obsidian users who want to manage Lif files with a Markdown editor that supports properties; readers who want to judge the state of the experiment.

## Required sections

### Notice
- Notice box at the top: experimental, not a supported integration, results may change.
- Links to the reference as the binding source for the item format (`reference/item-file-format`).
- Tag `experiment`.
- Source: DESIGN.md D34, D42

### The idea
- Explains the idea: items are Markdown files with YAML front matter, so files that an Obsidian vault can in principle open. (TODO: verify)
- Names the goal: read, organize and edit items via Obsidian, use Lif commands for validation.
- Source: CONCEPT.md §5.6 (Content); Obsidian behavior `TODO: verify` — FACTS §9(g): not found, check in the experiment

### Setup
- Describes how the store (`.lif/`) appears in the vault; `.lif/` is a dot directory, Obsidian's behavior with it is unchecked. (TODO: verify)
- Shows variants: vault at repository root, symlink or vault inside the store (choice is tested in the experiment).
- Names prerequisites: installed Lif (`overview/installation`), store present.
- Source: FACTS.md §9(g), §2.4; setup steps `TODO: verify` — perform in the experiment

### What the format offers an Obsidian-like tool
- Shows the format facts: one `.md` file per item; YAML front matter with scalar fields (`title`, `status`, `type`, `priority`, `created_at`, `updated_at`, `order`, `parent`) and list fields (`tags`, `blocking`, `blocked_by`).
- Unknown front matter keys are preserved.
- The first line in the front matter is a YAML comment `# <id>`; ID and slug come from the file name, not from the front matter.
- File name: `<id>--<slug>.md`.
- Source: FACTS.md §2.1, §2.2, §2.4, §9(g)

### Obsidian Bases
- Explains: whether Obsidian Bases work with the item format is not verified.
- The code and the existing documentation contain no mention of Obsidian or Bases and no `.base` file.
- Shows after the experiment: a Base view over items, if it works. (TODO: verify)
- Source: FACTS.md §9(g); compatibility `TODO: verify` — check in the experiment, no statement until then

### Validating changes with lif check
- Explains: run `lif check` after changes in Obsidian; exit code 1 on problems.
- Lists what it detects: unknown item type, broken links, self-references, cycles (parent, blocking, blocked_by), missing required fields (warning), unknown SHAs (warning), orphaned attachment directories.
- Lists what it does not detect: parent/child ranks, status values, priority values, tag format, duplicate IDs, front matter syntax beyond loading.
- Note: a parse error in one file aborts loading.
- Source: FACTS.md §3.2, §6.9, §9(e), §2.4

### Limits
- Manual edits bypass the hierarchy rules of the CLI; `lif check` does not report rank violations.
- The file name carries the ID; renaming in Obsidian can change the ID. (TODO: verify)
- Blocking links live on one side only; `lif check` does not report asymmetry.
- Obsidian behavior (properties view, Bases, dot directory) is not documented.
- Source: FACTS.md §3.2, §5.6, §2.1, §9(d), §9(g); renaming `TODO: verify` — check in the experiment

### Reference links
- Item file format (`reference/item-file-format`), Configuration (`reference/configuration`), `lif check` (`reference/commands`).
- Features: Files in your repo (`features/files-in-your-repo`), Surface independence (`features/surface-independence`).
- Blog overview (`blog`).
- Source: DESIGN.md D34, D42
