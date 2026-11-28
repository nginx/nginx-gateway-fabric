#!/usr/bin/env bash
# Tests for ginkgo-summary.sh against Ginkgo JSON report fixtures. Exit status:
# 0 all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/ginkgo-summary.sh"
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
has() {
    if grep -qF -- "$3" <<<"$2"; then ok "$1"; else no "$1" "missing: $3" "in: $2"; fi
}

mkdir -p "${TMP}/r/functional" "${TMP}/r/telemetry" "${TMP}/r/waf"
cat >"${TMP}/r/functional/report.json" <<'JSON'
[{"SuiteSucceeded":false,"SpecReports":[
 {"LeafNodeType":"SynchronizedBeforeSuite","LeafNodeText":"","State":"passed","NumAttempts":1},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Policy"],"LeafNodeText":"works","State":"passed","NumAttempts":1},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Policy"],"LeafNodeText":"is flaky","State":"passed","NumAttempts":2},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Other"],"LeafNodeText":"not selected","State":"skipped","NumAttempts":0},
 {"LeafNodeType":"It","ContainerHierarchyTexts":["Routes"],"LeafNodeText":"a|b breaks","State":"failed","NumAttempts":1,
  "Failure":{"Message":"Expected 200 | got 500\nsecond line","Location":{"FileName":"/work/repo/tests/suite/routes_test.go","LineNumber":42}}}
]}]
JSON
echo '[{"SuiteSucceeded":true,"SpecReports":[]}]' >"${TMP}/r/telemetry/report.json"
: >"${TMP}/r/waf/output.log"

# CI sets GITHUB_STEP_SUMMARY, which would take the markdown away from stdout.
out="$(env -u GITHUB_STEP_SUMMARY GITHUB_WORKSPACE=/work/repo "${SCRIPT}" "${TMP}/r" "Functional tests (nginx)")"
rc=$?
[ "${rc}" = 0 ] && ok "exits 0" || no "exits 0" "got ${rc}"
has "titles the summary" "${out}" "### Functional tests (nginx)"
has "counts specs, not suite nodes" "${out}" "**functional**: 2 passed, 1 failed, 1 flaky, 1 skipped"
has "tables the failed spec with relative location and the whole message" "${out}" '| Routes a\|b breaks | `tests/suite/routes_test.go:42` | Expected 200 \| got 500 second line |'
has "lists the flaky spec" "${out}" "- Policy is flaky (2 attempts)"
has "reports a suite with no specs" "${out}" "**telemetry**: 0 passed, 0 failed"
has "reports a suite with no report" "${out}" "**waf**: no report"
has "annotates the failure on stdout" "${out}" "::error file=tests/suite/routes_test.go,line=42::Routes a|b breaks: Expected 200 | got 500 second line"

GITHUB_WORKSPACE=/work/repo GITHUB_STEP_SUMMARY="${TMP}/summary.md" "${SCRIPT}" "${TMP}/r" >"${TMP}/stdout"
has "summary file gets the markdown" "$(cat "${TMP}/summary.md")" "**functional**"
if grep -q '^###' "${TMP}/stdout"; then no "markdown stays out of stdout"; else ok "markdown stays out of stdout"; fi
has "annotations stay on stdout" "$(cat "${TMP}/stdout")" "::error "

"${SCRIPT}" >/dev/null 2>&1
[ $? = 2 ] && ok "no arguments is a usage error" || no "no arguments is a usage error"

echo "${PASSED} passed, ${FAILED} failed"
[ "${FAILED}" = 0 ]
