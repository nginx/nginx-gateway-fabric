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
# is $RESULTS_DIR/<name>/output.log. If the command fails, each failed spec is
# printed from $RESULTS_DIR/<name>/report.json: the failure, its own output, the
# nginx logs and events, and the NGF controller log lines from the time it ran.
# Without a report (or without a failed spec in it) the end of output.log is
# printed instead: it is the only error when the command failed before any spec ran.
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

# One block per failed spec. The NGF log is one per Ginkgo process for the whole
# run (the "NGF Logs" entry of that process's SynchronizedAfterSuite), so it is cut
# to the lines the spec was running for. Its timestamps are whole seconds, hence
# the 2 seconds of margin before the spec. Every line is indented so that nothing in
# the logs can be read as a workflow command.
failures='
def fullname: ((.ContainerHierarchyTexts // []) + [(.LeafNodeText | select(. != "")) // .LeafNodeType]) | join(" ");
def failed: select(.State | test("^(failed|panicked|timedout|interrupted|aborted)$"));
def secs: sub("\\.[0-9]+"; "") | fromdateiso8601;
def entry($n): [.ReportEntries[]? | select(.Name == $n) | .Value.Representation] | join("\n");
def group($title; $body): if $body == "" then empty else "::group::\($title)", ($body | split("\n") | map("  " + .) | join("\n")), "::endgroup::" end;
[.[].SpecReports[]] as $all
| ($all | map(select(.LeafNodeType == "SynchronizedAfterSuite") | {key: (.ParallelProcess | tostring), value: entry("NGF Logs")}) | from_entries) as $ngf
| $all[] | failed | . as $s
| (($s.StartTime | secs) - 2) as $from
| ($s.EndTime | secs) as $to
| ($ngf[$s.ParallelProcess | tostring] // "" | split("\n")
    | map(select((capture("\"ts\":\"(?<t>[^\"]+)\"").t | secs)? as $t | $t >= $from and $t <= $to))) as $ngflines
| "",
  "FAILED  \($s | fullname)",
  "  \($s.Failure.Location.FileName // ""):\($s.Failure.Location.LineNumber // "")",
  ($s.Failure.Message // "" | split("\n") | map("  " + .) | join("\n")),
  group("Test output"; $s.CapturedGinkgoWriterOutput // ""),
  group("NGF logs (process \($s.ParallelProcess), \($ngflines | length) lines, from 2s before the spec)"; $ngflines | join("\n")),
  group("Nginx logs"; $s | entry("Nginx Logs")),
  group("Events"; $s | entry("Test Events"))
'

counts="$(cat "${out}"/proc-*/test.log 2>/dev/null |
    jq -Rrn "[inputs | ${finished}] | group_by(.status) | map(\"\\(length) \\(.[0].status)\") | join(\", \")")"
echo "== ${name}: ${counts:-no specs ran}"
if [ "${rc}" -ne 0 ]; then
    shown="$(jq -r "${failures}" "${out}/report.json" 2>/dev/null)"
    if [ -n "${shown}" ]; then
        echo "== ${name}: failed (exit ${rc})"
        echo "${shown}"
    else
        echo "== ${name}: failed (exit ${rc}); last 100 lines of ${log}"
        tail -n 100 "${log}"
    fi
fi
exit "${rc}"
