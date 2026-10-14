---
title: "Blog"
description: "Experiments, integrations and field notes around Lif."
---

## Purpose
Overview of blog posts: experiences, experiments, integrations and extensions around Lif, without a fixed narrative structure.

## Audience
Readers who want to look beyond the core scope, and authors who decide whether a text belongs on the blog or in Use cases.

## Required sections

### What belongs on the blog
- Explains: the blog shares a concrete experience, experiment, integration or extension; use cases, in contrast, show generally applicable solutions.
- Example of the scope boundary: use case "Managing a Software Project with Lif" versus blog "How I connect items with Git worktrees".
- Rule: blog posts link to the reference instead of repeating technical definitions.
- Possible topics per seed: Git worktree integration, traceability from item to implementation, release workflow, custom scripts, Obsidian.
- Source: CONCEPT.md §6 (Purpose, Potential Articles, Editorial Distinction); DESIGN.md D34, D42

### Post list
- Shows the post list chronologically, newest first, with title, date, tags and short description.
- Current posts: Managing Lif items in Obsidian (`blog/obsidian-experiment`).
- List mechanism open: the Starlight blog plugin from D34 is not installed. (TODO: verify)
- Source: DESIGN.md D34; mechanism `TODO: verify` — check `site/` (plugins in `astro.config.mjs`) in phase 5

### Tags and the experiment tag
- Explains: posts carry tags; experimental posts carry the tag `experiment`.
- Shows the tag list with counts as soon as more than one post exists.
- Explains what the tag `experiment` means: tried out, not promised as a supported integration.
- Source: DESIGN.md D34, D42; tag mechanics of the plugin `TODO: verify`

### Notice box for experimental posts
- Shows the notice text that every post with tag `experiment` carries at the top.
- Explains the purpose: readers see that behavior is not guaranteed by Lif and reference pages take precedence.
- Determines how the box is implemented (plugin function or Starlight aside). (TODO: verify)
- Source: DESIGN.md D34, D42; implementation `TODO: verify` — align with page generator and plugin in phase 5
