#!/usr/bin/env bash
# Tests for prep-summary.sh, with gh and the results script stubbed. Exit
# status: 0 all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/prep-summary.sh"
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

# gh answers the artifacts query through its --jq filter.
cat >"${TMP}/gh" <<'STUB'
#!/usr/bin/env bash
filter=""
while [ $# -gt 0 ]; do [ "$1" = "--jq" ] && filter="$2"; shift; done
jq -r "${filter}" <"${ARTIFACTS}"
STUB
chmod +x "${TMP}/gh"
cat >"${TMP}/artifacts.json" <<'EOF'
{"artifacts":[
 {"name":"dist-42","expires_at":"2026-10-21T09:00:00Z"},
 {"name":"digest-nginx-42","expires_at":"2026-10-21T09:30:00Z"},
 {"name":"release-manifest-v2.9.0","expires_at":"2026-11-13T10:00:00Z"}
]}
EOF
printf '{"artifacts":[]}\n' >"${TMP}/none.json"

cat >"${TMP}/results" <<'STUB'
#!/usr/bin/env bash
[ -n "${RESULTS_FAIL:-}" ] && { echo "no jobs named 'Functional tests (...)'" >&2; exit 1; }
printf '%b' "${RESULTS_OUT}"
STUB
chmod +x "${TMP}/results"

ALL_PASS='passed\tFunctional tests (nginx) / Run Tests\npassed\tFunctional tests (plus) / Run Tests\n'
ONE_FAIL='passed\tFunctional tests (nginx) / Run Tests\nfailed\tFunctional tests (plus) / Run Tests\n'

summ() {
    env GH="${TMP}/gh" ARTIFACTS="${TMP}/artifacts.json" FUNCTIONAL_RESULTS="${TMP}/results" \
        REPO=o/m RUN_ID=42 RELEASE_VERSION=v2.9.0 OPERATOR_VERSION=v1.7.0 SHA=abc123 \
        GITHUB_REF_NAME=internal/release-2.9 "$@" "${SCRIPT}" 2>&1
}
has() { if grep -qF -- "$2" <<<"${out}"; then ok "$1"; else no "$1" "missing: $2" "${out}"; fi; }
lacks() { if grep -qF -- "$2" <<<"${out}"; then no "$1" "unexpected: $2" "${out}"; else ok "$1"; fi; }

out="$(summ RESULTS_OUT="${ALL_PASS}" MANIFEST_RESULT=success MANIFEST_SHA256=feed)"
has "the manifest sha256 is shown, for a manual publish" '| Manifest sha256 | `feed` |'
has "the commit and operator version are shown" '| Commit | `abc123` |'
has "all passing legs are reported" "All 2 legs passed."
has "the promote check inputs are given" '| `mode` | `check` |'
has "the prep run ID is given for promote" '| `prep_run_id` | `42` |'
has "the workflow ref to use is given" 'from `internal/release-2.9`'
lacks "no rerun advice when everything passed" "Re-run failed jobs"

out="$(summ RESULTS_OUT="${ONE_FAIL}" MANIFEST_RESULT=success)"
has "a failed leg is listed" "- failed: Functional tests (plus) / Run Tests"
has "the rerun deadline is the earliest dist or digest expiry" 'before `2026-10-21T09:00:00Z`'
has "accepting the failure is offered" "accept_failed_tests"
has "promote inputs are still given" '| `release_version` | `v2.9.0` |'

out="$(summ RESULTS_OUT="${ALL_PASS}" MANIFEST_RESULT=failure)"
lacks "no sha256 row without a manifest" "Manifest sha256"
has "a failed manifest says nothing was published" "No manifest. Nothing has been published."
has "and gives the rerun deadline" '2026-10-21T09:00:00Z'
has "and warns off re-running everything" "Do not use **Re-run all jobs**"
lacks "and gives no promote inputs" '| `mode` |'

out="$(summ RESULTS_OUT="${ALL_PASS}" MANIFEST_RESULT=failure ARTIFACTS="${TMP}/none.json")"
has "with the binaries gone, it says to run prep again" "run prep again"
lacks "and offers no rerun" 'before `'

out="$(summ RESULTS_OUT="${ALL_PASS}" MANIFEST_RESULT=skipped)"
has "a skipped manifest names the gating jobs" "conformance or Helm failed"

out="$(summ RESULTS_FAIL=1 RESULTS_OUT='' MANIFEST_RESULT=success)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "unreadable results do not fail the summary"; else no "unreadable results do not fail the summary" "rc ${rc}"; fi
has "unreadable results are reported" "Could not read the results"
has "and are treated as not passed" "promote will refuse this run"

(env REPO=o/m RUN_ID=42 "${SCRIPT}" >/dev/null 2>&1)
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a missing release version is a usage error"; else no "a missing release version is a usage error" "rc ${rc}"; fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
