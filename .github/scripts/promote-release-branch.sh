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
#   3. It hands publish the manifest prep signed, fetched from the prep run
#      in this repository, and only after checking that manifest was built
#      from the exact commit being promoted. A manifest for a different
#      commit means prep did not test what is about to ship; the branch is
#      not pushed either, so nothing moves until prep is re-run.
#
# The credential lives in the mirror and points at the public repository,
# never the reverse: a token in the public repository able to read the mirror
# would unlock embargoed fixes before disclosure.
#
# Usage:
#   promote-release-branch.sh --release-branch release-2.8 \
#                             --release-version v2.8.0 [options]
#
# Options:
#   --release-branch X   promote internal/X onto public X. Required.
#   --release-version V  the release tag, e.g. v2.8.0. Required to dispatch:
#                        it names the prep artifact and is passed to publish.
#   --public-repo R      Default: nginx/nginx-gateway-fabric
#   --mirror-repo R      where the prep run lives. Default: $GITHUB_REPOSITORY
#   --prep-run-id N      use this prep run rather than the newest one that
#                        produced a manifest for the version.
#   --manifest-dir D     where to put the fetched manifest. Default: mktemp
#   --publish-workflow W workflow to dispatch after promoting.
#                        Default: release-publish.yml
#   --publish-dry-run    dispatch publish in dry-run mode: it verifies and
#                        promotes nothing. The branch is still promoted.
#   --no-dispatch        promote but do not dispatch.
#   --dry-run            report what would happen and change nothing.
#   -h, --help
#
# Exit status:
#   0  promoted, or already up to date
#   1  not a fast-forward, a branch is missing, the push did not land, or
#      the prep manifest is missing, unsigned or for a different commit
#   2  bad usage
#
# Requires: git, gh, jq. PUBLIC_REPO_TOKEN must carry contents:write and
# actions:write on the public repository, and is what gh uses by default
# here. MIRROR_TOKEN, if set, is used instead for the calls that read this
# repository's own artifacts; in Actions that is github.token.

PUBLIC_REPO="${PUBLIC_REPO:-nginx/nginx-gateway-fabric}"
PUBLISH_WORKFLOW="${PUBLISH_WORKFLOW:-release-publish.yml}"
RELEASE_BRANCH=""
RELEASE_VERSION=""
MIRROR_REPO="${MIRROR_REPO:-${GITHUB_REPOSITORY:-}}"
MIRROR_TOKEN="${MIRROR_TOKEN:-}"
PREP_RUN_ID=""
MANIFEST_DIR=""
PUBLISH_DRY_RUN=false
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
        --release-version)
            [ $# -ge 2 ] || die "--release-version needs a value"
            RELEASE_VERSION="$2"
            shift 2
            ;;
        --public-repo)
            [ $# -ge 2 ] || die "--public-repo needs a value"
            PUBLIC_REPO="$2"
            shift 2
            ;;
        --mirror-repo)
            [ $# -ge 2 ] || die "--mirror-repo needs a value"
            MIRROR_REPO="$2"
            shift 2
            ;;
        --prep-run-id)
            [ $# -ge 2 ] || die "--prep-run-id needs a value"
            PREP_RUN_ID="$2"
            shift 2
            ;;
        --manifest-dir)
            [ $# -ge 2 ] || die "--manifest-dir needs a value"
            MANIFEST_DIR="$2"
            shift 2
            ;;
        --publish-dry-run)
            PUBLISH_DRY_RUN=true
            shift
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
    if [ -n "${RELEASE_VERSION}" ]; then
        echo "${RELEASE_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+' ||
            die "--release-version must look like vX.Y.Z, got '${RELEASE_VERSION}'"
    fi
    if [ "${DISPATCH}" = "true" ]; then
        [ -n "${RELEASE_VERSION}" ] || die "--release-version is required to dispatch ${PUBLISH_WORKFLOW} (or pass --no-dispatch)"
        [ -n "${MIRROR_REPO}" ] || die "--mirror-repo is required to find the prep run (or set GITHUB_REPOSITORY)"
    fi
}

