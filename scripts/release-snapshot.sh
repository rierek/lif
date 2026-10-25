#!/usr/bin/env bash
set -euo pipefail

cache="$(mktemp -d)"
trap 'rm -rf "$cache"' EXIT

mise run build:embed
GOTOOLCHAIN=local GOCACHE="$cache" mise exec goreleaser@latest -- goreleaser release --snapshot --clean --skip=publish
