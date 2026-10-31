### Root picker: `-i`/`--interactive`

`lif roadmap -i` lists the roadmap roots and renders the roadmap of the one you choose. The list is drawn on the controlling terminal (`/dev/tty`), so stdout carries only the roadmap.

- **Roots.** The list holds the milestones the plain roadmap shows, as `id  status  title`. `--status`, `--no-status` and `--include-done` decide which milestones appear.
- **Rendering.** The chosen root is rendered as `lif roadmap <id>` renders it. `--depth`, `--tags`, `--view`, `--format`, `--no-links`, `--link-prefix`, `--max-width` and `--include-done` apply as usual.
- **No roots.** The command prints `No roadmap roots found.` and exits 0.
- **Terminal requirement.** Without a terminal on stdin and stdout the command exits non-zero at once and never waits for input. `-i` cannot be combined with `--json` or with a root ID argument.
- **Cancel.** `Ctrl+C` exits non-zero with no stderr line.

```
lif roadmap -i --depth 2 --tags
```
