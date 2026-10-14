---
title: "Installation"
description: "Install Lif and check that it works."
---

## Purpose
Target of the hero button on the start page: install Lif and check with a first command that it runs. Shows only the installation path, no configuration.

## Audience
New users who have chosen Lif and want a working `lif` as quickly as possible, plus agents that look up installation steps.

## Required sections

### Requirements
- Names supported operating systems and architectures (table).
- Prerequisites: Git recommended, Go only for building from source, Node only for the frontend (TODO: verify).
- Note: `lif` itself needs no running server (see `overview/index`).
- Source: `TODO: verify` — FACTS.md has no installation section; check in `.goreleaser.yaml` (build matrix), `go.mod` (Go version), `README.md`, `wiki/installation.md`.

### Install methods
- Four ways as tabs or a subsection list: install script · package manager · release download · build from source.
- Per way one copyable command and one line "when to use" (verify commands only in phase 5).
- Note: macOS quarantine on Homebrew install is handled with `xattr` (TODO: verify; FACTS mentions only the post-install hook).
- Note on `just install` as a developer path and its guard (`LIF_INSTALL_FORCE`) belongs on the Contribute page, not here.
- Source: `TODO: verify` — script URL, package manager names and release artifacts are missing in FACTS; check in `install.sh`, `.goreleaser.yaml` (Homebrew section), `README.md`, `wiki/installation.md`. FACTS.md §6.10 only documents: goreleaser Homebrew hook with `xattr`, `scripts/install-guard.sh` with `LIF_INSTALL_FORCE=1`.

### The binaries
- `lif`: CLI and TUI (`lif tui`); the TUI is no longer a separate binary.
- `lif-serve`: separate binary for the web server; without an argument it starts `serve`.
- The command `lif serve` in the binary `lif` is a hidden stub and is not documented; it points to `lif-serve`.
- Table: binary · purpose · default port (8080 for `lif-serve`).
- Source: FACTS.md §1.0, §1.1 (`lif tui`, `lif serve` stub, `lif-serve`), §4.3

### Verify the installation
- Shows `lif version` with example output; `--json` for structured output.
- Explains: a development build reports the version `dev`.
- Shows `lif --help`: 28 commands are listed (recount the number in phase 5).
- First useful command: `lif init` in the project folder; link to First steps (`overview/first-steps`).
- Source: FACTS.md §1.1 (`version`, command count), §1.0 (bootstrap skip: `version` runs without a store)

### Shell completion
- Shows `lif completion bash|zsh|fish|powershell` (one shared command page, D25).
- Names item ID completion as the benefit.
- Optional: zsh widget `Alt+i` for `lif pick` (extras, TODO: verify installation path).
- Source: FACTS.md §1.1 (Cobra built-ins, `lif pick`); DESIGN.md D25. The installation path of the zsh widget is `TODO: verify` (check in `extras/zsh/lif-pick.zsh`).

### Update and uninstall
- Update per installation way (script again, package manager upgrade, new download).
- Uninstall: remove the binaries; Stores in the project (`.lif/`, `.lif.yml`) stay as normal files.
- Note on upgrade compatibility of old `.beans` Stores (link only as a title, check source).
- Source: `TODO: verify` — FACTS.md contains neither update nor uninstall paths; check in `install.sh`, `.goreleaser.yaml`, `wiki/installation.md`, `wiki/compatibility-and-upgrading.md`. Legacy reading of `.beans/`: FACTS.md §2.4, §4.1.

### Next step
- Link to First steps (`overview/first-steps`): create a store, choose a preset, first items.
- Link to Concepts (`overview/concepts`) and, for agents, Agent onboarding (`overview/agent-onboarding`).
- Source: DESIGN.md D37; CONCEPT.md §3.1
