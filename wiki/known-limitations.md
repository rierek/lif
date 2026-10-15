---
title: "Known limitations"
description: "Open defects and limits of Lif, each with a short ID that release notes reuse."
---

## How to read an ID

`[abcd]` is the ID of a tracking item without the store prefix. The same form appears on feature and reference pages and in release notes. This list is maintained by hand. Each entry shows its status: open, or fixed in a version.

Measured on Lif 0.14 (development build), 2026-10-14. Workarounds marked as convention are not enforced by Lif.

## Open limitations

| ID | Title | Effect | Workaround | Status |
|---|---|---|---|---|
| [et2a] | Mixed-field dependency cycles are accepted and not reported | A cycle that mixes `blocking` and `blocked_by` links is not found by `lif check`. | Keep dependencies in one field per chain. | Open |
| [9r10] | The `--if-match` flag does not protect two simultaneous processes | Two writers can both pass the check and one overwrites the other. | Run writers one at a time. | Open |
| [8h4v] | `lif-serve` has no authentication or TLS and exposes a terminal | Anyone who reaches the port can change items and open a shell as the server user. The server listens on all interfaces. | Convention, not enforced by Lif: block the port in a firewall and use an SSH tunnel or a VPN. | Open |
| [xa2j] | AND, OR, NOT promised in search help and GraphQL schema do not work | `user AND login` is an OR of three terms. Parentheses do not group. | Use `+term` (required) and `-term` (excluded). | Open |
| [z9e6] | Publish JSON schema files generated from the code | No schema files for `--json`; the shapes can change with any release. | Run the command against your own version. | Open |
| [179g] | TUI ignores `display.theme`, fixed value lists, quit text says `bt` | Menus and the create form offer only the built-in statuses, types and priorities. The theme is fixed. The quit dialog says `Really quit bt?`. | Set custom values with `lif update` or in the item editor (`e`). | Open |
| [z69m] | `lif promote` can fail part-way | The empty-status part is fixed in 0.14. A run can still stop part-way and leave a half-promoted set. | Check the result with `lif list` after a failed run. | Open (partly fixed in 0.14) |
| [9a66] | `lif-serve` startup log prints `localhost` but binds all interfaces | You may believe the server is local-only. See [8h4v]. | Bind or firewall explicitly. | Open |
| [0g6b] | `show --json` returns the body with a leading newline, `create --json` does not | Body comparisons differ between commands. | Trim the body. | Open |
| [ju1q] | A create refused for its parent returns `FILE_ERROR` instead of `VALIDATION_ERROR` | You cannot tell a validation refusal from an I/O failure by code. | Match the message text. | Open |
| [hh0o] | TUI lobby footer shows `i/k`, but those keys edit the filter | Misleading key hint. | Use the arrow keys. | Open |
| [nhcz] | `lif promote` review artifact format is undocumented | You cannot write an artifact without reading the source. | Read `internal/commands/findings-schema.json` in the repository. | Open |
| [1y6w] | `lif init` silently overwrites an existing config | An existing `.lif.yml` or `.beans.yml` is replaced by the default config, exit 0. | Back up the file before running `lif init` in an existing project. | Open |
| [mgx8] | `create`/`update` help lists built-in names; completion and help fail outside a project | Help shows built-in type names even with custom types. `lif completion`, `lif help <cmd>` and `lif __complete` fail outside an initialised project. | Run them inside a project. | Open |
| [3slb] | `lif --version` is rejected | `lif --version` prints `Error: unknown flag: --version`. | Run `lif version`. | Open |
| [o8bs] | A blocker inside the blocked item's own subtree deadlocks it | A section with `blocked_by` pointing at its own child is never ready, because a blocked parent blocks its children. `lif check` passes. | Put the blocker outside the subtree. | Open |
| [tgph] | `lif promote` creates the item without a parent | A promoted bug sits outside the release subtree and its progress number. | `lif update <id> --parent <feature-id>`. | Open |
| [1wnd] | Unrecovered panic on a `.nan` front matter value | Lif exits with code 2 and a panic trace. | Remove or quote the value in the file. | Open |
| [ih2r] | UTF-8 BOM before front matter makes `lif update` duplicate it | The front matter block appears twice after an update. | Save item files without a BOM. | Open |
| [t5k2] | Persisted search index path is built but never filled | No effect for users: every call indexes in memory. | None needed. | Open |
| [29wh] | Wiki and README statements disagree with the code | Some older statements (cycle entry fields, `display.theme`, `default_status` check, i386 platforms, `go install` binaries) are wrong. | Trust this site and a run of your binary. | Open |
| [0ish] | Cycle path in `lif check` starts at a different item each run | The same cycle is printed with a different first item between runs. | Read the path as a ring. | Open |
| [gbl5] | `lif rename --dry-run` hits the same refusals as a real rename | The dry run stops at the first refusal instead of listing the full plan. | Fix the refusal and run again. | Open |

## Fixed

| ID | Title | Effect | Workaround | Status |
|---|---|---|---|---|
| [0pv6] | GraphQL `createItem` left status empty | Fixed in 0.14: the default status `todo` is applied. | n/a | Fixed in 0.14 |
| [6y1a] | Type change skipped hierarchy validation | Fixed in 0.14: the hierarchy is validated before the first write. | n/a | Fixed in 0.14 |
| [9kej] | Help text omitted manual order in the default sort | Fixed in 0.14: help names the manual order. | n/a | Fixed in 0.14 |
| [g5dn] | `lif next` and `lif list --ready` returned container items | Fixed in 0.15: both leave out every container type (rank 1 to 3), so the ready list is leaf work only. | n/a | Fixed in 0.15 |
