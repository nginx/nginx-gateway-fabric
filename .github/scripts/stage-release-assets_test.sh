#!/usr/bin/env bash
# Tests for stage-release-assets.sh, against a stubbed az. Exit status: 0 all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/stage-release-assets.sh"
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

# An az that records uploads and can be told to fail.
AZ_LOG="${TMP}/az.log"
AZ_RC="${TMP}/az.rc"
printf '0' >"${AZ_RC}"
cat >"${TMP}/az" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${AZ_LOG}"
exit "$(cat "${AZ_RC}")"
STUB
chmod +x "${TMP}/az"

# A complete GoReleaser dist: two archives, checksums, its bundle, two SBOMs.
DIST="${TMP}/dist"
mkdir -p "${DIST}"
printf 'amd64 archive\n' >"${DIST}/nginx-gateway-fabric_2.8.0_linux_amd64.tar.gz"
printf 'arm64 archive\n' >"${DIST}/nginx-gateway-fabric_2.8.0_linux_arm64.tar.gz"
printf 'checksums\n' >"${DIST}/nginx-gateway-fabric_2.8.0_checksums.txt"
printf 'bundle\n' >"${DIST}/nginx-gateway-fabric_2.8.0_checksums.txt.sig.bundle"
printf '{}\n' >"${DIST}/nginx-gateway-fabric_2.8.0_linux_amd64.tar.gz.spdx.json"
printf '{}\n' >"${DIST}/nginx-gateway-fabric_2.8.0_linux_arm64.tar.gz.spdx.json"

run_stage() {
    : >"${AZ_LOG}"
    env AZ="${TMP}/az" AZ_LOG="${AZ_LOG}" AZ_RC="${AZ_RC}" \
        DIST_DIR="${DIST}" RELEASE_VERSION=v2.8.0 \
        STORAGE_ACCOUNT=acct STORAGE_CONTAINER=bucket "$@" "${SCRIPT}" 2>/dev/null
}

out="$(run_stage)"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "a complete inventory stages"; else no "a complete inventory stages" "rc ${rc}"; fi

n="$(printf '%s' "${out}" | jq 'length')"
if [ "${n}" = "6" ]; then ok "all six assets are recorded"; else no "all six assets are recorded" "got ${n}"; fi

uploads="$(grep -c '^storage blob upload' "${AZ_LOG}")"
if [ "${uploads}" = "6" ]; then ok "all six assets are uploaded"; else no "all six assets are uploaded" "got ${uploads}"; fi

blob="$(printf '%s' "${out}" | jq -r '.[] | select(.name == "nginx-gateway-fabric_2.8.0_checksums.txt") | .blob')"
if [ "${blob}" = "nginx-gateway-fabric/v2.8.0/nginx-gateway-fabric_2.8.0_checksums.txt" ]; then
    ok "the blob path is <prefix>/<version>/<file>"
else
    no "the blob path is <prefix>/<version>/<file>" "got '${blob}'"
fi
if grep -q -- "--name nginx-gateway-fabric/v2.8.0/nginx-gateway-fabric_2.8.0_checksums.txt" "${AZ_LOG}"; then
    ok "the upload targets the recorded blob path"
else
    no "the upload targets the recorded blob path" "$(cat "${AZ_LOG}")"
fi
if grep -q -- "--container-name bucket --account-name acct" "${AZ_LOG}"; then
    ok "the upload names the container and account"
else
    no "the upload names the container and account"
fi
if grep -q -- "--auth-mode=login --overwrite" "${AZ_LOG}"; then
    ok "uploads use login auth and overwrite a re-run"
else
    no "uploads use login auth and overwrite a re-run"
fi

want="$(sha256sum "${DIST}/nginx-gateway-fabric_2.8.0_linux_amd64.tar.gz" | cut -d' ' -f1)"
got="$(printf '%s' "${out}" | jq -r '.[] | select(.name == "nginx-gateway-fabric_2.8.0_linux_amd64.tar.gz") | .sha256')"
if [ "${got}" = "${want}" ]; then ok "the recorded sha256 is the file's"; else no "the recorded sha256 is the file's" "want ${want}" "got  ${got}"; fi

bytes="$(printf '%s' "${out}" | jq -r '.[] | select(.name == "nginx-gateway-fabric_2.8.0_linux_amd64.tar.gz") | .bytes')"
if [ "${bytes}" = "14" ]; then ok "the recorded size is the file's"; else no "the recorded size is the file's" "got ${bytes}"; fi

out2="$(run_stage BLOB_PREFIX=other/prefix/)"
blob2="$(printf '%s' "${out2}" | jq -r '.[0].blob')"
case "${blob2}" in
other/prefix/v2.8.0/*) ok "a prefix override is honoured and a trailing slash does not double" ;;
*) no "a prefix override is honoured and a trailing slash does not double" "got '${blob2}'" ;;
esac

printf '1' >"${AZ_RC}"
run_stage >/dev/null
rc=$?
if [ "${rc}" -eq 1 ]; then ok "a failed upload fails the staging"; else no "a failed upload fails the staging" "rc ${rc}"; fi
printf '0' >"${AZ_RC}"

rm "${DIST}/nginx-gateway-fabric_2.8.0_checksums.txt.sig.bundle"
run_stage >/dev/null
rc=$?
if [ "${rc}" -eq 1 ]; then ok "an inventory missing its signature bundle stages nothing"; else no "an inventory missing its signature bundle stages nothing" "rc ${rc}"; fi
if [ ! -s "${AZ_LOG}" ]; then ok "nothing is uploaded when the inventory is incomplete"; else no "nothing is uploaded when the inventory is incomplete" "$(cat "${AZ_LOG}")"; fi
printf 'bundle\n' >"${DIST}/nginx-gateway-fabric_2.8.0_checksums.txt.sig.bundle"

run_stage RELEASE_VERSION=2.8.0 >/dev/null
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a version without the v prefix is a usage error"; else no "a version without the v prefix is a usage error" "rc ${rc}"; fi

run_stage STORAGE_CONTAINER= >/dev/null
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a missing container is a usage error"; else no "a missing container is a usage error" "rc ${rc}"; fi

run_stage DIST_DIR="${TMP}/nope" >/dev/null
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a missing dist directory is a usage error"; else no "a missing dist directory is a usage error" "rc ${rc}"; fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
