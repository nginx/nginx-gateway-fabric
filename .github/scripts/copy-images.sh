#!/usr/bin/env bash
set -euo pipefail

# copy-images.sh
#
# Promotes already-built images between registries with `skopeo copy --all`.
# The image-to-repository mapping comes from resolve-image-target.sh, the same
# script build.yml uses, so a promotion cannot drift from what was built.
#
# Usage:
#   copy-images.sh [options]
#
# Options:
#   --config NAME              registry set from .github/config/registries-NAME
#                              (no .env suffix: **/*.env is gitignored)
#   --images "ngf nginx ..."   subset of: ngf nginx plus plus-nap-waf operator
#   --variants "default ubi"   which OS variants to promote
#   --source-tag TAG
#   --target-tag TAG
#   --additional-target-tag T  also write the source to this tag
#   --source-oss-registry R    --source-plus-registry R
#   --target-oss-registry R    --target-plus-registry R
#   --image-prefix P
#   --dry-run                  print what would be copied, copy nothing
#   -h, --help
#
# Precedence: CLI > config > environment > default. No registry has a default,
# and the staging config refuses to resolve a public target from any source.
#
# Exit status: 0 copied, 1 a copy failed, 2 bad usage or configuration.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${CONFIG_DIR:-${SCRIPT_DIR}/../config}"
RESOLVE_IMAGE_TARGET="${RESOLVE_IMAGE_TARGET:-${SCRIPT_DIR}/resolve-image-target.sh}"
SKOPEO="${SKOPEO:-skopeo}"

ALL_IMAGES="ngf nginx plus plus-nap-waf operator"

die() {
    echo "error: $*" >&2
    exit 2
}

usage() {
    sed -n '/^# Usage:/,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# Hosts an outside party can pull from. Matched on the whole host, never a
# substring, since the staging registries are near-prefixes of production ones.
is_public_registry() {
    local host="${1%%/*}"
    case "${host}" in
    ghcr.io | docker.io | registry-1.docker.io) return 0 ;;
    docker-mgmt.nginx.com | private-registry.nginx.com) return 0 ;;
    *.pkg.dev | gcr.io | *.gcr.io) return 0 ;;
    esac
    return 1
}

need_value() {
    [ "$2" -gt 1 ] || die "$1 needs a value"
}

parse_args() {
    ARG_CONFIG=""
    ARG_IMAGES=""
    ARG_VARIANTS=""
    ARG_SOURCE_TAG=""
    ARG_TARGET_TAG=""
    ARG_ADDITIONAL_TARGET_TAG=""
    ARG_SOURCE_OSS_REGISTRY=""
    ARG_SOURCE_PLUS_REGISTRY=""
    ARG_TARGET_OSS_REGISTRY=""
    ARG_TARGET_PLUS_REGISTRY=""
    ARG_IMAGE_PREFIX=""
    ARG_DRY_RUN=""

    while [ $# -gt 0 ]; do
        case "$1" in
        --config) need_value "$1" $# && ARG_CONFIG="$2" && shift 2 ;;
        # Guarded against the empty string: `--images ""` would otherwise fall
        # through the default and promote everything.
        --images)
            need_value "$1" $#
            [ -n "${2// /}" ] || die "--images was given an empty list"
            ARG_IMAGES="$2"
            shift 2
            ;;
        --variants) need_value "$1" $# && ARG_VARIANTS="$2" && shift 2 ;;
        --source-tag) need_value "$1" $# && ARG_SOURCE_TAG="$2" && shift 2 ;;
        --source-digests) need_value "$1" $# && ARG_SOURCE_DIGESTS="$2" && shift 2 ;;
        --target-tag) need_value "$1" $# && ARG_TARGET_TAG="$2" && shift 2 ;;
        --additional-target-tag) need_value "$1" $# && ARG_ADDITIONAL_TARGET_TAG="$2" && shift 2 ;;
        --source-oss-registry) need_value "$1" $# && ARG_SOURCE_OSS_REGISTRY="$2" && shift 2 ;;
        --source-plus-registry) need_value "$1" $# && ARG_SOURCE_PLUS_REGISTRY="$2" && shift 2 ;;
        --target-oss-registry) need_value "$1" $# && ARG_TARGET_OSS_REGISTRY="$2" && shift 2 ;;
        --target-plus-registry) need_value "$1" $# && ARG_TARGET_PLUS_REGISTRY="$2" && shift 2 ;;
        --image-prefix) need_value "$1" $# && ARG_IMAGE_PREFIX="$2" && shift 2 ;;
        --dry-run) ARG_DRY_RUN=true && shift ;;
        -h | --help)
            usage
            exit 0
            ;;
        *) die "unrecognised argument '$1' (try --help)" ;;
        esac
    done
}

