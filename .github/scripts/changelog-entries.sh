#!/usr/bin/env bash
# changelog-entries.sh
#
# Prints the changelog lines for a release: one line per public pull request
# in the release that carries the `release-notes` label and not
# `skip-changelog`, the same selection .github/release-notes.yml makes.
#
# Pull requests are found from the commits themselves, not from GitHub's
# generated notes. The release is prepared on an internal branch GitHub cannot
# compare against, and a patch's fixes exist only there until release day, so
# the commit range is the only thing that knows what the release contains.
# Squash merges and cherry-picks both carry "(#N)" in the subject.
#
# Usage:
#   changelog-entries.sh --since TAG [--ref REF] [--repo OWNER/NAME]
#
#   --since TAG   the previous release; commits reachable from it are excluded
#   --ref REF     the release commit. Default: HEAD
#   --repo R      where the pull requests live. Default: nginx/nginx-gateway-fabric
#
# Output lines look like the ones GitHub generates:
#   * Add a thing by @someone in https://github.com/OWNER/NAME/pull/123
#
# Exit status: 0 printed (possibly nothing), 1 a lookup failed, 2 bad usage.
#
# GIT and GH may be set to substitute git and gh, for tests.

set -euo pipefail

GIT="${GIT:-git}"
GH="${GH:-gh}"
REPO="nginx/nginx-gateway-fabric"
SINCE=""
REF="HEAD"
INCLUDE_LABEL="release-notes"
EXCLUDE_LABEL="skip-changelog"

die() {
    echo "error: $*" >&2
    exit 2
}

while [ $# -gt 0 ]; do
    case "$1" in
    --since)
        [ $# -ge 2 ] || die "--since needs a value"
        SINCE="$2"
        shift 2
        ;;
    --ref)
        [ $# -ge 2 ] || die "--ref needs a value"
        REF="$2"
        shift 2
        ;;
    --repo)
        [ $# -ge 2 ] || die "--repo needs a value"
        REPO="$2"
        shift 2
        ;;
    *) die "unrecognised argument '$1'" ;;
    esac
done

[ -n "${SINCE}" ] || die "--since is required"
"${GIT}" rev-parse --verify --quiet "${SINCE}^{commit}" >/dev/null || die "cannot resolve --since '${SINCE}'"
"${GIT}" rev-parse --verify --quiet "${REF}^{commit}" >/dev/null || die "cannot resolve --ref '${REF}'"

# A subject can name several numbers: a fix picked onto a release branch
# through a pull request of its own reads "Fix a thing (#123) (#130)", and
# #130 only moved #123. The first number that is a merged pull request here is
# the change; one that is not (a number from another repository, an issue)
# is passed over. Merge commits are skipped: their subjects name branches.
lookup() {
    local n="$1" out
    if out="$("${GH}" api "repos/${REPO}/pulls/${n}" 2>&1)"; then
        printf '%s' "${out}"
        return 0
    fi
    case "${out}" in
    *"Not Found"* | *"HTTP 404"*) return 1 ;;
    esac
    echo "error: could not read #${n} from ${REPO}: ${out}" >&2
    return 2
}

declare -A seen=()
# Oldest first, as the list is read top to bottom next to the release.
while IFS= read -r subject; do
    for n in $(printf '%s' "${subject}" | grep -oE '\(#[0-9]+\)' | tr -d '(#)'); do
        [ -z "${seen[${n}]:-}" ] || continue 2
        # Called in a subshell, so a failure is a return code, not an exit:
        # 1 is "not a pull request here", anything else stops the run.
        rc=0
        pr="$(lookup "${n}")" || rc=$?
        [ "${rc}" -ne 1 ] || continue
        [ "${rc}" -eq 0 ] || exit 1
        seen[${n}]=1
        printf '%s' "${pr}" | jq -r \
            --arg inc "${INCLUDE_LABEL}" --arg exc "${EXCLUDE_LABEL}" '
            select(.merged_at != null)
            | select([.labels[].name] | index($inc))
            | select([.labels[].name] | index($exc) | not)
            | "* \(.title) by @\(.user.login) in \(.html_url)"'
        continue 2
    done
done < <("${GIT}" log --no-merges --reverse --format=%s "${SINCE}..${REF}")
