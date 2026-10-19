#!/usr/bin/env bash
set -euo pipefail

# promote-release-branch.sh
#
# Fast-forwards the public release branch from the internal one, then
# dispatches the publish workflow. Refuses to promote unless prep's signed
# manifest was built from the exact commit being promoted.
#
# Usage:
#   promote-release-branch.sh --release-branch release-2.8 \
#                             --release-version v2.8.0 [options]
#
# Options:
#   --release-branch X   promote internal/X onto public X. Required.
#   --release-version V  the release tag, e.g. v2.8.0. Required to dispatch.
#   --public-repo R      Default: nginx/nginx-gateway-fabric
#   --mirror-repo R      where the prep run lives. Default: $GITHUB_REPOSITORY
#   --prep-run-id N      use this prep run instead of the newest manifest run.
#   --manifest-dir D     where to put the fetched manifest. Default: mktemp
#   --publish-workflow W workflow to dispatch after promoting.
#                        Default: release-publish.yml
#   --publish-mode M     publish's mode: publish (default) or verify, which
#                        checks everything and publishes nothing. The branch
#                        is promoted either way.
#   --no-dispatch        promote but do not dispatch.
#   --accept-failed-tests REASON
#                        promote even though functional legs failed, recording
#                        REASON in the run summary. Legs still running are
#                        never accepted.
#   --dry-run            report what would happen and change nothing.
#   -h, --help
#
# Exit status:
#   0  promoted, or already up to date
#   1  not a fast-forward, a branch is missing, the push did not land, the
#      prep manifest is missing, unsigned or for a different commit, or
#      functional legs failed without --accept-failed-tests
#   2  bad usage
#
# Requires: git, gh, jq. PUBLIC_REPO_TOKEN (the bot token, in the workflow)
# needs to push the public release branch, including past its protection
# rules and with the `workflow` scope, and to dispatch workflows there. MIRROR_TOKEN, if set, is used
# instead for calls that read this repository's own artifacts.

PUBLIC_REPO="${PUBLIC_REPO:-nginx/nginx-gateway-fabric}"
PUBLISH_WORKFLOW="${PUBLISH_WORKFLOW:-release-publish.yml}"
RELEASE_BRANCH=""
RELEASE_VERSION=""
MIRROR_REPO="${MIRROR_REPO:-${GITHUB_REPOSITORY:-}}"
MIRROR_TOKEN="${MIRROR_TOKEN:-}"
PREP_RUN_ID=""
MANIFEST_DIR=""
PUBLISH_MODE=publish
ACCEPT_FAILED_TESTS=""
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/null}"
DRY_RUN=false
DISPATCH=true

GIT="${GIT:-git}"
GH="${GH:-gh}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FUNCTIONAL_RESULTS="${FUNCTIONAL_RESULTS:-${SCRIPT_DIR}/functional-results.sh}"

die() {
    echo "error: $*" >&2
    exit 2
}

fail() {
    echo "error: $*" >&2
    exit 1
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
        --publish-mode)
            [ $# -ge 2 ] || die "--publish-mode needs a value"
            PUBLISH_MODE="$2"
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
        --accept-failed-tests)
            [ $# -ge 2 ] || die "--accept-failed-tests needs a reason"
            ACCEPT_FAILED_TESTS="$2"
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
    case "${PUBLISH_MODE}" in
    publish | verify) ;;
    *) die "--publish-mode must be publish or verify, got '${PUBLISH_MODE}'" ;;
    esac
    if [ -n "${RELEASE_VERSION}" ]; then
        echo "${RELEASE_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
            die "--release-version must look like vX.Y.Z, got '${RELEASE_VERSION}'"
    fi
    if [ "${DISPATCH}" = "true" ]; then
        [ -n "${RELEASE_VERSION}" ] || die "--release-version is required to dispatch ${PUBLISH_WORKFLOW} (or pass --no-dispatch)"
        [ -n "${MIRROR_REPO}" ] || die "--mirror-repo is required to find the prep run (or set GITHUB_REPOSITORY)"
    fi
}

