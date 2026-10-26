#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    failures=$((failures + 1))
}

assert_file_contains() {
    local name="$1"
    local file="$2"
    local expected="$3"
    if ! grep -Fqx -- "$expected" "$file"; then
        fail "$name"
    fi
}

assert_file_not_contains() {
    local name="$1"
    local file="$2"
    local expected="$3"
    if grep -Fqx -- "$expected" "$file"; then
        fail "$name"
    fi
}

mkdir -p "$tmp/wiki/releases" "$tmp/bin"
cat >"$tmp/wiki/releases/v0-18-0.md" <<'EOF'
---
title: "v0.18.0"
description: "test release"
---

## Headline

Curated release body.
EOF

cat >"$tmp/bin/gh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$GH_LOG"
for ((i = 1; i <= $#; i++)); do
    if [[ "${!i}" == "--notes-file" ]]; then
        next=$((i + 1))
        cp "${!next}" "$GH_BODY"
    fi
done
EOF
chmod +x "$tmp/bin/gh"

(
    cd "$tmp"
    PATH="$tmp/bin:$PATH" GH_LOG="$tmp/gh.log" "$repo_root/scripts/release-github-notes.sh" check v0.18.0
)
if [[ -e "$tmp/gh.log" ]]; then
    fail "check must not invoke gh"
fi

(
    cd "$tmp"
    PATH="$tmp/bin:$PATH" GH_LOG="$tmp/gh.log" GH_BODY="$tmp/gh-body" "$repo_root/scripts/release-github-notes.sh" publish v0.18.0 test/lif
)
assert_file_contains "publish invokes release edit" "$tmp/gh.log" "release"
assert_file_contains "publish targets release tag" "$tmp/gh.log" "v0.18.0"
assert_file_contains "publish targets repository" "$tmp/gh.log" "test/lif"
assert_file_contains "publish passes note file" "$tmp/gh.log" "--notes-file"
assert_file_contains "publish extracts Markdown body" "$tmp/gh-body" "## Headline"
assert_file_contains "publish keeps Markdown text" "$tmp/gh-body" "Curated release body."
assert_file_not_contains "publish strips frontmatter" "$tmp/gh-body" "---"

: >"$tmp/gh.log"
sed -i.bak 's/v0.18.0/v0.18.1/' "$tmp/wiki/releases/v0-18-0.md"
if (
    cd "$tmp"
    PATH="$tmp/bin:$PATH" GH_LOG="$tmp/gh.log" "$repo_root/scripts/release-github-notes.sh" publish v0.18.0 test/lif
); then
    fail "mismatched title must fail"
fi
if [[ -s "$tmp/gh.log" ]]; then
    fail "invalid note must not invoke gh"
fi

if (( failures > 0 )); then
    exit 1
fi

printf 'release-github-notes: all assertions passed\n'
