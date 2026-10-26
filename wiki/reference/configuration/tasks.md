---
title: "Tasks and TUI actions"
description: "Named shell tasks in .lif.yml, run with lif run, and the TUI actions that start them."
---

## Tasks

`lif run` runs named shell tasks. Define them under `tasks:` in `.lif.yml` or in the global file `~/.lif/tasks.yml`:

```yaml
tasks:
    po-accept:
        desc: Accept an item as PO
        args: [id]
        deps: [test]
        steps:
            - '"$LIF" update "$id" --tag accepted'
```

- Schema. A task is a mapping with the keys `desc` (text that `lif run` lists), `args` (required positional arguments), `deps` (tasks that run first) and `steps` (a list of shell scripts). Any other key is an error. A multi-line step is one script. Each step must be a YAML string: `steps: [1, true]` is an error, so quote such a step.
- Validation. A task name uses lowercase letters, digits, `-` and `_`, and starts with a letter or digit. `steps` needs at least one entry, and no step is empty. An argument name uses lowercase letters, digits and `_`, and does not start with a digit. An argument name appears once per task. A dependency name is not empty.
- Global file. `~/.lif/tasks.yml` may contain only the keys `tasks:` and `hooks:`. Any other top-level key is an error. A missing file means no global tasks. The global file is a standard that all your projects share.
- Override. A project task replaces a global task of the same name as a whole. `lif run -g <task>` runs the global one instead. `lif run` lists both files; a project task that hides a global one shows `[project, overrides global]`.
- Dependencies. `deps` resolve against both files, so a global task can depend on a project task. With `lif run -g`, the dependencies resolve in the global file only: a repository cannot run its own task as a dependency of a global task that you asked for with `-g`. A global task that names a project-only dependency fails under `-g`. A dependency runs once, even when several tasks name it. A dependency takes no arguments: a dependency with `args` is an error. A dependency cycle is an error. A global task may depend on a project task of the same name (`deps: [build]` in the global `build` runs the project `build` first): a cycle is a loop of one origin and name.
- Execution. Each step runs with `sh -ec` in the project root: the directory of `.lif.yml`, or the current directory when there is none. This needs a POSIX `sh`: Lif takes `sh` from `PATH` and falls back to `/bin/sh`. Because of `-e`, a failing command in a multi-line step stops the step, not only a failing last command. End a command with `|| true` when its failure is deliberate and the step goes on. Hook and gate tasks run through the same runner with the same `-e`. A step that starts a background process with `&` can keep `lif run` waiting when the output of `lif run` goes into a pipe (`lif run bg | cat`) until that process ends; redirect the output of the background process, for example `cmd >/dev/null 2>&1 &`.
- Environment. Every step gets `LIF` (path of the lif binary), `LIF_ROOT` (the directory the steps run in), `LIF_TASK` (name of the running task; a dependency has its own name) and `LIF_TASK_ORIGIN` (`project` or `global`). When `--lif-path` or `LIF_PATH` names the store, Lif also sets `LIF_PATH` to its absolute path, so a step that calls `"$LIF"` uses the same store. Each declared argument is also a shell variable, and all arguments are `$1`, `$2`, ... of every step. Everything after the task name goes to the task unparsed.
- Quoting. Quote arguments in the step: `"$id"`. Arguments are shell variables, not text that Lif substitutes into the script.
- Calling tasks. A global task that calls another global task uses `"$LIF" run -g <task>`. Without `-g`, a project task of the same name wins; Lif prints `running project task "<name>" (overrides global; use -g for the global one)` on stderr when it does. `lif run` runs project tasks without a trust check: a `.lif.yml` that you did not write can run commands on your machine. Text from `.lif.yml` that Lif prints (a task `desc`, a gate `message`) is shown without control characters, so an escape sequence in a cloned repository does not reach your terminal.
- Exit code. A failing step stops the run. Lif prints `Error: task "<name>": step <n> of <total> failed (exit code <c>)` and exits with the exit code of that step. A step that a signal killed (`step <n> of <total>: signal: terminated`) gives 128 plus the signal number: 143 for SIGTERM. Every error of Lif itself exits with 2: an unknown task, a missing argument, an unknown flag of `lif run`, a cycle or an invalid config. A step that exits with 2 and a refusal by Lif look the same in the exit code; the message on stderr tells them apart. When Lif gets SIGINT, SIGTERM or SIGHUP, passes it on and the step exits with 0, Lif exits with 1. An empty value satisfies a declared argument: check `[ -n "$id" ]` in the step when that matters.

  | Exit code | Meaning |
  | --- | --- |
  | 0 | Every step succeeded. |
  | n | The step failed with its own exit code n; it passes through unchanged. |
  | 128+n | A signal n killed the step. |
  | 2 | An error of Lif: unknown task, missing argument, unknown flag, dependency cycle, bad task config. |
  | 1 | Lif got SIGINT, SIGTERM or SIGHUP and the step exited with 0. |
- Listing. `lif run` prints text only, no `--json`. `lif run --json` is an unknown flag, and `lif run <task> --json` passes `--json` to the task. For a machine-readable list use `lif __complete run ""` or read `tasks:` from the files.
- Errors. Errors of `lif run` that are not step failures (unknown task, cycle, invalid config) end with `Run 'lif run --help' for usage.` An unknown task name is case-sensitive and gets no suggestion. `--config <file>` with a file that does not exist is an error (`config file not found`).
- Global tasks and a broken project. `lif run -g` still reads the project `.lif.yml`, so an invalid project config fails it too. Outside a project, or in a worktree without `.lif.yml`, `lif run` lists the global tasks only and gives no hint. Completion (`lif __complete run`) with an invalid project config prints the config error on stderr and offers no task names.
- Signals. While a step runs, SIGINT, SIGTERM and SIGHUP go to the step instead of ending `lif`. `lif` waits for the step, so a `trap` in the step can clean up, and returns the step's exit code. When stdin is not a terminal, the step runs in its own process group and the signal goes to the whole group: no child is orphaned. With a terminal on stdin, the step stays in the foreground group: the terminal sends Ctrl-C to it directly, and `lif` forwards only SIGTERM and SIGHUP to the step itself, not to its children. A step killed by a signal without a `trap` has no exit code of its own: the error shows the signal and `lif` exits with 128 plus the signal number.
- `lif rename` keeps the `tasks:` block.
- An invalid task makes every lif command in that project fail with the config error, not only `lif run`.

See [Automation](../../../features/automation/#tasks) for examples.

## TUI actions

`lif tui` shows no task by default. `tui.actions` in `.lif.yml` exposes tasks as entries of the command palette (`run: <label>`) and optionally as a key:

```yaml
tui:
    actions:
        - task: po-accept
          key: A
          label: accept
          statuses: [to-review]
```

- `task`. The name of a task in `tasks:` or in `~/.lif/tasks.yml`. An unknown name is dropped and the TUI shows a warning.
- `key`. Optional single key. A key that has more than one character or that the TUI already uses is dropped, and the TUI shows a warning.
- `label`. Text of the palette entry and the key help. Default: the task `desc`, else the task name.
- `statuses`. Optional list. The action is offered only while the focused item has one of these statuses.
- Arguments. A task with one argument gets the focused item ID. A task without an argument gets none. A task with more than one argument is an error. Every step also gets `LIF_ITEM_ID` (the focused item ID, or empty) and `LIF_PATH`.
- Execution. The task runs in the TUI process without a trust check, like `lif run`, with empty stdin. The TUI shows the output and the exit code in an overlay (`esc` closes it); a task without output and without error shows a short toast.
- `lif rename` keeps the `tui:` block.
