#!/usr/bin/env bash
# run-functional-suite.sh
#
# Runs one functional test command with its full output sent to a log file, and
# prints one PASSED/FAILED line per spec to the console as each spec finishes.
#
# Usage: run-functional-suite.sh <name> <command> [args...]
#
# Reads from the environment:
#   RESULTS_DIR  directory the results are kept in. Required.
#
# The command runs with TEST_OUTDIR=$RESULTS_DIR/<name>. The test suite writes a
# JSON line per finished spec to $TEST_OUTDIR/proc-N/test.log (see
# tests/suite/system_suite_test.go), which is what this follows. The full output
# is $RESULTS_DIR/<name>/output.log. If the command fails, the end of that log
# is printed too: it holds Ginkgo's failure summary, and the only error when the
# command failed before any spec ran.
#
# Exit status: that of the command, 2 bad invocation.

set -uo pipefail

[ $# -ge 2 ] || {
    echo "usage: $0 <name> <command> [args...]" >&2
    exit 2
}
[ -n "${RESULTS_DIR:-}" ] || {
    echo "error: RESULTS_DIR is required" >&2
    exit 2
}

name="$1"
shift
out="${RESULTS_DIR}/${name}"
log="${out}/output.log"
mkdir -p "${out}"
export TEST_OUTDIR="${out}"

"$@" >"${log}" 2>&1 &
pid=$!

# Prints the lines added to the per-process logs, until the command has exited.
# Liveness is checked before reading, so the pass after the exit gets the rest.
follow() {
    local -A seen=()
    local alive f n
    while :; do
        alive=1
        kill -0 "${pid}" 2>/dev/null || alive=0
        for f in "${out}"/proc-*/test.log; do
            [ -f "${f}" ] || continue
            n="$(wc -l <"${f}")"
            if [ "${n}" -gt "${seen[${f}]:-0}" ]; then
                tail -n +"$((${seen[${f}]:-0} + 1))" "${f}" | head -n "$((n - ${seen[${f}]:-0}))"
                seen[${f}]="${n}"
            fi
        done
        [ "${alive}" = 1 ] || break
        sleep 1
    done
}

# Skipped specs are the ones the label filter left out; they are not news.
finished='fromjson? | select(.msg == "finished test" and .status != "skipped" and .status != "pending")'
echo "== ${name}: full output in ${log}"
follow | jq --unbuffered -Rr "${finished} | \"\\(.status | ascii_upcase)  \\(.test)\""

wait "${pid}"
rc=$?

counts="$(cat "${out}"/proc-*/test.log 2>/dev/null |
    jq -Rrn "[inputs | ${finished}] | group_by(.status) | map(\"\\(length) \\(.[0].status)\") | join(\", \")")"
echo "== ${name}: ${counts:-no specs ran}"
if [ "${rc}" -ne 0 ]; then
    echo "== ${name}: failed (exit ${rc}); last 100 lines of ${log}"
    tail -n 100 "${log}"
fi
exit "${rc}"
