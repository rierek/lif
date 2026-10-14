---
title: "Known limitations"
description: "Open defects and limits of Lif, each with a short ID that release notes reuse."
---

## Purpose
One list of what Lif does not do right yet. Every limitation named on another page as `[abcd]` is listed here with the same ID. Release notes use the same IDs, so a reader can follow a limitation from the page that mentions it to the release that fixes it (D64, D65, D66).

## Audience
Users who hit a surprise and want to know whether it is known; readers who see `[abcd]` on another page; maintainers who keep the list current.

## Required sections

### How to read an ID
- Explains the notation: `[abcd]` is the tracking item ID without the prefix of the store; the same form appears on feature and reference pages and in release notes.
- States that the list is maintained by hand and shows the status of each entry (open, fixed in a version).
- Source: DESIGN.md D65, D66

### Open limitations
- Table: ID, title, one-line effect, workaround, status.
- Groups: data safety (mixed-field cycles [et2a], `lif init` overwrite [1y6w], `lif promote` [z69m]); web server ([8h4v]); search ([xa2j]); TUI ([179g]); CLI help and completion ([mgx8], [9kej]); front matter handling ([1wnd], [ih2r]); other ([0ish], [gbl5], [9r10], [t5k2], [29wh], [0pv6], [6y1a]).
- Each row's effect text comes from the tracking item and FACTS.md §22.4; a workaround appears only where FACTS.md has one.
- Source: FACTS.md §22.1, §22.2, §22.4; DESIGN.md D64

Limitations cited on Reference pages (Lif 0.14, measured 2026-10-14):

| ID | Title | Effect | Workaround | Status |
|---|---|---|---|---|
| [8h4v] | `lif-serve` has no authentication or TLS and exposes a terminal | Anyone who reaches the port can change items and open a shell as the server user. The server listens on all interfaces. | Convention, not enforced by Lif: block the port in a firewall and use an SSH tunnel or a VPN. | Open |
| [xa2j] | AND, OR, NOT promised in search help and GraphQL schema do not work | `user AND login` is an OR of three terms. Parentheses do not group. | Use `+term` (required) and `-term` (excluded). | Open |
| [z9e6] | Publish JSON schema files generated from the code | No schema files for `--json`; the shapes can change with any release. | Run the command against your own version. | Open |
| [179g] | TUI ignores `display.theme`, fixed value lists, quit text says `bt` | Menus and the create form offer only the built-in statuses, types and priorities. The theme is fixed. The quit dialog says `Really quit bt?`. | Set custom values with `lif update` or in the item editor (`e`). | Open |
| [z69m] | `lif promote` can fail part-way | The empty-status part is fixed in 0.14. A run can still stop part-way and leave a half-promoted set. | Check the result with `lif list` after a failed run. | Open (partly fixed in 0.14) |

### Fixed limitations
- Table: ID, title, version that fixed it with a link to its release note.
- Empty until the first fix ships; the section stays so the structure is stable.
- Source: DESIGN.md D66

### Report a limitation
- Links to Contribute (`contribute`) for the issue tracker and to the Discussion rule for new features.
- Source: DESIGN.md D63
