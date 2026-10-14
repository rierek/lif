---
title: "Search syntax"
description: "Query syntax for lif list --search and the GraphQL search filter."
---

## Purpose
Reference page for the full-text query syntax of `lif list --search` and the GraphQL filter `ItemFilter.search`: terms, phrases, fuzzy search, prefixes, operators and field scopes.

## Audience
Users and agents who search a backlog in a targeted way. The page describes only what FACTS documents; everything else is marked as open.

## Required sections

### Where search applies
- Shows `lif list --search <query>` and the short form `-S`: full-text search in title and body.
- Shows GraphQL: `ItemFilter.search` (string) in `items(filter)`; link to `reference/graphql`.
- Explains: the syntax is the Bleve query string syntax, as named in the help text of `lif list`; that `ItemFilter.search` uses the same syntax is not documented (TODO: verify)
- Note: in the flag directory of FACTS only `lif list` has a `--search`; GraphQL uses `ItemFilter.search`.
- Source: FACTS.md §1.1 (`lif list`), §7.4; `TODO: verify` for the syntax of `ItemFilter.search`: look at `pkg/itemgraph/filters.go`

### Terms and phrases
- Shows a single term and several terms (combination without an operator → TODO: verify).
- Shows phrase search in double quotes, e.g. `"exact phrase"`.
- Note on shell quoting of the quotes in examples.
- Source: FACTS.md §1.1 (`lif list`: term, "phrase"); `TODO: verify` for combining several terms without an operator: Bleve documentation and test run in Phase 5

### Fuzzy, prefix and wildcard
- Shows fuzzy search `term~` and with edit distance `term~2`.
- Shows prefix `prefix*`.
- General wildcards beyond the prefix (TODO: verify)
- Source: FACTS.md §1.1 (`lif list`: `term~`, `term~2`, `prefix*`); `TODO: verify` for further wildcards: Bleve documentation and test run in Phase 5

### Boolean operators
- Shows `AND` and `OR` in uppercase.
- Shows grouping with parentheses (TODO: verify)
- Source: FACTS.md §1.1 (`lif list`: AND/OR); `TODO: verify` for parentheses: Bleve documentation and test run in Phase 5

### Field scopes
- Shows `slug:`, `title:` and `body:` as field scopes, e.g. `title:release`.
- Explains the difference: without a field, search covers title and body (flag help: "Full-text search in title and body").
- Further searchable fields (tags, extra keys) are not documented (TODO: verify)
- Source: FACTS.md §1.1 (`lif list`); `TODO: verify` for further fields: look at the index definition in the code of `lif list`

### Combining with filters
- Shows `--search` together with filter flags of `lif list`: `--status`, `--type`, `--priority`, `--tag`, `--parent`, `--ready`, `--is-blocked`, `--unblocked`, `--where`.
- Explains how the filters combine with each other only where documented: `--tag` OR within the flag, `--where` repeated AND.
- Explains GraphQL: `search` sits next to `status`, `type`, `tags` etc. in the same `ItemFilter`.
- Whether search and filters are strictly AND-combined (TODO: verify)
- Source: FACTS.md §1.1 (`lif list`), §7.4; `TODO: verify` for the combination of search and filters: look at `internal/commands/list.go` and `pkg/itemgraph/filters.go`

### Examples
- Shows four to six real calls with output (term, phrase, fuzzy, prefix, field scope, operator, combination with `--status`) (TODO: verify)
- Shows the same through `lif graphql` with `ItemFilter.search` (TODO: verify)
- Source: `TODO: verify` — run the examples against a real store in Phase 5; FACTS contains no example calls

### Limits
- Negation (`-term`, `NOT`) (TODO: verify)
- Weighting (`term^2`) (TODO: verify)
- Case sensitivity and handling of hyphens or umlauts (TODO: verify)
- Note: filters by `status`, `type`, `priority` and `tag` run through their own flags of `lif list`, not through the query.
- Source: `TODO: verify` — FACTS names only the elements listed in "Fuzzy, prefix and wildcard", "Boolean operators" and "Field scopes"; Bleve query string documentation and test run in Phase 5
