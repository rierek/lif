#!/usr/bin/env bash
set -euo pipefail

# mise inserts its tool dirs where its shims dir sits on PATH. Behind a Homebrew
# `go` that pairs mise's GOROOT with the wrong `go`: "compile: version ... does
# not match go tool version ...". Without the shims dir mise prepends instead.
PATH="$(printf %s "$PATH" | tr ':' '\n' | grep -v '/mise/shims$' | paste -sd: -)"

mise run build:embed
mise exec goreleaser@latest -- goreleaser release --snapshot --clean --skip=publish
