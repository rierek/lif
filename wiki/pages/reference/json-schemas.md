---
title: "JSON schemas"
description: "Machine-readable output of lif commands."
sidebar:
  order: 5
---

:::caution[Raw draft]
Source: `wiki/commands/querying-and-automation.md`, `internal/commands/findings_schema_test.go`.
:::

- `--json` on commands. Success payload = domain model, no wrapper.
  - `list`, `show`, `progress`, `roadmap`: domain model.
  - `create`, `update`, `start`, `complete`, `scrap`, `delete`, `tag`, `order`: the affected item.
  - `check`: own `success` + diagnostics. Rename dry run: rename plan.
- Error payload: `{ "success": false, "error": "...", "code": "..." }`.
- Codes: `NOT_FOUND`, `NO_BEANS_DIR` (rename pending), `INVALID_STATUS`, `FILE_ERROR`, `VALIDATION_ERROR`, `CONFLICT`, `POLICY_VIOLATION`.
- Without `--json`: two stderr lines (error + `Run 'lif <cmd> --help'`), exit 1.
- Findings schema for review artifacts (`lif promote`).
- TODO: publish JSON Schema files, generate from Go types, jq recipes.
