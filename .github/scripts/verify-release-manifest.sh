#!/usr/bin/env bash
#
# verify-release-manifest.sh
#
# Release publish's first gate. Runs before anything is promoted, and decides
# whether this manifest may be published at all.
#
# It answers three questions, in order:
#
#   1. Did the mirror's prep workflow produce this manifest? The manifest is
#      the only thing publish trusts, so who wrote it matters more than what
#      it says. Prep signs it with cosign keyless, and the signing certificate
#      names the exact workflow and branch that ran. Anything not signed by
#      release-prep.yml on an internal/release-X.Y branch of the signer
#      repository is refused before its contents are even read. Without this
#      the manifest is trusted by whoever pastes it.
#
#   2. Do I understand this manifest? Prep and publish are separated in time
#      as well as in place -- a patch release can be published weeks after the
#      tooling around it moved on -- so an unrecognised schema version must
#      produce a refusal rather than a partial promotion.
#
#   3. Is the public tree the thing that was built? Prep builds from an
#      internal release branch whose commit does not exist publicly. The
#      merge-back brings that work to the public release branch, and the
#      commit metadata will always differ -- parents, author, timestamp,
#      message -- so the **tree** is the only correct invariant. If the trees
#      differ we are about to publish artifacts built from a source state
#      nobody built or tested.
#
# It prints the verified commit SHA. Publish must tag *that* SHA rather than
# the branch head, or a commit landing mid-run gets tagged as the release and
# the tree check stops meaning anything.
#
# Read from the environment:
#   MANIFEST           path to the release manifest JSON. Required.
#   SIGNATURE_BUNDLE   path to the Sigstore bundle prep produced. Required.
#   SIGNER_REPOSITORY  owner/name of the repository whose prep signed it.
#                      Required. Comes from a repository variable in the
#                      public repository so the mirror's name stays out of
#                      this tree.
#   SIGNER_WORKFLOW    the signing workflow file. Default: release-prep.yml
#   VERIFY_REF         git ref or SHA of the public commit to check. Required.
#   REPO_DIR           repository to resolve VERIFY_REF in. Default: cwd.
#   COSIGN             cosign binary. Default: cosign
#
# Prints, one per line:
#   sha        the verified commit SHA, for tagging
#   tree_hash  the tree both sides agree on
#
# Exit status: 0 verified, 1 unsigned, mismatched or unusable manifest,
#              2 bad invocation.

set -euo pipefail

# Bump only when a change would make this reader misread an older manifest.
# Publish refuses anything it does not recognise rather than guessing.
SUPPORTED_SCHEMA=1

MANIFEST="${MANIFEST:-}"
SIGNATURE_BUNDLE="${SIGNATURE_BUNDLE:-}"
SIGNER_REPOSITORY="${SIGNER_REPOSITORY:-}"
SIGNER_WORKFLOW="${SIGNER_WORKFLOW:-release-prep.yml}"
VERIFY_REF="${VERIFY_REF:-}"
REPO_DIR="${REPO_DIR:-.}"
COSIGN="${COSIGN:-cosign}"
OIDC_ISSUER="https://token.actions.githubusercontent.com"

usage_die() {
    echo "error: $*" >&2
    exit 2
}

refuse() {
    echo "REFUSED: $*" >&2
    exit 1
}

[ -n "${MANIFEST}" ] || usage_die "MANIFEST is required"
[ -n "${SIGNATURE_BUNDLE}" ] || usage_die "SIGNATURE_BUNDLE is required: an unsigned manifest is not publishable"
[ -n "${SIGNER_REPOSITORY}" ] || usage_die "SIGNER_REPOSITORY is required (set RELEASE_SIGNER_REPOSITORY in the public repository)"
[ -n "${VERIFY_REF}" ] || usage_die "VERIFY_REF is required"
[ -f "${MANIFEST}" ] || usage_die "manifest not found: ${MANIFEST}"
[ -f "${SIGNATURE_BUNDLE}" ] || usage_die "signature bundle not found: ${SIGNATURE_BUNDLE}"

