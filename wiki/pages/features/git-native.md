---
title: "Git-native"
description: "Items live in your repository and travel with your code."
---

## Purpose
Explains why items live in the repository, how they interact with branches and worktrees, and how to use them with Git. Shows a short hook snippet for `lif check` and states clearly that Lif ships no hook.

## Audience
Developers who want to version backlog and code together, and teams with several worktrees or branches. They want to know what happens in conflicts and commits and how to prevent drift in the store.

## Required sections

### What
- The store lives as the directory `.lif/` next to the configuration `.lif.yml`, so inside the repository; items are flat files (new items go into the root of the store, the archive into `archive/`).
- One file per item with stable file name `<id>--<slug>.md`; fields in fixed order, extra keys alphabetical.
- Manual ordering via the field `order` (fractional index): `lif order` writes only the placed item.
- `lif.anchor: repo-root` lets all Git worktrees of a repository share one store; without the anchor each worktree uses its own.
- Settings `worktree.*` (`base_ref`, `path`, `setup`, `run`, `integrate`, `fetch_timeout`) configure worktrees; which interface uses them is only partly proven (`worktree.run` creates a run button in the Web UI) (TODO: verify)
- `lif.commit_field` (default `commit`) holds a Git SHA; `lif complete --commit` writes it.
- Source: FACTS.md §1.1 (`lif order`, `lif complete`), §2.1, §2.2, §2.4, §4.2, §4.3; (TODO: verify) for the scope of `worktree.*`: check `internal/worktree` and the configuration docs

### Why
- Items are files: they become diffable, reviewable and mergeable together with the code.
- The backlog travels with the branch; no separate ticket system needed.
- A shared store across worktrees prevents parallel work from drifting apart.
- Source: DESIGN.md D30; CONCEPT.md §4.1; FACTS.md §4.2

### How it looks
- Diff of an item after a command: only front matter lines change (generate the example in phase 5) (TODO: verify)
- Flow: create a branch, adjust the item with `lif update`, commit code and item together.
- Worktrees: table or diagram of how two worktrees with `lif.anchor: repo-root` read the same store.
- `lif complete --commit HEAD`: stores the SHA in the field `commit`; `lif check` warns if the SHA is missing in the repository and skips the check outside a Git repository.
- Source: FACTS.md §1.1 (`lif complete`), §4.2, §6.5; (TODO: verify) for diff example: generate in phase 5 in a test repository

### Use with Git
- How-to: Treat items like code: commit, review, merge.
- Commit convention with item ID and one branch per container: from the draft, not in FACTS (TODO: verify)
- Worktrees: set `lif.anchor: repo-root` so all worktrees see the same store.
- Merge conflicts in item files: how to proceed on a conflict in front matter or body, then `lif check` (TODO: verify)
- Fixed field order and alphabetical extra keys keep diffs small; a parse error aborts loading (`loading <path>: ...`).
- Source: FACTS.md §2.1, §4.2, §6.8; (TODO: verify) for commit convention and conflict resolution: replay the flow in phase 5 in a test repository

### Pre-commit hook snippet
- Show snippet: a Git `pre-commit` hook that calls `lif check`; exit 1 reports a problem and aborts the commit.
- Optionally `lif check --strict`, so warnings also stop the commit.
- Important: There is no shipped hook, no installer function and no recipe; the page shows only an example, not product code.
- Note: `lif check` uses the current directory to resolve the store; the commit check runs from the current directory.
- Run the snippet in a test repository in phase 5 (TODO: verify)
- Source: FACTS.md §6.8, §6.10, §9(f), §4.2, §6.5; DESIGN.md D41; (TODO: verify) for the snippet itself: run in phase 5

### Limits
- No hook is shipped; CI does not run `lif check` either.
- `lif check` has gaps: it does not check rank of parent and child, status values, priority values, tags, duplicate IDs and file name against ID.
- With a duplicate ID the file loaded later wins, without a message.
- `lif rename --prefix` refuses as long as `lif-serve` runs or active worktrees exist.
- Without `lif.anchor: repo-root` each worktree has its own store; changes only come together through a merge.
- Source: FACTS.md §4.2, §6.9, §6.10, §1.1 (`lif rename`); DESIGN.md D40, D41

### Reference links
- Configuration (`reference/configuration`): `lif.path`, `lif.anchor`, `worktree.*`, `lif.commit_field`.
- Store management (`overview/store-management`): worktrees with their own or a shared store, store location, several projects.
- `lif check` (`reference/commands/lif-check`): scope of checks, exit codes, `--strict`, `--fix`.
- `lif complete`, `lif order`, `lif rename`, `lif init` (`reference/commands/lif-complete`, `reference/commands/lif-order`, `reference/commands/lif-rename`, `reference/commands/lif-init`).
- Item file format (`reference/item-file-format`): fields and file names.
- Source: DESIGN.md D24, D25
