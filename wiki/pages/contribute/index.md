---
title: "Contribute"
description: "Report bugs and requests, build Lif from source, license and credit."
---

## Purpose
Short page: tells readers where to report problems, how to build Lif from source with `just`, under which license Lif is distributed and whom the project credits. It welcomes focused pull requests, requires a Discussion before any feature pull request, and makes no promise about review times (D61, D63).

## Audience
Users who found a bug or want a feature; developers who build Lif from source.

## Required sections

### Report bugs and requests
- Links to the Known limitations page (`known-limitations`) before asking readers to report a bug.
- Names the issue tracker `https://github.com/xRiErOS/lif/issues` as the place for bug reports and feature requests; reports about Lif go there, not to `hmans/beans`.
- Lists what a good report contains: observed behavior, expected behavior, Lif version (`lif version`), smallest reproducible store shape.
- States that focused pull requests are welcome (D63). A new feature needs a GitHub Discussion opened first at `https://github.com/xRiErOS/lif/discussions`; feature pull requests without one are declined. Makes no promise about review or response times.
- Source: `wiki/fork-lineage.md` (section "How to reference Lif"); `README.md` (section "Project status"); DESIGN.md D61, D63; `README.md` line 238

### Build from source
- Prerequisite: `mise` installs the toolchain declared in `mise.toml` (Go, Node.js, pnpm, each pinned to `latest`); the docs site itself builds with `npm ci` and `npm run build` (justfile `docs-build`), not with pnpm; the Go module requires Go 1.26 (`go.mod`).
- Shows `git clone https://github.com/xRiErOS/lif.git`, `mise install`, `mise run setup`.
- `just build` builds the stamped binaries `lif` and `lif-serve` (it runs `mise run build`).
- `just test` runs the Go test suite (`go test -count=1 ./...`); `just test-race`, `just test-web` and `just check-web` run the race detector, the frontend unit tests and the frontend type check.
- `just docs-gen`, `just docs-build` and `just docs-dev` generate the command pages, build the docs site and serve it with live reload.
- Source: `justfile` (recipes `build`, `test`, `test-race`, `test-web`, `check-web`, `docs-gen`, `docs-build`, `docs-dev`); `mise.toml`; `go.mod`; `wiki/installation.md` (sections "Prerequisites", "Building from source")

### License
- Names the license Apache License 2.0 and links to the file `LICENSE` in the repository.
- Source: `LICENSE`; `README.md` (section "License")

### Credit
- Lif originated as a fork of `hmans/beans` by Hendrik Mans; the original idea and the first implementation come from that project.
- Links to the About page (`about`) for the origin and the attribution.
- Source: `README.md` (section "Acknowledgements"); `wiki/fork-lineage.md` (sections "Origin", "License and attribution")
