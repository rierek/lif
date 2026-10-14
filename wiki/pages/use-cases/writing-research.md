---
title: "Writing or research project"
description: "Organise chapters, sources and open questions."
---

## Purpose
Non-technical example: structure a writing or research project (book, thesis, study) with custom types.

## Audience
Authors, researchers and students who track chapters, sources and open questions in files; agents that build a non-software setup.

## Required sections

### Problem
- Describes the situation: many chapters and sources, open questions get lost, unclear what a section is waiting for.
- Names requirements: own terms instead of software vocabulary, topic filters, progress per chapter.
- Shows: Lif is not limited to software (D33).
- Source: DESIGN.md D33

### Solution
- Explains: Custom `types` with `types_exclusive: true` replace the software vocabulary; status and priority stay usable.
- Explains: tags form topics; `blocked_by` models "waiting for a source"; the item body carries working notes.
- Explains: status `draft` ("Needs refinement before it can be worked on") fits unfinished sections.
- Source: FACTS.md §4.3, §2.3, §2.2 (Tags), §5.1, §9(b)

### Structure
- Shows a `types` table (suggestion): `chapter` (rank 1), `section` (rank 2), `source` and `question` (rank 4). (TODO: verify)
- Explains: A type without `rank` gets rank 4; ranks determine valid parents (child has a higher rank).
- Hierarchy diagram (Mermaid, D38): chapter > section > source/question.
- Example `.lif.yml` with `types_exclusive: true`: build with `lif init` in phase 5 and verify with `lif check`. (TODO: verify)
- Explains: The default type (`lif.default_type`) falls back to the first type if `task` is missing.
- Source: FACTS.md §4.3, §3, §9(b); type names and example config `TODO: verify` — suggestion, build in phase 5

### Workflow
- Create: `lif create "…" -t chapter`, `lif create "…" -t section --parent <chapter-id> --tag <theme>`.
- Capture a source: `lif create "…" -t source --blocked-by …` or an open question as `question`; the section waits via `blocked_by`.
- Continue: `lif next --tag <theme>`, `lif start`, `lif complete --summary "…"`.
- Overview: `lif list --tag <theme>`, `lif progress <chapter-id>`, `lif list --view tree`.
- Order of sections: `lif order <id> --after <id>`.
- Source: FACTS.md §1.1 (create, next, start, complete, list, progress, order), §5.1

### Benefits
- Text, notes and planning live side by side as files; changes are traceable via Git.
- Progress per chapter in percent; open sources become visible via `blocked_by`.
- Same commands as in software projects, own vocabulary.
- Source: FACTS.md §1.1 (progress), §5.1, §4.3

### Limits
- Lif does not manage citations or a bibliography database and does not assemble text. (TODO: verify)
- `lif roadmap` builds on milestone and container ranks; check the behavior with custom types in the example. (TODO: verify)
- `lif check` checks type names against the configuration, but not parent/child ranks, status values or priority values (D40).
- Source: FACTS.md §3, §3.2, §6.9; DESIGN.md D40; citations/export and `lif roadmap` with custom types `TODO: verify` — check in the example in phase 5

### References
- Features: Flexible planning (`features/project-planning`), Markdown-native (`features/markdown-native`).
- Reference: Configuration (`reference/configuration`), Item file format (`reference/item-file-format`).
- Overview: Concepts (`overview/concepts`).
- Source: DESIGN.md D32, D33
