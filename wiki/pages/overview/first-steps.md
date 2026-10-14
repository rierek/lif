---
title: "First steps"
description: "Create a store, pick a preset and work your first items."
---

## Purpose
Walks through the first hour: create a store, choose a preset, create first items, view, edit and complete them. The same walk-through is also offered as a prompt that a coding agent runs together with the user. Scope boundary: everything that is configurable is in Reference: Configuration (`reference/configuration`).

## Audience
New users right after installation. They want a working workflow in the terminal without prior knowledge.

## Required sections

### Initialise a store
- Shows `lif init` in the project folder; creates `.lif.yml` and the store directory `.lif/`.
- Explains what `init` writes: `lif.prefix` = `<folder name>-`, `project.name` = folder name, `.gitignore` in the store with `.conversations/`.
- Explains that `init` runs without an existing store; all other commands need one.
- Shows the error without a store: `no .lif directory found at <path> (run 'lif init' to create one)`.
- Shows `lif path` to check the resolved store directory.
- Links to Store management (`overview/store-management`) for several projects, worktrees and a shared store.
- Source: FACTS.md §1.1 (`init`, `path`), §1.0 (bootstrap skip), §2.4, §4.2

### Choose a preset
- Table of the four presets for `lif init --profile`: `classic`, `todo`, `simple`, `complex` with types and ranks.
- `classic`: milestone, epic, feature, bug, task (up to four ranks); `todo`: only `task`.
- `simple` and `complex`: ranks 1, 2 and 4 (three levels); the `bucket` type is hidden from the roadmap (`roadmap: false`).
- Note: `init --profile` sets `types_exclusive` and writes the types into `.lif.yml`; later changes by hand are possible.
- Note: without `--profile` the built-in types apply. (TODO: verify whether `init` without `--profile` writes a type table.)
- Links to Reference: Configuration (`reference/configuration`) for the full preset definitions.
- Source: FACTS.md §1.1 (`init`), §3, §9(a), §9(b). `init` without `--profile` is `TODO: verify` (check with `lif init` in an empty folder in phase 5).

### Create your first items
- Shows `lif create "Title" -t epic` and `lif create "Title" -t task --parent <id>` with example output (phase 5: run for real).
- Explains the built-in defaults: status `todo`, type `task`, no default for `priority`; a title without an argument becomes "Untitled".
- Flags in short: `--status`, `--type`, `--priority`, `--tag`, `--parent`, `--blocked-by`, `--body`/`--body-file`.
- Explains item IDs: prefix plus 4 characters from `0-9a-z`; the short ID without the prefix is accepted everywhere.
- Shows the rule violation: a task under a task is rejected (`task items can only have milestone, epic, or feature as parent, not task`).
- Source: FACTS.md §1.0 (short IDs), §1.1 (`create`), §2.3, §2.4, §3, §3.1

### Define your defaults
- Explains that the defaults for new items can be defined in `.lif.yml` and changed at any time: `lif.default_status` (built-in `todo`) and `lif.default_type` (built-in `task`; if the type table has no `task`, the first type is used).
- Names the ID defaults in the same section: `lif.prefix` (`init` writes `<folder name>-`) and `lif.id_length` (built-in 4).
- Shows a short excerpt of the `lif:` section of `.lif.yml` (phase 5: copy it from a fresh `lif init`).
- Explains that flags on `lif create` (`--status`, `--type`) override the defaults for one item.
- Known limit: there is no setting for a default priority; a new item gets none and is loaded as `normal`.
- Links to Reference: Configuration (`reference/configuration`) for all keys.
- Source: FACTS.md §4.3 (`lif.default_status`, `lif.default_type`, `lif.prefix`, `lif.id_length`), §2.3 (defaults); `pkg/config/config.go:452-453, 470-471, 725-755`; DESIGN.md D54

### Look at them
- `lif list` (table) and `lif list --view tree`; important filters: `--status`, `--type`, `--tag`, `--ready`, `--search`.
- `lif show <id>`: formatted in the terminal, raw Markdown text when piped or with `--raw`.
- `lif roadmap` and `lif milestones` for an overview; `lif progress` for counts and percentages.
- `lif next`: the highest-priority ready item (the definition of "ready" is in Concepts, `overview/concepts`).
- `--json` on `list`, `show`, `next` for scripts; link to JSON schemas (`reference/json-schemas`).
- Source: FACTS.md §1.1 (`list`, `show`, `roadmap`, `milestones`, `progress`, `next`), §5.2

