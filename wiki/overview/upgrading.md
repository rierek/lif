---
title: "Upgrading"
description: "Breaking changes with old form, new form and migration step, plus how to check your project after an upgrade."
---

This page is the upgrade guide. It covers the GitHub owner rename in 0.14.1 and the move of an existing install and project from the former product name, beans, to Lif. Lif is a hard cut for names: the binaries, flags, environment variables, JSON keys, GraphQL names and integration files all have new names, and the old names are gone. There is no `beans` alias and there is no migration command. Your data does not need migrating: a project's item files, IDs and ID prefix stay exactly as they are, and the legacy data names (`.beans/`, `.beans.yml`, section key `beans:`, `BEANS_PATH`) keep being read in place, so a project works with `lif` the day you install it.

Do not assume a specific past or future release is compatible with your data purely from a version number; verify it against your own project as described below, since this document cannot make version-specific compatibility guarantees on your behalf.

## Before upgrading: back up your data

Because the store directory is meant to be tracked in the same version control system as your code, commit any pending item changes (or otherwise snapshot the store directory) before installing the new binaries, so that you can diff or revert if something behaves unexpectedly.

Do not rely on any binary's internal safety mechanisms as a substitute for your own backup; those mechanisms exist to protect against interrupted internal operations, not to replace normal version control hygiene.

## Review changes before upgrading

Review the [Releases](../../releases/) pages for the version you are upgrading to, paying particular attention to changes marked as breaking; each release's notes are generated from the Conventional Commits merged since the previous tag. For unreleased changes, review the [commit history](https://github.com/rierek/lif/commits/main) directly. The project uses Conventional Commit prefixes such as `feat:`, `fix:`, `perf:`, and `refactor:`; a `!` in the commit type or scope marks a breaking change.

## Upgrade guide: breaking changes

Each entry lists the old form, the new form and the migration step. Work through the groups that apply to your setup; the order does not matter. The owner rename is the latest change; the other groups come from the rename from beans to Lif.

### GitHub owner and Go module path (0.14.1)

The GitHub owner changed from `xRiErOS` to `rierek` in 0.14.1. The repository is now `https://github.com/rierek/lif`. GitHub redirects old repository URLs, but the Go module path changed, so `go install` with the old path fails.

| Old form | New form | Migration step |
| --- | --- | --- |
| `go install github.com/xRiErOS/lif/cmd/lif@latest` | `go install github.com/rierek/lif/cmd/lif@latest` | Use the new path in scripts and CI jobs. In Go code that imports Lif packages, replace `github.com/xRiErOS/lif` with `github.com/rierek/lif`. |
| tap `xRiErOS/lif` (`brew install xRiErOS/lif/lif`) | tap `rierek/lif` (`brew install rierek/lif/lif`) | Run `brew uninstall --cask lif`, then `brew untap xRiErOS/lif`, then `brew install rierek/lif/lif`. |
| `curl … raw.githubusercontent.com/xRiErOS/lif/main/install.sh \| sh` | `curl … raw.githubusercontent.com/rierek/lif/main/install.sh \| sh` | Use the new URL. The default of `LIF_REPO` is now `rierek/lif`; set `LIF_REPO` only for a fork. |
| `nix profile install github:xRiErOS/lif` | `nix profile install github:rierek/lif` | Remove the old profile entry with `nix profile remove lif`, then install from the new flake reference. |
| git remote `https://github.com/xRiErOS/lif.git` | `https://github.com/rierek/lif.git` | Run `git remote set-url origin https://github.com/rierek/lif.git` in each checkout. |

### Binaries and the path flag

| Old form | New form | Migration step |
| --- | --- | --- |
| `beans` | `lif` | Install the new binary and replace every call in scripts, shell functions, CI jobs and agent instructions. |
| `beans-serve` | `lif-serve` | Replace the call; the default port and the routes are unchanged, and the data-path flag follows the `--lif-path` rename below. |
| `bt` / `beans-tui` | `lif tui` | Replace the call; the terminal UI is a subcommand of `lif` and needs no separate install. Existing config in `~/.config/beans-tui/` is still read as a fallback; see [Settings and themes](../../reference/tui/settings-and-themes/). |
| `--beans-path` | `--lif-path` | Replace the flag in every script and in `lif-serve` and `lif tui` calls. |

