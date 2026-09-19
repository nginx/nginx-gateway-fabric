#!/usr/bin/env bash
# stage-release-assets.sh
# Uploads the release assets GoReleaser produced to blob storage and prints
# the manifest references for them. The store is untrusted -- publish checks
# downloaded bytes against the sha256 recorded here, not against the store.
#
# Reads from the environment:
#   DIST_DIR           GoReleaser output directory. Required.
#   RELEASE_VERSION    e.g. v2.8.0; becomes part of the blob path. Required.
#   STORAGE_ACCOUNT    Azure storage account name. Required.
#   STORAGE_CONTAINER  container within it. Required.
#   BLOB_PREFIX        path prefix inside the container. Default: nginx-gateway-fabric
#   AZ                 az binary. Default: az
#
# Prints one JSON object per asset: {name, blob, sha256, bytes}.
# Exit status: 0 staged, 1 upload or inventory failure, 2 bad invocation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COLLECT="${COLLECT:-${SCRIPT_DIR}/collect-release-assets.sh}"

DIST_DIR="${DIST_DIR:-}"
RELEASE_VERSION="${RELEASE_VERSION:-}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-}"
STORAGE_CONTAINER="${STORAGE_CONTAINER:-}"
BLOB_PREFIX="${BLOB_PREFIX:-nginx-gateway-fabric}"
AZ="${AZ:-az}"

die() {
    echo "error: $*" >&2
    exit 2
}

fail() {
    echo "error: $*" >&2
    exit 1
}

[ -n "${DIST_DIR}" ] || die "DIST_DIR is required"
[ -n "${RELEASE_VERSION}" ] || die "RELEASE_VERSION is required"
[ -n "${STORAGE_ACCOUNT}" ] || die "STORAGE_ACCOUNT is required (from the vault secret azure-storage-account)"
[ -n "${STORAGE_CONTAINER}" ] || die "STORAGE_CONTAINER is required (from the vault secret azure-storage-bucket)"
[ -d "${DIST_DIR}" ] || die "dist directory not found: ${DIST_DIR}"
printf '%s' "${RELEASE_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+' ||
    die "RELEASE_VERSION must look like vX.Y.Z, got '${RELEASE_VERSION}'"

# Captured via plain substitution, not `mapfile < <(cmd) || ...`: a process
# substitution's failure is not propagated to mapfile.
inventory="$(DIST_DIR="${DIST_DIR}" "${COLLECT}")" ||
    fail "the release asset inventory is incomplete; nothing staged"
mapfile -t assets <<<"${inventory}"
[ "${#assets[@]}" -gt 0 ] && [ -n "${assets[0]}" ] || fail "the inventory is empty; nothing to stage"

refs=()
for path in "${assets[@]}"; do
    [ -f "${path}" ] || fail "inventory names a file that does not exist: ${path}"
    name="$(basename -- "${path}")"
    blob="${BLOB_PREFIX%/}/${RELEASE_VERSION}/${name}"
    sha="$(sha256sum "${path}" | cut -d' ' -f1)"
    bytes="$(wc -c <"${path}" | tr -d ' ')"

    echo "staging ${name} -> ${blob}" >&2
    # --overwrite: a re-run of prep for the same version supersedes the earlier upload.
    "${AZ}" storage blob upload --auth-mode=login --overwrite \
        --file "${path}" \
        --container-name "${STORAGE_CONTAINER}" \
        --account-name "${STORAGE_ACCOUNT}" \
        --name "${blob}" >/dev/null ||
        fail "upload failed for ${name}"

    refs+=("$(jq -n --arg name "${name}" --arg blob "${blob}" --arg sha "${sha}" --argjson bytes "${bytes}" \
        '{name: $name, blob: $blob, sha256: $sha, bytes: $bytes}')")
done

printf '%s\n' "${refs[@]}" | jq -s 'sort_by(.name)'
