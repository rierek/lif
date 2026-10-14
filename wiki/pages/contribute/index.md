---
title: "Contribute"
description: "Repository layout, where to change what and how to build the docs."
---

## Purpose
Helps new contributors find their way around the repository: layout, places to change, docs build and maintenance rules.

## Audience
Developers and documentation authors who change Lif or the docs; agents that make changes to the repository.

## Required sections

### Repository layout
- Shows a tree sketch: `cmd/lif` (CLI), `cmd/lif-serve` (web server), `cmd/lif-docgen` (generates command reference pages), `internal/commands`, `pkg/item`, `wiki/pages`, `wiki/assets`, `site`, `.lif` store.
- Explains each line in a half sentence.
- Note: `lif-serve` is a separate binary; `lif serve` in the main binary is a hidden stub.
- Source: FACTS.md §8.1 (binaries); directory tree `TODO: verify` — check checkout tree, `justfile` and `README.md` in phase 5

### Where to change what
- Change a command: definition under `internal/commands`; help, completion and reference pages follow at build. (TODO: verify)
- Change docs: pages under `wiki/pages`, media under `wiki/assets`.
- TUI key bindings: single source is the `keyMap` in `internal/tui/keymap.go`; the help in the TUI and the Key bindings page (`reference/tui/key-bindings`) follow from it.
- Item file format: `pkg/item`.
- Source: FACTS.md §8.3 (keyMap), §2.1 (`pkg/item`); command→reference chain `TODO: verify` — check `cmd/lif-docgen` and `justfile`

### Build and preview the docs
- Shows `just docs-gen`, `just docs-build`, `just docs-dev` and what each does.
- Explains prerequisites (Go, Node) and where the output goes.
- Source: `TODO: verify` — check recipe names and prerequisites in `justfile` and `site/package.json`

### Report issues and propose changes
- Explains where to report bugs and propose changes (issue tracker, branch, pull request).
- Links to the commit and branch conventions of the repository.
- Source: `TODO: verify` — check `README.md`, `.github/` and process documents in the checkout

### License
- Names the license and links to the file `LICENSE`.
- Links to the attribution to the upstream project (`about`).
- Source: `TODO: verify` — check `LICENSE` in the checkout (trial draft names Apache-2.0)

### Maintenance rules
- Checklist per command change: definition, help text, completion, generated reference, `lif prime`, tests, docs.
- Rule: behavior statements in docs come from code or `lif --help`, never from assumptions.
- Rule: documentation follows the terminology of the code (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset).
- Source: DESIGN.md D45, D47; checklist and `lif prime` relationship `TODO: verify` — clarify the process with maintainers
