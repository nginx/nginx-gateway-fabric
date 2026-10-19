#!/usr/bin/env bash
# Tests for release-manifest-store.sh, against a stubbed az backed by a
# directory. Exit status: 0 all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/release-manifest-store.sh"
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

# An az whose container is a directory: upload copies in, download copies out.
STORE="${TMP}/store"
AZ_LOG="${TMP}/az.log"
mkdir -p "${STORE}"
cat >"${TMP}/az" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${AZ_LOG}"
verb="$3"; file=""; name=""
shift 3
while [ $# -gt 0 ]; do
    case "$1" in
    --file) file="$2"; shift 2 ;;
    --name) name="$2"; shift 2 ;;
    *) shift ;;
    esac
done
case "${verb}" in
upload) mkdir -p "$(dirname "${STORE}/${name}")" && cp "${file}" "${STORE}/${name}" ;;
download) [ -f "${STORE}/${name}" ] && cp "${STORE}/${name}" "${file}" ;;
*) exit 9 ;;
esac
STUB
chmod +x "${TMP}/az"

run_store() {
    local action="$1"
    shift
    env AZ="${TMP}/az" AZ_LOG="${AZ_LOG}" STORE="${STORE}" \
        RELEASE_VERSION=v2.8.0 STORAGE_ACCOUNT=acct STORAGE_CONTAINER=bucket \
        "$@" "${SCRIPT}" "${action}"
}

# The manifest ends in a newline, as jq writes it: the bytes the signature covers.
SRC="${TMP}/src"
mkdir -p "${SRC}"
printf '{"release_version":"v2.8.0"}\n' >"${SRC}/m.json"
printf '{"bundle":true}\n' >"${SRC}/b.json"
want_sha="$(sha256sum "${SRC}/m.json" | cut -d' ' -f1)"

: >"${AZ_LOG}"
out="$(run_store put MANIFEST="${SRC}/m.json" SIGNATURE_BUNDLE="${SRC}/b.json" 2>/dev/null)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "put stores the manifest and its bundle"; else no "put stores the manifest and its bundle" "rc ${rc}"; fi
stored="${STORE}/nginx-gateway-fabric/v2.8.0/manifests/${want_sha}"
if [ -f "${stored}/release-manifest.json" ] && [ -f "${stored}/release-manifest.sigstore.json" ]; then
    ok "they go under <prefix>/<version>/manifests/<sha256>/"
else
    no "they go under <prefix>/<version>/manifests/<sha256>/" "$(find "${STORE}" -type f)"
fi
if [ "${out}" = "sha256=${want_sha}" ]; then ok "put prints the manifest's sha256"; else
    no "put prints the manifest's sha256" "got '${out}'"
fi
if grep -q -- "--auth-mode=login" "${AZ_LOG}" && grep -q -- "--container-name bucket --account-name acct" "${AZ_LOG}"; then
    ok "uploads use login auth and name the container and account"
else
    no "uploads use login auth and name the container and account" "$(cat "${AZ_LOG}")"
fi

# A second prep for the same version: stored beside the first, not over it.
printf '{"release_version":"v2.8.0","operator_version":"v9"}\n' >"${SRC}/m2.json"
printf '{"bundle":2}\n' >"${SRC}/b2.json"
second_sha="$(sha256sum "${SRC}/m2.json" | cut -d' ' -f1)"
run_store put MANIFEST="${SRC}/m2.json" SIGNATURE_BUNDLE="${SRC}/b2.json" >/dev/null 2>&1
if cmp -s "${SRC}/m.json" "${stored}/release-manifest.json" &&
    cmp -s "${SRC}/b.json" "${stored}/release-manifest.sigstore.json" &&
    [ -f "${STORE}/nginx-gateway-fabric/v2.8.0/manifests/${second_sha}/release-manifest.json" ]; then
    ok "a later prep for the same version does not replace an earlier manifest"
else
    no "a later prep for the same version does not replace an earlier manifest" "$(find "${STORE}" -type f)"
fi

