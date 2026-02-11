#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Required environment variable
if [ -z "${NAP_WAF_REPO_URL}" ]; then
    echo "Error: NAP_WAF_REPO_URL environment variable must be set"
    echo "Example: export NAP_WAF_REPO_URL=https://your.artifactory.server/path/to/repo"
    exit 1
fi

# Optional variables with defaults
NGINX_PLUS_PREFIX="${NGINX_PLUS_PREFIX:-nginx-gateway-fabric/nginx-plus}"
TAG="${TAG:-edge}"
GOARCH="${GOARCH:-amd64}"
NJS_DIR="${NJS_DIR:-internal/controller/nginx/modules/src}"
NGINX_CONF_DIR="${NGINX_CONF_DIR:-internal/controller/nginx/conf}"
BUILD_AGENT="${BUILD_AGENT:-local}"
APP_PROTECT_VERSION="${APP_PROTECT_VERSION:-5.591.0-r1}"

DOCKERFILE="${ROOT_DIR}/build/Dockerfile.nginxplus"
DOCKERFILE_TMP="${ROOT_DIR}/build/Dockerfile.nginxplus.nap-dev"

cleanup() {
    rm -f "${DOCKERFILE_TMP}"
}
trap cleanup EXIT

echo "Creating temporary Dockerfile with NAP dev repo..."

# Read the original Dockerfile and create a modified version
# Replace the NAP WAF section with dev repo URLs
cat "${DOCKERFILE}" | awk -v repo_url="${NAP_WAF_REPO_URL}" -v app_protect_version="${APP_PROTECT_VERSION}" '
/APP_PROTECT_VERSION=/ {
    sub(/APP_PROTECT_VERSION=[^ ]+/, "APP_PROTECT_VERSION=" app_protect_version)
}
/https:\/\/pkgs\.nginx\.com\/app-protect-x-plus/ {
    gsub(/https:\/\/pkgs\.nginx\.com\/app-protect-x-plus\/alpine\/v\$\(grep -E -o/, repo_url "/release-napx/$(grep -E -o")
    gsub(/\/main/, "")
    # Add the master branch repo and allow-untrusted flag
    sub(/>> \/etc\/apk\/repositories \\$/, ">> /etc/apk/repositories \\")
    print
    print "           && printf \"%s\\n\" \"" repo_url "/master/$(grep -E -o '"'"'^[0-9]+\\.[0-9]+'"'"' /etc/alpine-release)\" >> /etc/apk/repositories \\"
    next
}
/apk add --no-cache app-protect-module-plus/ {
    gsub(/apk add --no-cache app-protect-module-plus/, "apk add --no-cache --allow-untrusted app-protect-module-plus")
}
{ print }
' >"${DOCKERFILE_TMP}"

echo "Building NAP dev image..."
docker build \
    --platform "linux/${GOARCH}" \
    --build-arg NJS_DIR="${NJS_DIR}" \
    --build-arg NGINX_CONF_DIR="${NGINX_CONF_DIR}" \
    --build-arg BUILD_AGENT="${BUILD_AGENT}" \
    --build-arg INCLUDE_NAP_WAF=true \
    --secret id=nginx-repo.crt,src="${ROOT_DIR}/nginx-repo.crt" \
    --secret id=nginx-repo.key,src="${ROOT_DIR}/nginx-repo.key" \
    -f "${DOCKERFILE_TMP}" \
    -t "${NGINX_PLUS_PREFIX}:${TAG}" \
    "${ROOT_DIR}"

echo "Successfully built ${NGINX_PLUS_PREFIX}:${TAG}"
