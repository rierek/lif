---
title: "Start and repositories"
description: "Start lif tui and choose the repository to work in."
---

## Purpose
Explains how the TUI starts, which repository it opens and how to switch between repositories. Entry point to the TUI subsection of the reference.

## Audience
Users who start `lif tui` for the first time or want to manage several projects in one interface.

## Required sections

### Starting the TUI
- Shows `lif tui [repo-path]`: at most one argument (repo path, directory completion); no flags of its own besides the global ones.
- Explains: `lif tui` is the TUI inside the `lif` binary (the former separate `lif-tui` binary no longer exists); it also runs without a loaded store.
- Names `--lif-path` as a global flag that determines the store of the opened repository.
- Describes the TUI as a companion to the CLI (one short sentence, link to `reference/commands/lif-tui`).
- Source: FACTS.md §1.0, §1.1 (`lif tui`), §8.1, §8.2

### Which repository opens
- Order as a numbered list: 1) explicit path argument, 2) repository found by searching upward from the current directory, 3) without a repository and with at least two entries in `repos` of the settings file, the repository picker (lobby), 4) otherwise the error message "not found".
- Shows example commands for each case.
- Note: With exactly one repository in the settings and outside a repository, no lobby appears (error).
- Source: FACTS.md §8.1, §1.1 (`lif tui`)

### The repository picker
- Explains the lobby as the list of repositories from the settings file; selecting one opens the repository.
- Key `p` opens the picker also from an opened repository (for the shortcut see `reference/tui/key-bindings`).
- Describes the prerequisite: at least two repositories in `repos`.
- Screenshot `lobby.png` (see section "Screenshots").
- Source: FACTS.md §8.1, §8.2, §8.3

### Settings that matter at start
- Names `repos` (list of repository paths) and `repo_store_paths` (mapping of repository to store path) in `~/.config/lif/tui/config.yaml`.
- Names `--lif-path` as a way to set the store once.
- Links to `reference/tui/settings-and-themes` for all keys.
- Shows how a repository is added to `repos`. (TODO: verify)
- Source: FACTS.md §8.2, §1.1 (`lif tui`); adding new repositories — `TODO: verify` (Phase 5: check whether the TUI writes `repos` itself, `internal/tui/config/settings.go`)

### Quit and help
- `q` and `ctrl+c` quit after confirmation; `esc` goes back.
- `?` opens help; it is generated from the same key map (`keyMap`) as the page `reference/tui/key-bindings`.
- `K` opens the command palette.
- Source: FACTS.md §8.3, §8.2

### Screenshots
- Plan: embed the existing image `wiki/screenshots/lobby.png` (lobby); further images on the pages `reference/tui/views`.
- Media type per design: VHS GIF for the start (`lif tui`), screenshots as webp.
- Check whether the images match the current interface (re-record if not). (TODO: verify)
- Source: DESIGN.md D38, D44; currency of the images — `TODO: verify` (Phase 5: compare with a running TUI)
