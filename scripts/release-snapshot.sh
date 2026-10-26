#!/usr/bin/env bash
set -euo pipefail

mise run build:embed
mise exec goreleaser@latest -- bash scripts/check-go-toolchain.sh
mise exec goreleaser@latest -- goreleaser release --snapshot --clean --skip=publish