There is no `beans` alias: a script that still calls `beans` fails with `command not found` after you remove the old binary. Remove the old binaries once everything calls `lif`.

### Installer variables

| Old form | New form | Migration step |
| --- | --- | --- |
| `BEANS_REPO` | `LIF_REPO` | Rename the variable wherever you pipe `install.sh` into a shell (CI jobs, provisioning scripts, dotfiles). |
| `BEANS_BIN_DIR` | `LIF_BIN_DIR` | Same. |
| `BEANS_VERSION` | `LIF_VERSION` | Same. |

### Homebrew

| Old form | New form | Migration step |
| --- | --- | --- |
| cask `beans` from tap `xRiErOS/beans` | cask `lif` from tap `rierek/lif` | Run `brew uninstall --cask beans`, then `brew untap xRiErOS/beans`, then `brew install rierek/lif/lif`. |

### Contributor tooling

These apply if you build Lif from a checkout and use its `just` recipes.

| Old form | New form | Migration step |
| --- | --- | --- |
| `BEANS_BIN_DIR` (install directory of `just install`) | `LIF_BIN_DIR` | Rename the variable in your shell profile. |
| `BEANS_WIP_SUFFIX` | `LIF_WIP_SUFFIX` | Same. |
| `BEANS_INSTALL_FORCE` | `LIF_INSTALL_FORCE` | Same. |
| `BEANS_RELEASE_REMOTE` | `LIF_RELEASE_REMOTE` | Same. |
| `BEANS_PORT` | `LIF_PORT` | Same. |
| `BEANS_DEMO_ROOT` | `LIF_DEMO_ROOT` | Same; the demo session and default root are named `lif-demo` and `/tmp/lif-demo`. |
| git config key `beans.installHome` | `lif.installHome` | In the checkout that you install from, run `git config --local lif.installHome true` before `just install`. |

### JSON keys

Machine-readable output uses the new vocabulary. A parser that reads an old key finds nothing; there are no doubled keys.

| Old key | New key | Where | Migration step |
| --- | --- | --- | --- |
| `bean` | `item` | `lif milestones --json` entries; message envelopes of single-item results | Read `item`. |
| `beans` | `items` | message envelopes of multi-item results, for example a failed batch (`{"success": false, "items": [...], "count": n}`) | Read `items`. |
| `bean_id` | `item_id` | entries of `broken_links`, `self_links` and `cycles` in `lif check --json` | Read `item_id`. |
| `bean_issues` | `item_issues` | `lif check --json` | Read `item_issues`. |
| `epics` / `features` arrays inside a milestone or the unscheduled group | one ordered `containers` array whose entries hold an `epic` or a `feature` | `lif roadmap --json` | Iterate `containers` instead of two arrays. |

### GraphQL

The schema printed by `lif graphql --schema` is the reference. Every name that contained the old noun changed, including fields and arguments:

| Old name | New name |
| --- | --- |
| type `Bean` | `Item` |
| query `bean(id:)`, `beans(filter:)` | `item(id:)`, `items(filter:)` |
| input `BeanFilter` | `ItemFilter` |
| mutations `createBean`, `updateBean`, `deleteBean`, `archiveBean`, `saveBean`, `saveDirtyBeans` | `createItem`, `updateItem`, `deleteItem`, `archiveItem`, `saveItem`, `saveDirtyItems` |
| inputs `CreateBeanInput`, `UpdateBeanInput` | `CreateItemInput`, `UpdateItemInput` |
| query `hasDirtyBeans` | `hasDirtyItems` |
| subscription `beanChanged`, type `BeanChangeEvent` | `itemChanged`, `ItemChangeEvent` |
| fields of `BeanChangeEvent`: `bean`, `beans`, `beanId` | fields of `ItemChangeEvent`: `item`, `items`, `itemId` |
| field `Worktree.beans` | `Worktree.items` |
| fields `beanId` on `AgentSession` and `ActiveAgentStatus` | `itemId` |
| argument `beanId` of `agentSession`, `agentActions`, `agentSessionChanged`, `sendAgentMessage`, `stopAgent`, `setAgentPlanMode`, `setAgentActMode`, `setAgentEffort`, `setAgentPendingInteraction`, `clearAgentSession`, `executeAgentAction` | `itemId` |

