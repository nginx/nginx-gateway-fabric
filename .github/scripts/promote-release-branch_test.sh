#!/usr/bin/env bash
#
# Tests for promote-release-branch.sh.
#
# The classification is the whole safety argument of the release-day push, so
# it is what gets tested: promoting a diverged branch would discard commits
# from the public release branch, and there is no undo for that.
#
# Run against a real temporary git repository, since the classification is a
# `git merge-base` query and stubbing git would test the stub.
#
# Run directly: bash .github/scripts/promote-release-branch_test.sh
# Exit status: 0 all passed, 1 one or more failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/promote-release-branch.sh"
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

# --- a repository with the three shapes the release day can be in ----------
REPO="${TMP}/repo"
mkdir -p "${REPO}"
(
    cd "${REPO}" || exit 1
    git init --quiet -b main
    git config user.email t@example.invalid
    git config user.name Test

    echo base >file
    git add file
    git commit --quiet -m base
    git branch public-at-base

    echo more >>file
    git commit --quiet -am "a fix that only the mirror has"
    git branch internal-ahead

    # A commit on the public side that the mirror does not have: someone
    # pushed straight to the release branch, which is what the freeze exists
    # to prevent and what this must refuse to discard.
    git checkout --quiet -b public-diverged public-at-base
    echo other >>file
    git commit --quiet -am "a commit only the public branch has"
) || {
    echo "could not build the fixture repository"
    exit 1
}

sha_of() { (cd "${REPO}" && git rev-parse "$1"); }

kind_is() {
    local want="$1" desc="$2" internal="$3" public="$4" got
    got="$(cd "${REPO}" && promotion_kind "$(git rev-parse "${internal}")" "$(git rev-parse "${public}")")"
    if [ "${got}" = "${want}" ]; then ok "${desc}"; else
        no "${desc}" "want ${want}, got ${got}"
    fi
}

kind_is fast-forward "the mirror ahead of public is a fast-forward" internal-ahead public-at-base
kind_is up-to-date "the same commit on both sides needs no promotion" internal-ahead internal-ahead
kind_is diverged "a public commit the mirror lacks is a divergence" internal-ahead public-diverged

# The direction matters. Public ahead of the mirror is still a divergence, not
# a fast-forward: pushing would move the branch backwards.
kind_is diverged "public ahead of the mirror is not a fast-forward" public-at-base internal-ahead

# --- the run refuses to promote a divergence -------------------------------
# gh is stubbed to report the diverged public tip; git is real, so the
# ancestry check is the genuine one.
cat >"${TMP}/gh" <<STUB
#!/usr/bin/env bash
# Only the commits lookup is used before the refusal.
$(cd "${REPO}" && printf 'echo %s\n' "$(git rev-parse public-diverged)")
STUB
chmod +x "${TMP}/gh"

out="$(cd "${REPO}" && GH="${TMP}/gh" "${SCRIPT}" --release-branch release-9.9 2>&1)"
rc=$?
if [ "${rc}" -ne 0 ]; then
    ok "a run against a missing internal branch fails"
else
    no "a run against a missing internal branch fails" "exit 0, output: ${out}"
fi

# --- argument validation ---------------------------------------------------
rc_of() {
    (
        set +e
        "${SCRIPT}" "$@" >/dev/null 2>&1
        echo $?
    )
}

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
assert_rc 2 "an unknown argument is rejected" --nonsense
assert_rc 2 "a flag with no value is rejected" --release-branch

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
