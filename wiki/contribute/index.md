---
title: "Contribute"
description: "Report bugs and requests, build Lif from source, license and credit."
---

## Report bugs and requests

Read [Known limitations](../known-limitations/) first. Your problem may already be listed there, with a workaround.

Report bugs and request features in the issue tracker: <https://github.com/rierek/lif/issues>. Reports about Lif go there, not to `hmans/beans`.

A good report contains:

- the observed behavior
- the expected behavior
- your Lif version (`lif version`)
- the smallest store shape that reproduces the problem

Focused pull requests are welcome. A new feature needs a GitHub Discussion first: <https://github.com/rierek/lif/discussions>. Feature pull requests without a Discussion are declined. There is no promise about review or response times.

## Build from source

You need Git and [mise](https://mise.jdx.dev). `mise` installs the toolchain declared in `mise.toml`: Go, Node.js and pnpm. The Go module requires Go 1.26 (`go.mod`). The docs site builds with `npm ci` and `npm run build`, not with pnpm.

```sh
git clone https://github.com/rierek/lif.git
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

### Nix development shell

`nix develop` opens a shell with Go, `gopls`, `gotools`, `go-tools`, Node.js, pnpm 10, `just` and `mise`.

### Maintain the Nix flake

`flake.nix` pins hashes of the Go module set (`vendorHash`) and the pnpm dependency set (`pnpmDeps`). A change to `go.mod`, `go.sum` or `frontend/pnpm-lock.yaml` without a hash update breaks `nix profile install`.

Run `just nix-hashes` after such a change, or `just nix-hashes 0.16.0` to also set `version` for an upcoming release. It builds the flake, writes each `got:` hash from a mismatch into `flake.nix` and repeats until the build passes. It uses a local `nix` if present, otherwise the `nixos/nix` Docker image. Commit `flake.nix` with the lockfile change.

Three guards catch a forgotten update:

- The `nix` workflow builds the flake on pull requests and on pushes to `main` and `milestone/**` that touch the flake, `go.mod`/`go.sum` or the frontend lockfile.
- The `release` workflow runs the same build, and checks that `version` matches the tag, before goreleaser publishes anything.
- `just release` refuses to tag when `version` in `flake.nix` differs from the next version.

To refresh the nixpkgs pin, run `nix flake update`, then `just nix-hashes`, and commit `flake.nix` and `flake.lock` together.

## License

Lif is distributed under the Apache License 2.0. See the [`LICENSE`](https://github.com/rierek/lif/blob/main/LICENSE) file in the repository.

## Credit

Lif started as a fork of [`hmans/beans`](https://github.com/hmans/beans) by Hendrik Mans. The original idea and the first implementation come from that project. See [About](../about/) for the origin and the attribution.
