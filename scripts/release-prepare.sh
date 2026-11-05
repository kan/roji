#!/usr/bin/env bash
# Fills in the release pull request. tagpr runs this as postVersionCommand
# (.tagpr) each time it rebuilds the release PR from main.
#
#   TAGPR_CURRENT_VERSION=vX.Y.Z TAGPR_NEXT_VERSION=vX.Y.Z ROJI_CREDITS=<file> \
#     scripts/release-prepare.sh
#
# 1. CHANGELOG.md: scripts/tagpr/release-prepare.mjs (kan/tagpr-scripts,
#    pulled in with git subtree; do not edit it here) turns
#    "## [Unreleased]" into "## [X.Y.Z] - <date in JST>" and adds the link.
# 2. CREDITS: replaced with ROJI_CREDITS. tagpr.yml generates that file in a
#    job without write access, so this script runs no dependency code.
#
# Every check comes before the first write, so a failed check changes
# nothing. When it fails, tagpr still updates the release PR but the run fails: do
# not merge the PR then.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f "${ROJI_CREDITS:-}" ]; then
    echo "ROJI_CREDITS must name the generated CREDITS file, got \"${ROJI_CREDITS:-}\"" >&2
    exit 1
fi

node scripts/tagpr/release-prepare.mjs --changelog-only
cp "$ROJI_CREDITS" CREDITS
