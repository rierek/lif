---
title: "Glossary"
description: "Short definitions of Lif terms, each with the page that has the details."
---

## How to read the glossary

The terms follow the code: item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset, store. "Level" is only a synonym for rank in running text, never a field name.

**Liff** is the name for the Markdown file that stores an item. Field names, commands, JSON and GraphQL keep the word `item`.

Each entry has one line of definition and the page with the details.

## Core terms

- **item, Liff:** an item is a unit of work (task, feature, epic, milestone, ...). A Liff is the Markdown file for it. [Item file format](../item-file-format/).
- **type, rank:** the type names the kind of item. The integer `rank` decides which type may sit under which: the child rank must be strictly greater than the parent rank. [Configuration](../configuration/types-and-statuses/#types-ranks-and-hierarchy-rules).
- **status, priority:** status is the state of the work (`in-progress`, `todo`, `draft`, `completed`, `scrapped`). Priority is the urgency (`critical` to `deferred`). [Item file format](../item-file-format/#allowed-values-and-defaults), [Configuration](../configuration/types-and-statuses/#statuses-types-and-priorities).
- **tag, parent:** a tag is a lower case label with hyphens. `parent` is the ID of the one item above. [Item file format](../item-file-format/#front-matter-fields).
- **blocking, blocked_by:** two lists of IDs. `blocking` names what this item blocks, `blocked_by` names what blocks it. A link is stored on one side only. [Dependencies](../../features/dependencies/), [Item file format](../item-file-format/#relationships).
- **preset:** a type table that `lif init --profile` writes once into `.lif.yml` (`classic`, `todo`, `simple`, `complex`). [Configuration](../configuration/types-and-statuses/#presets).

## Store and files

- **store, archive:** the store is the directory with the Liffs (default `.lif/`). `archive/` is the subdirectory for items with an archive status; they stay loaded and addressable. [Item file format](../item-file-format/#where-files-live), [Store management](../../overview/store-management/).
- **store location:** where Lif looks for the store: `--lif-path`, then the config file (`lif.path` or the default directory next to it), then `LIF_PATH`, then `.lif/` in the current directory. `lif path` prints the result. [Configuration](../configuration/#store-resolution).
- **attachments:** `attachments/<item-id>/` holds evidence per item. [Item file format](../item-file-format/#where-files-live).
- **ID, prefix, slug:** the ID is the prefix plus a random suffix. The slug is a short form of the title in the file name `<id>--<slug>.md`. The short ID is the ID without the prefix. [Item file format](../item-file-format/#file-name-and-id).
- **anchor:** `lif.anchor: repo-root` lets all Git worktrees of a repository share one store. [Configuration](../configuration/#store-resolution).
- **ETag:** a hash of the file bytes, used for `--if-match` and GraphQL `ifMatch`. [Item file format](../item-file-format/#body).

## Planning terms

- **container, leaf:** a container is an item on rank 1 to 3, a leaf is on rank 4 (built-in scale). Custom types can have other ranks. [Configuration](../configuration/types-and-statuses/#types-ranks-and-hierarchy-rules).
- **order:** the manual order key (`order` in the front matter, a base62 fractional index) that places an item among its siblings. Set it with `lif order`. [Concepts](../../overview/concepts/), [Item file format](../item-file-format/#front-matter-fields).
- **ready:** a leaf (not a container type) that is not blocked (directly or through an ancestor), whose status is not `in-progress`, `draft` or an archive status, and that has no ancestor `completed` or `scrapped`. [Dependencies](../../features/dependencies/), [`lif next`](../commands/lif-next/).
- **blocked, explicit and implicit:** explicit means the item has active blockers itself. Implicit means an ancestor has active blockers. Active means the blocker's status is not `completed` or `scrapped`. [Dependencies](../../features/dependencies/).
- **roadmap:** a tree of milestones, epics and their children. Types with `roadmap: false` do not appear. [`lif roadmap`](../commands/lif-roadmap/), [Configuration](../configuration/types-and-statuses/#presets).

## Interface terms

- **CLI, `lif prime`:** the CLI is the binary `lif`. `lif prime` prints a prompt that introduces a coding agent to Lif. [Command overview](../commands/), [Agent onboarding](../../overview/agent-onboarding/).
- **TUI:** the terminal interface, started with `lif tui`. [Start and repositories](../tui/start-and-repositories/).
- **`lif-serve`:** the separate binary for the web server. [Web server](../web-server/).
- **GraphQL:** the query and mutation interface. Use it in process with `lif graphql` (alias `lif query`) or over HTTP through `lif-serve`. [GraphQL](../graphql/).

## Legacy terms

- **bean:** the former name for item; today it is item or Liff. The JSON fields `bean_id` and `bean_issues` are now `item_id` and `item_issues`. [JSON output](../json-output/).
- **`.beans/`, `.beans.yml`, `beans:`:** the old store folder, config file and config key. Lif reads them in place and never renames them. `lif check` reports them under "Legacy names". [Configuration](../configuration/).
- **`BEANS_PATH`:** the old environment variable. It is read when `LIF_PATH` is not set; setting both is an error. [Configuration](../configuration/#store-resolution), [Store management](../../overview/store-management/).
- **`beans-` prefix:** the old ID prefix. Change a project-wide prefix with `lif rename --prefix "<new>-"`; add `--dry-run` to see the plan first. [`lif rename`](../commands/lif-rename/).
