---
title: "Automation and agents"
description: "JSON output, GraphQL and lif prime for scripts and coding agents."
---

Scripts and coding agents use Lif through `--json`, GraphQL, `lif prime` and `lif promote`. No server is needed. Measured on Lif 0.14, 2026-10-14.

## What

- Many commands take `--json`, for example `list`, `next`, `create`, `complete`, `progress`, `roadmap`, `check`. `lif graph` uses `--format json`. In JSON mode the human-readable lines are omitted. Errors carry codes such as `NOT_FOUND` or `VALIDATION_ERROR`.
- GraphQL without a server: `lif graphql '<query>'` (alias `lif query`), also via stdin. With a server, `lif-serve` offers `/api/graphql` and a playground at `/playground`.
- `lif prime` prints a prompt that teaches a coding agent how to use Lif. It runs without a loaded store.
- `lif promote <artifact> <finding-id>...` creates items from the findings of a review artifact. It writes nothing if a chosen finding is malformed. It skips findings that were already taken over.
- Exit codes: 0 on success, 1 on error. `lif check` returns 1 on at least one problem. Normal errors do not use exit code 2.

## Why

- Lif can take part in automated workflows. It is not only a tool you operate by hand.
- The store needs no server. Scripts and agents work on the files.
- Agents learn Lif from `lif prime` and `--help`, not from prior knowledge.

## How it looks

A pipeline with `jq`. `--quiet` prints IDs only, `--full` adds the body to the JSON:

```bash
lif list --json | jq -c '.[] | {id, title, status}'
lif list --json --full | jq -r '.[] | select(.body) | .body'
lif list --quiet
```

A GraphQL one-liner (real output):

```text
$ lif graphql '{ items { id title status } }'
{
  "items": [
    {
      "id": "ft-2gae",
      "title": "Ship v1",
      "status": "todo"
    }
  ]
}
```

Other options: variables with `-v`, a query on stdin, `--schema`. See [`lif graphql`](../../reference/commands/lif-graphql/).

A mutation through the CLI works the same way, for example `createItem` with `title` and `type`. Set `status` yourself: see Limits.

`lif promote` takes a review artifact that follows the findings schema. It validates every selected record first and creates one item per record. The artifact format is not documented on the command page. TODO: finding (promote artifact format undocumented). See [`lif promote`](../../reference/commands/lif-promote/). `lif promote` can still fail part-way [z69m].

<details>
<summary>How to set up agent hooks</summary>

A session hook runs `lif prime` at session start and before context compaction. The agent then starts each session with the Lif instructions. The Lif repository does this for Claude Code with `SessionStart` and `PreCompact` hooks:

```json
{
  "hooks": {
    "SessionStart": [{ "hooks": [{ "type": "command", "command": "lif prime" }] }],
    "PreCompact":   [{ "hooks": [{ "type": "command", "command": "lif prime" }] }]
  }
}
```

This snippet is the hook file shipped in `extras/claude/plugins/lif-prime/hooks/hooks.json` in the Lif repository. It applies to Claude Code. Other agents need their own hook mechanism, or you paste the `lif prime` output once. [Agent onboarding](../../overview/agent-onboarding/) covers hooks and drift prevention.

Convention, not enforced by Lif: put the item ID in the commit message so agent work stays traceable.

</details>

<details>
<summary>How to validate after automated changes</summary>

Run `lif check` after every automated change.

- Exit 1 means at least one problem. Exit 0 means none.
- `--strict` counts policy warnings (missing required fields, unknown commit SHAs) as problems.
- `--fix` removes broken links and self-references. Cycles, configuration errors, attachment orphans and policy warnings remain.
- `--json` returns `success`, `config_errors`, `item_issues` (`broken_links`, `self_links`, `cycles`) and more.
- `lif check` does not check rank, status, priority or tags. An agent that writes files directly stays unchecked there.

</details>

## Limits

- GraphQL `createItem` without `status` leaves the status empty. Only `lif create` sets the default `todo`. `lif promote` had the same defect before 0.14.
- On a GraphQL error the CLI prints `Error:` and a message. It returns no partial data.
- Server-only parts of the schema (agent, terminal, worktrees) fail in the CLI with messages such as "agent manager not available".
- Subscriptions run only over the WebSocket of `lif-serve`, not through `lif graphql`.
- There are no JSON schema files for `--json`. The shapes can change with any release [z9e6].
- Files you write directly bypass the rules. `lif check` covers only part of them.

See [Known limitations](../../known-limitations/) for [z69m] and [z9e6].

## Reference links

- [GraphQL](../../reference/graphql/): schema, queries, mutations, errors.
- [JSON output](../../reference/json-output/): payloads, error codes, exit codes.
- [`lif graphql`](../../reference/commands/lif-graphql/), [`lif prime`](../../reference/commands/lif-prime/), [`lif promote`](../../reference/commands/lif-promote/), [`lif check`](../../reference/commands/lif-check/).
- [Web server](../../reference/web-server/): HTTP endpoints of `lif-serve`.
- [Agent onboarding](../../overview/agent-onboarding/): recommended hooks.
