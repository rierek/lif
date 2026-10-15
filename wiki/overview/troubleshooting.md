---
title: "Troubleshooting"
description: "Symptom-first fixes for the problems people hit running Lif."
---

This page collects symptom-first fixes for the problems people actually hit running Lif. Each entry gives a diagnostic command first, then the corrective one, so you can confirm the cause before you act on it.

## Wrong or missing store

Symptom: a command reports `no .lif directory found at <path> (run 'lif init' to create one)`, or `lif show`/`lif list` returns items you did not expect.

If the symptom is a wrong store rather than a missing one, run `lif path`. It prints the store directory the current invocation resolved to, following the same precedence every other command uses (`--lif-path`, then a found `.lif.yml`, then `LIF_PATH` when no config was found, then the default `.lif` directory), so you see exactly what a real command would have used without touching any data. An explicit relative `--lif-path` is echoed unchanged. When the resolved directory does not exist, `lif path` fails like every other command with `no .lif directory found at <path>`; that error already names the directory that was looked up.

If the printed path is wrong, pass the right one explicitly for a single call: `lif list --lif-path /path/to/.lif`. If the wrong path keeps coming back, check for a stray `.lif.yml` above your working directory (the search walks upward) or an exported `LIF_PATH` taking effect in a repository that has no config file of its own.

If the path is correct but empty or missing while a `.lif.bak-<timestamp>` directory sits next to it, an interrupted rename left the store mid-swap. `lif check` and every other command detect this automatically: with exactly one backup they repair `.lif` from it and log the recovery; with more than one backup they leave `.lif` as an empty placeholder and print each backup's path plus the exact recovery command, e.g. `rm -rf .lif && mv .lif.bak-1730000000000000000 .lif` (the suffix is a nanosecond timestamp). Inspect each backup's contents with `ls .lif.bak-*` before choosing one — `.lif` is already empty at this point, so nothing is lost by picking; the `rm -rf` in that message only clears the empty placeholder, not real data.

## Config discovery picked the wrong file

Symptom: settings you edited (theme, types, statuses, `require_fields_on`) do not seem to apply.

Diagnose with `lif check`, which prints the resolved configuration checks under a `Configuration` heading, including `Default type '<type>' is valid` and any theme or type mismatch. Config discovery searches upward from the current directory for the nearest `.lif.yml`, so a command run from a subdirectory of a monorepo can pick up a different `.lif.yml` than the one you are editing.

Fix by passing the file explicitly: `lif check --config /path/to/.lif.yml`. To confirm which file that command would have found on its own, run `lif path` from the same working directory — a store path under a directory other than the one holding your edited `.lif.yml` means discovery stopped at a different file first.

## Unexpected LIF_PATH override

Symptom: `lif` operates on a store you did not intend, even though the current directory has its own `.lif.yml`.

`LIF_PATH` is commonly exported by direnv and inherited into unrelated repositories. The resolver only lets it win when no `.lif.yml` was found for the current invocation; a `.lif.yml` present at or above the working directory always outranks it, because that file is the repository's own declaration of where its store lives. Diagnose with `lif path` and `env | grep LIF_PATH` (or `printenv LIF_PATH`) side by side: if `lif path` matches your repository's `.lif` directory even with the env var set, the config file is already winning and there is nothing to fix.

If `lif path` shows the env var's target instead, the repository has no `.lif.yml` of its own yet — run `lif init` in it, which writes a `.lif.yml` and immediately demotes `LIF_PATH` for every later invocation in that directory. For a single one-off call without creating a config file, override explicitly: `lif list --lif-path ./.lif`.

## No ready item

Symptom: `lif next` prints "No ready items found" (or, filtered, "No ready items found matching …"), or `lif list --ready` returns nothing.

`--ready` and `lif next` both apply the same filter: not blocked, excluding `in-progress`, `completed`, `scrapped`, and `draft` status (plus any item whose status is only implicitly terminal because a scrapped/completed ancestor covers it), and excluding every container type (rank 1 to 3: `milestone`, `epic` and `feature` by default). A milestone or epic therefore never shows up as ready, even when nothing blocks it; the ready items are the leaves below it. Diagnose with the two commands the empty message itself names: `lif list --is-blocked` shows what is blocked and by what, and a bare `lif list` shows every item's actual status. If you passed filters (`--type`, `--tag`, `--parent`), rerun the equivalent `lif list` with the same flags — the empty-result message echoes them back for exactly this purpose.

