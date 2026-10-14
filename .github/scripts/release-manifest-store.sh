#!/usr/bin/env bash
# release-manifest-store.sh
#
# Puts the signed release manifest and its Sigstore bundle in blob storage, or
# gets them back. Prep puts them beside the release assets it stages; publish
# gets them by version, so the manifest never passes through a dispatch input
# or a shell, where its bytes -- and so its signature -- can change. The store
# is untrusted: publish verifies the signature before reading anything in it.
#
# Usage:
#   release-manifest-store.sh put
#   release-manifest-store.sh get
#
# Reads from the environment:
#   RELEASE_VERSION    e.g. v2.8.0; part of the blob path. Required.
#   STORAGE_ACCOUNT    Azure storage account name. Required.
#   STORAGE_CONTAINER  container within it. Required.
#   MANIFEST           local manifest path. Default: release-manifest.json
#   SIGNATURE_BUNDLE   local bundle path. Default: release-manifest.sigstore.json
#   BLOB_PREFIX        path prefix inside the container. Default: nginx-gateway-fabric
#   AZ                 az binary. Default: az
#
# Prints sha256=<manifest sha256> on stdout.
# Exit status: 0 done, 1 an upload or download failed, 2 bad invocation.

set -euo pipefail

RELEASE_VERSION="${RELEASE_VERSION:-}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-}"
STORAGE_CONTAINER="${STORAGE_CONTAINER:-}"
MANIFEST="${MANIFEST:-release-manifest.json}"
SIGNATURE_BUNDLE="${SIGNATURE_BUNDLE:-release-manifest.sigstore.json}"
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

[ $# -eq 1 ] || die "usage: release-manifest-store.sh put|get"
action="$1"
case "${action}" in
put | get) ;;
*) die "unknown action '${action}': use put or get" ;;
esac

[ -n "${RELEASE_VERSION}" ] || die "RELEASE_VERSION is required"
printf '%s' "${RELEASE_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+' ||
    die "RELEASE_VERSION must look like vX.Y.Z, got '${RELEASE_VERSION}'"
[ -n "${STORAGE_ACCOUNT}" ] || die "STORAGE_ACCOUNT is required (from the vault secret azure-storage-account)"
[ -n "${STORAGE_CONTAINER}" ] || die "STORAGE_CONTAINER is required (from the vault secret azure-storage-bucket)"

dir="${BLOB_PREFIX%/}/${RELEASE_VERSION}"

transfer() {
    local verb="$1" file="$2" blob="$3"
    local args=(storage blob "${verb}" --auth-mode=login
        --file "${file}"
        --container-name "${STORAGE_CONTAINER}"
        --account-name "${STORAGE_ACCOUNT}"
        --name "${blob}")
    # A re-run of prep for the same version supersedes the earlier manifest.
    [ "${verb}" = "upload" ] && args+=(--overwrite)
    "${AZ}" "${args[@]}" >/dev/null
}

if [ "${action}" = "put" ]; then
    [ -f "${MANIFEST}" ] || die "manifest not found: ${MANIFEST}"
    [ -f "${SIGNATURE_BUNDLE}" ] || die "signature bundle not found: ${SIGNATURE_BUNDLE}"
    transfer upload "${MANIFEST}" "${dir}/release-manifest.json" ||
        fail "could not upload the manifest to ${dir}"
    transfer upload "${SIGNATURE_BUNDLE}" "${dir}/release-manifest.sigstore.json" ||
        fail "could not upload the signature bundle to ${dir}"
    echo "stored the manifest and its bundle under ${dir}" >&2
    echo "sha256=$(sha256sum "${MANIFEST}" | cut -d' ' -f1)"
    exit 0
fi

transfer download "${MANIFEST}" "${dir}/release-manifest.json" ||
    fail "no manifest for ${RELEASE_VERSION} under ${dir}. Has prep run for ${RELEASE_VERSION}?"
transfer download "${SIGNATURE_BUNDLE}" "${dir}/release-manifest.sigstore.json" ||
    fail "the manifest for ${RELEASE_VERSION} has no signature bundle under ${dir}"
echo "sha256=$(sha256sum "${MANIFEST}" | cut -d' ' -f1)"
