#!/usr/bin/env bash
#
# Tests for verify-release-manifest.sh.
#
# This is the gate that stands between a manifest and a public registry. Its
# failures are the expensive kind: publishing artifacts nobody tested, or
# promoting images a re-tag moved. Every case here is a way that could happen
# quietly.
#
# The tree comparison is exercised against real temporary git repositories
# rather than stubbed, because the thing under test is a `git rev-parse`
# question and a stub would only prove the stub agrees with itself.
#
# Usage: verify-release-manifest_test.sh
# Exit status: 0 all passed, 1 one or more failed.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERIFY="${SCRIPT_DIR}/verify-release-manifest.sh"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "${TMP_ROOT}"' EXIT

PASSED=0
FAILED=0

DIG="sha256:$(printf 'a%.0s' {1..64})"

pass() {
    printf 'ok    %s\n' "$1"
    PASSED=$((PASSED + 1))
}

fail() {
    printf 'FAIL  %s\n      %s\n' "$1" "$2"
    FAILED=$((FAILED + 1))
}

check_eq() {
    local name="$1" want="$2" got="$3"
    if [ "${got}" = "${want}" ]; then
        pass "${name}"
    else
        fail "${name}" "expected '${want}', got '${got}'"
    fi
}

# new_repo <name> -- a real repo with one commit; echoes its path
new_repo() {
    local d="${TMP_ROOT}/$1"
    mkdir -p "${d}"
    git -C "${d}" init --quiet -b main
    git -C "${d}" config user.email t@example.com
    git -C "${d}" config user.name Test
    printf 'content\n' >"${d}/file.txt"
    git -C "${d}" add file.txt
    git -C "${d}" commit --quiet -m "one"
    printf '%s' "${d}"
}

tree_of() { git -C "$1" rev-parse "HEAD^{tree}"; }
sha_of() { git -C "$1" rev-parse HEAD; }

# write_manifest <path> <tree> [schema] [extra-jq-filter]
write_manifest() {
    local path="$1" tree="$2" schema="${3:-1}" filter="${4:-.}"
    jq -n --argjson schema "${schema}" --arg tree "${tree}" --arg dig "${DIG}" '{
      schema_version: $schema,
      release_version: "v2.8.0",
      operator_version: "v0.3.0",
      source: { internal_branch: "internal/release-2.8",
                internal_sha: "1111111111111111111111111111111111111111",
                tree_hash: $tree },
      images: [ { image: "ngf", "base-os": "", target: "stage.example/ngf",
                  digest: $dig, platforms: "linux/amd64" } ],
      chart: { digest: null }, nginx_versions: {}, assets: []
    }' | jq "${filter}" >"${path}"
}

run_verify() {
    local manifest="$1" repo="$2" ref="${3:-HEAD}"
    MANIFEST="${manifest}" VERIFY_REF="${ref}" REPO_DIR="${repo}" "${VERIFY}" 2>&1
}

# expect_refusal <name> <substring> <manifest> <repo> [ref]
expect_refusal() {
    local name="$1" want="$2" manifest="$3" repo="$4" ref="${5:-HEAD}"
    local out
    if out="$(run_verify "${manifest}" "${repo}" "${ref}")"; then
        fail "${name}" "expected a refusal, got success"
        return
    fi
    if ! printf '%s' "${out}" | grep -Fq "${want}"; then
        fail "${name}" "expected message containing '${want}', got: ${out}"
        return
    fi
    pass "${name}"
}

# ---------------------------------------------------------------------------
# The happy path
# ---------------------------------------------------------------------------
repo="$(new_repo happy)"
m="${TMP_ROOT}/happy.json"
write_manifest "${m}" "$(tree_of "${repo}")"

out="$(run_verify "${m}" "${repo}")"
check_eq "a matching tree verifies" "sha=$(sha_of "${repo}")" "$(printf '%s' "${out}" | sed -n 's/^sha=/sha=/p')"
check_eq "the tree hash is reported" "tree_hash=$(tree_of "${repo}")" "$(printf '%s' "${out}" | grep '^tree_hash=')"

# The SHA it prints is what publish tags. It must be the resolved commit, not
# the ref it was handed.
out="$(run_verify "${m}" "${repo}" main)"
check_eq "a branch ref resolves to its commit SHA" "sha=$(sha_of "${repo}")" "$(printf '%s' "${out}" | grep '^sha=')"

# ---------------------------------------------------------------------------
# The merge-back gate -- the highest-value check in the design
# ---------------------------------------------------------------------------
repo2="$(new_repo drifted)"
m2="${TMP_ROOT}/drifted.json"
write_manifest "${m2}" "$(tree_of "${repo2}")"
printf 'a drive-by change\n' >>"${repo2}/file.txt"
git -C "${repo2}" add file.txt
git -C "${repo2}" commit --quiet -m "two"
expect_refusal "a changed tree is refused" "the public tree is not the tree that was built" "${m2}" "${repo2}"