### Work an item
- Lifecycle as a short command sequence: `lif start <id>` → `lif complete <id> --summary "..."` or `lif scrap <id> --reason "..."`.
- `complete --commit HEAD` stores the commit SHA; `--summary` appends the section "## Summary of Changes"; `scrap --reason` is required.
- Several IDs in one call are possible; all are checked before the first write.
- `lif archive` moves completed and scrapped items to `archive/`; they stay addressable.
- Useful: `lif update <id> --priority high`, `lif tag <id> --tag x`.
- Source: FACTS.md §1.1 (`start`, `complete`, `scrap`, `archive`, `update`, `tag`), §2.3, §2.4

### Open the Liff in your editor
- Introduces the word "Liff" once in running text and links to Reference: Item file format (`reference/item-file-format`) as the definition.
- Shows `lif path` and an item file with front matter (file name `<id>--<slug>.md`).
- Editing by hand is allowed; run `lif check` afterwards.
- Known limit: manual changes to `parent:`, `type:` and `status:` are not checked on load; `lif check` does not report rank, status, priority and tag errors (D40).
- What `lif check` catches: unknown type, broken links, self-references, cycles, orphaned attachment directories; exit code 1 on a finding.
- Source: FACTS.md §2.1, §2.4, §3.2, §6.8, §6.9, §9(d), §9(e); DESIGN.md D40, D47

### Agent prompt template
- A prompt the user pastes into a coding agent (Claude Code, OpenCode, others). The agent walks the user through the same steps as this page: init, preset choice, first items, defaults.
- Prerequisites: Lif is installed (`overview/installation`) and the agent can run shell commands. The agent learns the presets from `lif init --help` and the commands from `lif prime`.
- Rules inside the prompt: one step at a time, say what is about to happen, ask for a decision where there is one, wait for the answer, change nothing before the user agrees.
- Order matters: `lif prime` prints nothing before `lif init` (see Agent onboarding), so it comes right after the init step.
- Shows the prompt in a code block with a copy button; the draft below is the working proof, phase 5 tests the final wording with a real agent run (`TODO: verify`).
- Links to Agent onboarding (`overview/agent-onboarding`) for session hooks.
- Source: FACTS.md §1.1 (`version`, `init`, `prime`, `create`, `list`, `check`, `next`), §4.3; `internal/commands/prime.go:71-104` (silent without a config); DESIGN.md D54

```text
Help me set up Lif, a Markdown-native work tracker, in this project. Work through the steps one at a time. Before each step tell me what you are about to do, ask for my decision where there is one, and wait for my answer. Change nothing before I agree.

1. Run `lif version`. If the command is missing, stop and tell me to install Lif first.
2. Run `lif init --help`, then ask me which preset fits this project: classic, todo, simple or complex. Explain each in one line. Run `lif init --profile <preset>` in the project root and show me `lif path`.
3. Run `lif prime` (it prints nothing before `lif init`) and follow it for every command from now on.
4. Show me the `lif:` section of `.lif.yml`. Ask whether new items should get another default status (`lif.default_status`) or default type (`lif.default_type`), another ID prefix (`lif.prefix`) or another ID length (`lif.id_length`). Apply my answers and run `lif check`.
5. Ask me what this project is about. Create a first item for it with `lif create`, then two or three smaller items below it with `--parent` (plain items if the preset has one level only). Show the result with `lif list --view tree`.
6. Run `lif next`, tell me in five lines how I work with Lif from here, and ask me what to start with.
```

### Next steps
- Concepts (`overview/concepts`) for the mental model.
- Features (`features/index`) and Use cases (`use-cases/index`) for workflows.
- Reference: Commands (`reference/commands/index`) for all commands, Reference: Configuration (`reference/configuration`) for settings.
- Agent onboarding (`overview/agent-onboarding`) to wire a coding agent to the store; Store management (`overview/store-management`) for several projects.
- TUI: Start and repositories (`reference/tui/start-and-repositories`) as an alternative to the CLI.
- Source: DESIGN.md D29, D44
