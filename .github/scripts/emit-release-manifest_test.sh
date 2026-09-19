#!/usr/bin/env bash
#
# Tests for emit-release-manifest.sh.
#
# The manifest is the only thing publish trusts, and publish runs weeks later
# in a different repository against a public registry. Every guard here exists
# because the failure it prevents is silent: a manifest that parses but says
# the wrong thing produces a wrong release, not an error.
#
# Usage: emit-release-manifest_test.sh
# Exit status: 0 all passed, 1 one or more failed.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EMIT="${SCRIPT_DIR}/emit-release-manifest.sh"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "${TMP_ROOT}"' EXIT

PASSED=0
FAILED=0

SHA_A="1111111111111111111111111111111111111111"
TREE_A="2222222222222222222222222222222222222222"
DIG_A="sha256:$(printf 'a%.0s' {1..64})"
DIG_B="sha256:$(printf 'b%.0s' {1..64})"

# digest_dir <name> -- a directory holding one valid ngf record
digest_dir() {
    local d="${TMP_ROOT}/$1"
    mkdir -p "${d}"
    cat >"${d}/ngf.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/nginx-gateway-fabric","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
    printf '%s' "${d}"
}

# run_emit <digest-dir> [VAR=value ...] -- runs with a valid baseline env
run_emit() {
    local dir="$1"
    shift
    env \
        RELEASE_VERSION=v2.8.0 \
        OPERATOR_VERSION=v0.3.0 \
        INTERNAL_BRANCH=internal/release-2.8 \
        INTERNAL_SHA="${SHA_A}" \
        TREE_HASH="${TREE_A}" \
        DIGEST_DIR="${dir}" \
        "$@" \
        "${EMIT}" 2>&1
}

pass() {
    printf 'ok    %s\n' "$1"
    PASSED=$((PASSED + 1))
}

fail() {
    printf 'FAIL  %s\n      %s\n' "$1" "$2"
    FAILED=$((FAILED + 1))
}

# check_eq <name> <want> <got>
#
# Written as if/else rather than `[ ... ] && pass || fail`: that shape runs
# the third branch when the second one fails, so a bookkeeping slip inside
# pass would silently report a failure instead.
check_eq() {
    local name="$1" want="$2" got="$3"
    if [ "${got}" = "${want}" ]; then
        pass "${name}"
    else
        fail "${name}" "expected '${want}', got '${got}'"
    fi
}

# expect_fail <name> <substring> <dir> [VAR=value ...]
expect_fail() {
    local name="$1" want="$2" dir="$3"
    shift 3
    local out
    if out="$(run_emit "${dir}" "$@")"; then
        fail "${name}" "expected a refusal, got success"
        return
    fi
    if ! printf '%s' "${out}" | grep -Fq "${want}"; then
        fail "${name}" "expected message containing '${want}', got: ${out}"
        return
    fi
    pass "${name}"
}

# ---------------------------------------------------------------------------
# The happy path, and the shape publish depends on
# ---------------------------------------------------------------------------
d="$(digest_dir happy)"
manifest="$(run_emit "${d}")"

check_field() {
    local name="$1" filter="$2" want="$3" got
    got="$(printf '%s' "${manifest}" | jq -r "${filter}")"
    check_eq "${name}" "${want}" "${got}"
}

check_field "a schema version is emitted" '.schema_version' "1"
check_field "the release version is recorded" '.release_version' "v2.8.0"
check_field "the operator version is recorded" '.operator_version' "v0.3.0"
check_field "the internal branch is recorded" '.source.internal_branch' "internal/release-2.8"
check_field "the internal SHA is recorded" '.source.internal_sha' "${SHA_A}"
check_field "the tree hash is recorded" '.source.tree_hash' "${TREE_A}"
check_field "the image digest is recorded" '.images[0].digest' "${DIG_A}"
check_field "the image target is recorded" '.images[0].target' "reg.example/nginx-gateway-fabric"
check_field "an unset chart digest is null, not empty string" '.chart.digest' "null"
check_field "nginx_versions defaults to an object" '.nginx_versions | type' "object"
check_field "assets defaults to an array" '.assets | type' "array"

# ---------------------------------------------------------------------------
# Required fields. Each of these would otherwise produce a manifest that
# publish accepts and misreads.
# ---------------------------------------------------------------------------
d="$(digest_dir required)"
for var in RELEASE_VERSION OPERATOR_VERSION INTERNAL_BRANCH INTERNAL_SHA TREE_HASH; do
    expect_fail "an empty ${var} is refused" "${var} is required" "${d}" "${var}="
done

# ---------------------------------------------------------------------------
# Source identity. publish's merge-back gate compares the tree hash, so a
# malformed one disables the highest-value check in the design.
# ---------------------------------------------------------------------------
d="$(digest_dir source)"
expect_fail "a short internal SHA is refused" "40-character hex SHA" "${d}" "INTERNAL_SHA=abc123"
expect_fail "a non-hex internal SHA is refused" "40-character hex SHA" "${d}" \
    "INTERNAL_SHA=zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
expect_fail "a short tree hash is refused" "40-character hex tree hash" "${d}" "TREE_HASH=deadbeef"

