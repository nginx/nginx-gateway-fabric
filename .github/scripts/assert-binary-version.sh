#!/usr/bin/env bash
#
# assert-binary-version.sh
#
# Fails when a built gateway binary is not stamped with the expected version.
# snapshot.version_template overrides the git tag, so a snapshot build asked
# for release artifacts would otherwise stamp them "edge" and ship.
#
# Read from the environment:
#   EXPECT_VERSION  version the binaries should carry. Required.
#   DIST_DIR        GoReleaser output directory to search for gateway binaries.
#   GO_CMD          the `go` to read build info with. Default: go.
#
# A leading "v" is ignored on both sides.
#
# Exit status: 0 all stamped, 1 one is not or none found, 2 bad input.

set -euo pipefail

GO_CMD="${GO_CMD:-go}"
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

while IFS= read -r found; do
    [ -n "${found}" ] && BINARIES+=("${found}")
done < <(find "${DIST}" -type f -name gateway | sort)

if [ "${#BINARIES[@]}" -eq 0 ]; then
    echo "FAIL: no gateway binaries to check."
    echo "Nothing was verified, which is not the same as everything being correct."
    exit 1
fi

# GoReleaser stamps 2.0.3 for tag v2.0.3.
normalise() {
    printf '%s' "${1#v}"
}

want="$(normalise "${EXPECT}")"

# Read -X main.version= from the Go build info, which survives the release
# build's -s -w and -trimpath.
stamped_version() {
    local bin="$1"
    "${GO_CMD}" version -m "${bin}" 2>/dev/null |
        awk '
      /-ldflags/ {
        if (match($0, /-X main\.version=[^ \t"]+/)) {
          v = substr($0, RSTART, RLENGTH)
          sub(/-X main\.version=/, "", v)
          print v
          exit
        }
      }
    '
}

failed=0
for bin in "${BINARIES[@]}"; do
    got="$(stamped_version "${bin}")"

    if [ -z "${got}" ]; then
        echo "FAIL: ${bin}: no -X main.version= found in the build info"
        echo "      The ldflag may have been renamed, or the binary was not built by GoReleaser."
        failed=$((failed + 1))
        continue
    fi

    if [ "$(normalise "${got}")" != "${want}" ]; then
        echo "FAIL: ${bin}"
        echo "      stamped:  ${got}"
        echo "      expected: ${EXPECT}"
        failed=$((failed + 1))
        continue
    fi

    echo "ok: ${bin} is stamped ${got}"
done

if [ "${failed}" -gt 0 ]; then
    cat <<EOF

${failed} binary/binaries are not stamped with the expected version.

A snapshot build takes its version from snapshot.version_template, which
overrides the git tag, so NGF_VERSION must be set for the stamp to be
anything other than "edge".
EOF
    exit 1
fi

echo "All ${#BINARIES[@]} binary/binaries stamped ${EXPECT}."
