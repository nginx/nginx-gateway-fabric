#!/usr/bin/env bash
#
# Tests for promote-release-branch.sh, against a real temporary git repository.
#
# Run directly: bash .github/scripts/promote-release-branch_test.sh

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

# A repository with the three shapes release day can be in.
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

    # A commit only the public branch has, e.g. pushed straight there; must not be discarded.
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

# Public ahead of the mirror is still a divergence, not a fast-forward.
kind_is diverged "public ahead of the mirror is not a fast-forward" public-at-base internal-ahead

# gh is stubbed to report the diverged public tip; git is real.
cat >"${TMP}/gh" <<STUB
#!/usr/bin/env bash
$(cd "${REPO}" && printf 'echo %s\n' "$(git rev-parse public-diverged)")
STUB
chmod +x "${TMP}/gh"

out="$(cd "${REPO}" && GH="${TMP}/gh" "${SCRIPT}" --release-branch release-9.9 --no-dispatch 2>&1)"
rc=$?
if [ "${rc}" -ne 0 ]; then
    ok "a run against a missing internal branch fails"
else
    no "a run against a missing internal branch fails" "exit 0, output: ${out}"
fi

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
assert_rc 2 "a branch without the release- prefix is rejected" --release-branch 2.8 --no-dispatch
assert_rc 2 "an unknown argument is rejected" --nonsense
assert_rc 2 "a flag with no value is rejected" --release-branch
assert_rc 2 "dispatching without a release version is a usage error" --release-branch release-2.8 --mirror-repo o/m
assert_rc 2 "a release version without the v prefix is rejected" --release-branch release-2.8 --release-version 2.8.0 --mirror-repo o/m
assert_rc 2 "a release version with trailing characters is rejected" --release-branch release-2.8 --release-version v2.8.0-rc1 --mirror-repo o/m
assert_rc 2 "dispatching without a mirror repository is a usage error" --release-branch release-2.8 --release-version v2.8.0 --mirror-repo ""
assert_rc 2 "an unknown publish mode is rejected" --release-branch release-2.8 --release-version v2.8.0 --mirror-repo o/m --publish-mode dry-run

# The manifest handoff: gh stubbed per subcommand, functions run in subshells
# since fail exits.
FIX="${TMP}/fixture"
mkdir -p "${FIX}"
INTERNAL_SHA="$(sha_of internal-ahead)"
cat >"${FIX}/release-manifest.json" <<EOF
{"schema_version":1,"release_version":"v2.8.0","source":{"internal_sha":"${INTERNAL_SHA}","internal_branch":"internal/release-2.8"},"images":[]}
EOF
printf '{"stub":"bundle"}\n' >"${FIX}/release-manifest.sigstore.json"

GH_LOG="${TMP}/gh.log"
GH_ARTIFACTS="${TMP}/artifacts.json"
cat >"${TMP}/gh2" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${GH_LOG}"
case "$1 $2" in
"api repos/"*"/actions/artifacts"*)
    jq -r "${4}" <"${GH_ARTIFACTS}"
    ;;
