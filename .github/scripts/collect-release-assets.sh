#!/usr/bin/env bash
#
# collect-release-assets.sh
#
# Lists the files in a GoReleaser dist directory that belong on a GitHub
# release. Fails if an expected class is missing, so that a build whose signing
# or SBOM step silently produced nothing cannot be published.
#
# Read from the environment:
#   DIST_DIR         GoReleaser output directory. Required.
#   EXPECT_ARCHIVES  archives required. Default 2, one per architecture.
#
# Prints one path per line, in a stable order.
#
# Exit status: 0 all present, 1 something missing, 2 bad input.

set -euo pipefail

DIST="${DIST_DIR:-}"
EXPECT_ARCHIVES="${EXPECT_ARCHIVES:-2}"

die() {
    echo "error: $*" >&2
    exit 2
}

[ -n "${DIST}" ] || die "DIST_DIR is required"
[ -d "${DIST}" ] || die "dist directory not found: ${DIST}"
printf '%s' "${EXPECT_ARCHIVES}" | grep -Eq '^[0-9]+$' ||
    die "EXPECT_ARCHIVES must be a whole number, got '${EXPECT_ARCHIVES}'"

# find_class <class> -- prints matching files, one per line
find_class() {
    case "$1" in
    archives)
        # Archives, but not their SBOM or signature side-cars.
        find "${DIST}" -maxdepth 1 -type f -name '*.tar.gz' ! -name '*.spdx.json' | sort
        ;;
    checksums)
        find "${DIST}" -maxdepth 1 -type f -name '*checksums.txt' | sort
        ;;
    signatures)
        find "${DIST}" -maxdepth 1 -type f -name '*.sig.bundle' | sort
        ;;
    sboms)
        find "${DIST}" -maxdepth 1 -type f -name '*.spdx.json' | sort
        ;;
    esac
}

errors=0
assets=()

for class in archives checksums signatures sboms; do
    mapfile -t found < <(find_class "${class}")
    count="${#found[@]}"

    if [ "${count}" -eq 0 ]; then
        echo "FAIL: no ${class} found in ${DIST}" >&2
        case "${class}" in
        signatures)
            echo "      The release would be published unsigned. Check that cosign ran" >&2
            echo "      and that the signs section of .goreleaser.yml still applies." >&2
            ;;
        sboms)
            echo "      The release would be published without an SBOM. Check that syft ran." >&2
            ;;
        checksums)
            echo "      Without a checksums file the signature covers nothing, because" >&2
            echo "      signing is configured over the checksum artifact." >&2
            ;;
        esac
        errors=$((errors + 1))
        continue
    fi

    if [ "${class}" = "archives" ] && [ "${count}" -ne "${EXPECT_ARCHIVES}" ]; then
        echo "FAIL: found ${count} archive(s) in ${DIST}, expected ${EXPECT_ARCHIVES}" >&2
        echo "      A release builds one archive per architecture. A different count" >&2
        echo "      usually means the build was single-target, or an architecture" >&2
        echo "      was added without updating the expectation." >&2
        errors=$((errors + 1))
    fi

    if [ "${class}" = "checksums" ] && [ "${count}" -ne 1 ]; then
        echo "FAIL: found ${count} checksums files in ${DIST}, expected exactly 1" >&2
        errors=$((errors + 1))
    fi

    assets+=("${found[@]}")
done

if [ "${errors}" -gt 0 ]; then
    exit 1
fi

printf '%s\n' "${assets[@]}"
