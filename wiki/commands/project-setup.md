# Project Setup and Introspection

This page covers the commands that initialize a Lif project and introspect the CLI itself: `init`, `path`, `prime`, `pick`, `version`, `help`, and `completion`. Use it to bootstrap a new project, hand the CLI to an AI agent, or wire up shell completions.

## `lif init`

`lif init` creates a `.lif` directory and writes a fresh `.lif.yml` config file in the current directory, giving the project a store to write items into. Run it once per project root: rerunning it without `--lif-path` overwrites `.lif.yml` with generated defaults, so preserve or review any project-specific configuration first. A project that already uses the legacy data names keeps them; see [Legacy names](../configuration.md#legacy-names).

Flags: `--profile string` selects a starting type profile (`classic`, `complex`, `simple`, `todo`) that determines the default set of item types and statuses written into `.lif.yml`; `--json` prints the result as JSON (`success`, `message`, `path`) instead of the human-readable `Initialized Lif project` confirmation line. `--profile` cannot be combined with `--lif-path`, because an explicit store path skips writing `.lif.yml`.

```sh
mkdir demo
cd demo
lif init --profile classic
```

See `../project-profiles.md` for what each profile actually configures.

## `lif path`

`lif path` prints the store path the current invocation resolves to, which is useful for scripts that should not reimplement the CLI's resolution order. An explicit relative `--lif-path` is returned unchanged; config- and default-derived paths are normally absolute. Resolution follows `--lif-path`, then a found `.lif.yml` (including `--config`), then `LIF_PATH` when no config was found, then the default `.lif` directory.

```
lif --lif-path ./demo/.lif path
```

## `lif prime`

`lif prime` outputs a prompt that primes AI coding agents on how to use the `lif` CLI to manage project issues, covering recipes for starting work, finding work with `next`, handling blocked items, completing work, scrapping work, and tagging. It has no flags beyond the global ones and is meant to be pasted into an agent's system prompt or referenced from a project's `AGENTS.md`/`CLAUDE.md` file; see `../agent-integration.md` for the full recipe set and how it is intended to be used.

```
lif prime
```

## `lif pick`

`lif pick` draws an interactive, fuzzy-filterable list of every item in the resolved store on the controlling terminal. Selecting one prints exactly that item's ID to stdout and exits 0; aborting the picker, an empty candidate list, or non-interactive stdin produce no stdout output and a non-zero exit, so scripts can capture the result with command substitution.

Flags: `--scope string` restricts candidates to a comma-separated list of item types; `--line string` and `--cursor int` (default `-1`) pass a partial command line and the cursor byte offset so the picker can derive its scope from the verb or `--parent` position under the cursor. The zsh widget below supplies those two flags for you.

At a `--parent` value the picker offers only items that can be the parent: the type rules of `--parent` completion apply, the edited item and its descendants are left out, and so is every item whose status is an archive status (by default `completed` and `scrapped`; a `statuses:` entry with `archive: false` keeps a status in). Typing an archived item's ID after `--parent` is still accepted.

```
id=$(lif pick --scope epic,milestone)
```

## `lif version`

`lif version` shows version information for the binary: the version string, commit, and build date on the first line, then a `custom front matter: preserved` line. Builds made from a source checkout add a `tree:` line naming the checkout and whether it is the main tree or a worktree. `--json` emits the same information as a JSON object with `version`, `commit`, `date`, and `custom_front_matter` fields, plus `tree` and `tree_kind` for source-checkout builds.

```
lif version --json
```

## `lif help`

`lif help [command]` prints the same help text a command would show for `--help`, and `lif help` alone lists every top-level command; it takes no flags of its own beyond `-h`/`--help`.

```
lif help create
```

## `lif completion`

`lif completion [shell]` generates a shell autocompletion script for `bash`, `fish`, `powershell`, or `zsh`; each shell subcommand's own `--help` documents exactly how to source or install the generated script for that shell. The parent command has no flags beyond the global ones and exists only to group the four shell subcommands.

```
source <(lif completion bash)
```

`extras/zsh/lif-pick.zsh` ships alongside this the same way: a source-able zsh file defining the `lif-pick-widget` ZLE widget, bound to `Alt+i` (`^[i`) by default, which calls `lif pick` and inserts the chosen ID at the cursor. Set `LIF_PICK_KEYBIND` to a `bindkey` keyspec before sourcing the file to use another key. Copy it out of the source checkout and source it by its absolute path:

```
source /path/to/lif/extras/zsh/lif-pick.zsh
```

## Related documentation

- [Installation](../installation.md)
- [Quick Start](../quick-start.md)
- [Configuration](../configuration.md)
- [Agent Integration](../agent-integration.md)
- [Project Profiles](../project-profiles.md)
- [Inspection and Search](inspection-and-search.md)
- [Lifecycle](lifecycle.md)
