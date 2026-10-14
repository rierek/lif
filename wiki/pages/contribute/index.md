---
title: "Contribute"
description: "Report bugs and requests, build Lif from source, license and credit."
---

## Purpose
Short page: tells readers where to report problems, how to build Lif from source with `just`, under which license Lif is distributed and whom the project credits. It makes no promise about pull requests or review times (D61).

## Audience
Users who found a bug or want a feature; developers who build Lif from source.

## Required sections

### Report bugs and requests
- Names the issue tracker `https://github.com/xRiErOS/lif/issues` as the place for bug reports and feature requests; reports about Lif go there, not to `hmans/beans`.
- Lists what a good report contains: observed behavior, expected behavior, Lif version (`lif version`), smallest reproducible store shape.
- Makes no statement about pull request acceptance, review times or response times.
- Source: `wiki/fork-lineage.md` (section "How to reference Lif"); `README.md` (section "Project status"); DESIGN.md D61

### Build from source
- Prerequisite: `mise` installs the toolchain declared in `mise.toml` (Go, Node.js, pnpm); the Go module requires Go 1.26.
- Shows `git clone https://github.com/xRiErOS/lif.git`, `mise install`, `mise run setup`.
- `just build` builds the stamped binaries `lif` and `lif-serve` (it runs `mise run build`).
- `just test` runs the Go test suite (`go test -count=1 ./...`); `just test-race`, `just test-web` and `just check-web` run the race detector, the frontend unit tests and the frontend type check.
- No `just lint` recipe exists in the justfile; the page does not mention one.
- `just docs-gen`, `just docs-build` and `just docs-dev` generate the command pages, build the docs site and serve it with live reload.
- Source: `justfile` (recipes `build`, `test`, `test-race`, `test-web`, `check-web`, `docs-gen`, `docs-build`, `docs-dev`); `mise.toml`; `go.mod`; `wiki/installation.md` (sections "Prerequisites", "Building from source")

### License
- Names the license Apache License 2.0 and links to the file `LICENSE` in the repository.
- Source: `LICENSE`; `README.md` (section "License")

### Credit
- Lif originated as a fork of `hmans/beans` by Hendrik Mans; the original idea and the first implementation come from that project.
- Links to the About page (`about`) for the origin and the attribution.
- Source: `README.md` (section "Acknowledgements"); `wiki/fork-lineage.md` (sections "Origin", "License and attribution")
