#!/usr/bin/env bash
# Prints the body of one CHANGELOG.md section, without its heading.
#
#   scripts/release-notes.sh 1.6.0        # the "## [1.6.0] - <date>" section
#   scripts/release-notes.sh Unreleased
#   scripts/release-notes.sh v1.6.0       # a tag: the section of its version
#   scripts/release-notes.sh v1.6.0-rc.1  # a pre-release tag: "Unreleased"
#
# release.yml passes the tag and uses the output as the GitHub release notes.
# A pre-release is tagged by hand on main, where its changes are still under
# "## [Unreleased]". release-prepare.sh uses this to check the sections, so
# the rules for reading the file live only here.
#
# Fails when the section is missing (exit status 2), appears more than once,
# or is empty (exit status 1). Headings inside code fences are ignored. A link
# definition line ("[x]: url") ends a section, so that the definitions at the
# end of the file are not part of the last section: write the links inside
# entries inline, not as reference-style definitions.
set -euo pipefail

name="${1:?usage: release-notes.sh <version|Unreleased|tag>}"
case "$name" in
    v*-*) name=Unreleased ;;
    v*) name="${name#v}" ;;
esac
changelog="$(dirname "$0")/../CHANGELOG.md"

awk -v name="$name" '
/^[[:space:]]*(```|~~~)/ { fence = !fence }
!fence && /^## / {
    inside = ($0 == "## [" name "]" || index($0, "## [" name "] ") == 1)
    if (inside) { found++; next }
}
!fence && /^\[[^]]+\]: / { inside = 0 }
inside { body[++n] = $0 }
END {
    if (found != 1) {
        printf "CHANGELOG.md has %d \"## [%s]\" sections; expected 1\n", found, name > "/dev/stderr"
        exit (found == 0 ? 2 : 1)
    }
    first = 1
    while (first <= n && body[first] ~ /^[[:space:]]*$/) first++
    last = n
    while (last >= first && body[last] ~ /^[[:space:]]*$/) last--
    if (first > last) {
        printf "The \"## [%s]\" section of CHANGELOG.md is empty\n", name > "/dev/stderr"
        exit 1
    }
    for (i = first; i <= last; i++) print body[i]
}' "$changelog"
