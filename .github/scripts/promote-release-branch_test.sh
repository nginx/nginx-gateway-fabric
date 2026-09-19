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
assert_rc 2 "dispatching without a mirror repository is a usage error" --release-branch release-2.8 --release-version v2.8.0 --mirror-repo ""

# The manifest handoff: gh stubbed per subcommand, functions run in subshells
# since fail exits.
FIX="${TMP}/fixture"
mkdir -p "${FIX}"
INTERNAL_SHA="$(sha_of internal-ahead)"
cat >"${FIX}/release-manifest.json" <<EOF
{"schema_version":1,"release_version":"v2.8.0","source":{"internal_sha":"${INTERNAL_SHA}"},"images":[]}
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
out="$( (with_stub check_manifest "${FIX}" v2.8.0 "$(sha_of public-at-base)") 2>&1)"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "re-run prep"; then
    ok "a manifest built from a different commit is refused and says to re-run prep"
else
    no "a manifest built from a different commit is refused and says to re-run prep" "${out}"
fi

: >"${GH_LOG}"
(with_stub dispatch_publish "${FIX}" v2.8.0) >/dev/null 2>&1
args="$(cat "${GH_LOG}")"
dispatch_has() {
    if printf '%s' "${args}" | grep -qF -- "$2"; then ok "$1"; else no "$1" "${args}"; fi
}
dispatch_has "publish is dispatched on the public repository" "workflow run release-publish.yml --repo example-org/public --ref release-2.8"
dispatch_has "the release version is passed" "-f release_version=v2.8.0"
dispatch_has "the release branch is passed" "-f release_branch=release-2.8"
dispatch_has "the manifest is passed inline" '-f manifest={"schema_version":1'
dispatch_has "the signature bundle is passed inline" '-f manifest_bundle={"stub":"bundle"}'
dispatch_has "publish runs for real by default" "-f dry_run=false"

: >"${GH_LOG}"
(PUBLISH_DRY_RUN=true with_stub dispatch_publish "${FIX}" v2.8.0) >/dev/null 2>&1
args="$(cat "${GH_LOG}")"
dispatch_has "--publish-dry-run is passed through to publish" "-f dry_run=true"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