# GH_TOKEN is the public repository's; the prep run's artifacts need this
# repository's own token instead.
mirror_gh() {
    if [ -n "${MIRROR_TOKEN}" ]; then
        GH_TOKEN="${MIRROR_TOKEN}" "${GH}" "$@"
    else
        "${GH}" "$@"
    fi
}

# The newest unexpired artifact named for the version: newest because a
# re-run of prep supersedes the earlier one, unexpired because it must download.
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

# Must match this version, this branch and, more importantly, this commit:
# otherwise prep tested something other than what is about to ship. Publish
# reads the release branch from the manifest, so it must be this one.
check_manifest() {
    local dir="$1" version="$2" internal_sha="$3" got
    got="$(jq -r '.release_version // empty' "${dir}/release-manifest.json")"
    [ "${got}" = "${version}" ] ||
        fail "the manifest is for '${got}' but ${version} is being promoted"
    got="$(jq -r '.source.internal_branch // empty' "${dir}/release-manifest.json")"
    [ "${got}" = "internal/${RELEASE_BRANCH}" ] ||
        fail "the manifest was built on '${got}' but internal/${RELEASE_BRANCH} is being promoted"
    got="$(jq -r '.source.internal_sha // empty' "${dir}/release-manifest.json")"
    [ "${got}" = "${internal_sha}" ] ||
        fail "the manifest was built from '${got}' but the branch being promoted is at ${internal_sha}. Prep did not build this commit; re-run prep before promoting."
}

summary() {
    printf '%s\n' "$@" >>"${SUMMARY}"
}

# Functional legs no longer gate the manifest, so a manifest can exist for a
# release whose functional tests failed. This is where that is caught: every
# leg must have passed on the run's latest attempt, or the failures must be
# accepted with a reason, which goes in the run summary.
check_functional() {
    local run_id="$1" results failed running
    results="$(GH="${GH}" GH_TOKEN="${MIRROR_TOKEN:-${GH_TOKEN:-}}" REPO="${MIRROR_REPO}" RUN_ID="${run_id}" \
        "${FUNCTIONAL_RESULTS}")" ||
        fail "could not read the functional test results of prep run ${run_id}"
    failed="$(awk -F'\t' '$1 == "failed" { print "  " $2 }' <<<"${results}")"
    running="$(awk -F'\t' '$1 == "running" { print "  " $2 }' <<<"${results}")"

    summary "### Functional tests (prep run ${run_id})" ""
    if [ -n "${running}" ]; then
        summary "Still running:" '```' "${running}" '```'
        fail "functional legs of prep run ${run_id} are still running; wait for them:
${running}"
    fi
    if [ -z "${failed}" ]; then
        echo "functional tests: all $(grep -c . <<<"${results}") legs passed"
        summary "All $(grep -c . <<<"${results}") legs passed."
        return 0
    fi
    summary "Failed:" '```' "${failed}" '```'
    if [ -z "${ACCEPT_FAILED_TESTS}" ]; then
        summary "Not accepted: promote refused."
        fail "functional legs of prep run ${run_id} failed:
${failed}
Re-run the failed jobs in that run, or promote with accept_failed_tests set to the reason."
    fi
    echo "functional tests failed and were accepted: ${ACCEPT_FAILED_TESTS}"
    echo "${failed}"
    summary "**Accepted** by @${GITHUB_ACTOR:-unknown}: ${ACCEPT_FAILED_TESTS}"
}

# The version, the mode, and the sha256 of the manifest checked here. Publish
# fetches the manifest stored under that sha256 and refuses other bytes, so it
# publishes exactly what promote checked, never a later prep run's manifest.
dispatch_publish() {
    local version="$1" sha="$2"
    "${GH}" workflow run "${PUBLISH_WORKFLOW}" \
        --repo "${PUBLIC_REPO}" \
        --ref "${RELEASE_BRANCH}" \
        -f "release_version=${version}" \
        -f "manifest_sha256=${sha}" \
        -f "mode=${PUBLISH_MODE}"
}

