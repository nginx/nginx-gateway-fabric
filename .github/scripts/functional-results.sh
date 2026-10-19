#!/usr/bin/env bash
# functional-results.sh
#
# Lists the functional test legs of a release prep run and how each ended, from
# the run's latest attempt: a leg that failed and passed on "Re-run failed
# jobs" counts as passed. Promote gates on this, and prep's summary reports it.
#
# Reads from the environment:
#   REPO     owner/name the prep run is in. Required.
#   RUN_ID   the prep run. Required.
#   GH       gh binary. Default: gh. Its token must read the run's jobs.
#
# Prints one line per leg, tab-separated: <state> <job name>, where state is
# passed, failed (any finished conclusion other than success, including
# cancelled and skipped) or running. Sorted by name.
#
# Exit status: 0 listed, 1 no legs found or the API call failed,
#              2 bad invocation.

set -euo pipefail

# The `name:` of the functional-tests job in release-prep.yml. GitHub reports
# each leg as "<name> (<matrix values>) / <job in functional.yml>".
# functional-results_test.sh fails if the two drift apart.
FUNCTIONAL_JOB_PREFIX="Functional tests"

REPO="${REPO:-}"
RUN_ID="${RUN_ID:-}"
GH="${GH:-gh}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${REPO}" ] || die "REPO is required"
[ -n "${RUN_ID}" ] || die "RUN_ID is required"
case "${RUN_ID}" in
*[!0-9]*) die "RUN_ID must be a number, got '${RUN_ID}'" ;;
esac

# filter=latest is the API default, named here because the answer depends on it.
jobs="$("${GH}" api --paginate "repos/${REPO}/actions/runs/${RUN_ID}/jobs?filter=latest&per_page=100" \
    --jq '.jobs[] | [.name, .status, (.conclusion // "")] | @tsv')" || {
    echo "error: could not list the jobs of run ${RUN_ID} in ${REPO}" >&2
    exit 1
}

legs="$(printf '%s\n' "${jobs}" | awk -F'\t' -v p="${FUNCTIONAL_JOB_PREFIX}" '
    index($1, p " (") == 1 || $1 == p {
        if ($2 != "completed") state = "running"
        else if ($3 == "success") state = "passed"
        else state = "failed"
        print state "\t" $1
    }' | sort -t$'\t' -k2)"

# None at all means the job was renamed or never ran; either way nothing here
# says the release was tested, so it is never read as "all passed".
if [ -z "${legs}" ]; then
    echo "error: run ${RUN_ID} has no jobs named '${FUNCTIONAL_JOB_PREFIX} (...)'" >&2
    exit 1
fi
printf '%s\n' "${legs}"
