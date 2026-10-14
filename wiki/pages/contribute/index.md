---
title: "Contribute"
description: "Report bugs and requests, build Lif from source, license and credit."
---

## Report bugs and requests

Read [Known limitations](../known-limitations/) first. Your problem may already be listed there, with a workaround.

Report bugs and request features in the issue tracker: <https://github.com/xRiErOS/lif/issues>. Reports about Lif go there, not to `hmans/beans`.

A good report contains:

- the observed behavior
- the expected behavior
- your Lif version (`lif version`)
- the smallest store shape that reproduces the problem

Focused pull requests are welcome. A new feature needs a GitHub Discussion first: <https://github.com/xRiErOS/lif/discussions>. Feature pull requests without a Discussion are declined. There is no promise about review or response times.

## Build from source

You need Git and [mise](https://mise.jdx.dev). `mise` installs the toolchain declared in `mise.toml`: Go, Node.js and pnpm. The Go module requires Go 1.26 (`go.mod`). The docs site builds with `npm ci` and `npm run build`, not with pnpm.

```sh
git clone https://github.com/xRiErOS/lif.git
cd lif
mise install
mise run setup
```

Then use `just`:

| Command | What it does |
|---|---|
| `just build` | Builds the stamped binaries `lif` and `lif-serve`. It runs `mise run build`. |
| `just test` | Runs the Go tests (`go test -count=1 ./...`). |
| `just test-race` | Runs the Go tests with the race detector. |
| `just test-web` | Runs the frontend unit tests. |
| `just check-web` | Runs the frontend type check. |
| `just docs-gen` | Generates the command reference pages. |
| `just docs-build` | Generates the command pages and builds the docs site. |
| `just docs-dev` | Serves the docs site with live reload. |

## License

Lif is distributed under the Apache License 2.0. See the [`LICENSE`](https://github.com/xRiErOS/lif/blob/main/LICENSE) file in the repository.

## Credit

Lif started as a fork of [`hmans/beans`](https://github.com/hmans/beans) by Hendrik Mans. The original idea and the first implementation come from that project. See [About](../about/) for the origin and the attribution.
