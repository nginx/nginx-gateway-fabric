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

# A cosign that records what it was asked to verify and answers from a
# control file, so the identity it is handed can be asserted on and a
# rejected signature can be simulated without a real Sigstore round trip.
COSIGN_LOG="${TMP_ROOT}/cosign.log"
COSIGN_RC="${TMP_ROOT}/cosign.rc"
printf '0' >"${COSIGN_RC}"
cat >"${TMP_ROOT}/cosign" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${COSIGN_LOG}"
exit "$(cat "${COSIGN_RC}")"
STUB
chmod +x "${TMP_ROOT}/cosign"
BUNDLE="${TMP_ROOT}/manifest.sigstore.json"
printf '{"stub":"bundle"}\n' >"${BUNDLE}"
SIGNER="example-org/the-mirror"

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

# run_verify <manifest> <repo> [ref] [VAR=value ...] -- a valid signature
# environment by default; trailing pairs override it.
run_verify() {
    local manifest="$1" repo="$2" ref="${3:-HEAD}"
    shift 3 2>/dev/null || shift $#
    : >"${COSIGN_LOG}"
    env COSIGN="${TMP_ROOT}/cosign" COSIGN_LOG="${COSIGN_LOG}" COSIGN_RC="${COSIGN_RC}" \
        SIGNATURE_BUNDLE="${BUNDLE}" SIGNER_REPOSITORY="${SIGNER}" \
        MANIFEST="${manifest}" VERIFY_REF="${ref}" REPO_DIR="${repo}" "$@" "${VERIFY}" 2>&1
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
# Provenance: the first gate, and the one that makes the others mean anything
# ---------------------------------------------------------------------------
repo5="$(new_repo signed)"
m5s="${TMP_ROOT}/signed.json"
write_manifest "${m5s}" "$(tree_of "${repo5}")"
out="$(run_verify "${m5s}" "${repo5}")"
check_eq "a signed manifest verifies" "sha=$(sha_of "${repo5}")" "$(printf '%s' "${out}" | grep '^sha=')"

# What cosign was asked to check is the whole of the trust decision, so it is
# pinned exactly: the issuer, the signer repository with its dots escaped, the
# workflow file, and a ref anchored to an internal release branch.
args="$(cat "${COSIGN_LOG}")"
check_eq "cosign is asked for a verify-blob" "yes" "$(printf '%s' "${args}" | grep -q '^verify-blob ' && echo yes || echo no)"
check_eq "the bundle is passed" "yes" "$(printf '%s' "${args}" | grep -qF -- "--bundle ${BUNDLE}" && echo yes || echo no)"
check_eq "the GitHub OIDC issuer is required" "yes" "$(printf '%s' "${args}" | grep -qF -- "--certificate-oidc-issuer https://token.actions.githubusercontent.com" && echo yes || echo no)"
check_eq "the identity names the signer repository" "yes" "$(printf '%s' "${args}" | grep -qF -- "github\\.com/example-org/the-mirror/" && echo yes || echo no)"
check_eq "the identity names the prep workflow file" "yes" "$(printf '%s' "${args}" | grep -qF -- "workflows/release-prep\\.yml@" && echo yes || echo no)"
check_eq "the identity is anchored to an internal release branch" "yes" "$(printf '%s' "${args}" | grep -qF -- "@refs/heads/internal/release-[0-9]+\\.[0-9]+\$" && echo yes || echo no)"
check_eq "the identity is anchored at the start" "yes" "$(printf '%s' "${args}" | grep -qF -- "--certificate-identity-regexp ^https://github" && echo yes || echo no)"
check_eq "the manifest itself is the verified blob" "yes" "$(printf '%s' "${args}" | grep -qF -- " ${m5s}" && echo yes || echo no)"

# A bad signature is refused before anything else is looked at. The fixture
# has a wrong schema AND a drifted tree; the refusal must be about the
# signature, or publish would be telling the owner to fix the wrong thing.
repo6="$(new_repo unsigned)"
m6s="${TMP_ROOT}/unsigned.json"
write_manifest "${m6s}" "$(tree_of "${repo6}")" 7
printf 'drift\n' >>"${repo6}/file.txt"
git -C "${repo6}" commit --quiet -am "drift"
printf '1' >"${COSIGN_RC}"
expect_refusal "a manifest cosign rejects is refused" "is not signed by release-prep.yml in example-org/the-mirror" "${m6s}" "${repo6}"
out="$(run_verify "${m6s}" "${repo6}")"
check_eq "the signature refusal comes before the schema check" "no" "$(printf '%s' "${out}" | grep -q 'schema_version' && echo yes || echo no)"
check_eq "the signature refusal comes before the tree check" "no" "$(printf '%s' "${out}" | grep -q 'public tree' && echo yes || echo no)"
printf '0' >"${COSIGN_RC}"

# Fail closed on the inputs the gate cannot do without.
if out="$(run_verify "${m5s}" "${repo5}" HEAD SIGNATURE_BUNDLE= 2>&1)"; then
    fail "no bundle is a usage error" "expected non-zero"
else
    check_eq "no bundle is a usage error" "2" "$?"
fi
if out="$(run_verify "${m5s}" "${repo5}" HEAD SIGNER_REPOSITORY= 2>&1)"; then
    fail "no signer repository is a usage error" "expected non-zero"
else
    check_eq "no signer repository is a usage error" "2" "$?"
fi
if out="$(run_verify "${m5s}" "${repo5}" HEAD SIGNATURE_BUNDLE="${TMP_ROOT}/nope.sigstore.json" 2>&1)"; then
    fail "a missing bundle file is a usage error" "expected non-zero"
else
    check_eq "a missing bundle file is a usage error" "2" "$?"
fi

# The workflow file is overridable, so a wrong override must show up in the
# identity rather than being silently ignored.
run_verify "${m5s}" "${repo5}" HEAD SIGNER_WORKFLOW=other.yml >/dev/null
check_eq "an overridden signer workflow reaches the identity" "yes" "$(grep -qF -- "workflows/other\\.yml@" "${COSIGN_LOG}" && echo yes || echo no)"

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
if MANIFEST="" SIGNATURE_BUNDLE="${BUNDLE}" SIGNER_REPOSITORY="${SIGNER}" COSIGN="${TMP_ROOT}/cosign" COSIGN_LOG="${COSIGN_LOG}" COSIGN_RC="${COSIGN_RC}" VERIFY_REF=HEAD REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing MANIFEST is an error" "expected non-zero"
else
    check_eq "a missing MANIFEST is an error" "2" "$?"
fi

if MANIFEST="${m}" SIGNATURE_BUNDLE="${BUNDLE}" SIGNER_REPOSITORY="${SIGNER}" COSIGN="${TMP_ROOT}/cosign" COSIGN_LOG="${COSIGN_LOG}" COSIGN_RC="${COSIGN_RC}" VERIFY_REF="" REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing VERIFY_REF is an error" "expected non-zero"
else
    check_eq "a missing VERIFY_REF is an error" "2" "$?"
fi

if MANIFEST="${TMP_ROOT}/nope.json" SIGNATURE_BUNDLE="${BUNDLE}" SIGNER_REPOSITORY="${SIGNER}" COSIGN="${TMP_ROOT}/cosign" COSIGN_LOG="${COSIGN_LOG}" COSIGN_RC="${COSIGN_RC}" VERIFY_REF=HEAD REPO_DIR="${repo4}" "${VERIFY}" >/dev/null 2>&1; then
    fail "a missing manifest file is an error" "expected non-zero"
else
    check_eq "a missing manifest file is an error" "2" "$?"
fi

# ---------------------------------------------------------------------------
printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