Once you have found the item you expected to see, either clear its blocker (see the next section) or move it out of an excluded status with `lif update <id> --status todo`.

## Blocked or parent relationships are wrong

Symptom: `lif list --is-blocked` or `lif check` reports a circular dependency, or an item you expect to be blocked is not.

Diagnose with `lif check`, which reports every broken link, self-reference, and cycle in `parent` links and in blocking links (`blocking` and `blocked_by` count as one set, in any mix) under its `Item Links` section, printing cycles as an arrow-joined path, e.g. `Circular dependency: lif-a1b2 → lif-c3d4 → lif-a1b2 (via blocking)`. The path starts at its smallest ID, reads in the direction of the links and closes on its first ID. A blocker that sits below the item it blocks, or above it, shows as `(via blocking+parent: a child is blocked while its parent is blocked and counts as done once its parent is done)`: the blocked item can never become ready. `lif show <id>` on any item in the path lists its own `parent`, `blocking`, and `blocked_by` front matter directly.

Fix a broken or self-referencing link with `lif update <id> --remove-blocked-by <target>` (or `--remove-blocking`, `--remove-parent`), naming the relationship the check reported. Do not remove a link whose target is the ID of a file that was renamed by hand: `lif check` reports that file with both IDs (`file name … gives ID … but the file records ID …`), `lif check --fix` keeps such links and prints them as `kept broken link`, and renaming the file back, or `lif rename <id> <recorded-id>`, makes them work again. Cycles cannot be auto-fixed: `lif check --fix` prints `Cannot auto-fix cycle: <path> (via <link type>)` and exits 1; when the same run also fixed other issues, its closing line reads `Fixed N issue(s), M require manual intervention`. Break one edge with the matching `--remove-*` flag and rerun `lif check`. Note that `lif update --blocking`/`--blocked-by` already refuses to add a link that would create a cycle, so a cycle appears only after a hand edit of item files.

## Completion policy failures

Symptom: `lif complete` or `lif update --status …` fails with `status "completed" requires front matter field(s) commit: supply them in the same write (e.g. `lif complete <id> --commit HEAD`)`. The policy gates only the statuses named under `lif.require_fields_on`; in the usual configuration that is `completed`, so `lif start` and `lif scrap` are not affected unless the configuration also names `in-progress` or `scrapped`.

This is `lif.require_fields_on` in `.lif.yml`, a fork-specific policy that names front matter fields an item must carry before it can enter a given status. The check runs before any item in a batch is written, so a violation on one ID leaves every item in that call untouched — nothing to roll back. Diagnose the full policy with `lif check`, which lists every item currently missing a required field under its `Policy` section as a warning (pass `--strict` to also make `check` exit non-zero on these warnings).

Fix by supplying the missing field in the same write that changes status: `lif complete <id> --commit HEAD` (or `--set <field>=<value>` for a non-commit field). If the configured commit field references a SHA, `lif check` separately verifies each referenced commit still exists in the current git repository and warns `<id>: commit <sha> not found in this repository` when it does not — outside a git repository it warns `commit verification skipped (not a git repository)` instead of failing.

## Invalid config or front matter

Symptom: any command exits with `loading config: <path>: unknown lif.anchor "…"` or a similarly worded `lif.require_fields_on[…]` error, or `loading items: loading <path>: parsing front matter: …`.

Config errors are caught eagerly at load, before any command runs: an unrecognized `lif.anchor` value, an empty field name or reserved schema field (`title`, `status`, `type`, …) inside `lif.require_fields_on`, or an unknown status name there all abort with a message naming the exact key and the config file's path. Diagnose by opening the named file at the reported path; the error text names the offending key directly, so no separate lookup is needed.

