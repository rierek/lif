---
title: "GraphQL"
description: "The GraphQL API for items: invoke it, read the schema, run queries and mutations."
---

## Purpose
Reference for the GraphQL interface for items: how to call it, what the schema contains, and what queries and mutations look like. In its final form the page consists of a hand-written introduction with examples plus the schema (SDL) embedded at build time.

## Audience
Script and agent authors and developers who want to read or change items via JSON instead of the command line, and web server operators who use the HTTP interface.

## Required sections

### What the API is and when to use it
- Explains: GraphQL is structured access to the item store next to Markdown and the CLI (third access path: JSON + GraphQL).
- Names use cases: scripts and agents that need exactly the fields (`items(filter)` instead of several `lif list` calls), relationships (`parent`, `children`, `blocking`, `blockedBy`) in one request.
- Shows the two modes of operation: in-process via the CLI (no server needed) and over HTTP with `lif-serve`.
- Links to `reference/web-server` and the generated command page `lif graphql` (`reference/commands/lif-graphql`).
- Source: FACTS.md §7, §7.5, §1.1 (`lif graphql`); DESIGN.md D27

### Invoking it
- Table or list of the two ways: CLI in-process and HTTP.
- CLI: `lif graphql '<query>'`, alias `lif query`; query from the argument or, if none, from stdin (pipe or file); more than one argument is an error; empty stdin: `no query provided (pass as argument or pipe to stdin)`.
- Flags: `--json` (without colors, both modes print indented), `-v`/`--variables` (JSON string), `-o`/`--operation` (operation name when there are several operations), `--schema` (print the schema and exit).
- HTTP: `lif-serve` serves the full schema at `/api/graphql` (WebSocket, OPTIONS, GET, POST) and the GraphQL Playground at `/playground`; default port 8080.
- Shows example calls: argument, stdin pipe, variables, `--schema` (the command help names: list items, get an item, filter by status, relationships, variables, stdin, schema).
- Source: FACTS.md §7.5, §1.1 (`lif graphql`), §8.1

### Schema overview
- Explains: one schema (`internal/graph/schema.graphqls`, a single file) contains the core item operations and, next to them, operations for the web server only.
- Core item operations: `item`, `items`, `createItem`, `updateItem`, `deleteItem`, `setParent`, `addBlocking`, `removeBlocking`, `addBlockedBy`, `removeBlockedBy`, `archiveItem`, `saveDirtyItems`, `saveItem`, subscription `itemChanged`.
- Web server only: worktrees, agent sessions, terminal, file changes/diffs, run/stop, open in editor; in CLI mode mutations of this group return errors such as "agent manager not available".
- Note: individual query-only fields of the server group can still respond in the CLI if they need no manager (observed: `{ agentEnabled }`).
- Stresses: `status`, `type` and `priority` are plain `String` fields, not enums; valid values come from the configuration (`reference/configuration`). Scalars: `Time`, `Map`.
- Shows the type `Item` (fields incl. derived `etag`, `isDirty`, `implicitStatus`, `implicitStatusFrom`, `extra`) in a compact table.
- Source: FACTS.md §7, §7.1, §7.2, §7.3, §7.4

### Queries
- `item(id: ID!)`: accepts a full or short ID without prefix, returns `null` if nothing is found.
- `items(filter: ItemFilter)`: list of all matching items.
- Table `ItemFilter`: `search`, `status`/`excludeStatus`, `type`/`excludeType`, `priority`/`excludePriority`, `tags`/`excludeTags`, parent filters (`hasParent`, `noParent`, `parentId`), blocking filters (`hasBlocking`, `noBlocking`, `blockingId`, `hasBlockedBy`, `noBlockedBy`, `blockedById`), blocked state (`isBlocked`, `isExplicitlyBlocked`, `isImplicitlyBlocked`), `excludeImplicitTerminal`.
- Further fields: `projectName`, `mainBranch`; relationship fields on the item (`parent`, `children`, `blocking`, `blockedBy` with their own `filter`).
- Shows one example each: item by ID, filter by status, children of a container.
- Source: FACTS.md §7.1, §7.4

