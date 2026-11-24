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
#   JOBS_PER_PAGE  jobs per page of the jobs listing, 1-100. Default: 20.
#            Pages of 100 on prep runs with ~76 jobs intermittently fail with
#            HTTP 502; smaller pages do not.
#   RETRY_SLEEP    seconds before the first retry of the jobs listing; each
#            later retry waits twice as long. Default: 5 (5s, 10s, 20s, 40s
#            over 5 attempts). The tests set it to 0.
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
JOBS_PER_PAGE="${JOBS_PER_PAGE:-20}"
RETRY_SLEEP="${RETRY_SLEEP:-5}"
JOBS_ATTEMPTS=5

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${REPO}" ] || die "REPO is required"
[ -n "${RUN_ID}" ] || die "RUN_ID is required"
case "${RUN_ID}" in
*[!0-9]*) die "RUN_ID must be a number, got '${RUN_ID}'" ;;
esac
case "${JOBS_PER_PAGE}" in
'' | *[!0-9]*) die "JOBS_PER_PAGE must be a number from 1 to 100, got '${JOBS_PER_PAGE}'" ;;
esac
[ "${JOBS_PER_PAGE}" -ge 1 ] && [ "${JOBS_PER_PAGE}" -le 100 ] ||
    die "JOBS_PER_PAGE must be a number from 1 to 100, got '${JOBS_PER_PAGE}'"
case "${RETRY_SLEEP}" in
'' | *[!0-9]*) die "RETRY_SLEEP must be a whole number of seconds, got '${RETRY_SLEEP}'" ;;
esac

# The jobs listing of a large run intermittently fails with HTTP 502, so it is
# retried with a growing wait. Each attempt starts over from the first page, and
# only a fully successful attempt is used. gh's stderr is held back and shown
# only for the last failed attempt: callers such as prep-summary.sh read stderr
# together with the leg list, so a retried 502 must not end up in it.
# filter=latest is the API default, named here because the answer depends on it.
gh_err="$(mktemp)"
trap 'rm -f "${gh_err}"' EXIT
list_jobs() {
    "${GH}" api --paginate "repos/${REPO}/actions/runs/${RUN_ID}/jobs?filter=latest&per_page=${JOBS_PER_PAGE}" \
        --jq '.jobs[] | [.name, .status, (.conclusion // "")] | @tsv' 2>"${gh_err}"
}
attempt=1
wait_s="${RETRY_SLEEP}"
until jobs="$(list_jobs)"; do
    if [ "${attempt}" -ge "${JOBS_ATTEMPTS}" ]; then
        cat "${gh_err}" >&2
        echo "error: could not list the jobs of run ${RUN_ID} in ${REPO}" >&2
        exit 1
    fi
    sleep "${wait_s}"
    attempt=$((attempt + 1))
    wait_s=$((wait_s * 2))
done

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
