#!/usr/bin/env bash
# fetch-release-assets.sh
#
# Downloads the release assets the manifest names, checked three ways: each
# asset's sha256 against the manifest, the checksums file's own cosign
# signature, and the archives against that checksums file. Publish attaches
# these to the GitHub release, so this is the last point a wrong byte is caught.
#
# Reads from the environment:
#   MANIFEST           path to the verified release manifest. Required.
#   OUT_DIR            where to put the assets. Default: assets
#   STORAGE_ACCOUNT    Azure storage account name. Required.
#   STORAGE_CONTAINER  container within it. Required.
#   SIGNER_REPOSITORY  owner/name that signed the checksums. Required.
#   SIGNER_WORKFLOW    Default: release-prep.yml
#   AZ                 az binary. Default: az
#   COSIGN             cosign binary. Default: cosign
#
# Prints count=<n> on stdout. Exit status: 0 verified, 1 a download or check
# failed, 2 bad invocation.

set -euo pipefail

MANIFEST="${MANIFEST:-}"
OUT_DIR="${OUT_DIR:-assets}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-}"
STORAGE_CONTAINER="${STORAGE_CONTAINER:-}"
SIGNER_REPOSITORY="${SIGNER_REPOSITORY:-}"
SIGNER_WORKFLOW="${SIGNER_WORKFLOW:-release-prep.yml}"
AZ="${AZ:-az}"
COSIGN="${COSIGN:-cosign}"
OIDC_ISSUER="https://token.actions.githubusercontent.com"

die() {
    echo "error: $*" >&2
    exit 2
}

refuse() {
    echo "REFUSED: $*" >&2
    exit 1
}

[ -n "${MANIFEST}" ] || die "MANIFEST is required"
[ -f "${MANIFEST}" ] || die "manifest not found: ${MANIFEST}"
[ -n "${STORAGE_ACCOUNT}" ] || die "STORAGE_ACCOUNT is required (from the vault secret azure-storage-account)"
[ -n "${STORAGE_CONTAINER}" ] || die "STORAGE_CONTAINER is required (from the vault secret azure-storage-bucket)"
[ -n "${SIGNER_REPOSITORY}" ] || die "SIGNER_REPOSITORY is required (set RELEASE_SIGNER_REPOSITORY in the public repository)"

n="$(jq -r '.assets | length' "${MANIFEST}")"
[ "${n}" -gt 0 ] || refuse "the manifest lists no assets. A release without its binaries is not a release; prep must stage them."

# A name with a path separator could write outside OUT_DIR.
bad="$(jq -r '.assets[] | select(
          ((.name // "") | test("^[A-Za-z0-9._-]+$") | not) or
          ((.blob // "") == "") or
          ((.sha256 // "") | test("^[0-9a-f]{64}$") | not))
        | .name // "<unnamed>"' "${MANIFEST}")"
[ -z "${bad}" ] || refuse "malformed asset entries in the manifest: $(printf '%s' "${bad}" | tr '\n' ' ')"

mkdir -p "${OUT_DIR}"

while IFS=$'\t' read -r name blob want; do
    echo "fetching ${name}" >&2
    "${AZ}" storage blob download --auth-mode=login \
        --file "${OUT_DIR}/${name}" \
        --container-name "${STORAGE_CONTAINER}" \
        --account-name "${STORAGE_ACCOUNT}" \
        --name "${blob}" >/dev/null ||
        refuse "could not download ${blob}"
    got="$(sha256sum "${OUT_DIR}/${name}" | cut -d' ' -f1)"
    [ "${got}" = "${want}" ] ||
        refuse "${name} does not match the manifest.
  manifest sha256 : ${want}
  downloaded      : ${got}
The store served something other than what prep recorded and signed."
done < <(jq -r '.assets[] | [.name, .blob, .sha256] | @tsv' "${MANIFEST}")

# The checksums file's own signature.
escape_re() {
    printf '%s' "$1" | sed 's/[.]/\\./g'
}
identity_re="^https://github\\.com/$(escape_re "${SIGNER_REPOSITORY}")/\\.github/workflows/$(escape_re "${SIGNER_WORKFLOW}")@refs/heads/internal/release-[0-9]+\\.[0-9]+$"

mapfile -t checksum_files < <(find "${OUT_DIR}" -maxdepth 1 -type f -name '*checksums.txt' | sort)
[ "${#checksum_files[@]}" -eq 1 ] ||
    refuse "expected exactly one checksums file among the assets, found ${#checksum_files[@]}"
checksums="${checksum_files[0]}"
bundle="${checksums}.sig.bundle"
[ -f "${bundle}" ] || refuse "the checksums file has no signature bundle (${bundle##*/} is not among the assets)"

"${COSIGN}" verify-blob \
    --bundle "${bundle}" \
    --certificate-oidc-issuer "${OIDC_ISSUER}" \
    --certificate-identity-regexp "${identity_re}" \
    "${checksums}" >/dev/null 2>&1 ||
    refuse "the checksums file is not signed by ${SIGNER_WORKFLOW} in ${SIGNER_REPOSITORY} on an internal release branch"

# --ignore-missing: the checksums file lists archives only, not the SBOMs and
# bundle beside them. --strict: a malformed line is an error, not a no-op.
(cd "${OUT_DIR}" && sha256sum --check --quiet --strict --ignore-missing "${checksums##*/}") ||
    refuse "an archive does not match the signed checksums file"

echo "count=${n}"