# ---------------------------------------------------------------------------
# 1. Provenance
#
# Before the contents are read. A manifest that fails this is not a release
# artifact, whatever it says inside, and reporting a schema or tree problem
# first would suggest fixing the wrong thing.
#
# The identity is anchored at both ends: the repository and workflow file are
# fixed, and the ref must be an internal release branch. A manifest signed by
# the same workflow running on some other branch -- a fork of the mirror, a
# test branch -- is refused, because that is not a release prep.
# ---------------------------------------------------------------------------
escape_re() {
    printf '%s' "$1" | sed 's/[.]/\\./g'
}
identity_re="^https://github\\.com/$(escape_re "${SIGNER_REPOSITORY}")/\\.github/workflows/$(escape_re "${SIGNER_WORKFLOW}")@refs/heads/internal/release-[0-9]+\\.[0-9]+$"

if ! "${COSIGN}" verify-blob \
    --bundle "${SIGNATURE_BUNDLE}" \
    --certificate-oidc-issuer "${OIDC_ISSUER}" \
    --certificate-identity-regexp "${identity_re}" \
    "${MANIFEST}" >/dev/null 2>&1; then
    refuse "the manifest is not signed by ${SIGNER_WORKFLOW} in ${SIGNER_REPOSITORY} on an internal release branch.
  expected identity : ${identity_re}
  expected issuer   : ${OIDC_ISSUER}
Either it was not produced by release prep, it was altered after signing, or
the bundle does not belong to this manifest. Nothing in it can be trusted."
fi

jq -e . "${MANIFEST}" >/dev/null 2>&1 || refuse "manifest is not valid JSON: ${MANIFEST}"

# ---------------------------------------------------------------------------
# 2. Schema
# ---------------------------------------------------------------------------
schema="$(jq -r '.schema_version // empty' "${MANIFEST}")"
[ -n "${schema}" ] || refuse "manifest has no schema_version; refusing to guess its shape"

case "${schema}" in
'' | *[!0-9]*) refuse "schema_version must be an integer, got '${schema}'" ;;
esac

if [ "${schema}" -ne "${SUPPORTED_SCHEMA}" ]; then
    refuse "manifest schema_version ${schema} is not supported by this publish (understands ${SUPPORTED_SCHEMA}).
A newer manifest needs a newer publish; an older one needs the publish of its era.
Publishing it with this reader could promote the wrong images or miss some entirely."
fi

# ---------------------------------------------------------------------------
# 3. Shape
#
# Checked before the tree comparison so a truncated manifest fails saying so,
# rather than failing on a missing tree hash and reading as a merge-back
# problem.
# ---------------------------------------------------------------------------
for field in .release_version .source.internal_sha .source.tree_hash; do
    v="$(jq -r "${field} // empty" "${MANIFEST}")"
    [ -n "${v}" ] || refuse "manifest is missing ${field}"
done

n_images="$(jq -r '.images | length' "${MANIFEST}")"
[ "${n_images}" -gt 0 ] || refuse "manifest records no images; there is nothing to promote"

# Every image must be a digest. A tag here would let a staging re-tag change
# what publish promotes, which is the whole reason the manifest exists.
bad="$(jq -r '.images[] | select((.digest // "") | test("^sha256:[0-9a-f]{64}$") | not)
              | .image + " " + (.digest // "<missing>")' "${MANIFEST}")"
[ -z "${bad}" ] || refuse "these images are not pinned to a sha256 digest: ${bad}"

manifest_tree="$(jq -r '.source.tree_hash' "${MANIFEST}")"

# ---------------------------------------------------------------------------
# 4. The merge-back
# ---------------------------------------------------------------------------
git -C "${REPO_DIR}" rev-parse --git-dir >/dev/null 2>&1 ||
    usage_die "not a git repository: ${REPO_DIR}"

sha="$(git -C "${REPO_DIR}" rev-parse --verify "${VERIFY_REF}^{commit}" 2>/dev/null)" ||
    refuse "cannot resolve VERIFY_REF '${VERIFY_REF}' in ${REPO_DIR}"

public_tree="$(git -C "${REPO_DIR}" rev-parse --verify "${sha}^{tree}")"

if [ "${public_tree}" != "${manifest_tree}" ]; then
    built_branch="$(jq -r '.source.internal_branch // "?"' "${MANIFEST}")"
    built_sha="$(jq -r '.source.internal_sha' "${MANIFEST}")"
    refuse "the public tree is not the tree that was built and tested.
  manifest tree : ${manifest_tree}  (built from ${built_branch} at ${built_sha})
  public tree   : ${public_tree}  (${VERIFY_REF} at ${sha})
The merge-back has not landed, is incomplete, or carried a change the release
was not built from. Publishing now would ship artifacts built from a source
state that was never tested."
fi

echo "sha=${sha}"
echo "tree_hash=${public_tree}"
