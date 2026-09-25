#!/usr/bin/env bash
#
# Tests for bump-nginx-versions.sh, run against a scratch copy of the pins.
#
# Run directly: bash .github/scripts/bump-nginx-versions_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/bump-nginx-versions.sh"
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

# A fresh scratch tree holding only the pins the script touches.
new_root() {
    local r="${TMP}/$1"
    mkdir -p "${r}/build/ubi" "${r}/internal/framework/waf"

    cat >"${r}/build/Dockerfile.nginx" <<'EOF'
ARG NGINX_VERSION=1.31.6
ARG NGINX_AGENT_VERSION=v3.12.0
EOF
    cat >"${r}/build/ubi/Dockerfile.nginx" <<'EOF'
ARG NGINX_VERSION=1.31.6
ARG NGINX_AGENT_VERSION=v3.12.0
EOF
    # The Plus images pin NGINX *source* for the Rust bindings, which is a
    # different thing from the OSS package version above despite the name.
    cat >"${r}/build/Dockerfile.nginxplus" <<'EOF'
ARG NGINX_VERSION=1.31.3
ARG NGINX_PLUS_VERSION=R37.1
ARG APP_PROTECT_VERSION=37.1.5.715
ARG NGINX_AGENT_VERSION=v3.12.0
EOF
    cat >"${r}/build/ubi/Dockerfile.nginxplus" <<'EOF'
ARG NGINX_VERSION=1.31.3
ARG NGINX_PLUS_VERSION=R37.1
ARG APP_PROTECT_VERSION=37.1+5.715
ARG NGINX_AGENT_VERSION=v3.12.0
EOF
    cat >"${r}/internal/framework/waf/waf.go" <<'EOF'
package waf

const Release = "5.15.0"
EOF
    printf '%s' "${r}"
}

OUT=""
RC=0
run() {
    local root="$1"
    shift
    OUT=$(BUMP_ROOT="${root}" "${SCRIPT}" "$@" 2>&1)
    RC=$?
}

assert_rc() {
    if [ "${RC}" -eq "$2" ]; then ok "$1"; else
        no "$1" "expected exit ${2}, got ${RC}" "${OUT}"
    fi
}

# assert_pin <description> <file> <ARG name> <expected value>
assert_pin() {
    local got
    got=$(sed -n "s/^ARG $3=//p" "$2")
    if [ "${got}" = "$4" ]; then ok "$1"; else
        no "$1" "expected $3=$4, got '${got}' in $2"
    fi
}

assert_says() {
    if printf '%s' "${OUT}" | grep -qF -- "$2"; then ok "$1"; else
        no "$1" "expected output to mention: $2" "${OUT}"
    fi
}

# One WAF version in, both formats out: apk 38.0.6.800, rpm 38.0+6.800.
r=$(new_root waf)
run "${r}" --nap-waf-module 38.0.6.800
assert_rc "a WAF module bump succeeds" 0
assert_pin "the alpine pin gets the apk format" "${r}/build/Dockerfile.nginxplus" APP_PROTECT_VERSION 38.0.6.800
assert_pin "the ubi pin gets the rpm format" "${r}/build/ubi/Dockerfile.nginxplus" APP_PROTECT_VERSION '38.0+6.800'

# The rpm spelling is a valid output but not a valid input.
r=$(new_root wafrpm)
run "${r}" --nap-waf-module '38.0+6.800'
assert_rc "the rpm format is rejected as input" 2
assert_says "and says what was expected" "expected format"

# Two different pins are both called NGINX_VERSION; bumping one must not touch the other.
r=$(new_root split)
run "${r}" --nginx-oss 1.31.9
assert_rc "an OSS bump succeeds" 0
assert_pin "the OSS image is bumped" "${r}/build/Dockerfile.nginx" NGINX_VERSION 1.31.9
assert_pin "the UBI OSS image is bumped" "${r}/build/ubi/Dockerfile.nginx" NGINX_VERSION 1.31.9
assert_pin "the Plus source pin is untouched" "${r}/build/Dockerfile.nginxplus" NGINX_VERSION 1.31.3
assert_pin "the UBI Plus source pin is untouched" "${r}/build/ubi/Dockerfile.nginxplus" NGINX_VERSION 1.31.3

r=$(new_root split2)
run "${r}" --nginx-source 1.31.7
assert_rc "a source bump succeeds" 0
assert_pin "the Plus source pin is bumped" "${r}/build/Dockerfile.nginxplus" NGINX_VERSION 1.31.7
assert_pin "the OSS package pin is untouched" "${r}/build/Dockerfile.nginx" NGINX_VERSION 1.31.6

# A dry run writes nothing.
r=$(new_root dry)
run "${r}" --dry-run --nginx-plus R38.0
assert_rc "a dry run succeeds" 0
assert_pin "a dry run changes nothing" "${r}/build/Dockerfile.nginxplus" NGINX_PLUS_VERSION R37.1

# A pin that has moved is reported, not silently skipped.
r=$(new_root missing)
sed -i '/^ARG NGINX_PLUS_VERSION=/d' "${r}/build/Dockerfile.nginxplus"
run "${r}" --nginx-plus R38.0
assert_rc "a missing pin fails" 1
assert_says "a missing pin is named" "has no 'ARG NGINX_PLUS_VERSION='"

# Malformed input is deliberately not covered beyond the rpm case above: those
# mistakes fail loudly at the prompt, and --show only reports what the cases
# here already assert. What is covered is what fails quietly or fails late.

# The WAF sidecar release lives in Go, not a Dockerfile.
r=$(new_root gorel)
run "${r}" --nap-waf-release 5.16.0
assert_rc "a WAF release bump succeeds" 0
if grep -q 'const Release = "5.16.0"' "${r}/internal/framework/waf/waf.go"; then
    ok "the Go constant is updated"
else
    no "the Go constant is updated" "$(cat "${r}/internal/framework/waf/waf.go")"
fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
