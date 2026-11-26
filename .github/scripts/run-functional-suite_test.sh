#!/usr/bin/env bash
# Tests for run-functional-suite.sh, against a stub test command that writes
# the per-process spec logs the way the functional suite does. Exit status: 0
# all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/run-functional-suite.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

PASSED=0
FAILED=0
ok() {
    printf 'ok    %s\n' "$1"
    PASSED=$((PASSED + 1))
}
no() {
    printf 'FAIL  %s\n' "$1"
    shift
    for l in "$@"; do printf '      %s\n' "${l}"; done
    FAILED=$((FAILED + 1))
}
has() { # has <name> <text> <needle>
    if grep -qF -- "$3" <<<"$2"; then ok "$1"; else no "$1" "missing: $3" "in: $2"; fi
}
lacks() {
    if grep -qF -- "$3" <<<"$2"; then no "$1" "unexpected: $3" "in: $2"; else ok "$1"; fi
}

# Writes spec logs into $TEST_OUTDIR as the suite does, then exits with $STUB_RC.
cat >"${TMP}/stub" <<'STUB'
#!/usr/bin/env bash
mkdir -p "${TEST_OUTDIR}/proc-1" "${TEST_OUTDIR}/proc-2"
echo "detail only for the log"
printf '%s\n' '{"msg":"starting test","test":"a"}' \
    '{"msg":"finished test","test":"a ok","status":"passed"}' \
    '{"msg":"finished test","test":"filtered out","status":"skipped"}' >>"${TEST_OUTDIR}/proc-1/test.log"
sleep 1.5
printf '%s\n' '{"msg":"finished test","test":"b broke","status":"failed"}' >>"${TEST_OUTDIR}/proc-2/test.log"
echo "Summarizing 1 Failure"
exit "${STUB_RC}"
STUB
chmod +x "${TMP}/stub"

export RESULTS_DIR="${TMP}/results"

out="$(STUB_RC=0 "${SCRIPT}" good "${TMP}/stub")"
rc=$?
[ "${rc}" = 0 ] && ok "exit status 0 passes through" || no "exit status 0 passes through" "got ${rc}"
has "prints a passed spec" "${out}" "PASSED  a ok"
has "prints a failed spec" "${out}" "FAILED  b broke"
lacks "hides skipped specs" "${out}" "filtered out"
has "counts by status" "${out}" "1 failed, 1 passed"
lacks "keeps detail off the console when it passes" "${out}" "detail only for the log"
grep -q "detail only for the log" "${RESULTS_DIR}/good/output.log" && ok "full output goes to the log" || no "full output goes to the log"

out="$(STUB_RC=3 "${SCRIPT}" bad "${TMP}/stub")"
rc=$?
[ "${rc}" = 3 ] && ok "failing exit status passes through" || no "failing exit status passes through" "got ${rc}"
has "shows the end of the log on failure" "${out}" "Summarizing 1 Failure"

out="$("${SCRIPT}" nospec bash -c 'echo "image build failed"; exit 1')"
rc=$?
[ "${rc}" = 1 ] && has "failure before any spec shows the log" "${out}" "image build failed" || no "failure before any spec" "exit ${rc}" "${out}"
has "says no specs ran" "${out}" "no specs ran"