# ---------------------------------------------------------------------------
# Digests. This is the reason the manifest exists: publish promotes by digest
# so a moved tag cannot change what ships.
# ---------------------------------------------------------------------------
d="${TMP_ROOT}/tagref"
mkdir -p "${d}"
cat >"${d}/ngf.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"v2.8.0","platforms":"linux/amd64"}
EOF
expect_fail "a tag where a digest belongs is refused" "must be a sha256 digest" "${d}"

d="${TMP_ROOT}/shortdigest"
mkdir -p "${d}"
cat >"${d}/ngf.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"sha256:abcd","platforms":"linux/amd64"}
EOF
expect_fail "a truncated digest is refused" "must be a sha256 digest" "${d}"

expect_fail "a chart tag where a digest belongs is refused" "CHART_DIGEST must be a sha256 digest" \
    "$(digest_dir charttag)" "CHART_DIGEST=v2.8.0"

d="$(digest_dir chartok)"
out="$(run_emit "${d}" "CHART_DIGEST=${DIG_B}")"
got="$(printf '%s' "${out}" | jq -r '.chart.digest')"
check_eq "a valid chart digest is recorded" "${DIG_B}" "${got}"

# ---------------------------------------------------------------------------
# The image set
# ---------------------------------------------------------------------------
d="${TMP_ROOT}/empty"
mkdir -p "${d}"
expect_fail "an empty digest directory is refused" "no digest records found" "${d}"

expect_fail "a missing digest directory is refused" "not a directory" "${TMP_ROOT}/nope"

d="${TMP_ROOT}/malformed"
mkdir -p "${d}"
printf 'not json\n' >"${d}/ngf.json"
expect_fail "a malformed digest record is refused" "not valid JSON" "${d}"

d="${TMP_ROOT}/missingfield"
mkdir -p "${d}"
cat >"${d}/ngf.json" <<EOF
{"image":"ngf","base-os":"","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
expect_fail "a record without a target is refused" "needs a non-empty 'target'" "${d}"

d="${TMP_ROOT}/emptyfield"
mkdir -p "${d}"
cat >"${d}/ngf.json" <<EOF
{"image":"","base-os":"","target":"reg.example/ngf","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
expect_fail "a record with an empty image is refused" "needs a non-empty 'image'" "${d}"

# A re-run leg, or two builds racing, leaves two records for the same image.
# There is no way to tell here which one was tested.
d="${TMP_ROOT}/dupe"
mkdir -p "${d}"
cat >"${d}/ngf-1.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
cat >"${d}/ngf-2.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"${DIG_B}","platforms":"linux/amd64"}
EOF
expect_fail "two records for the same image and OS are refused" "duplicate digest records" "${d}"

# The same image built for a different base OS is not a duplicate.
d="${TMP_ROOT}/notdupe"
mkdir -p "${d}"
cat >"${d}/ngf-1.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
cat >"${d}/ngf-2.json" <<EOF
{"image":"ngf","base-os":"ubi","target":"reg.example/ngf","digest":"${DIG_B}","platforms":"linux/amd64"}
EOF
out="$(run_emit "${d}")"
n="$(printf '%s' "${out}" | jq -r '.images | length')"
check_eq "the same image on a different base OS is kept" "2" "${n}"

# ---------------------------------------------------------------------------
# Optional JSON inputs must be the type they claim
# ---------------------------------------------------------------------------
d="$(digest_dir jsontypes)"
expect_fail "a non-object NGINX_VERSIONS is refused" "must be a JSON object" "${d}" \
    'NGINX_VERSIONS=["1.29.4"]'
expect_fail "malformed NGINX_VERSIONS is refused" "must be a JSON object" "${d}" \
    'NGINX_VERSIONS={oops'
expect_fail "a non-array ASSET_REFS is refused" "must be a JSON array" "${d}" \
    'ASSET_REFS={"a":1}'

out="$(run_emit "${d}" 'NGINX_VERSIONS={"nginx":"1.29.4","plus":"R35"}' \
    'ASSET_REFS=["dist/checksums.txt"]')"
got="$(printf '%s' "${out}" | jq -r '.nginx_versions.plus')"
check_eq "NGINX versions are carried through" "R35" "${got}"
got="$(printf '%s' "${out}" | jq -r '.assets[0]')"
check_eq "asset references are carried through" "dist/checksums.txt" "${got}"

# ---------------------------------------------------------------------------
# Determinism: publish may compare manifests across re-runs, and record files
# arrive in whatever order the artifact download produced.
# ---------------------------------------------------------------------------
# The filenames are deliberately in the opposite order to the image names:
# `find | sort` alone would put nginx first, so only sorting by content gives
# ngf. An earlier version of this fixture had both orders agreeing and passed
# with the sort removed.
d="${TMP_ROOT}/order1"
mkdir -p "${d}"
cat >"${d}/a-nginx.json" <<EOF
{"image":"nginx","base-os":"","target":"reg.example/nginx","digest":"${DIG_B}","platforms":"linux/amd64"}
EOF
cat >"${d}/z-ngf.json" <<EOF
{"image":"ngf","base-os":"","target":"reg.example/ngf","digest":"${DIG_A}","platforms":"linux/amd64"}
EOF
first="$(run_emit "${d}" | jq -r '.images[0].image')"
check_eq "images are sorted by content, not by filename" "ngf" "${first}"

# ---------------------------------------------------------------------------
printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
