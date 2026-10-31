### Guided search: `-i`/`--interactive`

`lif list -i` asks for a search query, lists the hits and prints the one you choose. The form is drawn on the controlling terminal (`/dev/tty`), so stdout carries only the result.

- **Query.** The input is pre-filled from `--search`/`-S`. The search is the one `lif list -S` runs, so the other filters (`--status`, `--type`, `--tag`, `--ready`, `--where`, ...) narrow the hits as usual. An empty query lists every item the other filters leave.
- **Selection.** The hits appear as `id  status  title` in the order `--sort` gives. The chosen item is printed exactly as `lif show <id>` prints it.
- **No hits.** The command prints `No items match.` and exits 0.
- **Terminal requirement.** Without a terminal on stdin and stdout the command exits non-zero at once with a message naming the missing terminal and never waits for input. `-i` cannot be combined with `--json`.
- **Cancel.** `Ctrl+C` exits non-zero with no stderr line.

```
lif list -i -S login --status todo
```
