---
title: "Store management"
description: "Where the store lives and how to work with Lif across several projects, worktrees and shared stores."
---

## Purpose
Explains where Lif keeps its store and how to steer that when you work with several projects, Git worktrees or a shared store. Task-oriented companion to Reference: Configuration (`reference/configuration`), which keeps the canonical precedence table. The page lives in Overview because it is a "how do I work with this" topic, not a look-up page (D56).

## Audience
Users who run Lif in more than one repository, use Git worktrees, keep the store outside the repository or wonder why a command shows "the wrong" items. Agents that must find out which store applies. Basic knowledge from First steps (`overview/first-steps`) and Concepts (`overview/concepts`) helps.

## Required sections

### One project, one store
- Shows the default: `lif init` creates `.lif.yml` and the store directory `.lif/` in the project folder; the store is plain files and is committed together with the code (link to Files in your repo, `features/files-in-your-repo`).
- Explains how Lif finds it: the config file is searched upward from the current directory; the store is the directory the config names (`lif.path`), by default `.lif/` next to the config.
- Shows `lif path` to print the resolved store directory.
- Source: FACTS.md §1.1 (`init`, `path`), §2.4, §4.1, §4.2

### Where the store lives
- Explains `lif.path` in `.lif.yml`: a relative path is joined to the directory of the config file, an absolute path is used as is; empty means the default `.lif`.
- Explains config discovery: `--config <path>` names the file; otherwise the search goes upward directory by directory, the nearest `.lif.yml` wins; without any config file the defaults apply in the current directory.
- Shows an example excerpt of `.lif.yml` with an absolute `lif.path` (phase 5: test it).
- Names the legacy spellings: `.beans.yml` with the `beans:` section (`beans.path`), the folder `.beans/` and the variable `BEANS_PATH` are read in place and never renamed; `.lif.yml` and `.beans.yml` in one directory is an error.
- Source: FACTS.md §2.4, §4.1, §4.2; `pkg/config/config.go:503-562, 612-625, 845-857`

### Steer the store per call
- Explains `--lif-path <dir>`: a global flag on every command; it beats the config and the environment.
- Explains `LIF_PATH`: used only when there is neither a flag nor a config file; a found config file always wins over the variable, so a variable exported by a tool such as direnv does not redirect unrelated repositories.
- `LIF_PATH` and the legacy `BEANS_PATH` both set is an error (`both LIF_PATH and BEANS_PATH are set; unset one of them`); `BEANS_PATH` alone is read as the legacy variable.
- Shows the precedence as a table: 1. `--lif-path`, 2. config file (`lif.path` or the default directory next to it), 3. `LIF_PATH`, 4. default directory in the current directory.
- Shows the two errors: `no .lif directory found at <path> (run 'lif init' to create one)` and, for an explicit flag or variable, `store path does not exist or is not a directory: <path>`.
- Explains `lif init --lif-path <dir>`: creates the store at that path and writes no config file; `--profile` cannot be combined with `--lif-path`.
- Source: FACTS.md §1.0, §1.1 (`init`), §4.2; `internal/commands/root.go:205-280` (precedence, errors); `internal/commands/init.go:31-37, 55-70`; `pkg/config/config.go:645-656` (both variables set)

### Worktrees: own store or one shared store
- Explains the default: a Git worktree is a separate checkout, so without `anchor` the CLI uses the worktree's own store directory.
- Explains `lif.anchor: repo-root`: from a secondary worktree the store is resolved against the main worktree root, so all worktrees of a repository share one store; in the main worktree and outside Git nothing changes; any other value is a config load error.
- Shows when to choose which: own store = items travel with the branch; shared store = every worktree sees the same items at once.
- Note: a relative `lif.path` is joined to the directory of the config file that was found, so it can point to a different place in every worktree. (TODO: verify with a real worktree)
- Names `lif-serve`: it watches the store directories of worktrees and merges their changes into the running state.
- Source: FACTS.md §4.1, §4.2, §8.1; `internal/commands/root.go:205-224, 282-302` (anchor); `pkg/config/config.go:73-76, 757-777`

### Several projects
- Explains the default: one store per project, versioned with that project's code; `lif init` per project gives each its own config and prefix.
- Explains a shared store: several projects point `lif.path` (absolute), `--lif-path` or `LIF_PATH` at one directory; then the items are not versioned with a single repository. Each project's `.lif.yml` still defines types, statuses and prefix. (TODO: verify what happens when the configs differ; FACTS has nothing on it.)
- Explains that a global `LIF_PATH` does not hijack projects that have their own config file (the config wins).
- Explains hopping between projects in the TUI: `lif tui [repo-path]`, the repository picker lists the `repos` of `~/.config/lif/tui/config.yaml`, and `repo_store_paths` maps a repository to an explicit store path that the TUI passes as `--lif-path`. Link to TUI: Start and repositories (`reference/tui/start-and-repositories`).
- Shows how to try things out without touching the real store: run commands with `--lif-path` pointing to a scratch directory. (TODO: verify)
- Source: FACTS.md §1.1 (`tui`), §4.2, §8.1, §8.2; `internal/tui/config/settings.go:52-62` (`repo_store_paths`); `internal/commands/init.go:55-70`

### ID prefix
- Explains `lif.prefix`: `lif init` writes `<folder name>-`; an ID is the prefix plus a random suffix of `lif.id_length` characters (default 4) from `0-9a-z`.
- Explains why distinct prefixes matter once several projects share a store: a duplicate ID is not reported by `lif check`, the later file silently overwrites the earlier one.
- Explains short IDs: only the configured prefix is prepended to a short ID; a full ID always works because the exact match is tried first.
- Shows how to change the prefix for the whole project: `lif rename --prefix <new>` (use `--dry-run` first; needs `--yes` or a confirmation; refuses while `lif-serve` runs or active worktrees exist).
- Names the one-off prefix on creation: `lif create --prefix` (starts with a letter, letters, digits and single hyphens only).
- Source: FACTS.md §1.1 (`rename`, `create`), §2.4, §3.2, §4.3; `pkg/itemcore/core.go:640-653` (short ID); DESIGN.md D56

### Find out which store applies
- Shows the three checks: `lif path` (the resolved store), the environment (`env | grep -E 'LIF_PATH|BEANS_PATH'`) and `lif check` (config problems, legacy names).
- Explains typical symptoms with the cause from the precedence rules: "no .lif directory found" (wrong or missing store: give `--lif-path` or look for a stray config file above the current directory); settings that have no effect (another `.lif.yml` was found first: give `--config`). (TODO: verify both in phase 5)
- Source: FACTS.md §1.1 (`path`, `check`), §4.1, §4.2, §6.6

### Related pages
- Links to Reference: Configuration (`reference/configuration`) for the full precedence and all keys, and to Reference: Command overview (`reference/commands/index`) for the global flags.
- Links to Features: Files in your repo (`features/files-in-your-repo`), Reference: TUI start (`reference/tui/start-and-repositories`) and the Glossary (`reference/glossary`).
- Source: DESIGN.md D56
