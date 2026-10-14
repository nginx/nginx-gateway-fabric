#!/usr/bin/env bash
#
# Tests for previous-release-tag.sh, against a stubbed `git tag -l`.
#
# Run directly: bash .github/scripts/previous-release-tag_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/previous-release-tag.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

PASSED=0
FAILED=0

# Tags as they are laid out in the real repository: patch tags on release
# branches, an rc and an early pre-release tag that `git describe` used to hit.
cat >"${TMP}/tags" <<'EOF'
v0.1.0-rc.1
v1.0.0
v2.6.0
v2.6.8
v2.7.0
v2.7.1
v2.7.2
v2.7.9
v2.7.10
v2.8.0-rc.1
EOF

cat >"${TMP}/git" <<STUB
#!/usr/bin/env bash
[ "\$1 \$2" = "tag -l" ] || { echo "unexpected git \$*" >&2; exit 1; }
cat "${TMP}/tags"
STUB
chmod +x "${TMP}/git"

check() {
    local name="$1" version="$2" want="$3" got rc
    got="$(GIT="${TMP}/git" "${SCRIPT}" "${version}" 2>&1)"
    rc=$?
    if [ "${rc}" -eq 0 ] && [ "${got}" = "${want}" ]; then
        printf 'ok    %s\n' "${name}"
        PASSED=$((PASSED + 1))
    else
        printf 'FAIL  %s\n      want %q (rc 0), got %q (rc %d)\n' "${name}" "${want}" "${got}" "${rc}"
        FAILED=$((FAILED + 1))
    fi
}

refuses() {
    local name="$1" got rc
    shift
    got="$(GIT="${TMP}/git" "${SCRIPT}" "$@" 2>&1)"
    rc=$?
    if [ "${rc}" -eq 2 ]; then
        printf 'ok    %s\n' "${name}"
        PASSED=$((PASSED + 1))
    else
        printf 'FAIL  %s\n      want rc 2, got rc %d: %s\n' "${name}" "${rc}" "${got}"
        FAILED=$((FAILED + 1))
    fi
}

check "a minor release follows the last patch of the previous line" v2.8.0 v2.7.10
check "a patch follows the patch before it" v2.7.2 v2.7.1
check "patches sort numerically, not as text" v2.7.10 v2.7.9
check "an older line ignores newer releases" v2.6.9 v2.6.8
check "an existing tag is found by its own position" v2.7.1 v2.7.0
check "pre-release tags are never the start" v1.0.1 v1.0.0
check "the first release has nothing before it" v0.9.0 ""

refuses "a pre-release version is refused" v2.8.0-rc.2
refuses "a version without the v is refused" 2.8.0
refuses "no version is refused"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