Migration step: search your saved queries, scripts and client code for those names and replace them, then run the query again. A subscription client that read `beanChanged { bean { … } }` now reads `itemChanged { item { … } }`. `lif graphql --schema` lists every current name.

### Web UI

| Old form | New form | Migration step |
| --- | --- | --- |
| deep link `?bean=<id>` | `?item=<id>` | Update bookmarks and shared links. |
| `localStorage` keys `beans-planning-chat`, `beans-changes-pane`, `beans-filter-text`, `beans-split-<panel>` | `lif-planning-chat`, `lif-changes-pane`, `lif-filter-text`, `lif-split-<panel>` | None required. There is no fallback, so the web UI starts once with its default layout (panes, sizes, filter text); rearrange it once. The old keys stay unused in the browser and can be cleared in the browser's developer tools. |

### Error code

| Old form | New form | Migration step |
| --- | --- | --- |
| `NO_BEANS_DIR` (the `code` of a `--json` error) | `NO_LIF_DIR` | Update every script or agent that matches on the code. |

### Stale search cache

The search index moved from `~/.beans/index/` to `~/.lif/index/`. Lif builds the new index on demand, so no step is required. The old directory is no longer used and is safe to delete: `rm -rf ~/.beans/index`.

### Claude Code plugin, hooks and command

| Old form | New form | Migration step |
| --- | --- | --- |
| plugin `beans-prime@beans` | plugin `lif-prime@lif` | The old plugin's hooks call `beans prime`, which no longer exists. In Claude Code run `/plugin uninstall beans-prime@beans`, remove the old marketplace with `/plugin marketplace remove beans`, add the Lif marketplace with `/plugin marketplace add rierek/lif`, then run `/plugin install lif-prime@lif`. |
| hand-written hooks in `.claude/settings.json` that run `beans prime` | hooks that run `lif prime` | Edit both the `SessionStart` and the `PreCompact` hook command. |
| slash command `/beans-release-notes` | `/lif-release-notes` | Use the new name. |

### OpenCode plugin

| Old form | New form | Migration step |
| --- | --- | --- |
| `.opencode/plugin/beans-prime.ts` | `.opencode/plugin/lif-prime.ts` | Copy `lif-prime.ts` from the repository into `.opencode/plugin/` and delete the old `beans-prime.ts`. A stale copy runs `beans` and silently does nothing. |

### zsh widget

| Old form | New form | Migration step |
| --- | --- | --- |
| file `extras/zsh/beans-pick.zsh` | `extras/zsh/lif-pick.zsh` | Replace the `source` line in `.zshrc` with the path of the new file. |
| widget `beans-pick-widget` | `lif-pick-widget` | Rebind it if you bound the widget by name. |
| variable `BEANS_PICK_KEYBIND` | `LIF_PICK_KEYBIND` | If you bound a custom key, set the new variable before the `source` line; the old variable is no longer read. The default stays `^[i`. |

### Workspace port variables

Where `lif-serve` exports a workspace port to shell and run sessions, it exports `LIF_WORKSPACE_PORT` and, with the same value, the old variable `BEANS_WORKSPACE_PORT`. User scripts keep working; read `LIF_WORKSPACE_PORT` in new scripts.

## Optional rename of legacy data names

The rename in this section is optional. Lif reads legacy data names in place and never renames them for you, and nothing stops working if you leave them. Item files and ID prefixes do not change in either case: only the container names listed here move, the files inside the store keep their names, contents and IDs.

`lif check` reports every legacy name in use as a warning, in a `Legacy names` section, and never changes its exit code for them. These are the warnings (paths shown here as `<dir>`, `<path>` and `<id>`); the store directory warning ends in a different clause depending on where the directory name comes from, see the table below:

