#!/usr/bin/env bash
#
# Tests for the release helper scripts.
#
# Deliberately small: only where each image is published, and whether an
# incomplete release can be published. Everything else fails loudly on its own.
#
# Usage: release-scripts_test.sh
# Exit status: 0 all passed, 1 one or more failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

PASSED=0
FAILED=0

ok() {
    printf 'ok    %s\n' "$1"
    PASSED=$((PASSED + 1))
}

no() {
    printf 'FAIL  %s\n' "$1"
    shift
    for l in "$@"; do printf '      %s\n' "${l}"; done
    FAILED=$((FAILED + 1))
}

# ---------------------------------------------------------------------------
# Where each image is published. These are the repositories build.yml used
# before the mapping was extracted, so an edit that moves one publishes a
# release somewhere else.
# ---------------------------------------------------------------------------
target() {
    IMAGE="$1" OWNER=nginx "${DIR}/resolve-image-target.sh" 2>&1 | sed -n "s/^target=//p"
}

check_target() {
    local got
    got="$(target "$1")"
    [ "${got}" = "$2" ] && ok "$1 publishes to $2" ||
        no "$1 publishes to the expected repository" "got:      ${got}" "expected: $2"
}

check_target ngf "ghcr.io/nginx/nginx-gateway-fabric"
check_target nginx "ghcr.io/nginx/nginx-gateway-fabric/nginx"
check_target operator "ghcr.io/nginx/nginx-gateway-fabric/operator"
check_target plus "docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus"
check_target plus-nap-waf "docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus-f5waf"

# An unknown image must not resolve to a plausible-looking repository.
if IMAGE=nginx-plus OWNER=nginx "${DIR}/resolve-image-target.sh" >/dev/null 2>&1; then
    no "an unrecognised image is rejected rather than passed through"
else
    ok "an unrecognised image is rejected rather than passed through"
fi

# ---------------------------------------------------------------------------
# An incomplete release must not be publishable.
# ---------------------------------------------------------------------------
dist="${TMP}/dist"
mkdir -p "${dist}"
for f in \
    nginx-gateway-fabric_1.0.0_linux_amd64.tar.gz \
    nginx-gateway-fabric_1.0.0_linux_arm64.tar.gz \
    nginx-gateway-fabric_1.0.0_linux_amd64.tar.gz.spdx.json \
    nginx-gateway-fabric_1.0.0_linux_arm64.tar.gz.spdx.json \
    nginx-gateway-fabric_1.0.0_checksums.txt \
    nginx-gateway-fabric_1.0.0_checksums.txt.sig.bundle; do
    : >"${dist}/${f}"
done
# Binaries and build metadata live here too and are not release assets.
mkdir -p "${dist}/gateway_linux_amd64_v1"
: >"${dist}/gateway_linux_amd64_v1/gateway"
: >"${dist}/metadata.json"

assets="$(DIST_DIR="${dist}" "${DIR}/collect-release-assets.sh" 2>&1)"
if [ "$(printf '%s\n' "${assets}" | grep -c .)" = "6" ] &&
    ! printf '%s' "${assets}" | grep -Fq "metadata.json"; then
    ok "a complete build yields exactly the six release assets"
else
    no "a complete build yields exactly the six release assets" "${assets}"
fi

rm "${dist}"/*.sig.bundle
if DIST_DIR="${dist}" "${DIR}/collect-release-assets.sh" >/dev/null 2>&1; then
    no "a build with no signature cannot be published"
else
    ok "a build with no signature cannot be published"
fi

printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
