# Item Lifecycle

This page covers the commands that create items and move them through their statuses: `create`, `start`, `complete`, `scrap`, `archive`, and `delete`. Of these, `delete` is the only command that removes data permanently; `archive`, `complete` and `scrap` keep every item file.

## `lif create` (aliases `c`, `new`)

`lif create [title]` creates a new item with a generated ID and an optional title. Flags: `-t`/`--type string` sets the item type (`milestone`, `epic`, `feature`, `bug`, `task` in the default profile; the actual set depends on the project's configured type profile, see `../project-profiles.md`); `-s`/`--status string` sets the initial status (`in-progress`, `todo`, `draft`, `completed`, `scrapped`); `-p`/`--priority string` sets the priority (`critical`, `high`, `normal`, `low`, `deferred`); `-d`/`--body string` sets body content (`-` reads it from stdin), or `--body-file` reads it from a file; `--parent string` sets a parent item ID; `--blocked-by`/`--blocking stringArray` record blocking relationships and can be repeated; `--tag stringArray` adds tags and can be repeated; `--order string` sets an explicit fractional-index order value; `--prefix string` overrides the configured ID prefix for this item; `--set`/`--unset stringArray` add or remove extra front matter `key=value` pairs and can be repeated; `--json` returns the created item as JSON.

An item created without `-s` gets the project's configured `lif.default_status`, which is `todo` unless the configuration changes it — check `lif list` after creating one if you expect it to show up under `lif next`.

```
lif --lif-path ./demo/.lif create "Fix login bug" -t bug -p high -s todo
```

```
lif --lif-path ./demo/.lif create "Design auth flow" -t feature --parent lif-vvat --tag auth
```

### Guided form: `-i`/`--interactive`

`lif create -i [title]` opens a guided form with four steps in this order: title, type, priority, parent. The form is drawn on the controlling terminal (`/dev/tty`), so stdout carries only the result: the `Created <id> <path>` line, or with `--json` the item exactly as `lif create --json` prints it.

- **Terminal requirement.** Without a terminal on stdin the command exits non-zero at once with a message naming the missing terminal and never waits for input, so agents and scripts are safe. `lif create` without `-i` never opens a form, on a terminal or not.
- **Pre-fill order.** A field takes its flag (title argument, `--type`, `--priority`, `--parent`, `--tag`, `--status`), else its config default (`lif.default_type`, `lif.default_priority`, `lif.default_tags`, `lif.default_status`), else stays empty. There is no parent default. Whatever you enter in the form overrides every pre-fill. An empty priority pre-selects `(none)`, and the item then carries no priority of its own, as with plain `lif create`.
- **Parent step.** It lists the valid parents for the type chosen in the form as a roadmap tree, one row per item, with a context panel for the highlighted row; press `/` to filter by ID fragment or title. A pre-filled `--parent` that is no valid parent for the chosen type is dropped. A type without a valid parent type offers only `(no parent)`.
- **Action menu.** After the parent step a menu offers, in this order: `go to body`, `create`, `set tags`, `set blocked_by`, `set blocking`, `set status`. The cursor starts on `create`, so Enter writes the item at once. `go to body` opens an inline text field pre-filled with the body (`--body`); `Ctrl+E` opens `$EDITOR` on it and takes the saved text. `set tags` takes a comma separated list and rejects an invalid tag in place; new tags are allowed. `set blocked_by` and `set blocking` are filterable multiple choices (`/`) over the same items `--blocked-by` completes. `set status` picks among the configured statuses instead of `lif.default_status`. Each step starts with the current draft value and returns to the menu; `create` writes the draft with everything entered.
- **Cancel.** `Ctrl+C` at the menu or in any step writes nothing and exits non-zero with no stderr line. `Esc` never cancels: in an active filter it ends filter entry.
- **Other flags.** `--prefix`, `--set`, `--unset`, `--order`, `--body`, `--body-file`, `--blocking` and `--blocked-by` keep their plain-create meaning. The form colours follow the configured `display.theme` flavour.

```
lif create -i "Fix login bug" --type bug
```

## `lif start`

`lif start <id> [id...]` marks one or more existing items as in-progress. A single item is displayed in full, the same as `lif show` would; several items each get one confirmation line instead, since printing many full bodies on one screen helps nobody. Every ID in the call is resolved before the first item is written, so an unknown ID leaves the whole batch untouched. Flags: `--json` returns the result as JSON.

```
lif --lif-path ./demo/.lif start lif-vvat
```

## `lif complete`

`lif complete <id> [id...]` marks one or more existing items as completed. `--summary`, `--commit`, and `--set` apply to every item named in the call. Flags: `--summary string` appends an optional summary of changes to each item's body; `--commit string` records a git ref (`HEAD`, a branch, a tag, or a SHA) in the configured commit field of each item; `--set stringArray` sets an extra front matter `key=value` on every item in the call and can be repeated; `--json` returns the result as JSON. Every ID is resolved and checked against the status policy before the first item is written, so an unknown ID or a policy violation leaves the whole batch alone.

```
lif --lif-path ./demo/.lif complete lif-vvat --summary "Fixed login" --commit HEAD
```

## `lif scrap`

`lif scrap <id> [id...]` marks one or more existing items as scrapped instead of completed, for work that will not be finished. `--reason` applies to every item named in the call and is required. Flags: `--reason string` records why the item was scrapped (required); `--json` returns the result as JSON. Every ID is resolved before the first item is written, so an unknown ID leaves the whole batch alone.

```
lif --lif-path ./demo/.lif scrap lif-gng9 --reason "Out of scope"
```

## `lif archive`

`lif archive` moves every item whose status is configured for archiving (`completed` and `scrapped` by default) into `.lif/archive/`. Archived items keep their relationships intact and remain visible in `lif list` and other queries; the command relocates files to keep the active directory tidy without deleting project history. It takes no ID arguments and processes every qualifying item. The reported count includes items that already sit under `archive/`, so running it twice reports the same number twice; when nothing qualifies at all it prints `No items with archive status to archive.` `--json` returns the result as JSON.

```
lif --lif-path ./demo/.lif archive
```

## `lif delete` (alias `rm`)

`lif delete <id> [id...]` permanently removes one or more items from the store, unlike `archive`, `complete`, and `scrap`, which all preserve the item file. By default it asks for confirmation; if other items reference the target(s) as a parent or via blocking, it also warns about those references and removes them after confirmation. Flags: `-f`/`--force` skips both the confirmation prompt and the reference warnings; `--json` outputs the result as JSON and implies `--force`, so a scripted `--json` call never blocks on a prompt.

Because `--json` implies `--force`, a `lif delete --json <id>` call in a script will silently drop any references to that item from other items' front matter — confirm the ID is the one you mean before scripting this call.

```
lif --lif-path ./demo/.lif delete lif-u76f --json
```

## Related documentation

- [Project Setup and Introspection](project-setup.md)
- [Inspection and Search](inspection-and-search.md)
- [Organization and Relations](organization-and-relations.md)
- [Planning and Reporting](planning-and-reporting.md)
- [Validation and Maintenance](validation-and-maintenance.md)
- [Data Model](../data-model.md)
- [Project Profiles](../project-profiles.md)
