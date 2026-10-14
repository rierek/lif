---
title: "JSON schemas"
description: "Machine-readable output of lif commands: payloads, errors and exit codes."
---

## Purpose
Reference page for everything scripts and agents read from the JSON output of the commands: which commands have `--json`, what success and error payloads look like, and which exit codes apply. What FACTS does not show stays marked as open.

## Audience
Script authors and agents that process Lif output programmatically. The page is limited to documented shapes; missing schemas are filled in from the code in phase 5.

## Required sections

### Using `--json`
- Shows the commands with `--json`: `archive`, `check`, `complete`, `create`, `delete`, `graphql`, `init`, `list`, `milestones`, `next`, `order`, `progress`, `promote`, `rename`, `roadmap`, `scrap`, `show`, `start`, `tag`, `update`, `version`.
- Shows the commands without `--json`: `graph` (uses `--format json`), `path`, `pick`, `prime`, `tui`.
- Explains: with `--json` the human-readable lines are omitted; `lif delete --json` implies `--force` (no confirmation prompt); `lif graphql --json` turns off colors.
- Explains: `lif list --json` includes the body only with `--full`.
- Source: FACTS.md §1.0, §1.1 (flags per command), §6.8, §7.5

### Success payloads
- Explains the three families: item objects (domain model), responses with the affected item, command-specific shapes (e.g. `check`, `graph`, `version`).
- Table per command: root shape (object or list), fields, example (TODO: verify)
- Explains the simple success object with a success flag and a message (e.g. `lif next` without a ready item); the JSON key names cannot be read from FACTS (TODO: verify)
- Shows `lif version --json` (TODO: verify)
- Link to `reference/item-file-format` (Front matter fields) for the meaning of the item fields.
- Source: FACTS.md §5.2 (`next --json`), §1.1 (`lif version`); `TODO: verify` for item and version payload: see `internal/output/output.go` and the `--json` paths of the commands, then run each command once

### Error payload and codes
- Table of error codes: `NOT_FOUND`, `NO_LIF_DIR`, `INVALID_STATUS`, `FILE_ERROR`, `VALIDATION_ERROR`, `CONFLICT`, `POLICY_VIOLATION`, one line each on when it occurs (mapping per code → TODO: verify).
- Shows the missing Store: with `--json` error code `NO_LIF_DIR` (not `NO_BEANS_DIR`).
- Shows examples of errors with a code: unknown item (`NOT_FOUND`), `lif graph --format` invalid or `--depth` without an ID (validation error in JSON format).
- Shows the shape of the error object (field names) (TODO: verify)
- Source: FACTS.md §1.0, §5.5; `TODO: verify` for error object and mapping of the codes: see `internal/output/output.go`

### Without `--json`
- Explains: without `--json` Lif prints text; errors appear once as text, the process exits with code 1, the Cobra usage is suppressed.
- Explains: `lif show` prints styled output on a terminal, and the raw Markdown of the source file when piped or redirected to a file.
- Shows exit codes: 0 on success, 1 on error; `lif check` returns 1 on at least one finding (policy warnings count only with `--strict`, legacy notices never); no exit code 2; `lif next` without a ready item exits with 0.
- Source: FACTS.md §1.0, §1.1 (`lif show`), §5.2, §6.8, §6.11

### `lif check --json`
- Table of fields: `success` (always; `true` exactly when there are no findings, with `--strict` including policy warnings), `config_errors` (list, `null` when empty), `item_issues` with `broken_links`, `self_links`, `cycles` (`link_type`, `path`), `fixed` (count; absent at 0), `policy_warnings`, `attachment_orphans`, `legacy_names` (only when present).
- Explains: output is indented with two spaces; exit code 1 on findings also with `--json`.
- Explains: old keys `bean_id` and `bean_issues` are now called `item_id` and `item_issues`.
- Shows an example with a broken link (TODO: verify)
- Field names of the entries in `broken_links` and `self_links`, and whether `cycles` entries also carry `item_id` (FACTS mentions both) (TODO: verify)
- Source: FACTS.md §6.8, §5.3, §6.11; `TODO: verify` for entry fields and example: see `internal/commands/check.go` and `pkg/itemcore/links.go`, run `lif check --json` once

### `lif graph --json`
- Explains: invoke with `--format json`, not `--json`.
- Shows the shape: `{"nodes":[{"id","title","status","type"}],"edges":[{"from","to","relation"}]}`; `type` is absent for an empty type; `edges` is `[]` without edges; indentation two spaces.
- Explains `relation`: `parent` (parent to child) or `blocks` (blocking to blocked item); `blocking` and `blocked_by` are merged into one edge.
- Explains: errors (invalid `--format`, `--relation`, `--depth`) appear in JSON format as a JSON error object.
- Shows an example (TODO: verify)
- Source: FACTS.md §5.5; `TODO: verify` for example output: run once in phase 5

### `lif next --json` message
- Explains: if no item is ready, `lif next --json` prints a success object with the message `no ready items found`, with an active filter `no ready items found matching <flags>`; exit code 0.
- Explains: if an item is ready, the output equals that of `lif show --json` (`next` delegates to `show`).
- Explains: a query error appears as the validation error `querying items: ...`.
- Source: FACTS.md §5.2

### Findings schema for `lif promote`
- Explains: `lif promote <artifact> <finding-id>...` checks the chosen finding records against an embedded findings schema and creates one item per record (fixed mapping from code to type).
- Explains: if a chosen record is invalid, the command writes nothing; findings already promoted are skipped.
- Shows the schema with all fields (TODO: verify)
- Shows the mapping from code to type (TODO: verify)
- Source: FACTS.md §1.1 (`lif promote`); `TODO: verify` for schema and mapping: see `internal/commands/findings-schema.json` and `internal/commands/promote.go`

### Stability and date of this page
- Prints a visible "State of this page" line near the top with the date the shapes were measured and the Lif version they were measured against (taken from `lif version` at writing time). The date is part of the page, not hidden metadata.
- Shows a warning box: the JSON shapes can change with any release, this page can be out of date, and no stable interface is promised until schema files exist (D69). Readers who need certainty run the command against their own version.
- States that the repository publishes no schema files for `--json` output; only the internal findings schema exists (FACTS.md §11.10). Links to the tracking item as `[z9e6]` (D65).
- Every example on the page is a real output pasted from FACTS.md §11, not a hand-written one.
- Recipe with `jq`: extract the IDs of open items from `lif list --json` (FACTS.md §11.9–§11.11; run it before publishing).
- Recipe: evaluate the exit code and `success` of `lif check --json` in CI (FACTS.md §11.9–§11.11; run it before publishing).
- Source: FACTS.md §11.10, §11.9–§11.11, §22.1; DESIGN.md D65, D69
