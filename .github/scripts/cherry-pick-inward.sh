#!/usr/bin/env bash
set -euo pipefail

# cherry-pick-inward.sh
#
# Cherry-picks merged, labelled pull requests from the public repository onto
# the internal release branch, and opens a pull request with the result
# (never pushes directly, so a clean-but-wrong pick still gets reviewed).
#
# Usage:
#   cherry-pick-inward.sh --release-branch release-2.8 [options]
#
# Options:
#   --release-branch X   target is internal/X. Required.
#   --public-repo R      where to read merged pull requests from.
#                        Default: nginx/nginx-gateway-fabric
#   --label L            label marking a pull request for cherry-picking.
#                        Default: "needs cherry pick"
#   --limit N            how many merged pull requests to consider. Default 50.
#   --dry-run            report what would be picked and change nothing.
#   -h, --help
#
# Exit status:
#   0  picked something, or found nothing to pick
#   1  a cherry-pick conflicted, or the branch could not be prepared
#   2  bad usage
#
# Requires: git, gh. `gh` needs a token that can read the public repository;
# the mirror's own GITHUB_TOKEN is enough, since that repository is public.

PUBLIC_REPO="${PUBLIC_REPO:-nginx/nginx-gateway-fabric}"
LABEL="${LABEL:-needs cherry pick}"
LIMIT="${LIMIT:-50}"
RELEASE_BRANCH=""
DRY_RUN=false

GIT="${GIT:-git}"
GH="${GH:-gh}"

die() {
    echo "error: $*" >&2
    exit 2
}

usage() {
    sed -n '/^# Usage:/,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
        --release-branch)
            [ $# -ge 2 ] || die "--release-branch needs a value"
            RELEASE_BRANCH="$2"
            shift 2
            ;;
        --public-repo)
            [ $# -ge 2 ] || die "--public-repo needs a value"
            PUBLIC_REPO="$2"
            shift 2
            ;;
        --label)
            [ $# -ge 2 ] || die "--label needs a value"
            LABEL="$2"
            shift 2
            ;;
        --limit)
            [ $# -ge 2 ] || die "--limit needs a value"
            LIMIT="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *) die "unrecognised argument '$1' (try --help)" ;;
        esac
    done

    [ -n "${RELEASE_BRANCH}" ] || die "--release-branch is required"
    echo "${RELEASE_BRANCH}" | grep -Eq '^release-[0-9]+\.[0-9]+$' ||
        die "--release-branch must look like release-X.Y, got '${RELEASE_BRANCH}'"
    echo "${LIMIT}" | grep -Eq '^[0-9]+$' || die "--limit must be a whole number, got '${LIMIT}'"
}

# True when a squash-commit subject on the branch contains "(#N)", or a previous
# run's cherry-pick "-x" trailer names the commit. "(#2)" cannot match "(#22)",
# because the closing bracket is part of the search string.
already_applied() {
    local branch="$1" number="$2" sha="${3:-}"

    if "${GIT}" log --format=%s "${branch}" | grep -qF "(#${number})"; then
        return 0
    fi

    if [ -n "${sha}" ] && "${GIT}" log --format=%B "${branch}" | grep -qF "cherry picked from commit ${sha}"; then
        return 0
    fi

    return 1
}

# Merged pull requests carrying the label, oldest merge first so an earlier
# fix applies before a later one that might depend on it.
list_candidates() {
    "${GH}" pr list \
        --repo "${PUBLIC_REPO}" \
        --state merged \
        --label "${LABEL}" \
        --limit "${LIMIT}" \
        --json number,title,mergeCommit,mergedAt \
        --jq 'sort_by(.mergedAt)[] | "\(.number)\t\(.mergeCommit.oid)\t\(.title)"'
}

main() {
    parse_args "$@"

    local target="internal/${RELEASE_BRANCH}"
    "${GIT}" rev-parse --verify --quiet "origin/${target}" >/dev/null ||
        die "${target} does not exist; cut it first with cut-internal-release-branch.yml"

    echo "target:      ${target}"
    echo "reading:     ${PUBLIC_REPO}"
    echo "label:       ${LABEL}"

    # Fetch the public history so the merge commits are reachable. The
    # repository is public, so this needs no credential.
    "${GIT}" remote get-url public >/dev/null 2>&1 ||
        "${GIT}" remote add public "https://github.com/${PUBLIC_REPO}.git"
    "${GIT}" fetch --quiet public

    local candidates
    candidates="$(list_candidates)"

    local picked=0 skipped=0 number sha title
    local -a picked_numbers=()

    local work="cherry-pick-inward/${RELEASE_BRANCH}"
    "${GIT}" checkout --quiet -B "${work}" "origin/${target}"

    while IFS=$'\t' read -r number sha title; do
        [ -n "${number:-}" ] || continue

        if already_applied "origin/${target}" "${number}" "${sha}"; then
            echo "  skip  #${number} ${title}"
            skipped=$((skipped + 1))
            continue
        fi

        echo "  pick  #${number} ${title}"
        picked_numbers+=("${number}")
        picked=$((picked + 1))

        [ "${DRY_RUN}" = "true" ] && continue

        # -x records the origin commit, which is what a later run reads to
        # decide the pick already happened.
        if ! "${GIT}" cherry-pick -x "${sha}"; then
            "${GIT}" cherry-pick --abort || true
            echo "error: #${number} (${sha}) does not apply cleanly onto ${target}" >&2
            echo "       cherry-pick it by hand, then re-run to pick up the rest" >&2
            exit 1
        fi
    done <<<"${candidates}"

    echo "${picked} to pick, ${skipped} already applied"

    if [ "${picked}" -eq 0 ]; then
        echo "nothing to do"
        return 0
    fi

    if [ "${DRY_RUN}" = "true" ]; then
        echo "dry run: nothing pushed"
        return 0
    fi

    "${GIT}" push --force-with-lease origin "${work}"

    local body="Cherry-picked into \`${target}\` from \`${PUBLIC_REPO}\`:"
    for number in "${picked_numbers[@]}"; do
        body="${body}"$'\n'"- ${PUBLIC_REPO}#${number}"
    done
    body="${body}"$'\n\n'"Opened by cherry-pick-inward.sh. Review before merging: a clean apply is not a correct apply."

    "${GH}" pr create \
        --base "${target}" \
        --head "${work}" \
        --title "Cherry-pick ${picked} change(s) into ${target}" \
        --body "${body}"
}

# Only run the driver when executed directly, not when sourced by the tests.
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
    main "$@"
fi
