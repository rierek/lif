#!/usr/bin/env bash
# Fails unless the `go` on PATH is the Go pinned in mise.toml (beans-zcex).
set -euo pipefail
want="go$(mise current go)"
have="$(go env GOVERSION)"
if [ "$have" != "$want" ]; then
  echo "go toolchain mismatch: $(command -v go) is $have, mise.toml pins $want" >&2
  exit 1
fi