"run download")
    dir=""; while [ $# -gt 0 ]; do [ "$1" = "--dir" ] && dir="$2"; shift; done
    mkdir -p "${dir}" && cp "${GH_FIXTURE}"/* "${dir}/"
    ;;
"workflow run") ;;
esac
STUB
chmod +x "${TMP}/gh2"

# Old, newest-unexpired (must win), and a newer expired artifact.
cat >"${GH_ARTIFACTS}" <<'EOF'
{"artifacts":[
 {"name":"release-manifest-v2.8.0","expired":false,"created_at":"2026-01-01T00:00:00Z","workflow_run":{"id":11}},
 {"name":"release-manifest-v2.8.0","expired":true, "created_at":"2026-04-01T00:00:00Z","workflow_run":{"id":44}},
 {"name":"release-manifest-v2.8.0","expired":false,"created_at":"2026-03-01T00:00:00Z","workflow_run":{"id":33}}
]}
EOF

with_stub() {
    GH="${TMP}/gh2" GH_LOG="${GH_LOG}" GH_ARTIFACTS="${GH_ARTIFACTS}" \
        MIRROR_REPO="example-org/the-mirror" PUBLIC_REPO="example-org/public" GH_FIXTURE="${GH_FIXTURE:-${FIX}}" \
        RELEASE_BRANCH="release-2.8" PUBLISH_WORKFLOW="release-publish.yml" "$@"
}

got="$(with_stub find_prep_run v2.8.0)"
if [ "${got}" = "33" ]; then ok "the newest unexpired manifest artifact is chosen"; else
    no "the newest unexpired manifest artifact is chosen" "want 33, got '${got}'"
fi

printf '{"artifacts":[{"name":"x","expired":true,"created_at":"2026-01-01T00:00:00Z","workflow_run":{"id":9}}]}\n' >"${TMP}/expired.json"
got="$(GH_ARTIFACTS="${TMP}/expired.json" with_stub find_prep_run v2.8.0)"
if [ -z "${got}" ]; then ok "only-expired artifacts yield no run"; else
    no "only-expired artifacts yield no run" "got '${got}'"
fi

: >"${GH_LOG}"
d="${TMP}/dl"
(with_stub fetch_manifest 33 v2.8.0 "${d}") >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 0 ] && [ -f "${d}/release-manifest.json" ] && [ -f "${d}/release-manifest.sigstore.json" ]; then
    ok "the manifest and its bundle are downloaded from the prep run"
else
    no "the manifest and its bundle are downloaded from the prep run" "rc ${rc}"
fi
if grep -q -- "run download 33 --repo example-org/the-mirror --name release-manifest-v2.8.0" "${GH_LOG}"; then
    ok "the download names the run, the mirror and the versioned artifact"
else
    no "the download names the run, the mirror and the versioned artifact" "$(cat "${GH_LOG}")"
fi

FIX_NOBUNDLE="${TMP}/fixture-nobundle"
mkdir -p "${FIX_NOBUNDLE}" && cp "${FIX}/release-manifest.json" "${FIX_NOBUNDLE}/"
(GH_FIXTURE="${FIX_NOBUNDLE}" with_stub fetch_manifest 33 v2.8.0 "${TMP}/dl2") >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 1 ]; then ok "a prep run without a signature bundle is refused"; else
    no "a prep run without a signature bundle is refused"
fi

(with_stub check_manifest "${FIX}" v2.8.0 "${INTERNAL_SHA}") >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 0 ]; then ok "a manifest for this version and commit passes"; else
    no "a manifest for this version and commit passes"
fi
(with_stub check_manifest "${FIX}" v2.9.0 "${INTERNAL_SHA}") >/dev/null 2>&1
rc=$?
if [ "${rc}" -eq 1 ]; then ok "a manifest for another version is refused"; else
    no "a manifest for another version is refused"
fi
FIX_BRANCH="${TMP}/fixture-branch"
mkdir -p "${FIX_BRANCH}"
sed 's#internal/release-2.8#internal/release-2.7#' "${FIX}/release-manifest.json" >"${FIX_BRANCH}/release-manifest.json"
out="$( (with_stub check_manifest "${FIX_BRANCH}" v2.8.0 "${INTERNAL_SHA}") 2>&1)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "internal/release-2.7" <<<"${out}"; then
    ok "a manifest built on another release branch is refused"
else
    no "a manifest built on another release branch is refused" "rc ${rc}: ${out}"
fi
out="$( (with_stub check_manifest "${FIX}" v2.8.0 "$(sha_of public-at-base)") 2>&1)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "re-run prep" <<<"${out}"; then
    ok "a manifest built from a different commit is refused and says to re-run prep"
else
    no "a manifest built from a different commit is refused and says to re-run prep" "${out}"
fi

: >"${GH_LOG}"
(with_stub dispatch_publish v2.8.0 "$(printf 'a%.0s' {1..64})") >/dev/null 2>&1
args="$(cat "${GH_LOG}")"
dispatch_has() {
    if grep -qF -- "$2" <<<"${args}"; then ok "$1"; else no "$1" "${args}"; fi
}
dispatch_lacks() {
    if grep -qF -- "$2" <<<"${args}"; then no "$1" "${args}"; else ok "$1"; fi
}
dispatch_has "publish is dispatched on the public repository" "workflow run release-publish.yml --repo example-org/public --ref release-2.8"
dispatch_has "the release version is passed" "-f release_version=v2.8.0"
dispatch_has "publish publishes by default" "-f mode=publish"
dispatch_has "the sha256 of the manifest promote checked is passed" "-f manifest_sha256=$(printf 'a%.0s' {1..64})"
dispatch_lacks "the manifest is not passed: publish fetches it" "manifest="
dispatch_lacks "the release branch is not passed: publish reads it from the manifest" "release_branch="

: >"${GH_LOG}"
(PUBLISH_MODE=verify with_stub dispatch_publish v2.8.0 "$(printf 'a%.0s' {1..64})") >/dev/null 2>&1
args="$(cat "${GH_LOG}")"
dispatch_has "--publish-mode verify is passed through to publish" "-f mode=verify"

# The functional gate, with the results script stubbed.
RESULTS="${TMP}/results"
cat >"${RESULTS}" <<'STUB'
#!/usr/bin/env bash
[ -n "${RESULTS_FAIL:-}" ] && exit 1
printf '%b' "${RESULTS_OUT}"
STUB
chmod +x "${RESULTS}"
SUM="${TMP}/summary.md"
gate() {
    : >"${SUM}"
    (FUNCTIONAL_RESULTS="${RESULTS}" SUMMARY="${SUM}" GITHUB_ACTOR=releaser with_stub check_functional 33) 2>&1
}

out="$(RESULTS_OUT='passed\tFunctional tests (nginx) / Run Tests\npassed\tFunctional tests (plus) / Run Tests\n' gate)"
rc=$?
if [ "${rc}" -eq 0 ] && grep -q "all 2 legs passed" <<<"${out}"; then ok "all functional legs passing promotes"; else
    no "all functional legs passing promotes" "rc ${rc}: ${out}"
fi

out="$(RESULTS_OUT='passed\tFunctional tests (nginx) / Run Tests\nfailed\tFunctional tests (plus) / Run Tests\n' gate)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "Functional tests (plus)" <<<"${out}" && grep -q "accept_failed_tests" <<<"${out}"; then
    ok "a failed leg refuses, names the leg and says how to accept it"
else
    no "a failed leg refuses, names the leg and says how to accept it" "rc ${rc}: ${out}"
fi

out="$(ACCEPT_FAILED_TESTS="known flake in graceful-recovery, #1234" \
    RESULTS_OUT='failed\tFunctional tests (plus) / Run Tests\n' gate)"
rc=$?
if [ "${rc}" -eq 0 ] && grep -q "known flake in graceful-recovery, #1234" "${SUM}" &&
    grep -q "@releaser" "${SUM}" && grep -q "Functional tests (plus)" "${SUM}"; then
    ok "an accepted failure promotes and records the leg, the reason and who accepted it"
else
    no "an accepted failure promotes and records the leg, the reason and who accepted it" "rc ${rc}: ${out}" "$(cat "${SUM}")"
fi

out="$(ACCEPT_FAILED_TESTS="anything" RESULTS_OUT='running\tFunctional tests (plus) / Run Tests\n' gate)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "still running" <<<"${out}"; then ok "a leg still running is never accepted"; else
    no "a leg still running is never accepted" "rc ${rc}: ${out}"
fi

out="$(RESULTS_FAIL=1 RESULTS_OUT='' gate)"
rc=$?
if [ "${rc}" -eq 1 ]; then ok "unreadable results refuse rather than pass"; else no "unreadable results refuse rather than pass" "rc ${rc}: ${out}"; fi

assert_rc 2 "--accept-failed-tests needs a reason" --release-branch release-2.8 --release-version v2.8.0 --mirror-repo o/m --accept-failed-tests

# A public fetch that fails must say so, not fall through to "diverged".
(cd "${REPO}" && git update-ref refs/remotes/origin/internal/release-2.8 internal-ahead)
cat >"${TMP}/git-nofetch" <<'STUB'
#!/usr/bin/env bash
[ "$1" = "fetch" ] && { echo "fatal: unable to access" >&2; exit 128; }
exec git "$@"
STUB
chmod +x "${TMP}/git-nofetch"
out="$(cd "${REPO}" && GIT="${TMP}/git-nofetch" GH="${TMP}/gh" "${SCRIPT}" \
    --release-branch release-2.8 --no-dispatch 2>&1)"
rc=$?
if [ "${rc}" -ne 0 ] && grep -q "could not fetch release-2.8" <<<"${out}" &&
    ! grep -q "diverged" <<<"${out}"; then
    ok "a failed public fetch is reported as a fetch failure, not a divergence"
else
    no "a failed public fetch is reported as a fetch failure, not a divergence" "rc ${rc}: ${out}"
fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
