---
title: "GraphQL"
description: "The GraphQL API for items: invoke it, read the schema, run queries and mutations."
---

GraphQL is the third way to reach items, next to the Markdown files and the CLI. Measured on Lif 0.14, 2026-10-14. JSON shapes of other commands: [JSON output](../json-output/).

## What the API is and when to use it

- A script or an agent asks for exactly the fields it needs. One `items(filter)` call replaces several `lif list` calls.
- Relationships (`parent`, `children`, `blocking`, `blockedBy`) come back in one request.
- Two modes: **in-process** through `lif graphql` (no server needed) and **over HTTP** through `lif-serve` ([Web server](../web-server/)). The in-process mode has only the item operations.

The command page is [`lif graphql`](../commands/lif-graphql/).

## Invoking it

| Mode | How |
|---|---|
| CLI | `lif graphql '<query>'` (alias `lif query`). Without an argument the query is read from stdin. |
| HTTP | `lif-serve` serves `/api/graphql` (WebSocket, OPTIONS, GET, POST) and a playground at `/playground`. Default port 8080. |

Flags of `lif graphql`: `--json` (no colors; both modes print indented), `-v`/`--variables` (JSON string), `-o`/`--operation` (name, when the document has several operations), `--schema` (print the schema and exit). More than one argument is an error; empty stdin gives `no query provided (pass as argument or pipe to stdin)`.

```sh
lif graphql '{ items { id title status } }'
echo '{ items { id title } }' | lif graphql
lif graphql -v '{"id":"abc"}' 'query($id: ID!) { item(id: $id) { title } }'
lif graphql --schema
```

Over HTTP a server on loopback, the default, has no authentication; a server beyond loopback asks every client for an access token ([8h4v]). Read [Web server](../web-server/) first. The example below works against a loopback server; beyond loopback add `-H 'Authorization: Bearer <token>'`.

```sh
curl -s -X POST http://localhost:8080/api/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"{ items { id title status } }"}'
```

## Schema overview

One schema file holds the item operations and, next to them, operations for `lif-serve` only: worktrees, agent sessions, terminal, file changes and diffs, run and stop, open in editor. In the CLI mode the mutations of that group fail (`worktree support not available`, `agent manager not available`, `terminal support not available`). Some query-only fields of the group still answer in the CLI when they need no manager, such as `{ agentEnabled }`.

Item operations: `item`, `items`, `createItem`, `updateItem`, `deleteItem`, `setParent`, `addBlocking`, `removeBlocking`, `addBlockedBy`, `removeBlockedBy`, `archiveItem`, `saveItem`, `saveDirtyItems`, and the subscription `itemChanged`.

`status`, `type` and `priority` are plain `String` fields, not enums. Valid values come from the [configuration](../configuration/). Custom scalars: `Time`, `Map`.

Main fields of `Item`:

| Field | Meaning |
|---|---|
| `id`, `slug`, `path`, `title`, `body` | Identity and text. |
| `status`, `type`, `priority`, `tags`, `order` | Classification. |
| `createdAt`, `updatedAt` | Timestamps (`Time`). |
| `parent`, `children`, `blocking`, `blockedBy` | Relationships; the lists take their own `filter`. |
| `etag` | Hash of the file; pass it as `ifMatch`. |
| `isDirty` | Unsaved changes in memory. |
| `implicitStatus`, `implicitStatusFrom` | Status derived from children or blockers. |
| `extra` | Extra front matter keys (`Map`). |

## Queries

- `item(id: ID!)`: full ID or short ID without prefix. Returns `null` if nothing matches.
- `items(filter: ItemFilter)`: all matching items, in the default sort order of `lif list`.
- `projectName`, `mainBranch`: project data.

`ItemFilter` fields:

| Group | Fields |
|---|---|
| Text | `search` (see [Search syntax](../search-syntax/)) |
| Classification | `status`/`excludeStatus`, `type`/`excludeType`, `priority`/`excludePriority`, `tags`/`excludeTags` |
| Parent | `hasParent`, `noParent`, `parentId` |
| Links | `hasBlocking`, `noBlocking`, `blockingId`, `hasBlockedBy`, `noBlockedBy`, `blockedById` |
| Blocked state | `isBlocked`, `isExplicitlyBlocked`, `isImplicitlyBlocked`, `excludeImplicitTerminal` |

Set fields combine with AND; values inside one list are OR. The nested `children`, `blocking` and `blockedBy` fields ignore `search`.

GraphQL has no `ready` argument. `lif next` and `lif list --ready` build "ready" from these fields: `isBlocked: false`, `excludeImplicitTerminal: true`, `excludeStatus` with `in-progress`, `draft` and every archive status, and `excludeType` with every container type (rank 1 to 3). A client that wants the same list sets the same fields.

