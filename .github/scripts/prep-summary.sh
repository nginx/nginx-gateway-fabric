#!/usr/bin/env bash
# prep-summary.sh
#
# Prints release prep's run summary as Markdown: what was built, how the
# functional legs ended, how long failed jobs can still be re-run, and the
# promote inputs to use next. Reads only; changes nothing.
#
# Reads from the environment:
#   REPO              owner/name of this repository. Required.
#   RUN_ID            this prep run. Required.
#   RELEASE_VERSION   e.g. v2.9.0. Required.
#   OPERATOR_VERSION  e.g. v1.7.0
#   SHA               the commit prep built
#   MANIFEST_RESULT   the manifest job's result: success, failure, skipped, ...
#   MANIFEST_SHA256   the stored manifest's sha256, when it succeeded
#   GH                gh binary. Default: gh
#
# Exit status: 0 printed, 2 bad invocation. A failed lookup is reported in the
# summary rather than failing the job.

set -euo pipefail

REPO="${REPO:-}"
RUN_ID="${RUN_ID:-}"
RELEASE_VERSION="${RELEASE_VERSION:-}"
OPERATOR_VERSION="${OPERATOR_VERSION:-}"
SHA="${SHA:-}"
MANIFEST_RESULT="${MANIFEST_RESULT:-}"
MANIFEST_SHA256="${MANIFEST_SHA256:-}"
GH="${GH:-gh}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FUNCTIONAL_RESULTS="${FUNCTIONAL_RESULTS:-${SCRIPT_DIR}/functional-results.sh}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${REPO}" ] || die "REPO is required"
[ -n "${RUN_ID}" ] || die "RUN_ID is required"
[ -n "${RELEASE_VERSION}" ] || die "RELEASE_VERSION is required"

echo "## Prep ${RELEASE_VERSION}"
echo
echo "| | |"
echo "| --- | --- |"
echo "| Run | \`${RUN_ID}\` |"
echo "| Commit | \`${SHA:-unknown}\` |"
echo "| Operator | \`${OPERATOR_VERSION:-unknown}\` |"
[ -n "${MANIFEST_SHA256}" ] && echo "| Manifest sha256 | \`${MANIFEST_SHA256}\` |"
echo

# The binaries and digest records are what manifest downloads; the first to
# expire is the deadline for re-running failed jobs.
deadline="$("${GH}" api "repos/${REPO}/actions/runs/${RUN_ID}/artifacts?per_page=100" \
    --jq '[.artifacts[] | select((.name | startswith("dist-")) or (.name | startswith("digest-"))) | .expires_at] | min // empty' \
    2>/dev/null || true)"

echo "### Functional tests"
echo
failed=""
if results="$(GH="${GH}" REPO="${REPO}" RUN_ID="${RUN_ID}" "${FUNCTIONAL_RESULTS}" 2>&1)"; then
    total="$(grep -c . <<<"${results}")"
    failed="$(awk -F'\t' '$1 != "passed" { print "- " $1 ": " $2 }' <<<"${results}")"
    if [ -z "${failed}" ]; then
        echo "All ${total} legs passed."
    else
        echo "Not all ${total} legs passed:"
        echo
        echo "${failed}"
    fi
else
    echo "Could not read the results: ${results}"
    failed="unknown"
fi
echo

echo "### Next"
echo
case "${MANIFEST_RESULT}" in
success)
    echo "The signed manifest is stored and the \`release-manifest-${RELEASE_VERSION}\` artifact is attached to this run."
    echo
    if [ -n "${failed}" ]; then
        echo "Functional legs did not all pass, so promote will refuse this run until they do. Either:"
        echo
        echo "- use **Re-run failed jobs** on this run${deadline:+ before \`${deadline}\`}, or"
        echo '- promote with `accept_failed_tests` set to the reason, if the failures are known flakes.'
        echo
    fi
    echo "Check first, any time before release day: **Promote Release Branch** from \`${GITHUB_REF_NAME:-internal/release-X.Y}\` with"
    echo
    echo "| Input | Value |"
    echo "| --- | --- |"
    echo "| \`release_version\` | \`${RELEASE_VERSION}\` |"
    echo '| `mode` | `check` |'
    echo "| \`prep_run_id\` | \`${RUN_ID}\` |"
    echo
    echo 'On release day, the same with `mode: release`.'
    echo
    echo "Pushing anything to the internal release branch now means running prep again: promote refuses a manifest for another commit."
    ;;
failure | cancelled)
    echo "No manifest. Nothing has been published."
    echo
    if [ -n "${deadline}" ]; then
        echo "If the failures were flaky, use **Re-run failed jobs** on this run before \`${deadline}\`; after that the binaries are gone and prep must run again. Do not use **Re-run all jobs**, which rebuilds and re-pushes every image."
    else
        echo "The binaries this run built have expired or were never uploaded: run prep again."
    fi
    echo
    echo "If something is broken, fix it on the internal release branch and run prep again."
    ;;
*)
    echo "No manifest: the manifest job did not run (\`${MANIFEST_RESULT:-unknown}\`), because a build, the staged digests, conformance or Helm failed. Nothing has been published."
    echo
    if [ -n "${deadline}" ]; then
        echo "If they were flaky, use **Re-run failed jobs** before \`${deadline}\`; otherwise fix the cause and run prep again."
    else
        echo "Fix the cause and run prep again."
    fi
    ;;
esac
