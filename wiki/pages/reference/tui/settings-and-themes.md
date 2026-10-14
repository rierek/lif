---
title: "Settings and themes"
description: "TUI settings file, editor, accent colour and themes."
---

Measured on Lif 0.13, code unchanged in 0.14 (checked 2026-10-14).

## Settings file

The TUI reads `~/.config/lif/tui/config.yaml`. A second file, `state.json` in the same directory, stores `last_repo`. If the new file does not exist, a legacy `~/.config/beans-tui/config.yaml` is read; it is never written. Starting `lif tui` creates nothing.

| Key | Meaning | Default |
|---|---|---|
| `repos` | List of repository paths for the [repository picker](../start-and-repositories/). | empty |
| `repo_store_paths` | Map from a `repos` entry (exact string) to a store path, passed as `--lif-path`. | empty |
| `editor` | Command to edit items. | empty (`$VISUAL`, `$EDITOR`, `vi`) |
| `theme.accent` | Accent color, `#rrggbb`. | empty (built-in mauve) |
| `layout.tree_width` | Minimum width of the tree pane, 24 to 60. | 36 |

The file belongs to the TUI and applies per user. Project settings live in `.lif.yml` ([Configuration](../../configuration/)).

The settings form opens from the command palette: `K`, then `go to settings`. It has no key. It shows `repos`, `editor`, `accent` and `tree_width`, rejects invalid values with a message, and applies `editor` and `accent` at once. Saving rewrites the whole file from these keys: comments and unknown keys are lost; `repo_store_paths` is kept.

## Example settings file

```yaml
repos:
    - /home/me/code/app
    - /home/me/code/site
editor: nvim
theme:
    accent: "#f5a97f"
layout:
    tree_width: 40
repo_store_paths:
    /home/me/code/site: /home/me/code/site/.lif
```

Quote the accent. Without quotes `#f5a97f` is a YAML comment and the value is empty.

## Choosing the editor

`e` or `ctrl+e` opens the item in an editor. Order: `editor` setting, then `$VISUAL`, then `$EDITOR`, then `vi`. Empty or blank values are skipped. If nothing works and `vi` is missing, a toast shows `exec: "vi": executable file not found in $PATH`.

- The value is split on spaces. There is no shell quoting, and a path with a space fails. Quotes in arguments are passed literally. Lif appends the temp file as the last argument.
- A terminal editor (`nvim`, `vim`) takes over the screen; the TUI resumes when it exits.
- A GUI editor must wait until the file is closed: `editor: code -w`. Otherwise the TUI reads the file before you edit it and a late save is lost.

## Themes

`display.theme` in `.lif.yml` (`latte`, `frappe`, `macchiato`, `mocha`; default `mocha`) colors the CLI output. It has **no effect in the TUI**: the TUI uses a fixed palette and the background is always dark. Only `theme.accent` changes the TUI. Tracked as [179g]. `lif check` still reports an unknown `display.theme`.

Custom themes do not exist.

## Display width

`display.max_width` (0 = 110, `-1` = no limit) and the flag `--max-width` apply to CLI output. The TUI does not read them.

## Invalid values

The TUI does not report problems in the file.

- An unreadable file, broken YAML, a value of the wrong type or an empty file: the whole file is ignored and defaults apply.
- `layout.tree_width` is clamped: above 60 becomes 60, 1 to 23 becomes 24, 0, negative or omitted becomes 36.
- An invalid `theme.accent` (`red`, `#fff`) is discarded and the default stays.
- Unknown keys are ignored.