# Sourced after the CLI is captured, so a config's assignments override the
# environment while an explicit flag still overrides the config.
load_config() {
    CONFIG_NAME="${ARG_CONFIG:-${REGISTRY_CONFIG:-}}"
    [ -n "${CONFIG_NAME}" ] || return 0

    local config_path="${CONFIG_DIR}/registries-${CONFIG_NAME}"
    [ -f "${config_path}" ] ||
        die "config '${CONFIG_NAME}' was requested but '${config_path}' does not exist"
    # shellcheck source=/dev/null
    . "${config_path}"
}

resolve_values() {
    SOURCE_OSS_REGISTRY="${ARG_SOURCE_OSS_REGISTRY:-${SOURCE_OSS_REGISTRY:-}}"
    SOURCE_PLUS_REGISTRY="${ARG_SOURCE_PLUS_REGISTRY:-${SOURCE_PLUS_REGISTRY:-}}"
    TARGET_OSS_REGISTRY="${ARG_TARGET_OSS_REGISTRY:-${TARGET_OSS_REGISTRY:-}}"
    TARGET_PLUS_REGISTRY="${ARG_TARGET_PLUS_REGISTRY:-${TARGET_PLUS_REGISTRY:-}}"
    IMAGE_PREFIX="${ARG_IMAGE_PREFIX:-${IMAGE_PREFIX:-nginx-gateway-fabric}}"
    IMAGES="${ARG_IMAGES:-${IMAGES:-${ALL_IMAGES}}}"
    VARIANTS="${ARG_VARIANTS:-${VARIANTS:-default ubi}}"
    # The operator is built without a build-os matrix, so it has no -ubi variant.
    OPERATOR_VARIANTS="${OPERATOR_VARIANTS:-default}"
    SOURCE_TAG="${ARG_SOURCE_TAG:-${SOURCE_TAG:-edge}}"
    SOURCE_DIGESTS="${ARG_SOURCE_DIGESTS:-${SOURCE_DIGESTS:-}}"
    TARGET_TAG="${ARG_TARGET_TAG:-${TARGET_TAG:-${SOURCE_TAG}}}"
    ADDITIONAL_TARGET_TAG="${ARG_ADDITIONAL_TARGET_TAG:-${ADDITIONAL_TARGET_TAG:-}}"
    DRY_RUN="${ARG_DRY_RUN:-${DRY_RUN:-false}}"
}

# No registry has a default. A config supplies them, usually from a repository
# variable, and anything else has to be passed explicitly.
check_registries_set() {
    local name value hint=""
    if [ -n "${CONFIG_NAME}" ]; then
        hint=" Config '${CONFIG_NAME}' left it empty, which usually means its repository variable is undefined here."
    fi
    for name in SOURCE_OSS_REGISTRY SOURCE_PLUS_REGISTRY TARGET_OSS_REGISTRY TARGET_PLUS_REGISTRY; do
        value="${!name}"
        [ -n "${value}" ] || die "${name} is not set: pass --config NAME or the matching flag.${hint}"
    done
}

# The staging set must never reach somewhere the public can pull from,
# whether the registry arrived from the config, the environment or a flag.
check_staging_targets() {
    [ "${CONFIG_NAME}" = "staging" ] || return 0
    local reg
    for reg in "${TARGET_OSS_REGISTRY}" "${TARGET_PLUS_REGISTRY}"; do
        if is_public_registry "${reg}"; then
            die "the staging config cannot target the public registry '${reg}'"
        fi
    done
}

# Asks resolve-image-target.sh where an image lives, so this script holds no
# copy of the mapping build.yml uses.
repo_for() {
    local image="$1" oss_registry="$2" plus_registry="$3" out
    out=$(
        IMAGE="${image}" \
            OSS_REGISTRY="${oss_registry}" \
            PLUS_REGISTRY="${plus_registry}" \
            IMAGE_PREFIX="${IMAGE_PREFIX}" \
            "${RESOLVE_IMAGE_TARGET}"
    ) || die "could not resolve a repository for image '${image}'"
    printf '%s' "${out}" | sed -n 's/^target=//p'
}

variants_for() {
    if [ "$1" = "operator" ]; then
        printf '%s' "${OPERATOR_VARIANTS}"
    else
        printf '%s' "${VARIANTS}"
    fi
}

# "default" is the image built with no build-os, which carries no tag suffix.
suffix_for() {
    if [ "$1" = "default" ]; then
        printf ''
    else
        printf -- '-%s' "$1"
    fi
}

