#!/usr/bin/env bash
# Tests for functional-results.sh, against a stubbed gh. Exit status: 0 all
# passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/functional-results.sh"
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

# gh prints the fixture through the --jq filter it was given, as gh does.
GH_LOG="${TMP}/gh.log"
cat >"${TMP}/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${GH_LOG}"
[ -n "${GH_FAIL:-}" ] && exit 1
filter=""
while [ $# -gt 0 ]; do [ "$1" = "--jq" ] && filter="$2"; shift; done
jq -r "${filter}" <"${GH_JOBS}"
STUB
chmod +x "${TMP}/gh"

run() {
    env GH="${TMP}/gh" GH_LOG="${GH_LOG}" REPO=example-org/the-mirror RUN_ID=42 "$@" "${SCRIPT}"
}

cat >"${TMP}/jobs.json" <<'EOF'
{"jobs":[
 {"name":"Checks and variables","status":"completed","conclusion":"success"},
 {"name":"Functional tests (nginx, , v1.32.0) / Run Tests","status":"completed","conclusion":"success"},
 {"name":"Functional tests (plus, ubi, v1.35.0) / Run Tests","status":"completed","conclusion":"failure"},
 {"name":"Functional tests (plus-waf, , v1.32.0) / Run Tests","status":"completed","conclusion":"cancelled"},
 {"name":"Functional tests (nginx, ubi, v1.35.0) / Run Tests","status":"in_progress","conclusion":null},
 {"name":"Conformance tests (nginx, , v1.32.0) / Run Tests","status":"completed","conclusion":"failure"},
 {"name":"Functional tests summary","status":"completed","conclusion":"failure"}
]}
EOF

: >"${GH_LOG}"
out="$(run GH_JOBS="${TMP}/jobs.json" 2>/dev/null)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "a run with functional legs lists them"; else no "a run with functional legs lists them" "rc ${rc}"; fi

state_of() { awk -F'\t' -v n="$1" '$2 == n { print $1 }' <<<"${out}"; }
expect() {
    local desc="$1" name="$2" want="$3" got
    got="$(state_of "${name}")"
    if [ "${got}" = "${want}" ]; then ok "${desc}"; else no "${desc}" "want ${want}, got '${got}'" "${out}"; fi
}
expect "success is passed" "Functional tests (nginx, , v1.32.0) / Run Tests" passed
expect "failure is failed" "Functional tests (plus, ubi, v1.35.0) / Run Tests" failed
expect "cancelled is failed, not passed" "Functional tests (plus-waf, , v1.32.0) / Run Tests" failed
expect "a leg still in progress is running" "Functional tests (nginx, ubi, v1.35.0) / Run Tests" running

n="$(grep -c . <<<"${out}")"
if [ "${n}" = "4" ]; then ok "only functional legs are listed, not conformance or look-alike names"; else
    no "only functional legs are listed, not conformance or look-alike names" "${out}"
fi
if grep -q "actions/runs/42/jobs?filter=latest" "${GH_LOG}" && grep -q -- "--paginate" "${GH_LOG}"; then
    ok "the latest attempt's jobs are read, every page"
else
    no "the latest attempt's jobs are read, every page" "$(cat "${GH_LOG}")"
fi

printf '{"jobs":[{"name":"Checks and variables","status":"completed","conclusion":"success"}]}\n' >"${TMP}/none.json"
err="$(run GH_JOBS="${TMP}/none.json" 2>&1 >/dev/null)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "no jobs named" <<<"${err}"; then
    ok "a run with no functional legs is an error, never all-passed"
else
    no "a run with no functional legs is an error, never all-passed" "rc ${rc}: ${err}"
fi

run GH_JOBS="${TMP}/jobs.json" GH_FAIL=1 >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 1 ]; then ok "an API failure is an error"; else no "an API failure is an error" "rc ${rc}"; fi

run GH_JOBS="${TMP}/jobs.json" RUN_ID=abc >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a non-numeric run ID is a usage error"; else no "a non-numeric run ID is a usage error" "rc ${rc}"; fi

# The prefix is matched against the job name in release-prep.yml; a rename of
# either must fail here rather than leave promote finding no legs.
prefix="$(sed -n 's/^FUNCTIONAL_JOB_PREFIX="\(.*\)"$/\1/p' "${SCRIPT}")"
prep_name="$(awk '/^  functional-tests:$/ { f = 1; next } f && /^    name:/ { sub(/^    name: */, ""); print; exit }' \
    "${DIR}/../workflows/release-prep.yml")"
if [ -n "${prefix}" ] && [ "${prefix}" = "${prep_name}" ]; then
    ok "the prefix matches the functional-tests job name in release-prep.yml"
else
    no "the prefix matches the functional-tests job name in release-prep.yml" "script '${prefix}', release-prep.yml '${prep_name}'"
fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
