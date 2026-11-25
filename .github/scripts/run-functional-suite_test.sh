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

"${SCRIPT}" >/dev/null 2>&1
[ $? = 2 ] && ok "no arguments is a usage error" || no "no arguments is a usage error"
(
    unset RESULTS_DIR
    "${SCRIPT}" x true >/dev/null 2>&1
)
[ $? = 2 ] && ok "RESULTS_DIR is required" || no "RESULTS_DIR is required"

echo "${PASSED} passed, ${FAILED} failed"
[ "${FAILED}" = 0 ]
