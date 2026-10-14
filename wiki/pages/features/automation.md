---
title: "Automation and agents"
description: "JSON output, GraphQL and lif prime for scripts and coding agents."
---

## Purpose
Shows how scripts and coding agents use Lif in a machine-readable way: `--json`, GraphQL, `lif prime` and `lif promote`. Contains the how-to guides for agent hooks and for validation after automated changes.

## Audience
Script authors, operators of coding agents and people who connect an agent to their backlog. They need concrete calls, exit codes and the known gaps.

## Required sections

### What
- `--json` on many commands (for example `list`, `next`, `create`, `complete`, `progress`, `roadmap`, `check`); `lif graph` uses `--format json`; in JSON mode the human-readable lines are omitted, errors carry codes such as `NOT_FOUND` or `VALIDATION_ERROR`.
- GraphQL without a server: `lif graphql '<query>'` (alias `lif query`), also via stdin; with a server through `lif-serve` at `/api/graphql` with the playground at `/playground`.
- `lif prime` prints a prompt that teaches a coding agent how to use Lif; runs without a loaded store.
- `lif promote <artifact> <finding-id>...` creates items from reviewed review findings; writes nothing if a chosen entry is faulty, and skips entries already taken over.
- Exit codes: 0 on success, 1 on error; `lif check` returns 1 on at least one problem; there is no exit code 2.
- Source: FACTS.md §1.0, §1.1 (`lif list`, `lif promote`, `lif prime`, `lif graph`), §6.8, §6.11, §7.5

### Why
- Lif can take part in automated workflows instead of only being operated manually.
- The store needs no server; scripts and agents work directly on the files.
- Agents learn Lif through `lif prime` and `--help`, not through prior knowledge.
- Source: CONCEPT.md §4.5; FACTS.md §7.5, §1.0

### How it looks
- `lif list --json` with `jq` in a pipeline; options `--full` (body in the JSON) and `--quiet` (IDs only).
- GraphQL one-liner with `lif graphql '{ items { id title status } }'`, variables with `-v`, query via stdin, `--schema`.
- Mutation via CLI: `createItem` with `title` and `type`; note on the status gap from Limits.
- Promote flow: review artifact, finding IDs, result as items; check the artifact format in phase 5 (TODO: verify)
- Run all examples for real in phase 5 and paste real outputs (TODO: verify)
- Source: FACTS.md §1.1 (`lif list`, `lif promote`), §7.5; (TODO: verify) for example outputs and promote artifact: run the commands in phase 5, check the schema in `internal/commands/findings-schema.json`

### Agent hooks
- How-to (agent side): A session hook runs `lif prime` at session start and before context compaction; this keeps the agent on track.
- The project uses Claude Code hooks for this, which call `lif prime` on `SessionStart` and `PreCompact`.
- Take the configuration snippet for the hooks from the template and check it against the current state (TODO: verify)
- Note: The example applies to Claude Code; other agents need their own hook mechanisms (TODO: verify)
- Commit message convention with item ID so agent work stays traceable: convention, not proven (TODO: verify)
- Link to drift avoidance in Agent onboarding (`overview/agent-onboarding`).
- Source: FACTS.md §6.10; (TODO: verify) for snippet, other agents and commit convention: read `.claude/settings.json` and `extras/claude/plugins/lif-prime/hooks/hooks.json` in the checkout

### Validation after automated changes
- Run `lif check` after automated changes; exit 1 reports at least one problem, exit 0 none.
- `--strict` counts policy warnings (missing required fields, unknown commit SHAs) as problems.
- `--fix` removes broken links and self-references; cycles, configuration errors, attachment orphans and policy warnings remain.
- `--json` returns `success`, `config_errors`, `item_issues` (`broken_links`, `self_links`, `cycles`) and further fields.
- Statement of the prompt: check configuration, links, front matter and policies with `lif check` before the automation continues.
- Limit: `lif check` checks neither rank, status, priority nor tags; an agent that writes files directly stays unchecked there.
- Source: FACTS.md §6.7, §6.8, §6.9, §6.10; DESIGN.md D40

### Limits
- Known bug: GraphQL `createItem` without `status` leaves the status empty; only `lif create` sets the default `todo`.
- On a GraphQL error the CLI prints an error message (`graphql: <message>`); whether partial data is returned is not proven (TODO: verify)
- Server-only parts of the schema (agent, terminal, worktrees) fail in the CLI with messages such as "agent manager not available".
- Subscriptions run only over the WebSocket of `lif-serve`, not through `lif graphql`.
- Files written directly bypass the rules; `lif check` covers only part of them.
- Source: FACTS.md §7, §7.3, §7.5, §6.9; DESIGN.md D43; (TODO: verify) for partial data on errors: check `internal/commands/graphql.go`

### Reference links
- GraphQL (`reference/graphql`): schema, queries, mutations, errors.
- JSON schemas (`reference/json-schemas`): payloads, error codes, exit codes.
- `lif graphql`, `lif prime`, `lif promote`, `lif check` (`reference/commands/lif-graphql`, `reference/commands/lif-prime`, `reference/commands/lif-promote`, `reference/commands/lif-check`).
- Web server (`reference/web-server`): HTTP endpoints of `lif-serve`.
- Agent onboarding (`overview/agent-onboarding`): recommended hooks.
- Source: DESIGN.md D24, D27, D28
