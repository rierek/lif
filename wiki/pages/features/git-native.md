---
title: "Git-native"
description: "Items live in your repository and travel with your code."
---

The store lives inside your repository. Items are files, so Git versions, diffs and merges them with the code.

## What

- The store is the directory `.lif/` next to the configuration `.lif.yml`. New items go into the root of the store. The archive goes into `archive/`.
- One file per item, with a stable name `<id>--<slug>.md`. Fields are in fixed order. Extra keys are in alphabetical order.
- Manual order uses the field `order` (a fractional index). `lif order` writes only the item you place.
- `lif.anchor: repo-root` lets all Git worktrees of a repository share one store. Without it, each worktree uses its own.
- `worktree.*` settings (`base_ref`, `path`, `setup`, `run`, `integrate`, `fetch_timeout`) configure worktrees. See [Configuration](../../reference/configuration/).
- `lif.commit_field` (default `commit`) holds a Git SHA. `lif complete --commit <ref>` writes it.

## Why

- Items are files. They are diffable, reviewable and mergeable together with the code.
- The backlog travels with the branch. You need no separate ticket system.
- A shared store across worktrees keeps parallel work from drifting apart.

## How it looks

A status change touches only front matter lines. This is `lif start` on one item, shown with `git diff` (context lines left out):

```diff
-status: todo
+status: in-progress
+priority: normal
-updated_at: 2026-10-14T19:17:18Z
+updated_at: 2026-10-14T19:17:23Z
```

`lif complete --commit HEAD` stores the commit SHA in the item (`commit: 22b4b97e...`). `lif check` warns if the SHA is not in the repository. Outside a Git repository the check is skipped. Lif refuses to resolve `HEAD` outside a repository, and reports `"HEAD" does not resolve to a commit`.

With `lif.anchor: repo-root`, two worktrees read the same store:

```text
repo/                  main worktree, holds .lif/ and .lif.yml
worktrees/feature-x/   second worktree, reads repo/.lif/
```

<details>
<summary>How to use Lif with Git</summary>

Treat items like code: commit, review, merge.

- Commit the item change with the code change it belongs to. Put the item ID in the commit title. This is a convention, not enforced by Lif.
- Use one branch per container item (epic or feature). This is a convention, not enforced by Lif.
- Set `lif.anchor: repo-root` so all worktrees see the same store.
- On a merge conflict in an item file, resolve the front matter or body by hand like any text file, then run `lif check`. Lif adds no merge tooling.
- The fixed field order and alphabetical extra keys keep diffs small. A parse error aborts loading (`loading <path>: ...`), so fix conflict markers before you run any `lif` command.

</details>

<details>
<summary>Example: pre-commit hook</summary>

Lif ships no hook, no installer and no recipe. This is an example, not product code. A Git `pre-commit` hook that calls `lif check`:

```bash
#!/bin/sh
# .git/hooks/pre-commit
lif check || exit 1
```

`lif check` exits 1 on at least one problem, and Git then aborts the commit. Use `lif check --strict` to let warnings stop the commit too. `lif check` resolves the store from the current directory, and Git runs the hook from the repository root.

</details>

## Limits

- No hook is shipped. CI does not run `lif check` either.
- `lif check` has gaps: parent and child rank, status values, priority values, tags, duplicate IDs and file name against ID are not checked.
- With a duplicate ID the file loaded later wins, without a message.
- `lif rename --prefix` refuses while `lif-serve` runs or active worktrees exist.
- Without `lif.anchor: repo-root` each worktree has its own store. Changes come together only through a merge.
- A merge can combine edits into a state `lif check` does not catch, for example mixed-field cycles [et2a]. See [Known limitations](../../known-limitations/).

## Reference links

- [Configuration](../../reference/configuration/): `lif.path`, `lif.anchor`, `worktree.*`, `lif.commit_field`.
- [Store management](../../overview/store-management/): own or shared store per worktree, store location, several projects.
- [`lif check`](../../reference/commands/lif-check/): scope of checks, exit codes, `--strict`, `--fix`.
- [`lif complete`](../../reference/commands/lif-complete/), [`lif order`](../../reference/commands/lif-order/), [`lif rename`](../../reference/commands/lif-rename/), [`lif init`](../../reference/commands/lif-init/).
- [Item file format](../../reference/item-file-format/): fields and file names.
