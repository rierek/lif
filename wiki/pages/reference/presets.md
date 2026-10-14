---
title: "About presets"
description: "Profiles that expand into your config: classic, todo, simple, complex."
sidebar:
  order: 2
---

:::caution[Raw draft]
Source: `wiki/project-profiles.md`. Topic outline, bullet points only.
:::

- Profile = named type-table preset, applied once with `lif init --profile <name>`.
- Expanded into `types` + `types_exclusive: true`. Not a live link: later Lif releases never change your types.
- Cannot combine with `--lif-path`. Unknown name fails before anything is created.
- Four fixed ranks; names and relations are yours to change.
- Presets are examples, not a required method.

| Preset | Shape | Typical user |
|---|---|---|
| classic | milestone > epic > feature > bug/task | software teams, default |
| todo | flat `task` | solo, personal |
| simple | milestone/bucket > epic/feature > bug/task | small product team |
| complex | release/bucket > feature/improvement/chore > story/bug/task | mature product org |

## Profiles

### classic

#### Description

- built-in default. `milestone`(1) > `epic`(2) > `feature`(3) > leaf `bug`, `task`(4). Same as plain `lif init`.

#### When to use

- team already works milestone/epic style; joining a multi-project setup where all use the default; baseline to fork.

#### When not to use

- solo backlog (too heavy); need customer-value vs internal split.

#### Good practices

- milestone = something that ships; keep epics themed; features user-facing; use tags for cross-cutting.


### todo

#### Description

- one leaf type `task`, no containers. Grouping only via tags.

#### When to use

- personal project, small backlog, short-lived spike.

#### When not to use

- releases, themes, multiple people needing structure.

#### Good practices

- few tags, use `priority` and `blocked_by` to order; migrate to `simple` when list outgrows one screen.


### simple

#### Description

- rank 1 `milestone` (ships) or `bucket` (someday parking lot, `roadmap: false`). Rank 2 `epic` (theme) and `feature` (capability), siblings. Leaf `bug`, `task`.

#### When to use

- small product team; wants visible someday ideas without roadmap clutter; leaving a flatter tracker.

#### When not to use

- need release tags as container; need customer-value split.

#### Good practices

- promote bucket content into a milestone when committed; do not nest epic under feature.


### complex

#### Description

- rank 1 `release` (tagged version) and `bucket`. Rank 2 `feature`, `improvement`, `chore`. Leaf `story`, `bug`, `task`.

#### When to use

- reports customer value apart from internal investment; ships tagged releases; needs `story` separate from `task`.

#### When not to use

- small team, early project, anything where four levels is overhead.

#### Good practices

- chore = no customer-visible benefit; one release per tag; stories demonstrable.

