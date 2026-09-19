#!/usr/bin/env bash
#
# Tests for the release helper scripts.
#
# Deliberately small: where each image is published, whether an incomplete
# release can be published, and whether a mis-stamped build can pass. Everything
# else fails loudly on its own.
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

# ---------------------------------------------------------------------------
# A build stamped with the wrong version must not pass. The version comes from
# GoReleaser's metadata.json, because -trimpath keeps it out of the binaries.
# ---------------------------------------------------------------------------
assert_version() {
    EXPECT_VERSION="$1" DIST_DIR="${dist}" "${DIR}/assert-binary-version.sh" >/dev/null 2>&1
}

printf '{"project_name":"nginx-gateway-fabric","version":"2.0.3","tag":"v2.0.3"}\n' >"${dist}/metadata.json"

if assert_version 2.0.3; then
    ok "a build stamped with the expected version passes"
else
    no "a build stamped with the expected version passes"
fi

if assert_version v2.0.3; then
    ok "a leading v on the expected version is ignored"
else
    no "a leading v on the expected version is ignored"
fi

if assert_version edge; then
    no "a release build stamped edge is rejected"
else
    ok "a release build stamped edge is rejected"
fi

# The stamp is compared as-is. A build that has grown a `v` is wrong even
# when the number is right: every release before the split stamped 2.0.3,
# and an assertion that normalised both sides could not tell the difference.
printf '{"project_name":"nginx-gateway-fabric","version":"v2.0.3","tag":"v2.0.3"}\n' >"${dist}/metadata.json"

if assert_version 2.0.3; then
    no "a build stamped with a leading v is rejected"
else
    ok "a build stamped with a leading v is rejected"
fi

printf '{"project_name":"nginx-gateway-fabric","version":"2.0.3","tag":"v2.0.3"}\n' >"${dist}/metadata.json"

rm "${dist}/gateway_linux_amd64_v1/gateway"
if assert_version 2.0.3; then
    no "a build that produced no binaries is rejected"
else
    ok "a build that produced no binaries is rejected"
fi

printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
