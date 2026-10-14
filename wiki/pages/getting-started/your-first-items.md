---
title: "Your first items"
description: "Map an existing topic to epic, feature and items."
sidebar:
  order: 3
---

:::caution[Raw draft]
Topic outline, bullet points only.
:::

- Take one real topic you already have (a feature, a release, a cleanup).
- Model it: container (epic) > features > leaf tasks. Names are yours.
- Steps: `lif create` epic, add features with `--parent`, tasks under them, `blocked_by` for order.
- Look at it: `lif list`, `lif show`, `lif roadmap`, `lif next`.
- Work it: `lif start`, `lif complete`.
- Opens `.lif/` in the editor: the same data as Markdown.
- TODO: worked example with real output and screenshot.
