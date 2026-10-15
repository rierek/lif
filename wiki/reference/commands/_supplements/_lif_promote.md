### Review artifact

The artifact is one JSON file with one top-level key, `findings`, an array of finding records (it may be empty). No other top-level key is allowed. The full schema is `internal/commands/findings-schema.json` in the source; this section is its short form.

```json
{
  "findings": [
    {
      "id": "B01",
      "code": "B",
      "title": "Missing axis on legacy record",
      "axis": "standards",
      "evidence": ["S02", "S03"],
      "blocking": true,
      "criterion": "acceptance-criteria-violation",
      "fields": {
        "Schwere": "hoch",
        "Beschreibung": "Record has no axis field.",
        "Empfehlung": "Reject at validation time."
      }
    }
  ]
}
```

Every record has these keys, and no key outside this table:

| Key | Rule |
|---|---|
| `id` | The code letter plus a two-digit number, for example `B01`. The first letter must equal `code`. Numbers restart at `01` per artifact. |
| `code` | One of `D`, `T`, `B`, `I`, `Q`. |
| `title` | A non-empty string. It becomes the item title. |
| `axis` | `standards` or `spec`: the review axis that produced the finding. Required, no default. |
| `fields` | An object of string values. The exact key set depends on `code` (next table). No other key is allowed, and a `Status` key is never allowed. |
| `evidence` | An array of source references, each `S` plus two digits (`S02`). It may be empty. |
| `blocking` | Optional boolean, allowed only when `code` is `B`. |
| `criterion` | Required when `blocking` is `true`, forbidden otherwise. One of `acceptance-criteria-violation`, `regression`, `security-or-data-integrity-risk`, `demonstrated-stability-risk`. |

The field labels are German and fixed. Each code requires exactly these keys in `fields`:

| Code | Required `fields` keys |
|---|---|
| `D` | `Hintergrund`, `Entscheidung`, `Empfehlung` |
| `T` | `Prio`, `Aufgabe` |
| `B` | `Schwere`, `Beschreibung`, `Empfehlung` |
| `I` | `Beschreibung`, `Nutzen`, `Empfehlung` |
| `Q` | `Hintergrund`, `Relevanz`, `Frage` |

### How a record becomes an item

Only `B` and `I` records can be promoted. A `D`, `T` or `Q` record passes the schema but has no item type, so the command refuses it before it writes anything.

| Finding | Item type | Tags |
|---|---|---|
| `B` | `bug` | none |
| `I` | `task` | `improvement` |

- **Status.** The item gets the configured default status (`default_status`), as with `lif create`.
- **Parent.** With `--parent <id>` every promoted item is created under that item, and the parent must be a valid parent for the item type. Without `--parent` the item has no parent, and you can attach it later with `lif update <id> --parent <id>`. The artifact itself names no parent.
- **Front matter.** `review` (the artifact path exactly as you typed it), `finding` (the record `id`), `axis`, and for a `B` record `severity` (the value of `Schwere`).
- **Body.** The sections `## Description` (`Beschreibung`), `## Benefit` (`Nutzen`), `## Recommendation` (`Empfehlung`) and `## Evidence` (the `evidence` list), each only when it has content, then `## Provenance` with the artifact path and finding ID. `Schwere`, `blocking` and `criterion` are not copied into the body.
- **Repeat calls.** A finding is skipped, with the line `skipped <id>: already promoted from <artifact>`, when an item with the same `review` path and `finding` ID exists. Naming the same finding twice in one call is an error.

### All or nothing

Before the first write, the command checks every named record against the schema, maps it to an item, checks the title, tags and body against the same rules as `lif create`, and checks the parent. If any record fails, nothing is written and the error names the record and the field. If a write fails after that check (for example a disk error), the error lists the items already written, so you know which part of the call happened.
