#!/usr/bin/env bash
# Fills in the release pull request. tagpr runs this as postVersionCommand
# (.tagpr) each time it rebuilds the release PR from main.
#
#   TAGPR_CURRENT_VERSION=vX.Y.Z TAGPR_NEXT_VERSION=vX.Y.Z TAGPR_ARTIFACT_DIR=<dir> \
#     scripts/release-prepare.sh
#
# 1. CHANGELOG.md: scripts/tagpr/release-prepare.mjs (kan/tagpr-scripts,
#    pulled in with git subtree; do not edit it here) turns
#    "## [Unreleased]" into "## [X.Y.Z] - <date in JST>" and adds the link.
# 2. CREDITS: replaced with TAGPR_ARTIFACT_DIR/CREDITS. tagpr.yml generates
#    that file in a job without write access and hands it over as the
#    "credits" artifact, so this script runs no dependency code. The shared
#    workflow leaves TAGPR_ARTIFACT_DIR empty when the download fails.
#
# Every check comes before the first write, so a failed check changes
# nothing. When it fails, tagpr still updates the release PR but the run fails
# and the release PR is turned into a draft: do not merge the PR then.
set -euo pipefail

cd "$(dirname "$0")/.."

credits="${TAGPR_ARTIFACT_DIR:+$TAGPR_ARTIFACT_DIR/CREDITS}"
if [ ! -f "$credits" ]; then
    echo "TAGPR_ARTIFACT_DIR must hold the generated CREDITS file, got \"${TAGPR_ARTIFACT_DIR:-}\"" >&2
    exit 1
fi

node scripts/tagpr/release-prepare.mjs --changelog-only
cp "$credits" CREDITS
