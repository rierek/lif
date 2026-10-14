---
title: "Installation"
description: "Install Lif and check that it works."
---

## Purpose
Target of the hero button on the start page: install Lif and check with a first command that it runs. Shows only the installation path, no configuration.

## Audience
New users who have chosen Lif and want a working `lif` as quickly as possible, plus agents that look up installation steps.

## Required sections

### Requirements
- Release archives exist for Linux (`x86_64`, `arm64`, `i386`), macOS (`x86_64`, `arm64`) and Windows (`x86_64`, `arm64`, `i386`); the file names use these arch names, not the Go names `amd64` and `386`.
- The install script supports Linux and macOS (`x86_64`, `arm64`, `i386` on Linux); Windows users download the `.zip` archive from the Releases page.
- Prerequisites for building from source only: `mise` installs Go, Node.js and pnpm (`mise.toml` pins `latest` for each); the Go module requires Go 1.26 (`go.mod`).
- The binary installs need no other prerequisite on the page; each binary is independent, `lif-serve` is only needed for the web UI.
- Source: `.goreleaser.yaml` (`builds`, `archives`); `install.sh` (`detect_os`, `detect_arch`); `wiki/installation.md` (sections "GitHub Releases", "Prerequisites (build from source only)", "The two binaries"); `go.mod`; `mise.toml`

### Install methods
- Four channels, shown as tabs or a subsection list: Homebrew, install script, `go install`, release archive; building from source is the fifth path. Homebrew, the install script, the release archive and the source build install both binaries `lif` and `lif-serve`; `go install` installs only `lif`.
- Homebrew (macOS): `brew install xRiErOS/lif/lif` (a Homebrew cask from the tap `xRiErOS/homebrew-lif`). On macOS the cask removes the quarantine attribute from both binaries with `xattr` after install.
- Install script (Linux, macOS): `curl -fsSL https://raw.githubusercontent.com/xRiErOS/lif/main/install.sh | sh`. It downloads the release archive, verifies it against `checksums.txt` when `sha256sum` or `shasum` exists (it only warns and still installs when `checksums.txt` cannot be downloaded or neither tool exists), installs both binaries and runs `lif version`. Environment variables: `LIF_VERSION` (default: latest release), `LIF_BIN_DIR` (default: `$HOME/.local/bin`), `LIF_REPO` (default: `xRiErOS/lif`). It prints a note when `LIF_BIN_DIR` is not on `PATH`.
- `go install` (Linux, macOS, Windows): `go install github.com/xRiErOS/lif/cmd/lif@latest` installs only `lif`. Caveat: a `lif-serve` built from the module source this way embeds no web assets and serves 404 for the browser UI; use Homebrew, the install script or a release archive for a working `lif-serve`.
- GitHub Releases: download the archive for the platform from the Releases page, verify it against `checksums.txt`, extract `lif` and `lif-serve` onto `PATH`. Archives are named `lif_<Os>_<arch>` with a title-cased OS and `x86_64`, `arm64` or `i386` as arch, for example `lif_Linux_x86_64.tar.gz` (`.zip` on Windows).
- Build from source: `git clone https://github.com/xRiErOS/lif.git`, `mise install`, `mise run setup`, `mise run build`; then install the stamped binaries from the repository root, for example with `install -m 755 lif lif-serve "$HOME/.local/bin/"`.
- Each way gets one copyable command and one line "when to use".
- `just install` and its guard (`LIF_INSTALL_FORCE`) are a developer path and are not on this page.
- Source: `README.md` (section "Installation"); `wiki/installation.md` (sections "Installation channels", "Building from source"); `install.sh`; `.goreleaser.yaml` (`homebrew_casks`, `archives`, `checksum`); FACTS.md §15 (installation, verified; takes precedence over `wiki/installation.md` where they differ)

### The binaries
- `lif`: CLI and TUI (`lif tui`); the TUI is no longer a separate binary.
- `lif-serve`: separate binary for the web UI and the GraphQL API; without an argument it starts `serve`.
- The command `lif serve` in the binary `lif` is a hidden stub and is not documented; it points to `lif-serve`.
- Table: binary · purpose · default port (8080 for `lif-serve`).
- Source: FACTS.md §1.0, §1.1 (`lif tui`, `lif serve` stub, `lif-serve`), §4.3; `wiki/installation.md` (section "The two binaries")

### Verify the installation
- Shows `lif version` with example output; `--json` for structured output.
- Explains: a development build reports the version `dev`; a binary from `go install pkg@version` reports the release tag, with commit and build date `unknown`. `lif-serve` has no version command or flag; the check uses `lif version`.
- Shows `lif --help`: 28 commands are listed.
- First useful command: `lif init` in the project folder; link to First steps (`overview/first-steps`).
- Source: FACTS.md §1.1 (`version`, command count), §1.0 (bootstrap skip: `version` runs without a store); `wiki/installation.md` (section "Installation channels", `go install` part)

### Shell completion
- Shows `lif completion bash|zsh|fish|powershell` (one shared command page, D25).
- Names item ID completion as the benefit.
- Optional: the zsh widget `extras/zsh/lif-pick.zsh` binds `Alt+i` to `lif pick`; it is part of the repository only, not of the release archives, the Homebrew cask or the install script; install by `source /path/to/lif-pick.zsh` in `.zshrc`, next to `lif completion zsh`; `LIF_PICK_KEYBIND` changes the key binding.
- Source: FACTS.md §1.1 (Cobra built-ins, `lif pick`); DESIGN.md D25; `extras/zsh/lif-pick.zsh` (header comment)

### Update and uninstall
- Update Homebrew: `brew upgrade --cask lif`.
- Update install script: run it again; `LIF_VERSION` pins a release tag.
- Update `go install`: run the command again. Update release archive: download and extract the new archive. Update source build: pull and run `mise run build` again.
- Before an upgrade, commit pending item changes and read the release notes for breaking changes; link to the Releases pages (`releases`).
- Uninstall: remove `lif` and `lif-serve` from the install directory (default of the install script: `$HOME/.local/bin`). No dedicated uninstall command exists; the page does not invent one. Stores in the project (`.lif/`, `.lif.yml`) stay as normal files.
- Old `.beans` stores keep being read in place after the install; link to the About page (`about`) for legacy compatibility.
- Source: `wiki/installation.md` (section "Homebrew (macOS)"); `install.sh`; `wiki/compatibility-and-upgrading.md` (sections "Before upgrading: back up your data", "Review changes before upgrading"); FACTS.md §2.4, §4.1 (legacy reading of `.beans/`)

### Next step
- Link to First steps (`overview/first-steps`): create a store, choose a preset, first items.
- Link to Concepts (`overview/concepts`) and, for agents, Agent onboarding (`overview/agent-onboarding`).
- Source: DESIGN.md D37; CONCEPT.md §3.1
