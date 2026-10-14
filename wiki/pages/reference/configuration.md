---
title: "Configure and customize Lif"
description: "Config file, stores, types, statuses, priorities, display."
sidebar:
  order: 4
---

:::caution[Raw draft]
Source: `wiki/configuration.md`. Candidate for generation from `pkg/config`.
:::

- Config file `.lif.yml` at project root, found by searching upward. `--config <path>` overrides. Legacy `.beans.yml` read when no `.lif.yml`.
- All sections optional: `project`, store section, `worktree`, `agent`, `server`, `display`. Missing = default.
- `lif init` writes only fields that differ from defaults.
- Store path precedence: `--lif-path` > found config (`path`, default `.lif`) > `LIF_PATH` (legacy `BEANS_PATH`) only when no config found > default dir.
  - Reason: config file is the repo's own declaration; env vars inherited via direnv must not redirect to a foreign store.
  - `lif path` prints the resolved dir.
- `anchor: repo-root`: all git worktrees share one store. Unset: each worktree has its own. Other values fail loading.
- `project.name`: display name in web UI and TUI only.
- Also: prefix, `id_length`, `default_status`, `default_type`, types (rank, color, `roadmap`), `types_exclusive`, statuses, priorities, display colors, `require_fields_on` policies.
- Terminology free, four ranks fixed.
- TODO: generate key/type/default table from config struct; annotated full example; recipes (rename a type, add a status, custom colors).