### Mutations
- Explains the core mutations: `createItem(input: CreateItemInput!)`, `updateItem(id, input: UpdateItemInput!)`, `deleteItem(id)` (removes incoming links automatically), `setParent(id, parentId, ifMatch)` (checks the type hierarchy), `addBlocking`/`removeBlocking`/`addBlockedBy`/`removeBlockedBy` (each with `ifMatch`), `archiveItem`.
- Table `CreateItemInput` (`title`, `type`, `status`, `priority`, `tags`, `body`, `parent`, `blocking`, `blockedBy`, `prefix`) and `UpdateItemInput` (among others `addTags`/`removeTags`, `bodyMod`, `addBlocking` etc., `order`, `ifMatch`).
- Explains `bodyMod` (`replace: [{old, new}]`, `append`) as an alternative to `body`; using both together is forbidden.
- Explains `ifMatch` (ETag comparison against concurrent changes) and the derived field `etag` on the item.
- Shows example: create an item, change status, set parent, add blocking.
- Source: FACTS.md §7.2, §7.4

### Subscriptions
- Explains `itemChanged(includeInitial: Boolean = false)` with `ItemChangeEvent { type, item, items, itemId }`; `ChangeType` = `INITIAL_SNAPSHOT`, `CREATED`, `UPDATED`, `DELETED`; a slow subscriber additionally receives `INITIAL_SNAPSHOT`.
- Clarifies: subscriptions run only over the WebSocket of `lif-serve`, not via `lif graphql`.
- Only name the other subscriptions of the server group: `worktreesChanged`, `agentSessionChanged`, `activeAgentStatuses`, `workspaceStatuses`.
- Links to the section "Live updates" in `reference/web-server`.
- Source: FACTS.md §7.3, §8.1

### Examples
- Three real runs with output: filter items (query), create an item (mutation), change an item with variables (`-v`).
- Each example shows the command, the output and a short explanation of the fields.
- The examples are to be run in phase 5 in a fresh store with `lif init`; no invented output.
- Source: `TODO: verify` — FACTS shows no complete runs except the observed `createItem`; run with `lif graphql` in the test store in phase 5

### Errors and validation
- Error format in the CLI: a single error as `graphql: <message>`, several as `graphql errors:` with an indented list.
- Input validation on write: title non-empty and at most 1000 characters, body at most 1 MiB, tags must match the tag pattern, prefix pattern; `body` and `bodyMod` are mutually exclusive, as are `tags` and `addTags`/`removeTags`.
- Type, status and priority names are checked against the configured tables (only on create and for supplied fields).
- Parent check on `createItem` (with parent), `setParent` and `updateItem` with `parent`: rank rule and cycle check; behaves as in the CLI.
- HTTP WebSocket: a disallowed origin returns 403 `origin not allowed` (cross-reference `reference/web-server`).
- Source: FACTS.md §7.4, §7.5, §3.1, §8.1

### Known gaps
- `createItem` without `status` leaves the status empty (`"status": ""`); the default status `todo` is set only by `lif create`. Recommendation: always specify `status`.
- Changing only the `type` via `updateItem(type)` does not re-check the parent/child ranks (same as with `lif update -t`).
- Both gaps are tracked as product bugs; update the page as soon as they are fixed.
- Source: FACTS.md §7.5, §3.1; DESIGN.md D43

### Schema (SDL)
- Embeds the complete SDL, generated at build time from `lif graphql --schema`; do not copy by hand.
- Note for the implementation: the generator step (embedding) belongs in phase 5.
- Optionally a short sentence that the schema is 845 lines long and is regenerated with `just gqlgen` (developers only).
- Source: FACTS.md §7, §7.5; DESIGN.md D27
