#!/usr/bin/env bash
#
# assert-staged-digests.sh
#
# Fails unless every staged image tag the release tests pull resolves to the
# digest its build recorded. The tests pull by tag while the release manifest
# records digests, so without this a tag moved after the build (a re-run, a
# concurrent push) would let the tests pass against something other than
# what gets signed and promoted.
#
# The repository is resolved with resolve-image-target.sh, the mapping
# build.yml and copy-images.sh use, so it cannot drift from where the image
# was pushed.
#
# Read from the environment:
#   DIGEST_DIR     directory of digest records written by build.yml. Required.
#   REGISTRY       registry to read from, e.g. the staging read mirror. Required.
#   TAG            tag the tests pull, without a build-os suffix. Required.
#   IMAGES         images to check. Default: ngf nginx plus plus-nap-waf
#   IMAGE_PREFIX   repository path prefix. Default: nginx-gateway-fabric
#   SKOPEO         skopeo binary. Default: skopeo
#
# Exit status: 0 every digest matches, 1 a mismatch or failed lookup,
# 2 missing or malformed input.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOLVE_IMAGE_TARGET="${RESOLVE_IMAGE_TARGET:-${SCRIPT_DIR}/resolve-image-target.sh}"
SKOPEO="${SKOPEO:-skopeo}"

DIGEST_DIR="${DIGEST_DIR:-}"
REGISTRY="${REGISTRY:-}"
TAG="${TAG:-}"
IMAGES="${IMAGES:-ngf nginx plus plus-nap-waf}"
IMAGE_PREFIX="${IMAGE_PREFIX:-nginx-gateway-fabric}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${DIGEST_DIR}" ] || die "DIGEST_DIR is required"
[ -d "${DIGEST_DIR}" ] || die "DIGEST_DIR is not a directory: ${DIGEST_DIR}"
[ -n "${REGISTRY}" ] || die "REGISTRY is required"
[ -n "${TAG}" ] || die "TAG is required"
[ -n "${IMAGES// /}" ] || die "IMAGES must not be empty"

repo_for() {
    local out
    out=$(
        IMAGE="$1" \
            OSS_REGISTRY="${REGISTRY}" \
            PLUS_REGISTRY="${REGISTRY}" \
            IMAGE_PREFIX="${IMAGE_PREFIX}" \
            "${RESOLVE_IMAGE_TARGET}"
    ) || die "could not resolve a repository for image '$1'"
    printf '%s' "${out}" | sed -n 's/^target=//p'
}

RAW="$(mktemp)"
trap 'rm -f "${RAW}"' EXIT

# The digest of the manifest exactly as the registry serves it: for a
# multi-platform image that is the index, which is what the build recorded.
# Hashed from a file, since command substitution would strip a trailing newline.
remote_digest() {
    "${SKOPEO}" inspect --raw --retry-times 3 "docker://$1" >"${RAW}" || return 1
    [ -s "${RAW}" ] || return 1
    printf 'sha256:%s' "$(sha256sum <"${RAW}" | cut -d' ' -f1)"
}

shopt -s nullglob
records=("${DIGEST_DIR}"/*.json)
shopt -u nullglob
[ "${#records[@]}" -gt 0 ] || die "no digest records in ${DIGEST_DIR}"

checked=0
failures=0
declare -A seen=()

for record in "${records[@]}"; do
    jq -e . "${record}" >/dev/null 2>&1 || die "not valid JSON: ${record}"
    image=$(jq -r '.image // ""' "${record}")
    os=$(jq -r '.["base-os"] // ""' "${record}")
    digest=$(jq -r '.digest // ""' "${record}")
    [ -n "${image}" ] || die "no image in ${record}"
    [[ ${digest} =~ ^sha256:[0-9a-f]{64}$ ]] || die "malformed digest '${digest}' in ${record}"

    case " ${IMAGES} " in
    *" ${image} "*) ;;
    *) continue ;;
    esac
    seen["${image}"]=1

    repo=$(repo_for "${image}")
    [ -n "${repo}" ] || die "resolve-image-target.sh printed no target for '${image}'"
    ref="${repo}:${TAG}${os:+-${os}}"

    if ! got=$(remote_digest "${ref}"); then
        echo "FAIL  ${ref}: could not read the manifest" >&2
        failures=$((failures + 1))
        continue
    fi
    if [ "${got}" != "${digest}" ]; then
        echo "FAIL  ${ref} is ${got}, but the build recorded ${digest}" >&2
        failures=$((failures + 1))
        continue
    fi
    echo "ok    ${ref} is ${digest}"
    checked=$((checked + 1))
done

# A missing record means an image the tests pull has no digest to hold it to.
for image in ${IMAGES}; do
    if [ -z "${seen[${image}]:-}" ]; then
        echo "FAIL  no digest record for image '${image}'" >&2
        failures=$((failures + 1))
    fi
done

if [ "${failures}" -ne 0 ]; then
    echo "${failures} staged image(s) do not match their recorded digest, ${checked} do" >&2
    exit 1
fi

echo "${checked} staged image(s) match their recorded digest."
