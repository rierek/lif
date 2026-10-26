---
title: "Hooks and gates"
description: "Hooks run a task when an item is created or changes; gates run before a write and can refuse it."
---

A hook runs a [task](../tasks/) by itself when an item is created or when one of its fields changes. Tasks are the *what*, hooks are the *when*: a hook adds no commands of its own, so `lif run` and a hook run the same steps the same way. A gate is a hook that runs before the write and can refuse it. `lif hooks` lists every hook and gate, and whether the project hooks run.

Define hooks under `hooks:` in `.lif.yml` or in the global file `~/.lif/tasks.yml`. The name follows the rule of task names. This hook opens a worktree when an item enters `in-progress`, whether it is created that way or changed later:

```yaml
tasks:
  wt-open:
    args: [id]
    steps: ['just -g wt start "$id" milestone/0.16']
hooks:
  open-worktree:
    event: [created, updated]
    changed: [status]
    to: [in-progress]
    type: [feature, task, bug]
    run: wt-open
    args: [id]
```

## Field reference

This table lists every key of a hook. It is a reference, not a template: some keys exclude each other, and a config that sets all of them is invalid.

| Key | Required | Meaning |
|---|---|---|
| `event` | yes | `created` and/or `updated`. The key is `event`, not `on`: a YAML 1.1 tool reads an unquoted `on` as `true`. |
| `run` | yes | Name of the task to run. It is resolved when the hook fires, so a typo shows in `lif hooks`, not at load time. |
| `stage` | no | `after` (default) or `before`. `before` makes the hook a gate. Only valid with `event: [updated]`: a new item has no ID before its write. |
| `changed` | no | At least one of these fields changed. Change filter. |
| `tags_added` | no | At least one of these tags was added. Change filter. |
| `tags_removed` | no | At least one of these tags was removed. Change filter. |
| `from` | no | Status before the change. State filter. Only valid with `event: [updated]`. |
| `to` | no | Status after the change. State filter. |
| `type` | no | Type after the change. State filter. |
| `tags` | no | The item carries at least one of these tags after the change. State filter. |
| `args` | no | Item fields that are passed to the task as arguments, in this order. |
| `message` | no | Fixed text that a blocked change shows. Only valid with `stage: before`. |
| `timeout` | no | Seconds that the task may run. Default 60; `0` means no limit. See [Running hooks](#running-hooks). |

Any other key is an error. In a project config, `from`, `to` and `type` must name known statuses and types. The global file skips that check, because statuses and types differ per project. Tags must be valid tags.

## What matches

- All conditions that a hook sets must hold (AND). The values of one list are alternatives (OR). An OR across different conditions needs two hooks.
- There is no expression language. A condition that needs more than one item, a file or a comparison of values belongs into the task. The script checks and exits 0 when there is nothing to do (after hook) or when the change is fine (gate).
- **Events.** `created` fires once after `lif create`, `lif promote` or the GraphQL mutation `createItem`. `updated` fires after every other change that goes through the write path of the commands and of GraphQL, and only when at least one watched field really changes.
- **Watched fields** are `status`, `type`, `parent`, `tags` and every extra front matter key whose name matches `^[A-Za-z0-9_-]+$`. Lif does not need to know the extra keys in advance. Other extra keys, for example one with a space in its name, are not watched: they are not in `changed`, `args` or the environment. Tags compare as a set, so a reorder is no change. Extra values compare deeply; an added or removed key is a change. A change of title, body, priority, order or blocking links fires nothing.
- **Change filters and state filters.** `changed`, `tags_added` and `tags_removed` filter *what changed*. `from`, `to`, `type` and `tags` filter the *state* before or after the change. For a hook and an after hook, `from` is the state before and `to`, `type` and `tags` are the state after. A gate also matches `type` and `tags` of the state before, so a change that moves an item out of the type or the tag does not take it out of the gate. A status transition therefore needs `changed: [status]`. A hook with `event: [updated]` and only `to: [completed]` also fires when a completed item later gets a new tag.
- **The before state of `created`.** It is an empty item: `changed` lists every watched field that has a value, all `prev_*` fields are empty, and every tag counts as added. So `event: [created, updated]` with `changed: [status]` and `to: [in-progress]` covers both ways into a status.

## Item fields and environment

`args` names the item fields that the task gets as its arguments. The built-in fields are `id`, `title`, `status`, `type`, `parent`, `tags`, `prev_status`, `prev_type`, `prev_parent`, `prev_tags` and `changed`. Every watched extra key `<key>` also gives `<key>` and `prev_<key>`. A built-in name wins over an extra key of the same name.

- Format. `tags`, `prev_tags` and `changed` are space-separated; a tag cannot contain a space. An extra value is a string as it is, a number or bool as `fmt.Sprint` prints it, a list of scalars space-separated, anything else as JSON. A missing value is the empty string.
- An extra key with an empty YAML value (`target_entry:`) reaches the task as the text `null`, not as an empty string. A test `[ -n "$target_entry" ]` is then true. A key that is missing gives the empty string.
- Environment. On top of the variables of a task (`LIF`, `LIF_ROOT`, `LIF_TASK`, `LIF_TASK_ORIGIN`) the task gets `LIF_NO_HOOKS=1`, `LIF_HOOK` (name), `LIF_STAGE` (`after` or `before`), `LIF_EVENT`, `LIF_CHANGED`, `LIF_ITEM_ID`, `LIF_ITEM_PATH`, `LIF_STATUS`, `LIF_TYPE`, `LIF_PARENT`, `LIF_TAGS` and the `LIF_PREV_STATUS`, `LIF_PREV_TYPE`, `LIF_PREV_PARENT`, `LIF_PREV_TAGS` counterparts. Every watched extra key gives `LIF_FIELD_<NAME>` and `LIF_PREV_FIELD_<NAME>`. `<NAME>` is the key in upper case with every character outside `A-Z` and `0-9` replaced by `_`. When two keys give the same name, the first key in sorted order wins and Lif prints a warning for the other.

## Stages and gates

- **After hook** (`stage: after`, the default). It runs after the change is written and cannot veto it.
- **Gate** (`stage: before`). It runs before the write. A non-zero exit blocks the change and nothing is written. With several targets (`lif complete a b`) the targets before the blocked one stay written, and the error ends with the line `(already written: a)`. A stale `--if-match` fails with `etag mismatch` before the gate runs. A gate task that writes its own item makes the outer write fail with `etag mismatch`; do not write the gated item from a gate. A gate compares an extra key as parsed YAML: `--set nn=5` stores the string `"5"`, so on a hand-written `nn: 5` (an integer) it counts as a change of `nn`.

`message:` is fixed text from the YAML file. Lif inserts nothing into it. The dynamic part comes from the output of the gate task (stdout and stderr), which Lif prints below the message. Without `message:`, only the task output follows the first line. A blocked change of an item command such as `lif complete` looks like this, with exit code 1 and no usage line:

```text
Error: hook "require-target-entry" blocked the change of lif-ab12: Entry missing in the target file.
lif-ab12 not found in docs/RELEASE.md
```

With `--json` the error is on stdout, with the code `POLICY_VIOLATION`:

```json
{
  "success": false,
  "error": "hook \"require-target-entry\" blocked the change of lif-ab12: Entry missing in the target file.\nlif-ab12 not found in docs/RELEASE.md",
  "code": "POLICY_VIOLATION"
}
```

- **Gates fail closed.** A gate that cannot run (unknown task, missing argument, killed by a signal, no `lif` binary for a task that uses `$LIF`) blocks the change with `could not run: <error>`. A global file or trust file that cannot be read blocks every change of a watched field with `hooks could not be loaded: <error> (fix it or set LIF_NO_HOOKS=1)`, also in a project without hooks: a typo in `~/.lif/tasks.yml` therefore stops watched-field changes in every project until you fix it. A trust file that does not parse is repaired by `lif hooks deny` or `lif hooks allow`: they replace it with a notice and drop all allows. A missing file is no error. A typo must not switch a policy off silently.
- **Batches.** With several targets (`lif update`, `start`, `complete`, `scrap`, `tag`), the first blocked target stops the run. Targets before it stay written, and the error ends with `(already written: <ids>)`; under `--json` the written items come with the error. The text error keeps its type: it is still the gate error, with exit code 1.
- **Relation to `require_fields_on`.** [`require_fields_on`](../types-and-statuses/#status-dependent-required-fields) checks that a field has a value. A gate checks anything that a script can check: a file, another item, a value against a list.

## Running hooks

- **Synchronous.** The command waits for its hooks, in `lif-serve` too: the HTTP write timeout is off for `/api/graphql` (see [Web server](../../web-server/)). Hook tasks run one at a time in `lif-serve`. A write that matches no hook or gate does not wait for a running hook. A create or write that matches one waits for the running hook, so a slow hook delays the matching writes of every client, and a hung one blocks them until its `timeout:`. A client that disconnects does not cancel a running hook.
- **A failing after hook** prints `warning: hook "<name>" (task "<task>"): <error>` on stderr. The change stays and the command still exits 0, so a caller does not repeat a write that succeeded. The next hook runs.
- **Output.** The output and the warnings of a hook go to stderr of the process that runs it: the terminal, or the log of `lif-serve`. Every output line of an after hook starts with `[<hook name>] ` and has its control characters removed, so a title with a newline cannot forge a log line and an escape sequence cannot clear the terminal. A client of `lif-serve` does not see a failing after hook: the mutation succeeds and the warning is only in the log. A client that drops the connection does not stop a running hook: it runs to its `timeout:` and holds the hook lock meanwhile. The TUI drops the stderr of a successful action; a gate block still reaches it, because the TUI reads the JSON envelope (`POLICY_VIOLATION`).
- **Timeout.** A hook or gate task runs at most `timeout:` seconds, default 60, `0` for no limit. At the limit Lif kills the task and its process group. On Windows only the `sh` process is killed, not its children. A gate that times out blocks the change with `could not run: ... timed out after <n>s` (fail closed); an after hook prints a warning and the next hook runs.
- **Output limit.** Lif keeps the first 64 KiB of the output of a hook or gate task and drops the rest; the output ends with `(output cut after 64 KiB)`. The task is not stopped by it. The error of a blocked gate carries only the first 8 KiB of the output (`(output cut after 8 KiB)`) without control characters, because it reaches every client of `lif-serve`.
- **Environment.** A hook or gate task gets the whole environment of the process that runs it, plus the `LIF_*` variables. For `lif-serve` that is the environment of the shell that started it, tokens in it included. A gate that prints its environment shows it to every client that triggers the gate.
- **No chaining.** A `lif` command inside a hook or gate task fires no hooks and no gates: the task gets `LIF_NO_HOOKS=1` and `LIF_HOOK_DEPTH`. As a second brake, a `lif` call with `LIF_HOOK_DEPTH` of 3 or more fires nothing and warns, so a hook that clears `LIF_NO_HOOKS` (`env -u`, an empty value) stops after 3 levels. A call that clears the whole environment (`env -i`, `sudo`, `ssh`, `docker exec`) loses both variables and is not covered: do not call `lif` that way from a hook.
- **No HTTP callbacks to `lif-serve`.** `LIF_NO_HOOKS` belongs to the process, so a task that calls the `lif-serve` API (for example with `curl`) does not inherit it: the server fires hooks for that write. When it matches a hook, it waits for the running hook, which waits for the call: a deadlock that ends at the `timeout:` of the hook. Give such a call `--max-time` and use `"$LIF"` instead.
- **Global hooks run only global tasks.** A global task that calls another global task uses `"$LIF" run -g <task>`: without `-g` a project task of the same name wins, and `lif run` does not check trust. In `lif-serve`, `$LIF` is the `lif` next to `lif-serve`, else the one on `PATH`.
- **Reload.** `lif-serve` reads `.lif.yml` at start: restart it after you edit hooks or tasks. Lif checks the file on disk before every mutation: when its hooks and tasks no longer match what `lif-serve` read at start (for example after `git pull`), the project after hooks do not run, and every project gate that `lif-serve` knows blocks the changes it watches with `<path> changed since this process started; restart it to apply the new config`, until you restart `lif-serve`. `lif hooks allow` does not lift this block: the gates stay on, never off. A gate that you removed from the file stays on too until the restart. A step that runs `"$LIF" run <task>` reads the file again, so the allow of the old content must not carry over. It reads the global file and the trust file on every mutation.
- **Order.** Hooks run in name order, global and project hooks mixed. A hook that writes an item changes what the next hook sees.
- **A stale item after a hook.** After an after hook ran for an item, Lif reads that item's file again: the item that the command prints (the `updateItem` result in `lif-serve`) and its ETag are what the task left on disk. Other items that the task wrote stay stale in memory until the file watcher of `lif-serve` reads them (the CLI watches no files): a following `--if-match` on them gets `CONFLICT`; read the item again with `lif show --json`. A later target of the same batch that the hook wrote keeps the hook's write: the batch then applies its change on top of it, and both writes stay. Example: a hook adds tag `from-hook` to B; `lif update A B --tag x` exits 0 and leaves B with the tags `from-hook` and `x`. `lif-serve` has one resolver for all requests, and the effect lasts for its whole life: once a hook has run, every later write without `If-Match` expects the ETag that it read, and fails with `CONFLICT` (`etag mismatch: provided <hex>, current is <hex>`, a value the client never sent) when the item changed in between, for example by a parallel write, instead of the last write winning. Restart `lif-serve` to reset it.
- **`lif create --order`.** The order goes into the write that creates the item, so a `created` hook that writes its own item causes no `CONFLICT`.
- **Environment size.** Every watched extra key is exported twice, as `LIF_FIELD_<KEY>` and `LIF_PREV_FIELD_<KEY>`, for every hook of the item, whether the hook reads it or not. The operating system limits the size of an environment (about 1 MB on macOS). A hook of an item with a very large extra value (hundreds of kilobytes) fails with `argument list too long`, and a NUL byte in a value fails with `environment variable contains NUL`: the change stays, the hook prints a warning (a gate fails closed). Keep large content in the body, and read it from the file at `LIF_ITEM_PATH`.
- **Names that clash.** Extra keys `x-y`, `x_y` and `X_Y` all map to `LIF_FIELD_X_Y`: the first in sorted order wins, and lif prints one warning per process and key. `args` uses the real key and has no such clash.
- **Lossy formats.** A list is passed space-separated (the element `"b c"` looks like two elements), a float as `fmt.Sprint` prints it (`2.50` becomes `2.5`), a map as compact JSON, and an absent key and an empty string are both the empty string. A task that needs the exact value reads the front matter of the file at `LIF_ITEM_PATH`.
- **Silent no-ops.** An `args` entry that names no existing extra key gives the empty string. `lif hooks` marks a hook that gives more `args` than its task declares, or that lists a capitalised built-in field (`changed: [Status]` matches only an extra key `Status`), with `(warning: ...)`.
- **`lif rename`** keeps the `hooks:` block.

## Paths that fire nothing

These paths change a watched field and fire no hook and no gate:

- `lif delete` (it clears `parent` of the children)
- `lif check --fix`
- `lif rename` (it rewrites `parent`)
- `lif order` (`order` is not a watched field)
- `saveItem` and `saveDirtyItems` of `lif-serve` (they persist worktree overlays into the main store)
- hand edits of item files and git merges
- creation: a gate has no item before the write, so `lif create -s completed`, `lif promote` and the GraphQL `createItem` can create an item directly in a gated status. A gate on `to: [completed]` does not cover an item that is born completed
- a run with another config or store: the project hooks come from the `.lif.yml` of the working directory, or from the file of `--config`. `--lif-path` and `LIF_PATH` only move the store. A run from another directory, or with `--config` pointing elsewhere, loads no project hook and no project gate, and prints no warning
- a project config that is not allowed, or that was edited after the last `lif hooks allow`: its gates do not run, the change exits 0 and Lif prints one warning on stderr (JSON on stdout carries no warning). Run `lif hooks allow` after every edit of the hooks and tasks, and check the exit of scripts and agents on stderr too

Hooks and gates are therefore not a security boundary. They guard the normal write path.

## `LIF_NO_HOOKS`

`LIF_NO_HOOKS` with any non-empty value, also `0`, skips after hooks and gates. A skipped matching gate prints `warning: gate "<name>" skipped (LIF_NO_HOOKS is set)`. It is a deliberate bypass.

Every process that a hook starts inherits it. An agent session, `lif-serve` or an editor that a hook starts therefore skips all gates for its whole life.

## Trust

A project config can come from a repository that you cloned, so project hooks and gates run only after `lif hooks allow`. Global hooks always run.

- `lif hooks allow` records the fingerprint of the hooks and tasks of this `.lif.yml`. `lif hooks deny` takes the record back. Run `lif hooks allow` yourself after you have read the file: it is your decision, not one for a script or an agent. `lif hooks allow` refuses to run when `LIF_HOOK` is set, that is from inside a hook or gate. `lif hooks --steps` prints the steps of every task that a project hook reaches, quoted so that control characters show; `lif hooks allow` prints the same list. A step that is `!!binary` or contains a bidirectional control character (U+202A to U+202E, U+2066 to U+2069) is a config error.
- **Not allowed.** Lif prints `warning: hooks in <path> are not allowed (1 gate not enforced); run 'lif hooks allow' to enable them` for one gate, or `(<n> gates not enforced)` for several, once per process. The count names the gates, because a gate that is off is a policy that is off. A project that is not allowed never hides a global hook. A project hook replaces a global hook of the same name only when project hooks are allowed. A project hook never replaces a global gate: Lif ignores it, prints `warning: hook "<name>" in <path> ignored: a global gate of that name cannot be replaced; rename it` once per process, and the global gate stays in force. `lif hooks` lists the global gate as the enforced row and marks the project hook `[project, ignored: global gate of that name]`.
- **Edits take the allow back.** An edit of the hooks or tasks in `.lif.yml` invalidates the allow, also an edit by an agent, and also a `.lif.yml` in a fresh clone or worktree whose fingerprint differs from the allowed one. Until the next `lif hooks allow`, project gates do not run: the change succeeds with exit code 0 and Lif only prints the warning above on stderr. `--json` output does not carry the warning.
- **One allow covers all worktrees** of the repository. The key is the path of `.lif.yml` in the main checkout, with symlinks of its directory resolved. For a bare repository (`proj.git` with its worktrees beside it) the main checkout is the bare directory, so repositories that share a parent directory keep separate keys. A directory whose `.git` file names a worktree that is not its own (for example an unpacked archive with a copied `.git` file) is not a checkout of that repository: its key is the path of its own `.lif.yml`. The fingerprint must still match in the worktree that you work in.
- **The fingerprint** covers `hooks` and `tasks` as parsed, so comments and formatting do not change it. It does not cover the scripts that task steps call. Any edit of a task, also one that no hook uses, takes the allow back.
- **Changed file in a running process.** `lif-serve` compares the fingerprint of `.lif.yml` on disk with the one it read at start before every mutation. A step that runs `"$LIF" run <task>` reads the file again, so a changed file takes the allow away until you restart `lif-serve` and run `lif hooks allow`.
- **Not covered.** The fingerprint covers `hooks` and `tasks`. `worktree.setup` and `worktree.run` are not covered and have no trust check: `lif-serve` runs them with `sh -c` when you create a worktree or start a run in the web UI. Read them in a cloned repository before you click. A global hook runs its task in the project root, so a global step such as `make check` or `./scripts/x` runs the repository's own Makefile or script; only the lookup of lif tasks is kept global. Write global steps with absolute paths when that matters.
- **Limits.** If `.lif.yml` itself is a symlink, the key is the path of the link, not of its target. `lif hooks allow` and `deny` hold a lock (`allowed-hooks.yml.lock`) while they update the trust file, so parallel calls do not undo each other, and they write through a symlinked trust file. Entries of moved or deleted repositories stay in the file: a different repository that later lands on the same path with identical hooks and tasks inherits the allow, because scripts are not fingerprinted. An edit that does not change the meaning (for example adding `stage: after`) can still change the fingerprint. Lif does not check the owner or the permissions of `~/.lif/tasks.yml` and the trust file: keep them private. `lif hooks allow` has no confirmation and `lif hooks` has no `--json`. Hook and task names have no length limit. A YAML merge key (`<<`) inside a hook or a task is an error; anchors on whole values work. `event:` must be a list. A global hook with a status that the project does not know is flagged in `lif hooks`, not rejected.

See [Tasks and hooks](../../../features/tasks-and-hooks/#hooks) for the worktree example and a gate.
