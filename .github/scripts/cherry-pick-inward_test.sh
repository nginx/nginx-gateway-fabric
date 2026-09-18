#!/usr/bin/env bash
#
# Tests for cherry-pick-inward.sh, against a real git repository; ordering is
# checked against a stubbed `gh`.
#
# Run directly: bash .github/scripts/cherry-pick-inward_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/cherry-pick-inward.sh"
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

# shellcheck source=/dev/null
source "${SCRIPT}"
set +eu # sourcing brings `set -euo pipefail`; assertions expect failures

# A repository standing in for the internal release branch.
REPO="${TMP}/repo"
mkdir -p "${REPO}"
(
    cd "${REPO}" || exit 1
    git init --quiet -b main
    git config user.email t@example.invalid
    git config user.name Test
    echo one >file
    git add file
    git commit --quiet -m "Add a thing (#11)"
    echo two >>file
    git commit --quiet -am "Fix another thing (#22)"
    echo three >>file
    git commit --quiet -am "A pick with a trailer

(cherry picked from commit abc123def456)"
) || {
    echo "could not build the fixture repository"
    exit 1
}

applied() {
    local want="$1" desc="$2" number="$3" sha="${4:-}" got
    if (cd "${REPO}" && already_applied main "${number}" "${sha}"); then
        got=yes
    else
        got=no
    fi
    if [ "${got}" = "${want}" ]; then ok "${desc}"; else
        no "${desc}" "want ${want}, got ${got}"
    fi
}

applied yes "a squashed pull request is detected by its subject" 11
applied yes "so is a later one" 22
applied no "a pull request that is not there is not detected" 33

# (#2) must not match "(#22)": a prefix of an applied number is dangerous.
applied no "a number that is a prefix of an applied one does not match" 2
applied no "nor a number that extends one" 222

# A previous run's -x trailer counts too, so a reworded subject is not reapplied.
applied yes "the cherry-pick trailer is detected" 99 abc123def456
applied no "an unrelated sha is not" 99 fedcba654321

# Deliberately out of chronological order, so the merge-order assertions below
# prove the script's own sort_by(.mergedAt) rather than the stub's arrangement.
cat >"${TMP}/gh" <<'STUB'
#!/usr/bin/env bash
JSON='[
  {"number":30,"title":"third","mergeCommit":{"oid":"ccc"},"mergedAt":"2026-03-01T00:00:00Z"},
  {"number":10,"title":"first","mergeCommit":{"oid":"aaa"},"mergedAt":"2026-01-01T00:00:00Z"},
  {"number":20,"title":"second","mergeCommit":{"oid":"bbb"},"mergedAt":"2026-02-01T00:00:00Z"}
]'
jq_expr=""
take_next=0
for arg in "$@"; do
  if [ "${take_next}" = "1" ]; then jq_expr="${arg}"; take_next=0; continue; fi
  if [ "${arg}" = "--jq" ]; then take_next=1; fi
done
printf '%s' "${JSON}" | jq -r "${jq_expr}"
STUB
chmod +x "${TMP}/gh"

if command -v jq >/dev/null; then
    # Read by list_candidates, which shellcheck cannot see.
    # shellcheck disable=SC2034
    GH="${TMP}/gh" PUBLIC_REPO=x/y LABEL=l LIMIT=50
    order="$(list_candidates | cut -f1 | tr '\n' ' ')"
    if [ "${order}" = "10 20 30 " ]; then
        ok "candidates come back oldest merge first"
    else
        no "candidates come back oldest merge first" "got '${order}'"
    fi
else
    echo "NOTE: jq not found, skipping the ordering test (it runs in CI)"
fi

rc_of() {
    (
        set +e
        "${SCRIPT}" "$@" >/dev/null 2>&1
        echo $?
    )
}

# assert_rc <expected code> <description> [args to the script]
assert_rc() {
    local want="$1" desc="$2" got
    shift 2
    got="$(rc_of "$@")"
    if [ "${got}" = "${want}" ]; then ok "${desc}"; else
        no "${desc}" "expected exit ${want}, got ${got}"
    fi
}

assert_rc 2 "no arguments is a usage error"
assert_rc 2 "a branch without the release- prefix is rejected" --release-branch 2.8
assert_rc 2 "a non-numeric limit is rejected" --release-branch release-2.8 --limit many
assert_rc 2 "an unknown argument is rejected" --nonsense
assert_rc 2 "a flag with no value is rejected" --release-branch

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
