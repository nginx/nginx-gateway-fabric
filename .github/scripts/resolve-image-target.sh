#!/usr/bin/env bash
#
# resolve-image-target.sh
#
# Prints the registry repository a given image is published to, as GitHub
# Actions step outputs. Kept out of the metadata-action images block so it can
# be tested and redirected with one input.
#
# Read from the environment:
#   IMAGE           ngf | nginx | plus | plus-nap-waf | operator. Required.
#   OWNER           GitHub owner, used by the default OSS registry
#   OSS_REGISTRY    OSS images. Default: ghcr.io/$OWNER
#   PLUS_REGISTRY   NGINX Plus images. Default: docker-mgmt.nginx.com
#   IMAGE_PREFIX    repository path prefix. Default: nginx-gateway-fabric
#
# Prints, one per line:
#   target  full repository, e.g. ghcr.io/nginx/nginx-gateway-fabric
#   host    registry host, e.g. ghcr.io, for choosing a login
#
# Exit status: 0 resolved, 2 missing or unrecognised input.

set -euo pipefail

IMAGE="${IMAGE:-}"
OWNER="${OWNER:-}"
OSS_REGISTRY="${OSS_REGISTRY:-}"
PLUS_REGISTRY="${PLUS_REGISTRY:-}"
PREFIX="${IMAGE_PREFIX:-nginx-gateway-fabric}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${IMAGE}" ] || die "IMAGE is required"
[ -n "${PREFIX}" ] || die "IMAGE_PREFIX must not be empty"

# Not a passthrough: an unrecognised image would otherwise produce a plausible
# repository that nothing consumes.
case "${IMAGE}" in
ngf)
    class="oss"
    suffix=""
    ;;
nginx)
    class="oss"
    suffix="/nginx"
    ;;
operator)
    class="oss"
    suffix="/operator"
    ;;
plus)
    class="plus"
    suffix="/nginx-plus"
    ;;
plus-nap-waf)
    class="plus"
    suffix="/nginx-plus-f5waf"
    ;;
*)
    die "unrecognised image '${IMAGE}' (expected ngf, nginx, plus, plus-nap-waf, or operator)"
    ;;
esac

# Empty means "use the default", so callers that do not care set nothing. Only
# the registry this image uses is defaulted, so a Plus build needs no owner.
if [ "${class}" = "oss" ]; then
    if [ -z "${OSS_REGISTRY}" ]; then
        [ -n "${OWNER}" ] || die "OWNER is required when OSS_REGISTRY is not set"
        OSS_REGISTRY="ghcr.io/${OWNER}"
    fi
    registry="${OSS_REGISTRY}"
else
    PLUS_REGISTRY="${PLUS_REGISTRY:-docker-mgmt.nginx.com}"
    registry="${PLUS_REGISTRY}"
fi

# Trailing and leading slashes would produce a doubled separator.
registry="${registry%/}"
PREFIX="${PREFIX%/}"
PREFIX="${PREFIX#/}"

target="${registry}/${PREFIX}${suffix}"

echo "target=${target}"
echo "host=${target%%/*}"
