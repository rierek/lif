#!/usr/bin/env bash
# Prints PATH with mise.toml's pinned Go first and the mise shims dir removed.
# `mise exec`/`mise run` insert their tool dirs at the shims position; behind a
# foreign go (Homebrew) that pairs it with mise's GOROOT: "compile: version ...
# does not match go tool version ...". Without the shims dir mise prepends.
set -euo pipefail
go_dir="$(mise where go)" || { echo "go from mise.toml not installed: run 'mise install go'" >&2; exit 1; }
printf '%s/bin:%s' "$go_dir" "$(printf %s "$PATH" | tr ':' '\n' | grep -v '/mise/shims$' | paste -sd: -)"
