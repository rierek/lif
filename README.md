<p align="center"><img src="wiki/assets/lif-banner.webp" alt="Lif" width="720"></p>

# Lif

[![License](https://img.shields.io/github/license/xRiErOS/lif?style=for-the-badge)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/xRiErOS/lif?style=for-the-badge)](https://go.dev/)

**A file-based issue tracker for developers, product teams, and AI coding agents.**

Lif stores work as Markdown files beside the code it describes. Humans, scripts, and coding agents share one Git-versioned source of truth instead of synchronizing a second task system. Each piece of work is an *item*: one Markdown file with structured YAML front matter.

Lif originated as a fork of [hmans/beans](https://github.com/hmans/beans) by Hendrik Mans and is licensed under the Apache License 2.0. It preserves the original local-first, agent-first idea and extends it for richer planning models, safer automation, and presentation-ready terminal workflows.

![A scoped Lif roadmap rendered in a terminal](wiki/assets/roadmap.webp)

[Install](#installation) · [Quick start](#quick-start) · [Choose a profile](#choose-a-project-profile) · [Browse the documentation](wiki/index.md) · [Upgrade guide](wiki/compatibility-and-upgrading.md)

## Why Lif

Hendrik Mans' original project established the core idea: issues should be plain files that live with the repository and remain equally accessible to people and coding agents. Lif builds on that Apache-2.0 codebase with clear attribution; see the [fork lineage](wiki/fork-lineage.md).

Real product work needed more than a flat task loop. Lif adds configurable project profiles, richer planning types, scoped roadmaps, consistent terminal rendering, batch lifecycle commands, ordering and rename workflows, completion policies, worktree-aware storage, structured automation interfaces, and dedicated browser and terminal applications.

The goal is not to turn Lif into a hosted project-management platform. Lif remains a small, inspectable data and workflow layer that composes with Git, shells, editors, coding agents, and optional user interfaces.

[Read the fork lineage](wiki/fork-lineage.md) · [Review compatibility and upgrading](wiki/compatibility-and-upgrading.md)

## Feature overview

### Markdown-native issue tracking

- Store each item as a Markdown file with structured YAML front matter.
- Review task changes with the same Git tools used for code.
- Keep active work and archived project memory readable without a service or proprietary export.
- Configure types, statuses, priorities, display rules, and lifecycle policies per repository.

### Planning that fits the project

- Select a bundled `classic`, `todo`, `simple`, or `complex` profile at initialization.
- Model milestones or releases, thematic work, executable stories and tasks, bugs, and roadmap-excluded buckets.
- Connect work through parent, child, blocking, and blocked-by relationships.
- Order siblings explicitly without rewriting the complete store.

### Human-readable terminal views

- Render `list`, `milestones`, and `roadmap` as trees or tables with a consistent visual language.
- Switch roadmap output between terminal presentation and portable Markdown.
- Scope roadmaps and progress reports to the part of the hierarchy that matters now.
- Preserve machine-readable JSON modes for scripts and agents.

| Hierarchical list | Item detail |
| --- | --- |
| ![lif list tree view](wiki/assets/list.webp) | ![lif show detail view](wiki/assets/show.webp) |

### Agent-native workflows

- Run `lif prime` to give a coding agent the current project's types, statuses, relationships, policies, and command contract.
- Use `lif list --ready` or `lif next` to select actionable work rather than merely open work.
- Query only the required fields through GraphQL and consume stable JSON output where supported.
- Validate configuration, links, front matter, and policies with `lif check` before automation proceeds.

### Complete work lifecycle

- Create and inspect work with `create`, `list`, `show`, and `next`.
- Move work through `start`, `complete`, `scrap`, and `archive`.
- Organize work with `update`, `tag`, `order`, and cascading `rename` modes.
- Report plans and outcomes with `roadmap`, `milestones`, `progress`, and `graph`.

[Explore every capability](wiki/feature-overview.md) · [Open the full command index](wiki/commands/index.md)

## Choose a project profile

Profiles are expanded into the repository's `.lif.yml` when `lif init --profile` runs. Later releases do not silently replace that explicit project configuration.

### `classic`

- **Existing project migration:** retain the original `milestone → epic → feature → task/bug` hierarchy when adopting Lif.
- **Milestone-driven software delivery:** organize checkpoints into thematic epics, user-facing features, and executable work.

### `todo`

- **Markdown todo list:** track a flat set of tasks without planning containers or hierarchy overhead.
- **AI agent backlog:** give a coding agent a small, unambiguous queue grouped through tags.

### `simple`

- **Small project roadmap:** plan milestones while keeping possible future work in roadmap-excluded buckets.
- **Feature and bug tracking:** separate thematic epics and user-facing features from executable tasks and defects.

### `complex`

- **Release planning for product teams:** group everything that ships together under an explicit release.
- **Customer-value prioritization:** distinguish new features, visible improvements, internal chores, demonstrable stories, bugs, and tasks.

[Compare exact profile hierarchies and use cases](wiki/project-profiles.md)

## Installation

Pick the channel that matches your platform. All five install the same two binaries: `lif` (CLI and terminal UI, started with `lif tui`) and `lif-serve` (web UI and GraphQL API).

### Homebrew (macOS)

````sh
brew install xRiErOS/lif/lif
````

### Install script (Linux, macOS)

````sh
curl -fsSL https://raw.githubusercontent.com/xRiErOS/lif/main/install.sh | sh
````

### Nix (Linux, macOS)

````sh
nix profile install github:xRiErOS/lif
````

Run it without installing with `nix run github:xRiErOS/lif -- version`, or open a temporary shell with `nix shell github:xRiErOS/lif`. The flake builds the frontend as part of the package, so `lif-serve` from this channel serves the web UI.

### go install (Linux, macOS, Windows)

````sh
go install github.com/xRiErOS/lif/cmd/lif@latest
````

A `lif-serve` built this way has no embedded web assets (only a placeholder is tracked in the source tree) and serves 404 for the browser UI. Use Homebrew, the install script, Nix, or a release archive if you need `lif-serve` with the web UI.

### Build from source

````sh
git clone https://github.com/xRiErOS/lif.git
cd lif

mise install
mise run setup
mise run build

install -d "$HOME/.local/bin"
install -m 755 lif lif-serve "$HOME/.local/bin/"
export PATH="$HOME/.local/bin:$PATH"

lif version
````

[Installation details and prerequisites](wiki/installation.md) · [Upgrade guide for existing installs](wiki/compatibility-and-upgrading.md)

## Quick start

Initialize a disposable project with the smallest profile and create the first ready task:

````sh
mkdir lif-demo
cd lif-demo

lif init --profile todo
lif create "Ship the first documented change" --type task --priority high --status todo
lif list --ready
lif next
````

Lif creates `.lif.yml` plus a `.lif/` directory. Commit both with the repository so task history and code history travel together.

For a hierarchical roadmap, initialize `simple` or `complex`, create top-level planning containers, and connect executable work with `--parent`.

[Follow the complete hierarchy-to-roadmap tutorial](wiki/quick-start.md)

## Use Lif with coding agents

The smallest portable integration is a repository instruction that makes the installed binary describe itself and the current project:

````markdown
Before managing project work, run `lif prime` and follow its project-specific instructions.
````

`lif prime` is preferable to a copied command manual because its output reflects the repository's configured types, statuses, priorities, relationships, and completion policies. Agents can then use natural-language requests while operating through exact CLI or GraphQL contracts.

Examples of useful requests:

````text
Inspect the ready items and recommend the highest-value next action.

Create a bug for the failure you found, relate it to the affected feature, and mark it blocked by the root-cause task.

Summarize progress for the current release and identify incomplete descendants.
````

[Configure Claude Code, OpenCode, and generic agents](wiki/agent-integration.md) · [Read the data model](wiki/data-model.md)

## Browser workspace with `lif-serve`

`lif-serve` runs the optional planning workspace and GraphQL service over the same `.lif/` store used by the CLI. The browser offers backlog, board, and workspace views without introducing a second database.

````sh
lif-serve --lif-path .lif
````

![lif-serve board view](wiki/assets/serve-board.webp)

Network binding, CORS, routes, and authentication posture matter before exposing the service beyond a local machine.

[Configure the Web UI and API](wiki/web-ui-and-api.md)

## Terminal interfaces

The terminal UI is built into the main binary. Run `lif tui` to browse and manage items interactively, or `lif tui <repo-path>` to open a specific repository. It replaces the former separate TUI binary and the `bt` (beans-tui) companion.

![lif tui: browse view with item detail](wiki/screenshots/browse.png)

[Read the terminal UI guide](wiki/tui-companion.md) · [Review the separate binary `lif-serve`](wiki/commands/separate-binaries.md)

## Full feature reference

The reference is grouped by user intent instead of reproducing one long `lif --help` block:

- [Project setup](wiki/commands/project-setup.md): `init`, `path`, `prime`, `pick`, `version`, `help`, `completion`.
- [Inspection and search](wiki/commands/inspection-and-search.md): `list`, `show`, `next`.
- [Lifecycle](wiki/commands/lifecycle.md): `create`, `start`, `complete`, `scrap`, `archive`, `delete`.
- [Organization and relationships](wiki/commands/organization-and-relations.md): `update`, `tag`, `order`, `rename`.
- [Planning and reporting](wiki/commands/planning-and-reporting.md): `roadmap`, `milestones`, `progress`, `graph`.
- [Querying and automation](wiki/commands/querying-and-automation.md): `graphql`, JSON output, and structured errors.
- [Validation and maintenance](wiki/commands/validation-and-maintenance.md): `check`, `promote`, integrity rules, and policies.
- [Terminal UI](wiki/tui-companion.md): `lif tui`.
- [Separate applications](wiki/commands/separate-binaries.md): `lif-serve`.

The installed binary remains authoritative for version-specific options:

````sh
lif --help
lif <command> --help
lif prime
````

## Documentation

- [Documentation index](wiki/index.md) — route to every guide and reference page.
- [Configuration reference](wiki/configuration.md) — stores, anchors, profiles, custom types, display, and policies.
- [Data model and file format](wiki/data-model.md) — front matter, bodies, IDs, relationships, order, and archive.
- [Compatibility and upgrading](wiki/compatibility-and-upgrading.md) — upgrade guide for existing installs and projects.
- [Troubleshooting](wiki/troubleshooting.md) — diagnose store discovery, ready work, policies, configuration, and server startup.

## Project status

Lif is an actively maintained independent continuation of the project it originated from. Features and file-format contracts may still evolve; review the commit history and the compatibility guide before upgrading a shared store.

Issues, discussions, and focused pull requests are welcome at [xRiErOS/lif](https://github.com/xRiErOS/lif). Pull requests target the current `milestone/0.N` branch, not `main`; see [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

Lif originated as a fork of [hmans/beans](https://github.com/hmans/beans), created by [Hendrik Mans](https://github.com/hmans). This project is possible because that work was released under the Apache License 2.0.

## License

Licensed under the [Apache License 2.0](LICENSE).