# gh reaches two repositories with two credentials. GH_TOKEN is the public
# repository's; the prep run's artifacts are in this repository and need its
# own token, which Actions provides as github.token.
mirror_gh() {
    if [ -n "${MIRROR_TOKEN}" ]; then
        GH_TOKEN="${MIRROR_TOKEN}" "${GH}" "$@"
    else
        "${GH}" "$@"
    fi
}

# The newest unexpired artifact named for the version, and the run that made
# it. Newest because a re-run of prep for the same version supersedes the
# earlier one; unexpired because an expired artifact cannot be downloaded
# and would fail later with a worse message.
find_prep_run() {
    local version="$1"
    mirror_gh api "repos/${MIRROR_REPO}/actions/artifacts?name=release-manifest-${version}&per_page=100" \
        --jq '[.artifacts[] | select(.expired == false)] | sort_by(.created_at) | last | .workflow_run.id // empty'
}

fetch_manifest() {
    local run_id="$1" version="$2" dir="$3"
    mirror_gh run download "${run_id}" --repo "${MIRROR_REPO}" \
        --name "release-manifest-${version}" --dir "${dir}" ||
        fail "could not download release-manifest-${version} from run ${run_id} in ${MIRROR_REPO}"
    [ -f "${dir}/release-manifest.json" ] ||
        fail "run ${run_id} produced no release-manifest.json"
    [ -f "${dir}/release-manifest.sigstore.json" ] ||
        fail "run ${run_id} produced no signature bundle; an unsigned manifest cannot be published"
}

# The manifest must be for this version and, more importantly, for this
# commit. Prep records the SHA it built; if that is not the SHA about to be
# fast-forwarded onto the public branch, prep tested something other than
# what would ship.
check_manifest() {
    local dir="$1" version="$2" internal_sha="$3" got
    got="$(jq -r '.release_version // empty' "${dir}/release-manifest.json")"
    [ "${got}" = "${version}" ] ||
        fail "the manifest is for '${got}' but ${version} is being promoted"
    got="$(jq -r '.source.internal_sha // empty' "${dir}/release-manifest.json")"
    [ "${got}" = "${internal_sha}" ] ||
        fail "the manifest was built from '${got}' but the branch being promoted is at ${internal_sha}. Prep did not build this commit; re-run prep before promoting."
}

dispatch_publish() {
    local dir="$1" version="$2"
    "${GH}" workflow run "${PUBLISH_WORKFLOW}" \
        --repo "${PUBLIC_REPO}" \
        --ref "${RELEASE_BRANCH}" \
        -f "release_version=${version}" \
        -f "release_branch=${RELEASE_BRANCH}" \
        -f "manifest=$(cat "${dir}/release-manifest.json")" \
        -f "manifest_bundle=$(cat "${dir}/release-manifest.sigstore.json")" \
        -f "dry_run=${PUBLISH_DRY_RUN}"
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

    # The manifest is checked before anything is written. If prep did not
    # build the commit being promoted there is nothing to publish, and
    # promoting the branch anyway would leave it moved with no release to
    # follow.
    local manifest_dir="" run_id=""
    if [ "${DISPATCH}" = "true" ]; then
        manifest_dir="${MANIFEST_DIR:-$(mktemp -d)}"
        run_id="${PREP_RUN_ID}"
        if [ -z "${run_id}" ]; then
            run_id="$(find_prep_run "${RELEASE_VERSION}")"
            [ -n "${run_id}" ] ||
                fail "no unexpired release-manifest-${RELEASE_VERSION} artifact in ${MIRROR_REPO}; has prep run for ${RELEASE_VERSION}?"
        fi
        echo "prep run ${run_id}"
        fetch_manifest "${run_id}" "${RELEASE_VERSION}" "${manifest_dir}"
        check_manifest "${manifest_dir}" "${RELEASE_VERSION}" "${internal_sha}"
        echo "manifest for ${RELEASE_VERSION} built from ${internal_sha}, signed"
    fi

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

    dispatch_publish "${manifest_dir}" "${RELEASE_VERSION}"
    echo "dispatched ${PUBLISH_WORKFLOW} on ${RELEASE_BRANCH} for ${RELEASE_VERSION} (publish dry run: ${PUBLISH_DRY_RUN})"
}

# Only run the driver when executed directly, not when sourced by the tests.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