# Release publish sources by digest, never by tag: a tag can move between the
# two stages after a prep re-run, but a digest can't. Comes from prep's manifest.
check_digest_source() {
    [ -n "${SOURCE_DIGESTS}" ] || return 0
    [ -f "${SOURCE_DIGESTS}" ] || die "--source-digests file not found: ${SOURCE_DIGESTS}"
    jq -e . "${SOURCE_DIGESTS}" >/dev/null 2>&1 ||
        die "--source-digests is not valid JSON: ${SOURCE_DIGESTS}"
}

# The manifest keys images by base OS, where the default build carries an
# empty one; copy-images speaks in variants, where that is called "default".
base_os_for_variant() {
    if [ "$1" = "default" ]; then printf ''; else printf '%s' "$1"; fi
}

digest_for() {
    local image="$1" variant="$2" os
    os="$(base_os_for_variant "${variant}")"
    jq -r --arg image "${image}" --arg os "${os}" \
        'first(.images[] | select(.image == $image and ((.["base-os"] // "") == $os)) | .digest) // empty' \
        "${SOURCE_DIGESTS}"
}

copy_one() {
    local src="$1" dst="$2"
    if [ "${DRY_RUN}" = "true" ]; then
        echo "  would copy ${src} -> ${dst}"
        return 0
    fi
    echo "  copying ${src} -> ${dst}"
    # --all carries every architecture of the manifest list, so promotion does
    # not quietly drop arm64.
    "${SKOPEO}" copy --all --retry-times 5 "docker://${src}" "docker://${dst}"
}

main() {
    parse_args "$@"
    load_config
    resolve_values
    check_registries_set
    check_staging_targets
    check_digest_source

    if [ -n "${SOURCE_DIGESTS}" ]; then
        echo "source: oss=${SOURCE_OSS_REGISTRY} plus=${SOURCE_PLUS_REGISTRY} by digest from ${SOURCE_DIGESTS}"
    else
        echo "source: oss=${SOURCE_OSS_REGISTRY} plus=${SOURCE_PLUS_REGISTRY} tag=${SOURCE_TAG}"
    fi
    echo "target: oss=${TARGET_OSS_REGISTRY} plus=${TARGET_PLUS_REGISTRY} tag=${TARGET_TAG}"
    if [ -n "${CONFIG_NAME}" ]; then echo "config: ${CONFIG_NAME}"; fi
    if [ "${DRY_RUN}" = "true" ]; then echo "dry run: nothing will be copied"; fi

    local failures=0 copied=0 selected=0
    local image src_repo dst_repo variant suffix src tag digest

    # An empty or whitespace-only list would otherwise loop zero times and
    # report success, which is a promotion that quietly did nothing.
    # shellcheck disable=SC2086
    set -- ${IMAGES}
    [ "$#" -gt 0 ] || die "no images selected: --images matched nothing"

    for image in "$@"; do
        case " ${ALL_IMAGES} " in
        *" ${image} "*) ;;
        *) die "unrecognised image '${image}' (expected one of: ${ALL_IMAGES})" ;;
        esac
        selected=$((selected + 1))

        src_repo=$(repo_for "${image}" "${SOURCE_OSS_REGISTRY}" "${SOURCE_PLUS_REGISTRY}")
        dst_repo=$(repo_for "${image}" "${TARGET_OSS_REGISTRY}" "${TARGET_PLUS_REGISTRY}")
        [ -n "${src_repo}" ] && [ -n "${dst_repo}" ] ||
            die "resolve-image-target.sh printed no target for '${image}'"

        echo "${image}: ${src_repo} -> ${dst_repo}"

        # shellcheck disable=SC2046
        for variant in $(variants_for "${image}"); do
            suffix=$(suffix_for "${variant}")
            if [ -n "${SOURCE_DIGESTS}" ]; then
                digest="$(digest_for "${image}" "${variant}")"
                [ -n "${digest}" ] ||
                    die "no digest recorded for '${image}' variant '${variant}' in ${SOURCE_DIGESTS}"
                src="${src_repo}@${digest}"
            else
                src="${src_repo}:${SOURCE_TAG}${suffix}"
            fi

            # shellcheck disable=SC2086
            for tag in ${TARGET_TAG} ${ADDITIONAL_TARGET_TAG}; do
                if copy_one "${src}" "${dst_repo}:${tag}${suffix}"; then
                    copied=$((copied + 1))
                else
                    echo "  FAILED ${src} -> ${dst_repo}:${tag}${suffix}" >&2
                    failures=$((failures + 1))
                fi
            done
        done
    done

    if [ "${failures}" -ne 0 ]; then
        echo "${failures} copy/copies failed, ${copied} succeeded" >&2
        exit 1
    fi

    echo "${copied} image(s) promoted."
}

# Only run the driver when executed directly, not when sourced (e.g. by tests
# that exercise is_public_registry on its own).
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
    main "$@"
fi