## Mutations

- `createItem(input: CreateItemInput!)`: `title`, `type`, `status`, `priority`, `tags`, `body`, `parent`, `blocking`, `blockedBy`, `prefix`.
- `updateItem(id, input: UpdateItemInput!)`: the same fields, plus `addTags`/`removeTags`, `bodyMod`, `order`, `ifMatch`. Only supplied fields change.
- `deleteItem(id)`: removes incoming links in other items automatically.
- `setParent(id, parentId, ifMatch)`: checks the type hierarchy.
- `addBlocking`, `removeBlocking`, `addBlockedBy`, `removeBlockedBy`: each with `ifMatch`.
- `archiveItem(id)`: moves an item with an archive status to `archive/`.

`bodyMod` edits the body in place: `replace: [{old, new}]` or `append`. It cannot be combined with `body`, and `tags` cannot be combined with `addTags` or `removeTags`.

`ifMatch` compares the ETag with the current file. If the item changed meanwhile, the call fails with a conflict. The item file is locked from the compare to the write, so of two simultaneous writers with the same ETag only one succeeds; a write without `ifMatch` takes no lock. The `etag` field gives the current value. With `lif.require_if_match: true` the value is mandatory.

## Subscriptions

`itemChanged(includeInitial: Boolean = false)` delivers `ItemChangeEvent { type, item, items, itemId }`. `type` is `INITIAL_SNAPSHOT`, `CREATED`, `UPDATED` or `DELETED`. A subscriber that is too slow also receives an `INITIAL_SNAPSHOT`. Subscriptions run only over the WebSocket of `lif-serve`, not through `lif graphql`. The server group adds `worktreesChanged`, `agentSessionChanged`, `activeAgentStatuses`, `workspaceStatuses`. See [Live updates](../web-server/#live-updates).

## Examples

Real runs in a fresh store (Lif 0.14, 2026-10-14). IDs are shortened by the store prefix `f014j-`.

**Filter items (query)** with a variable:

```sh
lif graphql --json -v '{"s":["todo"]}' \
  'query($s:[String!]){ items(filter:{status:$s}) { id title status } }'
```

```json
{
  "items": [
    { "id": "f014j-dsdd", "title": "Via GraphQL", "status": "todo" }
  ]
}
```

**Create an item (mutation).** Without `status` the configured default is used (`todo` here):

```sh
lif graphql --json 'mutation { createItem(input:{title:"Via GraphQL", type:"task"}) { id status type } }'
```

```json
{
  "createItem": { "id": "f014j-dsdd", "status": "todo", "type": "task" }
}
```

**Change an item with variables (`-v`):**

```sh
lif graphql --json -v '{"id":"ix1n","s":"in-progress"}' \
  'mutation($id:ID!,$s:String){ updateItem(id:$id, input:{status:$s}) { id status } }'
```

```json
{
  "updateItem": { "id": "f014j-ix1n", "status": "in-progress" }
}
```

**Children of a container** (the example output has one epic with one child):

```sh
lif graphql --json '{ items(filter:{type:["epic"]}) { id children { id status } } }'
```

## Errors and validation

- In the CLI one error prints as `Error: graphql: <message>`; several print as `graphql errors:` with an indented list. The exit code is 1.
- Over HTTP the response has `errors[0].message` and `data: null`.
- Write validation is the same as in the CLI: title not empty and at most 1000 characters, body at most 1 MiB, tags match the tag pattern, `body` and `bodyMod` exclusive. Type, status and priority names are checked against the configured tables for the supplied fields.
- Parent rules apply on `createItem` (with `parent`), `setParent` and `updateItem` with `parent` or a changed `type`: rank rule and cycle check, as in [Configuration](../configuration/#types-ranks-and-hierarchy-rules).

```text
Error: graphql: invalid title: must not be empty
Error: graphql: task items can only have milestone, epic, or feature as parent, not task
```

- A request that changes state from a disallowed origin, a WebSocket upgrade included, gets 403 `origin "…" is not allowed`. On loopback a request with a disallowed `Host` gets 403 `host "…" is not allowed` ([Web server](../web-server/#network-and-cors)).

## Known gaps

- `createItem` without `status` sets the default status since 0.14. `lif promote` goes through the same path.
- A search in a nested field is ignored (see [Search syntax](../search-syntax/)); `AND`/`OR`/`NOT` in `search` do not work ([xa2j]).

## Schema (SDL)

The full schema is about 1060 lines. Print it with `lif graphql --schema`. It is generated from `internal/graph/schema.graphqls`; developers regenerate code with `just gqlgen`.
