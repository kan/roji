#!/usr/bin/env bash
# Fills in the release pull request. tagpr runs this as postVersionCommand
# (.tagpr) each time it rebuilds the release PR from main.
#
#   TAGPR_NEXT_VERSION=vX.Y.Z ROJI_CREDITS=<file> scripts/release-prepare.sh
#
# 1. CHANGELOG.md: "## [Unreleased]" becomes "## [X.Y.Z] - <date in JST>",
#    an empty "## [Unreleased]" is put back above it for the next release,
#    and the link definition for X.Y.Z is added.
# 2. CREDITS: replaced with ROJI_CREDITS. tagpr.yml generates that file in a
#    job without write access, so this script runs no dependency code.
#
# Running it again on its own output changes nothing. When it fails, tagpr
# still updates the release PR but the run fails: do not merge the PR then.
set -euo pipefail

cd "$(dirname "$0")/.."

version="${TAGPR_NEXT_VERSION:-}"
version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "TAGPR_NEXT_VERSION must be vX.Y.Z, got \"${TAGPR_NEXT_VERSION:-}\"" >&2
    exit 1
fi
if [ ! -f "${ROJI_CREDITS:-}" ]; then
    echo "ROJI_CREDITS must name the generated CREDITS file, got \"${ROJI_CREDITS:-}\"" >&2
    exit 1
fi

# release-notes.sh decides what counts as a section, so both scripts read
# the file the same way. Exit status 2 means the section is missing.
notes() { bash scripts/release-notes.sh "$1" > /dev/null; }

status=0
notes "$version" 2> /dev/null || status=$?
case "$status" in
    0)
        if notes Unreleased 2> /dev/null; then
            echo "CHANGELOG.md has both \"## [$version]\" and entries under \"## [Unreleased]\"" >&2
            exit 1
        fi
        cp "$ROJI_CREDITS" CREDITS
        echo "CHANGELOG.md: \"## [$version]\" is already stamped"
        exit 0
        ;;
    2) ;;
    *)
        # Duplicated or empty: print why
        notes "$version"
        ;;
esac
# Prints why the section is missing, duplicated or empty
notes Unreleased

cp "$ROJI_CREDITS" CREDITS

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

awk -v v="$version" -v date="$(TZ=Asia/Tokyo date +%F)" '
BEGIN { link = "[" v "]: https://github.com/kan/roji/releases/tag/v" v }
/^[[:space:]]*(```|~~~)/ { fence = !fence }
!fence && ($0 == "## [Unreleased]" || index($0, "## [Unreleased] ") == 1) {
    print
    print ""
    print "## [" v "] - " date
    next
}
!fence && !linked && /^\[[^]]+\]: / {
    print link
    linked = 1
}
{ print }
END {
    if (!linked) {
        print ""
        print link
    }
}' CHANGELOG.md > "$tmp"
cat "$tmp" > CHANGELOG.md
echo "CHANGELOG.md: \"## [Unreleased]\" -> \"## [$version]\""
