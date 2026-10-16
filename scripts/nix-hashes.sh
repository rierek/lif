#!/usr/bin/env bash
# Rebuild the flake and write the hashes nix reports back into flake.nix.
# Usage: scripts/nix-hashes.sh [VERSION]   (VERSION also sets `version = "..."`)
# Uses a local nix if present, else the nixos/nix Docker image (store cached in volume nix-store-lif).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if [ -n "${1:-}" ]; then
    sed -i.bak -E "s/^( *version = )\"[^\"]*\";/\1\"${1#v}\";/" flake.nix && rm flake.nix.bak
fi

build() {
    local cmd='nix --extra-experimental-features "nix-command flakes" build --no-link -L path:/src#default'
    if command -v nix >/dev/null; then
        nix --extra-experimental-features "nix-command flakes" build --no-link -L "path:$PWD#default"
    else
        docker run --rm -v "$PWD":/src:ro -v nix-store-lif:/nix nixos/nix:latest sh -c "$cmd"
    fi
}

# Two fixed-output derivations (pnpm deps, Go modules) can be stale; each run fixes one.
for _ in 1 2 3; do
    if log=$(build 2>&1); then
        echo "flake builds"
        exit 0
    fi
    old=$(sed -n -E 's/^ *specified: *(sha256-[^ ]+)$/\1/p' <<<"$log" | head -1)
    new=$(sed -n -E 's/^ *got: *(sha256-[^ ]+)$/\1/p' <<<"$log" | head -1)
    if [ -z "$old" ] || [ -z "$new" ]; then
        tail -30 <<<"$log" >&2
        exit 1
    fi
    echo "replacing $old -> $new"
    sed -i.bak "s|$old|$new|" flake.nix && rm flake.nix.bak
done
echo "still failing after 3 hash updates" >&2
exit 1
