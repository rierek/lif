---
title: "JSON output"
description: "Machine-readable output of lif commands: payloads, errors and exit codes."
---

**State of this page:** shapes measured on 2026-10-14 against Lif 0.14 (development build `135e76e`).

:::caution[No stable interface promised]
The JSON shapes can change with any release, and this page can be out of date. The repository publishes no schema files for the `--json` output (only an internal findings schema exists). No stable interface is promised until schema files exist; tracked as [z9e6]. If you need certainty, run the command against your own version.
:::

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

- `lif list --json` and `lif show --json` use the same object; `list` returns an array. `show --json` returns the body with a leading newline, `create --json` does not [0g6b]. `priority` is `normal` when the file has none.
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
| `VALIDATION_ERROR` | Invalid input: unknown blocker, invalid `--format` or `--depth`, bad filter. |
| `FILE_ERROR` | Writing or reading the file failed, also a refused parent [ju1q] (`failed to create item: epic items can only have milestone as parent, not task`). |
| `CONFLICT` | ETag mismatch (`--if-match`). |
| `POLICY_VIOLATION` | A required field of `require_fields_on` is missing. See [Configuration](../configuration/#status-dependent-required-fields). |

The mapping above is from the code of the commands, not every code was triggered in this measurement. Observed: `NOT_FOUND`, `NO_LIF_DIR`, `VALIDATION_ERROR` (`blocker item not found: zzzz`, `invalid status: bogus (must be in-progress, todo, draft, completed, scrapped)`), `FILE_ERROR`.

`lif graphql` and `lif list --search` do not use this envelope. A failing GraphQL call prints `Error: graphql: <message>` on stderr, a failing search `Error: querying items: <message>`. Both exit with 1.

## Without `--json`

- Text output. An error appears once as text, the process exits with 1, Cobra's usage text is suppressed.
- `lif show` prints styled output on a terminal and the raw Markdown of the file when piped or redirected.
- Exit codes: 0 on success, 1 on error. `lif check` returns 1 on at least one finding (policy warnings count only with `--strict`, legacy notices never). There is no exit code 2. `lif next` without a ready item exits with 0.

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

`lif promote <artifact> <finding-id>...` checks the chosen finding records against an embedded findings schema and creates one item per record, with a fixed mapping from finding code to item type. If a chosen record is invalid, the command writes nothing. Findings already promoted are skipped. The schema is internal and not published ([z9e6]). A known problem of this command is tracked as [z69m].

## Recipe: open item IDs

```sh
lif list --json -s todo,in-progress | jq -r '.[].id'
```