# A Ginkgo report with one failed spec on process 3, which ran 10:00:10-10:00:20, and the
# NGF logs of processes 1 and 3. Only process 3's lines from 10:00:08 (2s before the
# spec) to 10:00:20 belong to the spec.
cat >"${TMP}/report.json" <<'JSON'
[{"SpecReports":[
 {"LeafNodeType":"SynchronizedAfterSuite","ParallelProcess":1,"State":"passed","ReportEntries":[
  {"Name":"NGF Logs","Value":{"Representation":"{\"ts\":\"2026-01-01T10:00:12Z\",\"msg\":\"other process\"}"}}]},
 {"LeafNodeType":"SynchronizedAfterSuite","ParallelProcess":3,"State":"passed","ReportEntries":[
  {"Name":"NGF Logs","Value":{"Representation":"Logs for container nginx-gateway:\n  {\"ts\":\"2026-01-01T10:00:07Z\",\"msg\":\"too early\"}\n{\"ts\":\"2026-01-01T10:00:08Z\",\"msg\":\"at the margin\"}\n{\"ts\":\"2026-01-01T10:00:20Z\",\"msg\":\"at the end\"}\n{\"ts\":\"2026-01-01T10:00:21Z\",\"msg\":\"too late\"}"}}]},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Routes"],"LeafNodeText":"breaks","State":"failed","ParallelProcess":3,
  "StartTime":"2026-01-01T10:00:10.5Z","EndTime":"2026-01-01T10:00:20.9Z",
  "CapturedGinkgoWriterOutput":"the spec output\n::error::not a command",
  "Failure":{"Message":"Timed out after 5s.\nget failed: context deadline exceeded","Location":{"FileName":"/w/routes_test.go","LineNumber":42}},
  "ReportEntries":[{"Name":"Nginx Logs","Value":{"Representation":"nginx says hi"}},{"Name":"Test Events","Value":{"Representation":"an event"}}]},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Routes"],"LeafNodeText":"works","State":"passed","ParallelProcess":3,
  "StartTime":"2026-01-01T10:00:00Z","EndTime":"2026-01-01T10:00:05Z","CapturedGinkgoWriterOutput":"passing output"}
]}]
JSON
cat >"${TMP}/stub-report" <<'STUB'
#!/usr/bin/env bash
cp "${REPORT}" "${TEST_OUTDIR}/report.json"
echo "make: Error 1"
exit 1
STUB
chmod +x "${TMP}/stub-report"

out="$(REPORT="${TMP}/report.json" "${SCRIPT}" failing "${TMP}/stub-report")"
has "names the failed spec" "${out}" "FAILED  Routes breaks"
has "shows where it failed" "${out}" "  /w/routes_test.go:42"
has "shows the whole failure message" "${out}" "  get failed: context deadline exceeded"
has "shows the spec output" "${out}" "  the spec output"
# What the runner would read as workflow commands: everything outside ::stop-commands:: and its token.
live="$(awk '/^::stop-commands::/ {t = "::" substr($0, 18) "::"; skip = 1; next} skip && $0 == t {skip = 0; next} !skip' <<<"${out}")"
has "keeps the groups as workflow commands" "${live}" "::group::Test output"
lacks "stops workflow commands inside the spec output" "${live}" "::error::not a command"
has "keeps the spec output in the log" "${out}" "  ::error::not a command"
has "shows the NGF log from 2s before the spec" "${out}" "at the margin"
has "shows the NGF log to the end of the spec" "${out}" "at the end"
lacks "leaves out NGF log from before the window" "${out}" "too early"
lacks "leaves out NGF log from after the window" "${out}" "too late"
lacks "leaves out the NGF log of other processes" "${out}" "other process"
has "shows the nginx logs" "${out}" "  nginx says hi"
has "shows the events" "${out}" "  an event"
lacks "leaves out passing specs" "${out}" "passing output"
lacks "does not tail the log when it has the failures" "${out}" "make: Error 1"

# A report with nothing failed still leaves the log as the only explanation.
echo '[{"SpecReports":[]}]' >"${TMP}/empty.json"
out="$(REPORT="${TMP}/empty.json" "${SCRIPT}" nofail "${TMP}/stub-report")"
has "falls back to the log when the report has no failed spec" "${out}" "make: Error 1"

"${SCRIPT}" >/dev/null 2>&1
[ $? = 2 ] && ok "no arguments is a usage error" || no "no arguments is a usage error"
(
    unset RESULTS_DIR
    "${SCRIPT}" x true >/dev/null 2>&1
)
[ $? = 2 ] && ok "RESULTS_DIR is required" || no "RESULTS_DIR is required"

echo "${PASSED} passed, ${FAILED} failed"
[ "${FAILED}" = 0 ]
