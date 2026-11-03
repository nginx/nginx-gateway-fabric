#!/usr/bin/env bash
# release-manifest-store.sh
#
# Puts the signed release manifest and its Sigstore bundle in blob storage, or
# gets them back. Prep puts them beside the release assets it stages; publish
# gets them, so the manifest never passes through a dispatch input or a shell,
# where its bytes -- and so its signature -- can change.
#
# Each manifest is stored under its own sha256, so a later prep run for the
# same version cannot replace the one promote checked,
# and a manifest and its bundle always come from the same run. Publish asks
# for the sha256 promote checked and gets exactly those bytes. The store is
# still untrusted: publish verifies the signature before using anything.
#
# Usage:
#   release-manifest-store.sh put
#   release-manifest-store.sh get
#
# Reads from the environment:
#   RELEASE_VERSION    e.g. v2.8.0; part of the blob path. Required.
#   STORAGE_ACCOUNT    Azure storage account name. Required.
#   STORAGE_CONTAINER  container within it. Required.
#   MANIFEST_SHA256    get only: the sha256 of the manifest to fetch. Required
#                      for get.
#   MANIFEST           local manifest path. Default: release-manifest.json
#   SIGNATURE_BUNDLE   local bundle path. Default: release-manifest.sigstore.json
#   BLOB_PREFIX        path prefix inside the container. Default: nginx-gateway-fabric
#   AZ                 az binary. Default: az
#
# Prints sha256=<manifest sha256> on stdout.
# Exit status: 0 done, 1 an upload, download or hash check failed,
#              2 bad invocation.

set -euo pipefail

RELEASE_VERSION="${RELEASE_VERSION:-}"
MANIFEST_SHA256="${MANIFEST_SHA256:-}"
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
printf '%s' "${RELEASE_VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
    die "RELEASE_VERSION must look like vX.Y.Z, got '${RELEASE_VERSION}'"
[ -n "${STORAGE_ACCOUNT}" ] || die "STORAGE_ACCOUNT is required (from the vault secret azure-storage-account)"
[ -n "${STORAGE_CONTAINER}" ] || die "STORAGE_CONTAINER is required (from the vault secret azure-storage-bucket)"

if [ "${action}" = "get" ]; then
    [ -n "${MANIFEST_SHA256}" ] || die "MANIFEST_SHA256 is required to get a manifest (promote passes it; prep's and promote's summaries show it)"
    printf '%s' "${MANIFEST_SHA256}" | grep -Eq '^[0-9a-f]{64}$' ||
        die "MANIFEST_SHA256 must be 64 lowercase hex characters, got '${MANIFEST_SHA256}'"
fi

manifests="${BLOB_PREFIX%/}/${RELEASE_VERSION}/manifests"

transfer() {
    local verb="$1" file="$2" blob="$3"
    local args=(storage blob "${verb}" --auth-mode=login
        --file "${file}"
        --container-name "${STORAGE_CONTAINER}"
        --account-name "${STORAGE_ACCOUNT}"
        --name "${blob}")
    # --overwrite only matters when the manifest job is re-run on the same
    # inputs: the manifest bytes, and so the path, are identical, and the new
    # bundle is an equally valid signature of them.
    [ "${verb}" = "upload" ] && args+=(--overwrite)
    "${AZ}" "${args[@]}" >/dev/null
}

if [ "${action}" = "put" ]; then
    [ -f "${MANIFEST}" ] || die "manifest not found: ${MANIFEST}"
    [ -f "${SIGNATURE_BUNDLE}" ] || die "signature bundle not found: ${SIGNATURE_BUNDLE}"
    sha="$(sha256sum "${MANIFEST}" | cut -d' ' -f1)"
    dir="${manifests}/${sha}"
    transfer upload "${MANIFEST}" "${dir}/release-manifest.json" ||
        fail "could not upload the manifest to ${dir}"
    transfer upload "${SIGNATURE_BUNDLE}" "${dir}/release-manifest.sigstore.json" ||
        fail "could not upload the signature bundle to ${dir}"
    echo "stored the manifest and its bundle under ${dir}" >&2
    echo "sha256=${sha}"
    exit 0
fi

dir="${manifests}/${MANIFEST_SHA256}"
transfer download "${MANIFEST}" "${dir}/release-manifest.json" ||
    fail "no manifest ${MANIFEST_SHA256} for ${RELEASE_VERSION} under ${manifests}. Use the sha256 from the prep or promote run summary."
transfer download "${SIGNATURE_BUNDLE}" "${dir}/release-manifest.sigstore.json" ||
    fail "manifest ${MANIFEST_SHA256} has no signature bundle under ${dir}"

got="$(sha256sum "${MANIFEST}" | cut -d' ' -f1)"
[ "${got}" = "${MANIFEST_SHA256}" ] ||
    fail "the stored manifest does not hash to its path.
  asked for : ${MANIFEST_SHA256}
  got       : ${got}
The store served something other than what prep stored."
echo "sha256=${got}"
