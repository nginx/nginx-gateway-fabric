#!/usr/bin/env bash
# previous-release-tag.sh
#
# Prints the release that comes immediately before VERSION, for use as the
# --notes-start-tag of its GitHub release: the highest `vX.Y.Z` tag in the
# repository that sorts below VERSION. Pre-release tags (anything with a
# suffix, such as -rc.1) are ignored on both sides.
#
# Version order rather than ancestry, because patch tags live on release
# branches: none is an ancestor of main, so `git describe` from a minor
# release cut from main lands on a tag from the start of the project.
#
#   v2.8.0 -> v2.7.2   (the last patch of the previous line)
#   v2.7.3 -> v2.7.2
#   v2.6.9 -> v2.6.8   (an older line is not confused by newer releases)
#
# Usage: previous-release-tag.sh VERSION
#
# Prints nothing and exits 0 when there is no earlier release.
# Exit status: 0 printed (or nothing to print), 2 bad usage.
#
# GIT may be set to substitute git, for tests.

set -euo pipefail

GIT="${GIT:-git}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ $# -eq 1 ] || die "usage: previous-release-tag.sh VERSION"
version="$1"
[[ ${version} =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION must look like v1.2.3, got '${version}'"

# sort -V places v2.7.10 after v2.7.9. The version itself is added so it has a
# position even when its tag does not exist yet, which is the normal case:
# publish looks this up before tagging.
{
    "${GIT}" tag -l 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true
    printf '%s\n' "${version}"
} | sort -uV | awk -v v="${version}" '$0 == v { print prev; exit } { prev = $0 }'
