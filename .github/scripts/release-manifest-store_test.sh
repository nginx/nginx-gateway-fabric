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
if [ -f "${STORE}/nginx-gateway-fabric/v2.8.0/release-manifest.json" ] &&
    [ -f "${STORE}/nginx-gateway-fabric/v2.8.0/release-manifest.sigstore.json" ]; then
    ok "they go under <prefix>/<version>/ beside the assets"
else
    no "they go under <prefix>/<version>/ beside the assets" "$(find "${STORE}" -type f)"
fi
if [ "${out}" = "sha256=${want_sha}" ]; then ok "put prints the manifest's sha256"; else
    no "put prints the manifest's sha256" "got '${out}'"
fi
if grep -q -- "--auth-mode=login" "${AZ_LOG}" && grep -q -- "--overwrite" "${AZ_LOG}" &&
    grep -q -- "--container-name bucket --account-name acct" "${AZ_LOG}"; then
    ok "uploads use login auth, overwrite a re-run, and name the container and account"
else
    no "uploads use login auth, overwrite a re-run, and name the container and account" "$(cat "${AZ_LOG}")"
fi

DST="${TMP}/dst"
mkdir -p "${DST}"
: >"${AZ_LOG}"
out="$(run_store get MANIFEST="${DST}/m.json" SIGNATURE_BUNDLE="${DST}/b.json" 2>/dev/null)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "get fetches the manifest and its bundle"; else no "get fetches the manifest and its bundle" "rc ${rc}"; fi
if cmp -s "${SRC}/m.json" "${DST}/m.json" && cmp -s "${SRC}/b.json" "${DST}/b.json"; then
    ok "the round trip keeps every byte, trailing newline included"
else
    no "the round trip keeps every byte, trailing newline included" "$(od -c "${DST}/m.json" | tail -2)"
fi
if [ "${out}" = "sha256=${want_sha}" ]; then ok "get prints the same sha256 put did"; else
    no "get prints the same sha256 put did" "got '${out}'"
fi
if grep -q -- "--overwrite" "${AZ_LOG}"; then no "downloads do not pass --overwrite" "$(cat "${AZ_LOG}")"; else
    ok "downloads do not pass --overwrite"
fi

err="$(run_store get RELEASE_VERSION=v9.9.9 MANIFEST="${DST}/x.json" SIGNATURE_BUNDLE="${DST}/y.json" 2>&1 >/dev/null)"
rc=$?
if [ "${rc}" -eq 1 ] && grep -q "Has prep run for v9.9.9" <<<"${err}"; then
    ok "a version prep never stored fails and says so"
else
    no "a version prep never stored fails and says so" "rc ${rc}: ${err}"
fi

rm "${STORE}/nginx-gateway-fabric/v2.8.0/release-manifest.sigstore.json"
err="$(run_store get MANIFEST="${DST}/m.json" SIGNATURE_BUNDLE="${DST}/b2.json" 2>&1 >/dev/null)"
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
assert_usage "a version without the v prefix is rejected" run_store get RELEASE_VERSION=2.8.0
assert_usage "a missing storage account is rejected" run_store get STORAGE_ACCOUNT=
assert_usage "putting a manifest that does not exist is rejected" run_store put MANIFEST="${TMP}/nope.json"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