An item file's YAML front matter is validated the same way: one malformed `.md` file (e.g. an unquoted colon inside a title) aborts loading for every command, not just the one touching that item, with the file's path in the error. A number JSON cannot carry (`.nan`, `.inf`) in an extra key is refused too, with the error `key "<key>" holds NaN: only finite numbers are supported, quote the value to keep it as text`. Diagnose by opening the exact path named after `loading`; fix the YAML by hand (quote the offending value) and rerun the command — there is no `--fix` for a parse error, since the file cannot be safely interpreted until it parses.

## Server binding or startup failures

Symptom: `lif-serve serve` (or `lif-serve serve --port <n>`) exits with `server error: listen tcp 127.0.0.1:<port>: bind: address already in use`, a browser request to the UI fails with a CORS error in its console, a request gets 403 `host "…" is not allowed` or `origin "…" is not allowed`, or another machine cannot reach the server.

Diagnose a port conflict with `lsof -i :<port>` (the default port is 8080, from `server.port` in `.lif.yml` or the `--port`/`-p` flag) to see what already holds it. Fix by picking a free port explicitly: `lif-serve serve --port 8081`, or by stopping the process holding the current one.

A CORS rejection shows up only in the browser (a blocked-by-CORS error in devtools), not in the server's own log, because the check happens per request against the allowed-origins list. Diagnose by checking the origin the server actually allows: `lif-serve serve` prints `[lif] Allowed origins: …` on startup, which defaults to `http://localhost:*` and `http://127.0.0.1:*` (from `server.cors_origins` in `.lif.yml`, or the `--cors-origin` flag, repeatable). Fix by naming the origin you are calling from. A given list replaces the default, so repeat the defaults you still need: `lif-serve serve --cors-origin 'http://localhost:*,http://192.168.1.50:*'`, or set `server.cors_origins` in `.lif.yml` for a persistent list.

A 403 with `host "…" is not allowed` or `origin "…" is not allowed` comes from the server's own host and origin check, and the 403 is in its request log. The usual cause is a `--cors-origin` list that replaced the default and no longer names `localhost`, or a UI opened under an address the list does not name. Name that host with `--cors-origin 'http://<host>:*'` (add `http://localhost:*` when you still use it). A server beyond loopback also answers 401 `access token required` until the browser has opened the address with `?token=<token>` that the startup log prints.

Since 0.15 `lif-serve` listens on `127.0.0.1` only: other machines, and a Docker port mapping to a container, cannot reach it. Check the first startup line, `[lif] Listening on …`. Pass `--host 0.0.0.0` only after you read [Web server](../../reference/web-server/).

## Profile choice at init

Symptom: `lif init --profile <name>` fails with `unknown profile "<name>" (must be classic, complex, simple, todo)`, or `--profile` together with `--lif-path` fails with `--profile writes a .lif.yml, which --lif-path skips; use one or the other`.

Both errors are validated before any file is written, so a typo never leaves a half-initialized project behind. Diagnose by rereading `lif init --help`, which lists the exact profile names accepted at that moment. Fix an initial setup by rerunning `lif init` with a valid `--profile` name in the intended project directory, without `--lif-path`.

Do not rerun `lif init` over an established project without first preserving `.lif.yml`: the command rewrites that file with generated defaults. To change an existing project's profile or type model, edit and review the `types`, `types_exclusive`, and `lif.default_type` settings in `.lif.yml` deliberately; see [Configuration](../../reference/configuration/#presets).

## A store from an earlier release behaves unexpectedly

A store written by an earlier release, or by the project Lif originated from, is read in place: its item files, config file, section key and store directory keep working under their legacy names. Start with `lif check`, which surfaces the same configuration and link issues as for any store, and lists every legacy name in use under its `Legacy names` heading. A `check` failure that names a key an older store never had (`lif.anchor`, `lif.require_fields_on`) means the store simply predates those fields — they are additive and safe to leave unset.

If a command fails with an error that names two files, two directories, two sections or two variables and ends in `keep one of them` or `unset one of them`, the project holds both a legacy name and its current name at the same level; see [Configuration](../../reference/configuration/).

See [About](../../about/#origin) for where the project comes from and [Upgrading](../upgrading/) for each breaking change with its migration step.

## Related pages

- [Configuration](../../reference/configuration/)
- [Command reference](../../reference/commands/)
- [Upgrading](../upgrading/)
- [About](../../about/)
- [First steps](../first-steps/)
