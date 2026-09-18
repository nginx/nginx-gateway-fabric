#!/usr/bin/env bash
set -euo pipefail

# promote-release-branch.sh
#
# Fast-forwards the public release branch from the internal one, then
# dispatches the publish workflow.
#
# This is the release-day transfer, and the only thing in the design that
# writes from the mirror to the public repository. Two properties matter more
# than anything else here:
#
#   1. It is a fast-forward, never a force. Git enforces that on its own as
#      long as --force is absent, but the precondition is asserted first so
#      the failure says "the public branch has commits yours does not" rather
#      than surfacing as a rejected push.
#   2. It never dispatches publish unless the push actually landed. Publishing
#      from a branch that was not promoted would build a release out of the
#      wrong source.
#
# The credential lives in the mirror and points at the public repository,
# never the reverse: a token in the public repository able to read the mirror
# would expose work that is not public yet.
#
# Usage:
#   promote-release-branch.sh --release-branch release-2.8 [options]
#
# Options:
#   --release-branch X   promote internal/X onto public X. Required.
#   --public-repo R      Default: nginx/nginx-gateway-fabric
#   --publish-workflow W workflow to dispatch after promoting.
#                        Default: release-publish.yml
#   --no-dispatch        promote but do not dispatch.
#   --dry-run            report what would happen and change nothing.
#   -h, --help
#
# Exit status:
#   0  promoted, or already up to date
#   1  not a fast-forward, a branch is missing, or the push did not land
#   2  bad usage
#
# Requires: git, gh. PUBLIC_REPO_TOKEN must carry contents:write and
# actions:write on the public repository.

PUBLIC_REPO="${PUBLIC_REPO:-nginx/nginx-gateway-fabric}"
PUBLISH_WORKFLOW="${PUBLISH_WORKFLOW:-release-publish.yml}"
RELEASE_BRANCH=""
DRY_RUN=false
DISPATCH=true

GIT="${GIT:-git}"
GH="${GH:-gh}"

die() {
    echo "error: $*" >&2
    exit 2
}

fail() {
    echo "error: $*" >&2
    exit 1
}

usage() {
    sed -n '4,44p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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
        --publish-workflow)
            [ $# -ge 2 ] || die "--publish-workflow needs a value"
            PUBLISH_WORKFLOW="$2"
            shift 2
            ;;
        --no-dispatch)
            DISPATCH=false
            shift
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
}

# Classifies the promotion without performing it.
#
#   up-to-date  the public branch is already at the internal commit
#   fast-forward the public commit is an ancestor of the internal one
#   diverged    it is not, so promoting would discard public commits
#
# Separated out because it is the whole safety argument, and the only part
# that can be tested without a second repository to push to.
promotion_kind() {
    local internal_sha="$1" public_sha="$2"

    if [ "${internal_sha}" = "${public_sha}" ]; then
        echo "up-to-date"
        return 0
    fi

    if "${GIT}" merge-base --is-ancestor "${public_sha}" "${internal_sha}"; then
        echo "fast-forward"
        return 0
    fi

    echo "diverged"
}

main() {
    parse_args "$@"

    local internal_branch="internal/${RELEASE_BRANCH}"

    local internal_sha
    internal_sha="$("${GIT}" rev-parse --verify --quiet "origin/${internal_branch}" || true)"
    [ -n "${internal_sha}" ] || fail "${internal_branch} does not exist in this repository"

    local public_sha
    public_sha="$("${GH}" api "repos/${PUBLIC_REPO}/commits/${RELEASE_BRANCH}" --jq .sha 2>/dev/null || true)"
    [ -n "${public_sha}" ] || fail "${RELEASE_BRANCH} does not exist in ${PUBLIC_REPO}"

    echo "internal ${internal_branch} ${internal_sha}"
    echo "public   ${RELEASE_BRANCH} ${public_sha}"

    # The public commit has to be reachable here to judge ancestry. It is,
    # because the sync copies the public branch in, but fetching the public
    # repository directly keeps this honest if the sync is behind.
    "${GIT}" fetch --quiet "https://github.com/${PUBLIC_REPO}.git" \
        "refs/heads/${RELEASE_BRANCH}" || true

    local kind
    kind="$(promotion_kind "${internal_sha}" "${public_sha}")"
    echo "promotion: ${kind}"

    case "${kind}" in
    up-to-date)
        echo "${RELEASE_BRANCH} is already at ${internal_sha}; nothing to promote"
        ;;
    diverged)
        fail "${RELEASE_BRANCH} has commits ${internal_branch} does not; promoting would discard them. Merge back first."
        ;;
    fast-forward) ;;
    *)
        fail "could not classify the promotion"
        ;;
    esac

    if [ "${DRY_RUN}" = "true" ]; then
        echo "dry run: nothing pushed, nothing dispatched"
        return 0
    fi

    if [ "${kind}" = "fast-forward" ]; then
        [ -n "${PUBLIC_REPO_TOKEN:-}" ] || fail "PUBLIC_REPO_TOKEN is not set"
        # No --force, deliberately: git refuses a non-fast-forward, which is
        # the backstop behind the check above.
        "${GIT}" push "https://x-access-token:${PUBLIC_REPO_TOKEN}@github.com/${PUBLIC_REPO}.git" \
            "${internal_sha}:refs/heads/${RELEASE_BRANCH}"

        # Confirm from the other side rather than trusting the exit code. A
        # release published from an unpromoted branch is built from the wrong
        # source, and that is worth one extra API call to rule out.
        local landed
        landed="$("${GH}" api "repos/${PUBLIC_REPO}/commits/${RELEASE_BRANCH}" --jq .sha 2>/dev/null || true)"
        [ "${landed}" = "${internal_sha}" ] ||
            fail "the push did not land: ${RELEASE_BRANCH} is at '${landed}', expected ${internal_sha}"
        echo "promoted ${RELEASE_BRANCH} to ${internal_sha}"
    fi

    if [ "${DISPATCH}" != "true" ]; then
        echo "not dispatching ${PUBLISH_WORKFLOW}"
        return 0
    fi

    "${GH}" workflow run "${PUBLISH_WORKFLOW}" \
        --repo "${PUBLIC_REPO}" \
        --ref "${RELEASE_BRANCH}"
    echo "dispatched ${PUBLISH_WORKFLOW} on ${RELEASE_BRANCH}"
}

# Only run the driver when executed directly, not when sourced by the tests.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
