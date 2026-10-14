---
title: "About"
description: "What Lif is, the philosophy behind it and where it comes from."
---

## What Lif is

Lif is a file-based issue tracker for developers, product teams and AI coding agents. It is lightweight, Markdown-native work management for humans, agents and automation.

Each piece of work is an *item*: one Markdown file with YAML front matter. Items live beside the code and are versioned with Git.

Three entry points reach the same data:

- the Markdown files themselves,
- the CLI, including `lif prime` and the built-in help,
- JSON output and GraphQL.

Lif is not a hosted project-management platform. It stays a small, inspectable data and workflow layer.

For details, see [Lif in brief](../overview/) and [Features](../features/).

## Philosophy

**Adapt to existing ways of working.** Lif does not prescribe one workflow. Types, statuses, priorities, display rules and lifecycle policies are configurable per repository. Terminology is configurable too.

**Local, readable, diff-friendly.** You need no service and no proprietary export to read active or archived work. You review task changes with the Git tools you already use for code. Plain Markdown also suits LLMs.

**Presets are examples, not a workflow engine.** A preset gives you a starting point. Presets use up to four ranks of types. Custom types can go deeper.

**Files are the source of truth.** The CLI, the terminal UI and the web interface all work on the same files.

## Origin

Lif started as a fork of [`hmans/beans`](https://github.com/hmans/beans), the agentic-first issue tracker by Hendrik Mans. It is now maintained independently at [`xRiErOS/lif`](https://github.com/xRiErOS/lif). It is an actively maintained continuation, not a hostile fork. It exists so that development can continue on its own schedule and roadmap.

**What comes from upstream:**

- the original idea,
- the data model: one Markdown file per item with YAML front matter,
- the initial command surface,
- the initial GraphQL query engine.

**What Lif added:**

- project profiles,
- scoped roadmaps,
- order and rename workflows,
- completion policies,
- structured JSON and error output,
- worktree-aware storage,
- the integrated terminal UI, `lif tui`. It was formerly the companion project `beans-tui`.

**License.** Upstream and Lif both use Apache-2.0. The `LICENSE` file is unchanged. Work carried over stays attributed to `hmans/beans` and Hendrik Mans.

**Names.** The project was first published as beans, like the upstream. The domain object is now an *item*, and the project is called Lif. The type names `milestone`, `epic`, `feature`, `bug` and `task` are unchanged.

**Where to report and whom to cite.** File issues against `xRiErOS/lif`. Cite `hmans/beans` and Hendrik Mans for the original concept.

## Legacy compatibility

Projects from the Beans era keep working. Lif reads these names in place:

- the config file `.beans.yml`, which Lif never renames,
- the store directory `.beans/`,
- the environment variable `BEANS_PATH`.

An existing ID prefix such as `beans-` is plain data. It is not a legacy name.

Some combinations are errors:

- `.lif.yml` and `.beans.yml` in the same directory,
- `LIF_PATH` and `BEANS_PATH` set at the same time.

`lif check` reports legacy names as a note (`Legacy names`). They never count as errors.

For the migration steps, see the [Releases](../releases/) pages.

## Where to go next

- [Releases](../releases/)
- [Contribute](../contribute/)
- [Lif in brief](../overview/)
