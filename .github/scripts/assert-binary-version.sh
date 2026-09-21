#!/usr/bin/env bash
#
# assert-binary-version.sh
#
# Fails when a GoReleaser build did not stamp the expected version.
# snapshot.version_template overrides the git tag, so a snapshot build asked
# for release artifacts would otherwise stamp them "edge" and ship.
#
# The version is read from dist/metadata.json, which records the value
# GoReleaser resolved for {{ .Version }}. That is the value substituted into
# `-X main.version={{ .Version }}`, so it is what the binaries carry.
#
# It is not read back out of the binaries themselves. Go omits the -ldflags
# setting from the embedded build info whenever -trimpath is set
# (https://go.dev/issue/52372), and .goreleaser.yml builds with -trimpath, so
# `go version -m` shows no ldflags for our binaries. There is no --version
# command to run instead, and the arm64 binary could not be run here anyway.
#
# Read from the environment:
#   EXPECT_VERSION  version the build should carry. Required.
#   DIST_DIR        GoReleaser output directory. Required.
#
# A leading "v" is ignored on both sides.
#
# Exit status: 0 stamped as expected, 1 not, or nothing was built, 2 bad input.

set -euo pipefail

EXPECT="${EXPECT_VERSION:-}"
DIST="${DIST_DIR:-}"
BINARIES=()

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${EXPECT}" ] || die "EXPECT_VERSION is required"
[ -n "${DIST}" ] || die "DIST_DIR is required"
[ -d "${DIST}" ] || die "dist directory not found: ${DIST}"
command -v jq >/dev/null || die "jq is required to read ${DIST}/metadata.json"

metadata="${DIST}/metadata.json"
[ -f "${metadata}" ] || die "GoReleaser metadata not found: ${metadata}"

# The version means nothing if no binary was built with it.
while IFS= read -r found; do
    [ -n "${found}" ] && BINARIES+=("${found}")
done < <(find "${DIST}" -type f -name gateway | sort)

if [ "${#BINARIES[@]}" -eq 0 ]; then
    echo "FAIL: no gateway binaries found under ${DIST}."
    echo "Nothing was built, which is not the same as everything being correct."
    exit 1
fi

# GoReleaser stamps 2.0.3 for tag v2.0.3.
normalise() {
    printf '%s' "${1#v}"
}

want="$(normalise "${EXPECT}")"
got="$(jq -r '.version // empty' "${metadata}")"

if [ -z "${got}" ]; then
    echo "FAIL: ${metadata} has no version field."
    echo "      GoReleaser did not resolve a version, so the binaries carry none."
    exit 1
fi

if [ "$(normalise "${got}")" != "${want}" ]; then
    cat <<EOF
FAIL: the build is stamped with the wrong version.
      stamped:  ${got}
      expected: ${EXPECT}

A snapshot build takes its version from snapshot.version_template, which
overrides the git tag, so NGF_VERSION must be set for the stamp to be
anything other than "edge".
EOF
    exit 1
fi

for bin in "${BINARIES[@]}"; do
    echo "ok: ${bin}"
done
echo "All ${#BINARIES[@]} binary/binaries built with version ${got}."
