#!/usr/bin/env bash
#
# stage-release-assets.sh
#
# Uploads the release assets GoReleaser produced to blob storage and prints
# the references release prep records in the manifest.
#
# Why this exists: prep runs in the private mirror and publish runs in the
# public repository, and an Actions artifact cannot cross that boundary. Blob
# storage is the store both can reach. What makes it safe is not the store
# but the manifest: every asset is recorded here with its sha256, the manifest
# is then signed, and publish refuses any downloaded byte that does not match.
# The store only has to be reachable, not trusted.
#
# The inventory comes from collect-release-assets.sh, which fails if any
# expected class -- archives, checksums, signature bundle, SBOMs -- is missing.
# A release cannot be staged with a piece absent.
#
# Read from the environment:
#   DIST_DIR           GoReleaser output directory. Required.
#   RELEASE_VERSION    e.g. v2.8.0; becomes part of the blob path. Required.
#   STORAGE_ACCOUNT    Azure storage account name. Required.
#   STORAGE_CONTAINER  container within it. Required.
#   BLOB_PREFIX        path prefix inside the container.
#                      Default: nginx-gateway-fabric
#   AZ                 az binary. Default: az
#
# Prints a JSON array, one object per asset:
#   { "name": "<file>", "blob": "<prefix>/<version>/<file>", "sha256": "<hex>", "bytes": <n> }
#
# Exit status: 0 staged, 1 an upload failed or the inventory is incomplete,
#              2 bad invocation.

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

# The inventory check is the collector's job; a missing class fails there
# with a message that says what to look at. Its exit status is captured
# through a plain substitution: `mapfile < <(cmd) || ...` would never see it,
# because a process substitution's failure is not propagated to mapfile.
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
    # --overwrite: a re-run of prep for the same version supersedes the
    # earlier upload, exactly as its manifest artifact does.
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
