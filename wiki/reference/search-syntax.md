---
title: "Search syntax"
description: "Query syntax for lif list --search and the GraphQL search filter."
---

Full-text search runs through `lif list --search <query>` (short form `-S`) and through the GraphQL filter `ItemFilter.search`. Both use the same code and the same query syntax: the [Bleve](https://blevesearch.com/) *query string* syntax. Measured on Lif 0.14, 2026-10-14.

:::caution[AND, OR and NOT do not work]
The words `AND`, `OR` and `NOT`, parentheses, `&&`, `||` and `!` are **not** operators. Every list of terms is an OR. Use `+term` (required) and `-term` (excluded) instead.
:::

## Where search applies

- CLI: `lif list --search <query>` or `lif list -S <query>`. `lif list` is the only command with a search flag.
- GraphQL: `items(filter: { search: "..." })`. See [GraphQL](../graphql/). Nested fields such as `Item.children(filter:)` ignore `search`.
- TUI: the `/` search sends text of three or more bytes through `lif list --search`, so the syntax is the same. See [Views](../tui/views/).

The query is one shell argument, because `lif list` takes no positional arguments. An empty `--search ''` means no search. A whitespace-only query is an error.

## Searched fields

The index has four fields: `id`, `slug`, `title`, `body`. A term without a field searches all four. The flag help says "title and body"; the slug and the exact ID are searched as well.

Status, type, priority, tags, parent, links and extra front matter keys are **not** searchable. Use the filter flags of `lif list` for them (see [Combining with filters](#combining-with-filters)).

## Terms and phrases

| Query | Meaning |
|---|---|
| `login` | Item with the word `login` (case-insensitive). |
| `user login` | OR: `user` or `login`. |
| `"user login"` | Phrase: the words next to each other, in this order. |
| `+login +user` | Both words required. |
| `login -user` | `login`, but not `user`. |

Details that matter:

- Matching is case-insensitive and whole-word. There is no stemming: `user` does not find `users`.
- Umlauts are kept as letters but not folded: `ärger` matches, `arger` does not.
- English stop words (`and`, `or`, `not`, `the`, `a`, `of`, ...) are dropped at index time and query time and can never match.
- A hyphen separates words: `well-known` is `well` OR `known`. Quote it (`"well-known"`) for a phrase.
- A dot between letters or digits keeps the token together: `v1.2.0` matches `v1.2.0`, not `1.2.0`.
- A phrase matches inside one field value, never across title and body.
- A colon inside a term always starts a field scope. `file.go:12` finds nothing.

Quote the query for the shell: single quotes outside, double quotes inside.

```sh
lif list -S '"user login"'
```

## Fuzzy, wildcard and regex

| Query | Meaning |
|---|---|
| `login~` | Fuzzy, edit distance 1. |
| `login~2` | Fuzzy, edit distance 2 (maximum; `~3` is an error). |
| `log*` | Prefix. `*` is any run of characters, `?` exactly one. |
| `*gin`, `l*n` | Wildcard anywhere in the term. |
| `/log.*/` | Regular expression (Go syntax), whole-term match. |

Wildcards and regular expressions are **not** lowercased: `LOG*` finds nothing, `log*` does. Fuzzy and phrase cannot be combined (`"a b"~2` is a syntax error). A lone `*` matches every item.

## Field scopes

Prefix a clause with `slug:`, `title:`, `body:` or `id:`.

- `title:release` searches the title only.
- `body:"user login"` searches the body only.
- A scope covers only the one clause after it. `title:(a b)` does not group.
- Field names are case-sensitive. An unknown name (`status:todo`) returns no hits and no error.
- `id:` needs the exact, case-sensitive full ID.

## Combining with filters

Search and filter flags are strictly AND-combined. The search runs first, then the filters narrow the hits.

```sh
lif list -S login --status todo
```

Within one flag the values are OR (`--status todo,in-progress`, `--tag a,b`). A repeated `--where key=value` is AND. The same holds in GraphQL: `search` sits next to `status`, `type` and `tags` in `ItemFilter`.

## Examples

Real output from a 19-item test store, Lif 0.14, 2026-10-14. Item IDs are shortened here.

```sh
lif list -S login
```

```text
Items
TYPE     ID         TITLE                                  STATUS       PRIORITY
feature  proj-f001  Add OAuth login via GitHub             in-progress
bug      proj-b001  Fix login redirect loop                todo         high
feature  proj-f003  Show the user login form              todo
task     proj-t003  Rename authentication module           completed
task     proj-t004  Obsolete login popup                   scrapped
```

More calls from the same store (IDs printed with `-q`):

| Call | Result |
|---|---|
| `lif list -S '"login page"' -q` | `b001 t004` |
| `lif list -S 'login~2' -q` | `f001 b001 f003 b004 t008 t003 t004` (`b004` has `loggin`, `t008` has `lgoin`) |
| `lif list -S 'auth*' -q` | `e001 f001 m001 t003` |
| `lif list -S 'title:login' -q` | `f001 b001 f003 t004` |
| `lif list -S 'login -user' -q` | `f001 t003 t004` |
| `lif list -S login -s todo -q` | `b001 f003` |

The same search through GraphQL:

```sh
lif graphql --json -v '{"s":"login"}' 'query($s:String){ items(filter:{search:$s}) { id title } }'
```

## Limits

- No AND, OR, NOT, no grouping. Use `+term` and `-term`.
- Boost `term^2` is accepted but changes only the score. The result order is the default sort order of `lif list` (status, manual order, priority, type, title), not the relevance order.
- Ranges (`title:>5`) never match; no numeric or date field is indexed.
- At most 1000 hits are returned. The cap applies before the other filters, so a filter can miss items beyond the first 1000 hits.
- An invalid query prints `Error: querying items: <message>` and exits with 1, also with `--json`. No JSON error envelope.
- Archived items are searched too.
- The index is built in memory on every `lif list --search` call. A long-running process (`lif-serve`) updates it incrementally.
