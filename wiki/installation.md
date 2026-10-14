# Installation

This page covers the five ways to install Lif and explains the difference between its two binaries. If you already run an install from an earlier release, follow the [upgrade guide](compatibility-and-upgrading.md) first.

## Origin

Lif is maintained at [xRiErOS/lif](https://github.com/xRiErOS/lif), and its Go module path is `github.com/xRiErOS/lif`. See [Fork lineage](fork-lineage.md) for the project's origin and attribution.

## Installation channels

All five channels install the same two binaries described below. Pick one:

### Homebrew (macOS)

```sh
brew install xRiErOS/lif/lif
```

Upgrade with `brew upgrade --cask lif`.

### Install script (Linux, macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/xRiErOS/lif/main/install.sh | sh
```

Configurable via environment variables: `LIF_VERSION` (default: latest release), `LIF_BIN_DIR` (default: `$HOME/.local/bin`), `LIF_REPO` (default: `xRiErOS/lif`).

### go install (Linux, macOS, Windows)

```sh
go install github.com/xRiErOS/lif/cmd/lif@latest
```

A `lif-serve` built this way embeds no web assets — the repository tracks only a placeholder (`internal/web/dist/.gitkeep`) so the `//go:embed` directive in `internal/web/embed.go` compiles, but the actual frontend build is not part of the module source. Requesting `/` returns 404. Use Homebrew, the install script, Nix, or a release archive for a working `lif-serve`.

A binary installed this way still reports the correct release version (`lif version` shows the release tag, not `dev`): `internal/version/buildinfo.go` reads it from `runtime/debug.ReadBuildInfo()` when the build did not receive `-ldflags` (which `go install` never does). Commit and build date stay `unknown` for a `go install pkg@version` install, since module proxy downloads carry no `.git` history to stamp from; they are only populated when building from a local git checkout (`go build`/`go install` run inside the cloned repository).

### Nix (Linux, macOS)

```sh
nix profile install github:xRiErOS/lif
```

Without installing: `nix run github:xRiErOS/lif -- version` runs the CLI, `nix run github:xRiErOS/lif#lif-serve` starts the server, and `nix shell github:xRiErOS/lif` opens a shell with both binaries on `PATH`.

The flake builds the SvelteKit frontend with pnpm and copies it into `internal/web/dist` before the Go build, so `lif-serve` from this channel does serve the web UI. Version, commit, and date are stamped via `-ldflags`, as in the release build.

To pin Lif in a system flake:

```nix
{
  inputs.lif.url = "github:xRiErOS/lif";

  outputs = { self, nixpkgs, lif }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      modules = [{
        environment.systemPackages = [ lif.packages.x86_64-linux.default ];
      }];
    };
  };
}
```

A development shell with the Go and Node.js toolchain, `just`, and `mise` is available with `nix develop`.

### GitHub Releases

Download the `linux`/`darwin`/`windows` × `amd64`/`arm64` (`i386` on Linux only) archive for your platform from the [Releases page](https://github.com/xRiErOS/lif/releases), verify against the accompanying `checksums.txt`, and extract `lif` and `lif-serve` onto your `PATH`.

## Prerequisites (build from source only)

Install [mise](https://mise.jdx.dev/getting-started.html) first. The repository's `mise.toml` declares the Go, Node.js, and pnpm toolchains the build needs and installs them; the current Go module requires Go 1.26.

## The two binaries

The fork builds two separate binaries, one per entrypoint under `cmd/`: `lif` (`cmd/lif`, the CLI you use for day-to-day issue tracking and the home of the terminal UI, started with `lif tui`) and `lif-serve` (`cmd/lif-serve`, the standalone web UI and GraphQL API server).

Each binary is independent; you only need `lif-serve` if you want the web UI. The terminal UI needs no extra install: `lif tui` is a subcommand of `lif`.

## Building from source

Clone the fork, let mise install the declared toolchain, install dependencies and generated code, then run the repository's canonical build:

```sh
git clone https://github.com/xRiErOS/lif.git
cd lif

mise install
mise run setup
mise run build
```

The build pipeline compiles the Svelte frontend into `internal/web/dist` before building the Go entrypoints, and stamps version, commit, and build date into the binaries. A plain `go build ./cmd/lif` also compiles, because the repository tracks a placeholder in `internal/web/dist` that satisfies the embed directive, but it produces an unstamped binary, and a `lif-serve` built that way embeds no web assets and serves 404 for the browser UI.

`mise run build` writes the stamped `lif` and `lif-serve` binaries to the repository root. Install them in a user-owned directory on `PATH`:

```sh
install -d "$HOME/.local/bin"
install -m 755 lif lif-serve "$HOME/.local/bin/"
export PATH="$HOME/.local/bin:$PATH"
```

Persist the `PATH` addition in your shell configuration if `$HOME/.local/bin` is not already present.

## Verifying the build

Confirm that the installed CLI is reachable and carries the version, commit, and build date injected by the build task:

```sh
lif version
lif --help
```

A plain local `go build` does not inject those linker values (it also has no embedded frontend, see the `go install` section above); use `mise run build` for a fully stamped binary with the web UI, or one of the release-artifact channels above.

## Maintaining the Nix flake

The flake pins the release version and two content hashes, so it needs an update after a release or a dependency change.

1. Set `version` in `flake.nix` to the new release.
2. If `go.mod` or `go.sum` changed, run `nix build --no-link`, take the `got:` hash from the failure, and write it into `vendorHash`.
3. If `frontend/pnpm-lock.yaml` changed, run `nix build --no-link .#frontend` and write the `got:` hash into the `pnpmDeps` hash.
4. Refresh the nixpkgs pin with `nix flake update` when you want a newer toolchain.
5. Verify with `nix run . -- version`, `nix flake check`, and `nix develop -c go version`.
6. Commit `flake.nix` and `flake.lock` together.

## License

The fork is distributed under the Apache License 2.0, the same license as the upstream project; see `LICENSE` in the repository root.

## Related documentation

- [Quick Start](quick-start.md)
- [Configuration](configuration.md)
- [Separate Binaries](commands/separate-binaries.md)
- [Web UI and API](web-ui-and-api.md)
- [Terminal UI](tui-companion.md)
- [Fork Lineage](fork-lineage.md)
- [Compatibility and Upgrading](compatibility-and-upgrading.md)
- [Troubleshooting](troubleshooting.md)
