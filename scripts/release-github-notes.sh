#!/usr/bin/env bash
set -euo pipefail

usage() {
    printf 'usage: %s <check|publish> <vX.Y.Z> [owner/repo]\n' "${0##*/}" >&2
    exit 2
}

[[ $# -ge 2 && $# -le 3 ]] || usage

mode="$1"
tag="$2"
repo="${3:-rierek/lif}"

case "$mode" in
    check|publish) ;;
    *) usage ;;
esac

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'invalid release tag: %s\n' "$tag" >&2
    exit 2
fi

version="${tag#v}"
note="wiki/releases/v${version//./-}.md"
if [[ ! -f "$note" ]]; then
    printf 'release note not found: %s\n' "$note" >&2
    exit 1
fi

body="$(mktemp)"
trap 'rm -f "$body"' EXIT

if ! awk -v title="title: \"$tag\"" '
    NR == 1 {
        if ($0 != "---") exit 10
        frontmatter = 1
        next
    }
    frontmatter {
        if ($0 == "---") {
            frontmatter = 0
            closed = 1
            next
        }
        if ($0 == title) expected_title = 1
        next
    }
    {
        print
        if ($0 ~ /[^[:space:]]/) body = 1
    }
    END {
        if (!closed || !expected_title || !body) exit 11
    }
' "$note" >"$body"; then
    printf 'invalid release note: %s must have matching frontmatter title and non-empty Markdown body\n' "$note" >&2
    exit 1
fi

if [[ "$mode" == check ]]; then
    printf 'validated release note: %s\n' "$note"
    exit 0
fi

gh release edit "$tag" -R "$repo" --notes-file "$body"
