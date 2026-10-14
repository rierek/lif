# Validation and Maintenance

This page documents `lif check` (including completion-policy validation and legacy-name reporting), `lif archive`, the command that moves finished items out of the active working set, and `lif promote`, which turns reviewed findings into items. Run `lif check` before trusting a store's integrity, and `lif archive` to keep a long-lived project's `.lif` directory small.

## `lif check`

`lif check` validates two independent things in one pass: the project's configuration, and the integrity of the item files it describes.

```
$ lif check
Configuration
  ✓ Statuses defined (5 hardcoded)
  ✓ Default status 'todo' exists
  ✓ Default type 'task' is valid
  ✓ Every item carries a known type
  ✓ All status colors valid
  ✓ All type colors valid
  ✓ Prefix consistency valid

Item Links
  ✓ No link issues found

All checks passed
```

Configuration checks cover: that `default_status` and `default_type` name entries that actually exist, that `default_priority` names a known priority and every `default_tags` entry is a valid tag, that every item on disk carries a type the current configuration still defines (a config edit or profile switch can leave items behind with a type nothing recognizes anymore), that every configured status and type `color` names a valid theme tone, and that the configured ID prefix (`lif.prefix`) matches what the item files on disk actually carry — archived items are excluded from that comparison, since a project can rename its live prefix without needing to touch history.

Item field checks cover what a hand-edited Markdown file can get wrong and no command would have accepted: a `status` or `priority` the configuration does not define, a tag that breaks the tag rules, and a `parent` whose type does not rank above the item's own type. Each finding is a configuration error and makes the command exit 1. A `parent` that does not exist is not reported here; the item-link check below covers it. The configuration holds no tag list, so "invalid tag" means a tag that violates the tag format rules, not one missing from a registry.

Item-link checks cover three classes of structural problem, all evaluated together:

- **Broken links** — a link field (`blocking`, `blocked_by`, `parent`, and similar) pointing at an ID that does not exist.
- **Self-references** — an item linking to itself.
- **Circular dependencies** — a cycle in `blocking`/`blocked_by` or `parent` relationships.

`--fix` removes broken links and self-references automatically; cycles cannot be auto-fixed and are always reported for manual resolution, even with `--fix` set. Flags:

| Flag | Description |
|---|---|
| `--fix` | Automatically remove broken links and self-references |
| `--json` | Output as JSON |
| `--strict` | Count policy warnings (see below) as issues, so the command exits 1 when any exist |

`--json` output is the same envelope every command uses, with `check`-specific fields:

```json
{
  "success": true,
  "config_errors": null,
  "item_issues": { "broken_links": [], "self_links": [], "cycles": [] }
}
```

`config_errors` is a flat list of human-readable strings, one per configuration problem found; `item_issues` mirrors the link-check sections shown in the plain-text output. On a real failure `success` is `false` and the process exits 1 regardless of `--json`.

When the project still uses a legacy data name, `lif check` adds a `Legacy names` section with one warning per name in use, and `--json` adds a `legacy_names` array with the same strings. The warnings never change the exit code. Each names the legacy name, the current name and the step that ends it:

```
Legacy names
  ! config file .beans.yml is a legacy name: run `git mv .beans.yml .lif.yml` in /path/to/project
  ! section key beans: in .beans.yml is a legacy name: rename the key to lif:
  ! store directory .beans is a legacy name: run `git mv .beans .lif` in /path/to/project, then change path: .beans to path: .lif in .beans.yml, and run `git mv .beans.yml .lif.yml`
  ! environment variable BEANS_PATH is a legacy name: export LIF_PATH instead
```

See [Legacy names](../configuration.md#legacy-names) for what is still read and [Compatibility and upgrading](../compatibility-and-upgrading.md) for each step.

## Completion-policy validation

When a project configures `lif.require_fields_on` in `.lif.yml` (see [Configuration Reference](../configuration.md)), `lif check` gains a third section, `Policy`, that re-validates the policy against every item's current on-disk state:

```
Policy
  ! ae67: status "completed" missing required field(s): summary
```

This section exists because the policy is primarily enforced at write time — `lif complete`, `lif update --status`, and the other status-changing commands refuse the write up front when a required field would be left unset:

```
$ lif complete i9qh
Error: status "completed" requires front matter field(s) summary: supply them in the same write (e.g. `lif complete i9qh --commit HEAD`)
Run 'lif complete --help' for usage.
```

`lif check` catches what the write-time guard cannot: an item edited directly on disk (bypassing the CLI), or a policy added or tightened after items already sit in the now-noncompliant status. A policy gap is reported as a warning, not a hard failure, because it describes an existing project state rather than a rejected write — by default it does not affect the process exit code. Pass `--strict` to treat these warnings as issues that fail the command (exit 1), which is the right mode for a CI check that must enforce the policy going forward.

When `lif.commit_field` names a field that also appears in `require_fields_on`, `check` additionally verifies that every recorded value resolves to a real commit in the current git repository; a value that doesn't (or a check run outside a git repository, where verification is skipped entirely) is reported as its own policy warning.

## `lif archive`

`lif archive` moves every item whose status is configured as an archive status (`completed` or `scrapped` by default) from the active `.lif` directory into `.lif/archive/`.

```
$ lif archive
Archived 1 item(s) to .lif/archive/
```

Archiving is a filesystem move, not a state change: an archived item keeps its status, relationships, and full front matter, and remains visible to `lif list`, `lif show`, `lif graphql`, and `lif check` at its path under `archive/`. Archiving keeps the main directory small; it does not delete or hide history. There is no `unarchive` command. Updating an archived item currently leaves its file in `archive/`; move the file back to the main items directory manually when you want it active again.

`lif archive --json` returns the standard message envelope (`{"success": true, "message": "Archived N item(s) to .lif/archive/"}` or `{"success": true, "message": "No items to archive"}` when nothing qualified).

## `lif promote`

`lif promote <artifact> <finding-id> [<finding-id>...]` validates the named finding records from a review artifact against the findings schema and, once every selected record passes, creates one item per record by a fixed code-to-type mapping. It writes nothing at all if any selected record is malformed, and it skips a finding that an earlier call already promoted rather than duplicating it. `--json` prints the result as JSON.

```
lif promote review-findings.json F-01 F-02
```

## Related documentation

- [Querying and Automation](querying-and-automation.md)
- [Configuration Reference](../configuration.md)
- [Web UI and API](../web-ui-and-api.md)
- [Troubleshooting](../troubleshooting.md)