# Classifies without performing: up-to-date, fast-forward, or diverged (the
# public commit is not an ancestor, so promoting would discard it).
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
    summary "## Promote ${RELEASE_VERSION:-${RELEASE_BRANCH}}" "" \
        "| | |" "| --- | --- |" \
        "| internal \`${internal_branch}\` | \`${internal_sha}\` |" \
        "| public \`${RELEASE_BRANCH}\` before | \`${public_sha}\` |" ""

    # Fetched directly rather than trusting the mirror sync, which can be behind.
    # A failed fetch is reported as one: otherwise the ancestry check below
    # cannot find the public commit and calls the branches diverged.
    "${GIT}" fetch --quiet "https://github.com/${PUBLIC_REPO}.git" \
        "refs/heads/${RELEASE_BRANCH}" ||
        fail "could not fetch ${RELEASE_BRANCH} from ${PUBLIC_REPO}"

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

    # Checked before anything is written: if prep did not build this commit,
    # there is nothing to publish.
    local manifest_dir="" run_id="" manifest_sha=""
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
        manifest_sha="$(sha256sum "${manifest_dir}/release-manifest.json" | cut -d' ' -f1)"
        echo "manifest for ${RELEASE_VERSION} built from ${internal_sha}, signed, sha256 ${manifest_sha}"
        summary "Manifest sha256: \`${manifest_sha}\` (prep run ${run_id})" ""
        check_functional "${run_id}"
    fi

    if [ "${DRY_RUN}" = "true" ]; then
        echo "dry run: nothing pushed, nothing dispatched"
        summary "### Checked, nothing changed" "" \
            "- Promotion: ${kind}." \
            "- Not checked here: the manifest's signature (publish verifies it) and the push to \`${RELEASE_BRANCH}\`." \
            "" '**Next:** run promote again with `mode: release` once the release may be public.'
        return 0
    fi

    if [ "${kind}" = "fast-forward" ]; then
        [ -n "${PUBLIC_REPO_TOKEN:-}" ] || fail "PUBLIC_REPO_TOKEN is not set"
        # No --force: git's own refusal of a non-fast-forward is the backstop.
        "${GIT}" push "https://x-access-token:${PUBLIC_REPO_TOKEN}@github.com/${PUBLIC_REPO}.git" \
            "${internal_sha}:refs/heads/${RELEASE_BRANCH}" || {
            summary "### The push to \`${RELEASE_BRANCH}\` was refused" "" \
                "Nothing is public yet. If a ruleset blocked it, push the commit yourself and run promote again with the same inputs; it will find the branch up to date and dispatch publish:" \
                '```' "git push git@github.com:${PUBLIC_REPO}.git ${internal_sha}:refs/heads/${RELEASE_BRANCH}" '```'
            fail "the push to ${RELEASE_BRANCH} was refused. Nothing is public. If a ruleset blocked it, push ${internal_sha} to ${RELEASE_BRANCH} yourself (no --force) and run promote again with the same inputs."
        }

        # Confirm from the other side rather than trusting the exit code.
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

    dispatch_publish "${RELEASE_VERSION}" "${manifest_sha}"
    echo "dispatched ${PUBLISH_WORKFLOW} on ${RELEASE_BRANCH} for ${RELEASE_VERSION} (mode: ${PUBLISH_MODE})"
    summary "### Promoted" "" \
        "- \`${RELEASE_BRANCH}\` in ${PUBLIC_REPO} is at \`${internal_sha}\`." \
        "- Dispatched **Release Publish** for ${RELEASE_VERSION} in \`${PUBLISH_MODE}\` mode, with \`manifest_sha256: ${manifest_sha}\`." \
        "" "**Next:** watch it at https://github.com/${PUBLIC_REPO}/actions/workflows/${PUBLISH_WORKFLOW}. If it did not start, run promote again with the same inputs."
}

# Only run the driver when executed directly, not when sourced by the tests.
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
    main "$@"
fi
