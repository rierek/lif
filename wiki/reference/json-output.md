---
title: "JSON output"
description: "Machine-readable output of lif commands: payloads, errors and exit codes."
---

## JSON schemas

The repository publishes a JSON Schema (draft 2020-12) for the `--json` output of each command family in [`schemas/`](https://github.com/rierek/lif/tree/main/schemas). The files are generated from the Go types the commands encode (`just schemas`), and a test fails when a committed file no longer matches the code. A change to an output shape therefore always appears as a change of a schema file.

| File | Output of |
|---|---|
| `item.schema.json` | One item: `show`, `create`, `update`, `tag`, `start`, `complete`, `scrap`, `order`, `delete` and `promote` with one ID, `next` with a ready item. |
| `items.schema.json` | An array of items: `list`, and the verbs above with several IDs. `list` prints `null` when nothing matches. |
| `response.schema.json` | The envelope: every error, a plain success with a message (`archive`, `next` without a ready item, `init`), and a partly failed batch. |
| `check.schema.json` | `lif check --json`. |
| `graph.schema.json` | `lif graph --format json`. |
| `milestones.schema.json` | `lif milestones --json`. |
| `progress.schema.json` | `lif progress --json`. |
| `roadmap.schema.json` | `lif roadmap --json`. |
| `rename.schema.json` | An applied `lif rename --json`. |
| `rename-plan.schema.json` | `lif rename --dry-run --json`. Its keys are capitalised (`Mode`, `Changes`). |
| `version.schema.json` | `lif version --json`. |

Not covered: `lif graphql` (the shape follows your query) and the findings artifact of `lif promote`, which has its own internal schema.

The schemas allow properties they do not list, so a reader must ignore unknown fields. A required array or object can be `null`, because Go prints an empty list as `null` in some commands.

## Using `--json`

Commands with `--json`: `archive`, `check`, `complete`, `create`, `delete`, `graphql`, `init`, `list`, `milestones`, `next`, `order`, `progress`, `promote`, `rename`, `roadmap`, `scrap`, `show`, `start`, `tag`, `update`, `version`.

Commands without it: `graph` (uses `--format json`), `path`, `pick`, `prime`, `tui`.

- With `--json` the human-readable lines are omitted.
- `lif delete --json` implies `--force`: there is no confirmation prompt, and references to the deleted item in other items' front matter are dropped without a warning.
- `lif graphql --json` turns off colors. Output is indented with two spaces.
- `lif list --json` includes the body only with `--full`. It prints `null`, not `[]`, when a filter matches nothing.

## Success payloads

There are three families: item objects (`list`, `show`, `next`), responses that carry the affected item (`create`, `update`, `complete`, ...), and command-specific shapes (`check`, `graph`, `version`). For the meaning of the item fields see [Item file format](../item-file-format/#front-matter-fields).

A new item, `lif create "Write notes" -t task --parent <epic> -p high --tag docs --json`:

```json
{
  "id": "f014j-ix1n",
  "slug": "write-notes",
  "path": "f014j-ix1n--write-notes.md",
  "title": "Write notes",
  "status": "todo",
  "type": "task",
  "priority": "high",
  "tags": [
    "docs"
  ],
  "created_at": "2026-10-14T18:54:05Z",
  "updated_at": "2026-10-14T18:54:05Z",
  "parent": "f014j-uzgy",
  "etag": "3c75a4149f169b2a"
}
```

- `lif list --json` and `lif show --json` use the same object; `list` returns an array. The `body` is the same text in every command, without a leading newline. `priority` is `normal` when the file has none.
- `etag` is the value for `--if-match`. `blocking` and `blocked_by` appear when set.
- `lif list --json` has no `body`; add `--full`.

`lif version --json`:

```json
{
  "version": "dev",
  "commit": "135e76e-dirty",
  "date": "2026-10-14T18:41:07Z",
  "custom_front_matter": true
}
```

A plain success with a message, for example `lif next --json` when nothing is ready (exit code 0):

```json
{
  "success": true,
  "message": "no ready items found"
}
```

With a filter the message is `no ready items found matching <flags>`. If an item is ready, the output equals `lif show --json`.

`lif archive --json` returns the same envelope: `{"success": true, "message": "Archived N item(s) to .lif/archive/"}`, or the message `No items to archive` when nothing qualifies.

`update`, `tag` and `start` with `--json` return the item itself, not an envelope: one object for one ID, an array for several IDs. All changes of one `update` call land in one write under one etag.

## Error payload and codes

With `--json` an error is one JSON object on stdout and the exit code is 1:

```json
{
  "success": false,
  "error": "item not found: nope",
  "code": "NOT_FOUND"
}
```

| Code | When |
|---|---|
| `NOT_FOUND` | Item ID unknown. |
| `NO_LIF_DIR` | No store found (not `NO_BEANS_DIR`). |
| `INVALID_STATUS` | Status not in the configured table. |
| `VALIDATION_ERROR` | The request is refused: invalid input, unknown blocker or parent, a parent the type hierarchy forbids (`failed to create item: epic items can only have milestone as parent, not task`), a dependency cycle, invalid `--format` or `--depth`, bad filter. |
| `FILE_ERROR` | Writing or reading the file failed. |
| `CONFLICT` | ETag mismatch (`--if-match`). |
| `POLICY_VIOLATION` | A required field of `require_fields_on` is missing (see [Configuration](../configuration/#status-dependent-required-fields)), or a gate refused the change (see [Hooks](../configuration/#hooks)). |

Each code is set by the command that fails. `VALIDATION_ERROR` examples: `blocker item not found: zzzz`, `invalid status: bogus (must be in-progress, todo, draft, completed, scrapped)`.

`lif graphql` and `lif list --search` do not use this envelope. A failing GraphQL call prints `Error: graphql: <message>` on stderr, a failing search `Error: querying items: <message>`. Both exit with 1.

## Without `--json`

- Text output. An error appears once as text, the process exits with 1, Cobra's usage text is suppressed. `lif run` is the exception: it exits with the exit code of the failing step and prints no `--help` pointer (see [Tasks](../configuration/#tasks)).
- `lif show` prints styled output on a terminal and the raw Markdown of the file when piped or redirected.
- Exit codes: 0 on success, 1 on error. `lif check` returns 1 on at least one finding (policy warnings count only with `--strict`, legacy notices never). Except for `lif run`, which returns the exit code of the failing step, there is no exit code 2. `lif next` without a ready item exits with 0.

## `lif check --json`

```json
{
  "success": true,
  "config_errors": null,
  "item_issues": {
    "broken_links": [],
    "self_links": [],
    "cycles": []
  }
}
```

| Field | Meaning |
|---|---|
| `success` | `true` exactly when there are no findings; with `--strict` policy warnings count. |
| `config_errors` | List of configuration problems, `null` when empty. |
| `item_issues` | `broken_links`, `self_links`, `cycles` (`link_type`: `blocking`, `parent` or `blocking+parent`; `path`). Formerly `bean_issues`. |
| `fixed` | Count of repaired links (`--fix`); absent at 0. |
| `policy_warnings`, `attachment_orphans`, `legacy_names` | Only when present. |

The exit code is 1 on findings also with `--json`. The 0.14 `check` also reports item field problems: unknown status or priority, invalid tags, a parent of the wrong type rank. The entry fields of those findings were not measured for this page.

Recipe: fail a CI job on findings.

```sh
lif check --json > check.json || { echo "lif check found issues"; exit 1; }
jq -e '.success' check.json
```

## `lif graph --format json`

Use `--format json`, not `--json`.

```json
{
  "nodes": [
    { "id": "f014j-ix1n", "title": "Write notes", "status": "todo", "type": "task" },
    { "id": "f014j-uzgy", "title": "Ship release", "status": "todo", "type": "epic" }
  ],
  "edges": [
    { "from": "f014j-uzgy", "to": "f014j-ix1n", "relation": "parent" }
  ]
}
```

(Compacted here; Lif prints one field per line.) `relation` is `parent` (parent to child) or `blocks` (blocking to blocked item); `blocking` and `blocked_by` merge into one edge. `type` is absent for an empty type, `edges` is `[]` without edges. Invalid `--format`, `--relation` or `--depth` produce the JSON error object.

## GraphQL output

`lif graphql --json` prints the `data` object without the envelope:

```json
{
  "createItem": {
    "id": "f014j-dsdd",
    "status": "todo",
    "type": "task"
  }
}
```

See [GraphQL](../graphql/).

## `lif promote` findings

`lif promote <artifact> <finding-id>...` checks the chosen finding records against an embedded findings schema and creates one item per record, with a fixed mapping from finding code to item type. The items get the configured default status, and `--parent <id>` puts them under that item. If a chosen record, a field or the parent is invalid, the command writes nothing; if a write still fails, the error lists the items already written. Findings already promoted are skipped. The findings schema is internal and not part of `schemas/`; the artifact format is on the [`lif promote`](../commands/lif-promote/) page.

## Recipe: open item IDs

```sh
lif list --json -s todo,in-progress | jq -r '.[].id'
```
