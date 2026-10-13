#!/usr/bin/env bash
#
# Tests for changelog-entries.sh, against a real temporary git repository and
# a stubbed `gh api`.
#
# Run directly: bash .github/scripts/changelog-entries_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/changelog-entries.sh"
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

REPO="${TMP}/repo"
mkdir -p "${REPO}"
(
    cd "${REPO}" || exit 1
    git init --quiet -b main
    git config user.email t@example.invalid
    git config user.name Test
    c() {
        echo "$1" >>file
        git add file
        git commit --quiet -m "$1"
    }
    c "Before the last release (#1)"
    git tag v1.0.0
    c "A labelled fix (#10)"
    c "An unlabelled change (#11)"
    c "A labelled fix, picked through its own pull request (#12) (#20)"
    c "Labelled but skipped (#13)"
    c "Not a pull request here (#99) (#14)"
    c "No number at all"
    c "A second pick of the same fix (#10) (#21)"
    git tag release
) || {
    echo "could not build the fixture repository"
    exit 1
}

# One JSON file per pull request; a missing one is a 404, as from the API.
PRS="${TMP}/prs"
mkdir -p "${PRS}"
pr() {
    jq -n --argjson n "$1" --arg t "$2" --arg l "$3" '{
        number: $n, title: $t, merged_at: "2026-01-01T00:00:00Z",
        user: {login: "someone"},
        html_url: ("https://github.com/o/r/pull/" + ($n | tostring)),
        labels: ($l | split(",") | map(select(. != "")) | map({name: .}))
    }' >"${PRS}/$1.json"
}
pr 1 "Before" "release-notes"
pr 10 "A labelled fix" "bug,release-notes"
pr 11 "An unlabelled change" "bug"
pr 12 "A picked fix" "release-notes"
pr 20 "The pick itself" "release-notes"
pr 13 "Skipped" "release-notes,skip-changelog"
pr 14 "After a foreign number" "release-notes"
pr 21 "The second pick" "release-notes"

cat >"${TMP}/gh" <<STUB
#!/usr/bin/env bash
n="\${2##*/}"
[ -f "${PRS}/\${n}.json" ] || { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
cat "${PRS}/\${n}.json"
STUB
chmod +x "${TMP}/gh"

OUT="$(cd "${REPO}" && GH="${TMP}/gh" "${SCRIPT}" --since v1.0.0 --ref release --repo o/r 2>&1)"
RC=$?

has() {
    if printf '%s\n' "${OUT}" | grep -qF -- "$2"; then ok "$1"; else no "$1" "output was:" "${OUT}"; fi
}
lacks() {
    if printf '%s\n' "${OUT}" | grep -qF -- "$2"; then no "$1" "output was:" "${OUT}"; else ok "$1"; fi
}

[ "${RC}" -eq 0 ] && ok "a normal run succeeds" || no "a normal run succeeds" "rc ${RC}: ${OUT}"
has "a labelled pull request is listed, in GitHub's format" \
    "* A labelled fix by @someone in https://github.com/o/r/pull/10"
lacks "an unlabelled pull request is not" "pull/11"
has "a pick through its own pull request lists the original" "pull/12"
lacks "the pull request that only moved a pick is not listed" "pull/20"
lacks "skip-changelog wins over release-notes" "pull/13"
has "a number that is not a pull request here is passed over" "pull/14"
if printf '%s\n' "${OUT}" | grep -qE 'pull/1$'; then
    no "a change from before the previous release is not listed" "${OUT}"
else
    ok "a change from before the previous release is not listed"
fi
lacks "a change picked twice is listed once, not as its second pick" "pull/21"
if [ "$(printf '%s\n' "${OUT}" | grep -c 'pull/10$')" -eq 1 ]; then
    ok "a change picked twice appears once"
else
    no "a change picked twice appears once" "${OUT}"
fi
order="$(printf '%s\n' "${OUT}" | grep -oE 'pull/[0-9]+' | tr '\n' ' ')"
if [ "${order}" = "pull/10 pull/12 pull/14 " ]; then
    ok "entries are oldest first"
else
    no "entries are oldest first" "got '${order}'"
fi

# A lookup that fails for any reason other than "not found" must not quietly
# produce a shorter changelog.
cat >"${TMP}/gh-down" <<'STUB'
#!/usr/bin/env bash
echo "gh: HTTP 502: Bad Gateway" >&2
exit 1
STUB
chmod +x "${TMP}/gh-down"
OUT="$(cd "${REPO}" && GH="${TMP}/gh-down" "${SCRIPT}" --since v1.0.0 --ref release --repo o/r 2>&1)"
RC=$?
if [ "${RC}" -eq 1 ] && printf '%s' "${OUT}" | grep -q "could not read"; then
    ok "an API failure fails the run"
else
    no "an API failure fails the run" "rc ${RC}: ${OUT}"
fi

OUT="$(cd "${REPO}" && GH="${TMP}/gh" "${SCRIPT}" --ref release 2>&1)"
RC=$?
[ "${RC}" -eq 2 ] && ok "--since is required" || no "--since is required" "rc ${RC}"
OUT="$(cd "${REPO}" && GH="${TMP}/gh" "${SCRIPT}" --since v0.0.0 2>&1)"
RC=$?
[ "${RC}" -eq 2 ] && ok "an unknown --since is refused" || no "an unknown --since is refused" "rc ${RC}"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
