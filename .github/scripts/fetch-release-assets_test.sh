#!/usr/bin/env bash
# Tests for fetch-release-assets.sh, against stubbed az and cosign.
# Exit status: 0 all passed, 1 any failed.

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/fetch-release-assets.sh"
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

STORE="${TMP}/store"
mkdir -p "${STORE}"
printf 'amd64 archive\n' >"${STORE}/ngf_2.8.0_linux_amd64.tar.gz"
printf 'arm64 archive\n' >"${STORE}/ngf_2.8.0_linux_arm64.tar.gz"
(cd "${STORE}" && sha256sum ngf_2.8.0_linux_amd64.tar.gz ngf_2.8.0_linux_arm64.tar.gz >ngf_2.8.0_checksums.txt)
printf 'bundle\n' >"${STORE}/ngf_2.8.0_checksums.txt.sig.bundle"
printf '{}\n' >"${STORE}/ngf_2.8.0_linux_amd64.tar.gz.spdx.json"

# An az that serves downloads from the store directory by blob basename.
cat >"${TMP}/az" <<'STUB'
#!/usr/bin/env bash
out=""; blob=""
while [ $# -gt 0 ]; do
  case "$1" in --file) out="$2"; shift ;; --name) blob="$2"; shift ;; esac
  shift
done
cp "${STORE}/$(basename -- "${blob}")" "${out}"
STUB
chmod +x "${TMP}/az"

COSIGN_LOG="${TMP}/cosign.log"
COSIGN_RC="${TMP}/cosign.rc"
printf '0' >"${COSIGN_RC}"
cat >"${TMP}/cosign" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${COSIGN_LOG}"
exit "$(cat "${COSIGN_RC}")"
STUB
chmod +x "${TMP}/cosign"

# A manifest whose asset entries match the store exactly.
sha_of() { sha256sum "${STORE}/$1" | cut -d' ' -f1; }
entry() { jq -n --arg n "$1" --arg s "$(sha_of "$1")" '{name: $n, blob: ("nginx-gateway-fabric/v2.8.0/" + $n), sha256: $s}'; }
MANIFEST="${TMP}/manifest.json"
jq -n --argjson assets "$(for f in ngf_2.8.0_linux_amd64.tar.gz ngf_2.8.0_linux_arm64.tar.gz ngf_2.8.0_checksums.txt ngf_2.8.0_checksums.txt.sig.bundle ngf_2.8.0_linux_amd64.tar.gz.spdx.json; do entry "$f"; done | jq -s .)" \
    '{schema_version: 1, release_version: "v2.8.0", assets: $assets}' >"${MANIFEST}"

run_fetch() {
    local out="$1"
    shift
    : >"${COSIGN_LOG}"
    env AZ="${TMP}/az" STORE="${STORE}" COSIGN="${TMP}/cosign" COSIGN_LOG="${COSIGN_LOG}" COSIGN_RC="${COSIGN_RC}" \
        MANIFEST="${MANIFEST}" OUT_DIR="${out}" STORAGE_ACCOUNT=acct STORAGE_CONTAINER=bucket \
        SIGNER_REPOSITORY="example-org/the-mirror" "$@" "${SCRIPT}" 2>&1
}

out="$(run_fetch "${TMP}/out1")"
rc=$?
if [ "${rc}" -eq 0 ]; then ok "matching assets fetch and verify"; else no "matching assets fetch and verify" "${out}"; fi
if printf '%s' "${out}" | grep -q '^count=5$'; then ok "the count is reported"; else no "the count is reported" "${out}"; fi
if [ -f "${TMP}/out1/ngf_2.8.0_linux_arm64.tar.gz" ]; then ok "assets land in OUT_DIR"; else no "assets land in OUT_DIR"; fi

args="$(cat "${COSIGN_LOG}")"
if printf '%s' "${args}" | grep -qF -- "--bundle ${TMP}/out1/ngf_2.8.0_checksums.txt.sig.bundle"; then
    ok "the checksums bundle is what cosign verifies"
else
    no "the checksums bundle is what cosign verifies" "${args}"
fi
if printf '%s' "${args}" | grep -qF -- 'github\.com/example-org/the-mirror/\.github/workflows/release-prep\.yml@refs/heads/internal/release-'; then
    ok "the checksums signer identity is the prep workflow on an internal release branch"