# Commit metadata always differs across the merge-back. Only the tree matters,
# so a different commit carrying an identical tree must still verify.
repo3="$(new_repo rewritten)"
m3="${TMP_ROOT}/rewritten.json"
write_manifest "${m3}" "$(tree_of "${repo3}")"
git -C "${repo3}" commit --quiet --amend -m "a totally different message" --date "2020-01-01T00:00:00Z"
out="$(run_verify "${m3}" "${repo3}")"
check_eq "a different commit with the same tree verifies" "sha=$(sha_of "${repo3}")" "$(printf '%s' "${out}" | grep '^sha=')"
if [ "$(sha_of "${repo3}")" = "1111111111111111111111111111111111111111" ]; then
    fail "the amended SHA really did change" "fixture did not amend"
else
    pass "the verified SHA is the public commit, not the manifest's internal one"
fi

expect_refusal "an unresolvable ref is refused" "cannot resolve VERIFY_REF" "${m}" "${repo}" "no-such-branch"

# ---------------------------------------------------------------------------
# Schema: the reason the version exists
# ---------------------------------------------------------------------------
repo4="$(new_repo schema)"
t4="$(tree_of "${repo4}")"

m4="${TMP_ROOT}/schema-new.json"
write_manifest "${m4}" "${t4}" 2
expect_refusal "a newer schema is refused" "schema_version 2 is not supported" "${m4}" "${repo4}"

m5="${TMP_ROOT}/schema-old.json"
write_manifest "${m5}" "${t4}" 0
expect_refusal "an older schema is refused" "schema_version 0 is not supported" "${m5}" "${repo4}"

m6="${TMP_ROOT}/schema-missing.json"
write_manifest "${m6}" "${t4}" 1 'del(.schema_version)'
expect_refusal "a manifest with no schema is refused" "no schema_version" "${m6}" "${repo4}"

m7="${TMP_ROOT}/schema-junk.json"
write_manifest "${m7}" "${t4}" 1 '.schema_version = "one"'
expect_refusal "a non-integer schema is refused" "must be an integer" "${m7}" "${repo4}"

# ---------------------------------------------------------------------------
# Shape
# ---------------------------------------------------------------------------
m8="${TMP_ROOT}/no-images.json"
write_manifest "${m8}" "${t4}" 1 '.images = []'
expect_refusal "a manifest with no images is refused" "records no images" "${m8}" "${repo4}"

m9="${TMP_ROOT}/tag-not-digest.json"
write_manifest "${m9}" "${t4}" 1 '.images[0].digest = "v2.8.0"'
expect_refusal "an image pinned to a tag is refused" "not pinned to a sha256 digest" "${m9}" "${repo4}"

m10="${TMP_ROOT}/short-digest.json"
write_manifest "${m10}" "${t4}" 1 '.images[0].digest = "sha256:abcd"'
expect_refusal "a truncated digest is refused" "not pinned to a sha256 digest" "${m10}" "${repo4}"

m11="${TMP_ROOT}/no-tree.json"
write_manifest "${m11}" "${t4}" 1 'del(.source.tree_hash)'
expect_refusal "a manifest with no tree hash is refused" "missing .source.tree_hash" "${m11}" "${repo4}"

m12="${TMP_ROOT}/no-version.json"
write_manifest "${m12}" "${t4}" 1 'del(.release_version)'
expect_refusal "a manifest with no release version is refused" "missing .release_version" "${m12}" "${repo4}"

printf 'not json at all\n' >"${TMP_ROOT}/junk.json"
expect_refusal "a non-JSON manifest is refused" "not valid JSON" "${TMP_ROOT}/junk.json" "${repo4}"

# ---------------------------------------------------------------------------
# Invocation
# ---------------------------------------------------------------------------
if MANIFEST="" VERIFY_REF=HEAD REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing MANIFEST is an error" "expected non-zero"
else
    check_eq "a missing MANIFEST is an error" "2" "$?"
fi

if MANIFEST="${m}" VERIFY_REF="" REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing VERIFY_REF is an error" "expected non-zero"
else
    check_eq "a missing VERIFY_REF is an error" "2" "$?"
fi

if MANIFEST="${TMP_ROOT}/nope.json" VERIFY_REF=HEAD REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing manifest file is an error" "expected non-zero"
else
    check_eq "a missing manifest file is an error" "2" "$?"
fi

# ---------------------------------------------------------------------------
printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