```
Legacy names
  ! config file .beans.yml is a legacy name: run `git mv .beans.yml .lif.yml` in <dir>
  ! section key beans: in .beans.yml is a legacy name: rename the key to lif:
  ! store directory .beans is a legacy name: run `git mv .beans .lif` in <dir>, then change path: .beans to path: .lif in .beans.yml, and run `git mv .beans.yml .lif.yml`
  ! environment variable BEANS_PATH is a legacy name: export LIF_PATH instead
  ! worktree root <path> is a legacy name: remove the remaining worktrees, then delete <path>; new worktrees go to ~/.lif/worktrees/<name>
  ! worktree <path> is on legacy branch beans/<id>: finish or remove it, or run `git branch -m beans/<id> lif/<id>`
```

Each warning has one step:

| Legacy name | Step |
| --- | --- |
| config file `.beans.yml` | `git mv .beans.yml .lif.yml` |
| section key `beans:` | Edit the config file and change the top-level key `beans:` to `lif:`. |
| store directory `.beans` | `git mv .beans .lif`, then change the name wherever it is set. The warning says which: for a name from the config file, `path: .beans` to `path: .lif` in that file (plus `git mv .beans.yml .lif.yml` if the config file is legacy too); for a name from `LIF_PATH` or `BEANS_PATH`, point the variable at the new directory (warning text `then point LIF_PATH at the new directory`); for a name from the flag, pass the new directory to `--lif-path` (warning text `then pass the new directory to --lif-path`); for the default name, nothing more. Other tools that name the directory, such as CI jobs or ignore files, need the same edit. |
| variable `BEANS_PATH` set and non-empty | Export `LIF_PATH` with the same value and unset `BEANS_PATH`. With both set and no config file found, `lif` fails with an error that names both. |
| worktree root `~/.beans/worktrees/<project>` in use | Finish or remove each remaining worktree in that root, then delete the root directory. New worktrees go to `~/.lif/worktrees/<project>/`. |
| a worktree on a `beans/<id>` branch | Finish or remove the worktree, or rename its branch with `git branch -m beans/<id> lif/<id>`. New worktrees are created on `lif/<id>`. |

A complete rename of a project that uses all three file-system names is, in this order:

```sh
git mv .beans .lif
git mv .beans.yml .lif.yml
# in .lif.yml: rename the top-level key beans: to lif:, and change path: .beans to path: .lif
lif check
```

Run `lif check` afterwards; it must print no `Legacy names` section. Having a legacy name and its current name side by side at the same level (for example both `.beans.yml` and `.lif.yml`) is an error that names both, not a state to pass through; see [Configuration](../../reference/configuration/) for the exact errors and for when each one applies.

## After upgrading: validate your project

Run `lif check` against your project to validate configuration and item integrity, including broken links, self-references, and circular dependencies; `lif check --strict` additionally treats policy warnings as failures, and `lif check --fix` can automatically remove broken links and self-references (cycles still require manual resolution).

Run `lif path` to confirm which data directory the binary resolved. Precedence is `--lif-path`, then a found or explicitly named `.lif.yml` (or the legacy config file, when no `.lif.yml` exists) and its anchored path, then `LIF_PATH` only when no config was found, then the default `.lif` directory.

Without `lif.anchor`, a secondary worktree resolves `lif.path` relative to its own config file location and therefore uses a separate store. Set `lif.anchor: repo-root` to opt into sharing the main worktree's store. Lif also auto-renames the older `worktrees/` and `conversations/` directories inside the store to `.worktrees/` and `.conversations/` when loading a project that still has the old names.

## Verifying a specific command against your data

For any command that mutates items, verify it first against a disposable copy of your data rather than your real store, for example:

```
lif --lif-path /tmp/lif-upgrade-check/.lif init
lif --lif-path /tmp/lif-upgrade-check/.lif check
```

The explicit `--lif-path` flag overrides both your project configuration and the `LIF_PATH` environment variable, which makes it the safest way to point any exploratory or destructive command at a throwaway directory instead of your real project.

## Related pages

- [Installation](../installation/)
- [Troubleshooting](../troubleshooting/)
- [Configuration](../../reference/configuration/)
- [About](../../about/)
- [Agent onboarding](../agent-onboarding/)
- [Web server](../../reference/web-server/)