else
    no "the checksums signer identity is the prep workflow on an internal release branch" "${args}"
fi

cp "${STORE}/ngf_2.8.0_linux_amd64.tar.gz" "${TMP}/amd64.orig"
printf 'tampered\n' >"${STORE}/ngf_2.8.0_linux_amd64.tar.gz"
out="$(run_fetch "${TMP}/out2")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "does not match the manifest"; then
    ok "an archive that differs from the manifest is refused"
else
    no "an archive that differs from the manifest is refused" "rc ${rc}" "${out}"
fi
if [ ! -s "${COSIGN_LOG}" ]; then ok "a manifest mismatch is refused before any signature check"; else no "a manifest mismatch is refused before any signature check"; fi
cp "${TMP}/amd64.orig" "${STORE}/ngf_2.8.0_linux_amd64.tar.gz"

# A checksums file whose signature does not verify.
printf '1' >"${COSIGN_RC}"
out="$(run_fetch "${TMP}/out3")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "checksums file is not signed"; then
    ok "an unverifiable checksums signature is refused"
else
    no "an unverifiable checksums signature is refused" "rc ${rc}" "${out}"
fi
printf '0' >"${COSIGN_RC}"

# The store's checksums file disagrees with the archives it lists -- prep's own checksums were wrong.
cp "${STORE}/ngf_2.8.0_checksums.txt" "${TMP}/checksums.orig"
sed -i 's/^[0-9a-f]\{64\}/0000000000000000000000000000000000000000000000000000000000000000/' "${STORE}/ngf_2.8.0_checksums.txt"
jq --arg s "$(sha_of ngf_2.8.0_checksums.txt)" '(.assets[] | select(.name == "ngf_2.8.0_checksums.txt") | .sha256) = $s' "${MANIFEST}" >"${TMP}/m2.json"
out="$(MANIFEST="${TMP}/m2.json" run_fetch "${TMP}/out4")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "does not match the signed checksums file"; then
    ok "archives that disagree with the checksums file are refused"
else
    no "archives that disagree with the checksums file are refused" "rc ${rc}" "${out}"
fi
cp "${TMP}/checksums.orig" "${STORE}/ngf_2.8.0_checksums.txt"

jq '.assets = []' "${MANIFEST}" >"${TMP}/empty.json"
out="$(MANIFEST="${TMP}/empty.json" run_fetch "${TMP}/out5")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "lists no assets"; then
    ok "a manifest with no assets is refused"
else
    no "a manifest with no assets is refused" "rc ${rc}" "${out}"
fi

jq '.assets[0].sha256 = "v2.8.0"' "${MANIFEST}" >"${TMP}/badsha.json"
out="$(MANIFEST="${TMP}/badsha.json" run_fetch "${TMP}/out6")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "malformed asset entries"; then
    ok "an asset without a usable sha256 is refused before downloading"
else
    no "an asset without a usable sha256 is refused before downloading" "rc ${rc}" "${out}"
fi

jq '.assets[0].name = "../escape.tar.gz"' "${MANIFEST}" >"${TMP}/traversal.json"
out="$(MANIFEST="${TMP}/traversal.json" run_fetch "${TMP}/out7")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "malformed asset entries"; then
    ok "an asset name with a path separator is refused"
else
    no "an asset name with a path separator is refused" "rc ${rc}" "${out}"
fi

jq 'del(.assets[] | select(.name == "ngf_2.8.0_checksums.txt.sig.bundle"))' "${MANIFEST}" >"${TMP}/nobundle.json"
out="$(MANIFEST="${TMP}/nobundle.json" run_fetch "${TMP}/out8")"
rc=$?
if [ "${rc}" -eq 1 ] && printf '%s' "${out}" | grep -q "no signature bundle"; then
    ok "assets without the checksums bundle are refused"
else
    no "assets without the checksums bundle are refused" "rc ${rc}" "${out}"
fi

out="$(run_fetch "${TMP}/out9" SIGNER_REPOSITORY=)"
rc=$?
if [ "${rc}" -eq 2 ]; then ok "a missing signer repository is a usage error"; else no "a missing signer repository is a usage error" "rc ${rc}"; fi

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
