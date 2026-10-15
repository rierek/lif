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
| [8h4v] | `lif-serve` has no authentication or TLS and exposes a terminal | Partly fixed in 0.15: the server listens on `127.0.0.1` only unless you pass `--host`, it refuses web pages whose `Host` is not on the origin list and, for changes, whose `Origin` is neither on the list nor its own, and beyond loopback it asks every client for a generated access token and serves the terminal only with `--allow-remote-terminal`. Still open: there is no TLS, so beyond loopback the token travels in clear text, and on loopback there is no authentication, so every local user of the machine can use the port. | Use the default address and an SSH tunnel or a VPN for remote access. Convention, not enforced by Lif: do not put a reverse proxy without its own authentication in front of it. | Open (partly fixed in 0.15) |
| [xa2j] | AND, OR, NOT promised in search help and GraphQL schema do not work | `user AND login` is an OR of three terms. Parentheses do not group. | Use `+term` (required) and `-term` (excluded). | Open |
| [z9e6] | Publish JSON schema files generated from the code | No schema files for `--json`; the shapes can change with any release. | Run the command against your own version. | Open |
| [179g] | TUI ignores `display.theme`, fixed value lists, quit text says `bt` | Menus and the create form offer only the built-in statuses, types and priorities. The theme is fixed. The quit dialog says `Really quit bt?`. | Set custom values with `lif update` or in the item editor (`e`). | Open |
| [0g6b] | `show --json` returns the body with a leading newline, `create --json` does not | Body comparisons differ between commands. | Trim the body. | Open |
| [ju1q] | A create refused for its parent returns `FILE_ERROR` instead of `VALIDATION_ERROR` | You cannot tell a validation refusal from an I/O failure by code. | Match the message text. | Open |
| [hh0o] | TUI lobby footer shows `i/k`, but those keys edit the filter | Misleading key hint. | Use the arrow keys. | Open |
| [mgx8] | `create`/`update` help lists built-in names; completion and help fail outside a project | Help shows built-in type names even with custom types. `lif completion`, `lif help <cmd>` and `lif __complete` fail outside an initialised project. | Run them inside a project. | Open |
| [3slb] | `lif --version` is rejected | `lif --version` prints `Error: unknown flag: --version`. | Run `lif version`. | Open |
| [t5k2] | Persisted search index path is built but never filled | No effect for users: every call indexes in memory. | None needed. | Open |
| [29wh] | Wiki and README statements disagree with the code | Some older statements (cycle entry fields, `display.theme`, `default_status` check, i386 platforms, `go install` binaries) are wrong. | Trust this site and a run of your binary. | Open |
| [gbl5] | `lif rename --dry-run` hits the same refusals as a real rename | The dry run stops at the first refusal instead of listing the full plan. | Fix the refusal and run again. | Open |

## Fixed

| ID | Title | Effect | Workaround | Status |
|---|---|---|---|---|
| [0pv6] | GraphQL `createItem` left status empty | Fixed in 0.14: the default status `todo` is applied. | n/a | Fixed in 0.14 |
| [z69m] | `lif promote` wrote an empty status and could fail part-way | Fixed in 0.15: promoted items get the configured default status, and `lif promote` checks every selected record (code mapping, create-time field rules, parent) before the first write, so an invalid candidate writes nothing. If a write still fails, the error names the items already written. | n/a | Fixed in 0.15 |
| [tgph] | `lif promote` created the item without a parent | Fixed in 0.15: `lif promote --parent <id>` creates every promoted item under that item, so it counts in the release progress. Without `--parent` an item still has no parent. | n/a | Fixed in 0.15 |
| [nhcz] | `lif promote` review artifact format was undocumented | Fixed in 0.15: the `lif promote` command page describes the artifact, the finding record fields per code and how a record becomes an item. | n/a | Fixed in 0.15 |
| [6y1a] | Type change skipped hierarchy validation | Fixed in 0.14: the hierarchy is validated before the first write. | n/a | Fixed in 0.14 |
| [9kej] | Help text omitted manual order in the default sort | Fixed in 0.14: help names the manual order. | n/a | Fixed in 0.14 |
| [g5dn] | `lif next` and `lif list --ready` returned container items | Fixed in 0.15: both leave out every container type (rank 1 to 3), so the ready list is leaf work only. | n/a | Fixed in 0.15 |
| [et2a] | Mixed-field dependency cycles were accepted and not reported | Fixed in 0.15: cycle detection treats `blocking` and `blocked_by` as one set of links, so `lif update`, `lif create`, GraphQL and `lif check` all refuse or report a cycle whichever field stores its links. | n/a | Fixed in 0.15 |
| [o8bs] | A blocker inside the blocked item's own subtree deadlocked it | Fixed in 0.15: a blocker below the blocked item, or above it, is refused on write and reported by `lif check` as a cycle over `blocking` and `parent` links, with a note that says why; the rule looks at the links only, so a blocker that is already `completed` or `scrapped` is refused too. | n/a | Fixed in 0.15 |
| [0ish] | Cycle path in `lif check` started at a different item each run | Fixed in 0.15: a cycle path starts at its smallest item ID, and cycles are listed in a fixed order. | n/a | Fixed in 0.15 |
| [ih2r] | UTF-8 BOM before front matter made `lif update` duplicate it | Fixed in 0.15: a leading BOM is ignored on read, so the item loads normally, and a write produces one front matter block and no BOM. | n/a | Fixed in 0.15 |
| [1wnd] | Unrecovered panic on a `.nan` front matter value | Fixed in 0.15: a `.nan`, `.inf` or `-.inf` number in an item's front matter is refused when the store loads, with an error that names the file and the key and exit code 1, not a panic; quote the value to keep it as text. | n/a | Fixed in 0.15 |
| [533z] | Renaming an item file by hand changed its ID without a word | Fixed in 0.15: `lif check` reports a file whose name gives another ID than the `# <id>` line in the file, naming both IDs and the links that dangle, and `lif check --fix` keeps those links instead of removing them; the ID still comes from the file name, so rename files with `lif rename`. | n/a | Fixed in 0.15 |
| [1y6w] | `lif init` silently overwrote an existing config | Fixed in 0.15: `lif init` stops with an error and exit code 1 when `.lif.yml` or `.beans.yml` exists in the directory, and keeps an existing store `.gitignore`; `lif init --force` replaces the config with the default config. | n/a | Fixed in 0.15 |
| [9r10] | The `--if-match` flag did not protect two simultaneous processes | Fixed in 0.15: an update with `--if-match` locks the item file from the etag check to the write, so of two simultaneous processes with the same etag exactly one writes and the other gets `etag mismatch`, also when the winner changes the title and so renames the file; a write without `--if-match`, `lif rename` and `lif archive` take no lock, and on a file system without working file locks (some NFS or FUSE mounts) an update with `--if-match` fails with a locking error instead of running unprotected. | n/a | Fixed in 0.15 |
| [9a66] | `lif-serve` startup log prints `localhost` but binds all interfaces | Fixed in 0.15: the startup log names the address the listener really got and whether the terminal is enabled, and the default address is `127.0.0.1`. | n/a | Fixed in 0.15 |