DST="${TMP}/dst"
mkdir -p "${DST}"
: >"${AZ_LOG}"
out="$(run_store get MANIFEST_SHA256="${want_sha}" MANIFEST="${DST}/m.json" SIGNATURE_BUNDLE="${DST}/b.json" 2>/dev/null)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "get fetches the manifest it is asked for"; else no "get fetches the manifest it is asked for" "rc ${rc}"; fi
if cmp -s "${SRC}/m.json" "${DST}/m.json" && cmp -s "${SRC}/b.json" "${DST}/b.json"; then
    ok "the round trip keeps every byte, trailing newline included, and the bundle from the same run"
else
    no "the round trip keeps every byte, trailing newline included, and the bundle from the same run" "$(od -c "${DST}/m.json" | tail -2)"
fi
if [ "${out}" = "sha256=${want_sha}" ]; then ok "get prints the same sha256 put did"; else
    no "get prints the same sha256 put did" "got '${out}'"
fi
if grep -q -- "--overwrite" "${AZ_LOG}"; then no "downloads do not pass --overwrite" "$(cat "${AZ_LOG}")"; else
    ok "downloads do not pass --overwrite"
fi

err="$(run_store get MANIFEST_SHA256="$(printf 'f%.0s' {1..64})" MANIFEST="${DST}/x.json" SIGNATURE_BUNDLE="${DST}/y.json" 2>&1 >/dev/null)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "run summary" <<<"${err}"; then
    ok "a sha256 prep never stored fails and says where to find the right one"
else
    no "a sha256 prep never stored fails and says where to find the right one" "rc ${rc}: ${err}"
fi

# The store serves the wrong bytes under the right path.
printf 'tampered\n' >"${stored}/release-manifest.json"
err="$(run_store get MANIFEST_SHA256="${want_sha}" MANIFEST="${DST}/t.json" SIGNATURE_BUNDLE="${DST}/tb.json" 2>&1 >/dev/null)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "does not hash to its path" <<<"${err}"; then
    ok "bytes that do not match the requested sha256 are refused"
else
    no "bytes that do not match the requested sha256 are refused" "rc ${rc}: ${err}"
fi

rm "${STORE}/nginx-gateway-fabric/v2.8.0/manifests/${second_sha}/release-manifest.sigstore.json"
err="$(run_store get MANIFEST_SHA256="${second_sha}" MANIFEST="${DST}/m2.json" SIGNATURE_BUNDLE="${DST}/b2.json" 2>&1 >/dev/null)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "no signature bundle" <<<"${err}"; then
    ok "a manifest without its bundle fails"
else
    no "a manifest without its bundle fails" "rc ${rc}: ${err}"
fi

assert_usage() {
    local desc="$1" rc
    shift
    "$@" >/dev/null 2>&1
    rc=$?
    if [ "${rc}" = "2" ]; then ok "${desc}"; else no "${desc}" "expected exit 2, got ${rc}"; fi
}

assert_usage "no action is a usage error" run_store ""
assert_usage "an unknown action is a usage error" run_store copy
assert_usage "a version without the v prefix is rejected" run_store get RELEASE_VERSION=2.8.0 MANIFEST_SHA256="${want_sha}"
assert_usage "a version with trailing characters is rejected" run_store put RELEASE_VERSION=v2.8.0foo MANIFEST="${SRC}/m.json" SIGNATURE_BUNDLE="${SRC}/b.json"
assert_usage "a version with a path separator is rejected" run_store put RELEASE_VERSION=v2.8.0/extra MANIFEST="${SRC}/m.json" SIGNATURE_BUNDLE="${SRC}/b.json"
assert_usage "get without a sha256 is a usage error" run_store get
assert_usage "a malformed sha256 is a usage error" run_store get MANIFEST_SHA256=abc
assert_usage "a missing storage account is rejected" run_store get STORAGE_ACCOUNT= MANIFEST_SHA256="${want_sha}"
assert_usage "putting a manifest that does not exist is rejected" run_store put MANIFEST="${TMP}/nope.json"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
