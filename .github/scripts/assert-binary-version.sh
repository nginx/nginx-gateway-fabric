#!/usr/bin/env bash
#
# assert-binary-version.sh
#
# Fails when a GoReleaser build did not stamp the expected version, e.g. a
# snapshot build (which stamps "edge") mistakenly used for release artifacts.
# Reads dist/metadata.json rather than the binaries themselves: a version
# stamped through -ldflags -X is not recoverable from a -trimpath build's
# embedded build info, so the stamp cannot be read back off the binary.
#
# Reads EXPECT_VERSION (the expected version) and DIST_DIR (GoReleaser output
# dir) from the environment; both required, leading "v" ignored on either.
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

# Only the *expectation* is normalised; the stamp is compared as-is, so a
# stamp that has grown a `v` prefix is caught rather than waved through.
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

if [ "${got}" != "${want}" ]; then
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
