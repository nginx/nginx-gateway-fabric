#!/usr/bin/env bash
#
# Tests for assert-staged-digests.sh, against a stubbed skopeo. Staging
# hostnames here are obvious fakes, as real ones come from repository variables.
#
# Run directly: bash .github/scripts/assert-staged-digests_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/assert-staged-digests.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

FAKE_READ="staging-read.invalid"
FAKE_WRITE="staging-write.invalid"
TAG="2.8.0"

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

# A skopeo that serves each reference's manifest from a file under
# MANIFESTS, named by the reference with every `/` and `:` made `_`. A
# reference with no file is an unknown tag.
cat >"${TMP}/skopeo" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${SKOPEO_LOG}"
ref="${*: -1}"
ref="${ref#docker://}"
file="${MANIFESTS}/${ref//[\/:]/_}"
[ -f "${file}" ] || { echo "manifest unknown: ${ref}" >&2; exit 1; }
cat "${file}"
STUB
chmod +x "${TMP}/skopeo"

LOG="${TMP}/skopeo.log"
OUT=""
RC=0

digest_of() {
    printf 'sha256:%s' "$(sha256sum <"$1" | cut -d' ' -f1)"
}

# serve <ref> <content>: publishes content as the manifest for ref and prints
# its digest.
serve() {
    local file="${MANIFESTS}/${1//[\/:]/_}"
    printf '%s\n' "$2" >"${file}"
    digest_of "${file}"
}

# record <image> <base-os> <digest>: writes a build.yml-shaped digest record,
# which names the image but not where it was pushed.
record() {
    local name="$1${2:+-$2}"
    jq -n --arg image "$1" --arg os "$2" --arg digest "$3" \
        '{image: $image, "base-os": $os, digest: $digest, platforms: "linux/amd64"}' \
        >"${DIGESTS}/${name}.json"
}

repo() {
    case "$1" in
    ngf) printf '%s/nginx-gateway-fabric' "${FAKE_READ}" ;;
    nginx) printf '%s/nginx-gateway-fabric/nginx' "${FAKE_READ}" ;;
    plus) printf '%s/nginx-gateway-fabric/nginx-plus' "${FAKE_READ}" ;;
    plus-nap-waf) printf '%s/nginx-gateway-fabric/nginx-plus-f5waf' "${FAKE_READ}" ;;
    operator) printf '%s/nginx-gateway-fabric/operator' "${FAKE_READ}" ;;
    esac
}

# fresh: a new staging registry where every image and variant is served and
# recorded with matching digests.
fresh() {
    MANIFESTS="$(mktemp -d "${TMP}/manifests.XXXXXX")"
    DIGESTS="$(mktemp -d "${TMP}/digests.XXXXXX")"
    local image os suffix d
    for image in ngf nginx plus plus-nap-waf; do
        for os in "" ubi; do
            suffix="${os:+-${os}}"
            d=$(serve "$(repo "${image}"):${TAG}${suffix}" "{\"image\":\"${image}${suffix}\"}")
            record "${image}" "${os}" "${d}"
        done
    done
}

run_assert() {
    : >"${LOG}"
    OUT=$(env SKOPEO="${TMP}/skopeo" SKOPEO_LOG="${LOG}" MANIFESTS="${MANIFESTS}" \
        DIGEST_DIR="${DIGESTS}" REGISTRY="${FAKE_READ}" TAG="${TAG}" \
        "$@" "${SCRIPT}" 2>&1)
    RC=$?
}

assert_rc() {
    if [ "${RC}" -eq "$2" ]; then
        ok "$1"
    else
        no "$1" "expected exit ${2}, got ${RC}" "output was:" "${OUT}"
    fi
}

assert_says() {
    if printf '%s' "${OUT}" | grep -qF -- "$2"; then
        ok "$1"
    else
        no "$1" "expected output to mention: $2" "output was:" "${OUT}"
    fi
}

assert_inspected() {
    if grep -qF -- "$2" "${LOG}"; then
        ok "$1"
    else
        no "$1" "expected an inspect of: $2" "log was:" "$(cat "${LOG}")"
    fi
}

assert_not_inspected() {
    if grep -qF -- "$2" "${LOG}"; then
        no "$1" "did not expect an inspect of: $2" "log was:" "$(cat "${LOG}")"
    else
        ok "$1"
    fi
}

# Every tag matches its recorded digest.
fresh
run_assert
assert_rc "matching digests pass" 0
assert_says "every image and variant is checked" "8 staged image(s) match"
assert_inspected "the read registry is used" \
    "docker://${FAKE_READ}/nginx-gateway-fabric:${TAG}"
assert_not_inspected "the write registry is never read" "${FAKE_WRITE}"
assert_inspected "the ubi variant carries its suffix" \
    "docker://${FAKE_READ}/nginx-gateway-fabric/nginx:${TAG}-ubi"
assert_inspected "plus-nap-waf resolves to nginx-plus-f5waf" \
    "docker://${FAKE_READ}/nginx-gateway-fabric/nginx-plus-f5waf:${TAG}"
assert_inspected "the raw manifest is hashed" "inspect --raw"

# serve() ends every manifest with a newline, so the pass above already shows
# hashing keeps a trailing newline. This is the case without one.
fresh
printf '{"no":"newline"}' >"${MANIFESTS}/$(repo ngf | tr '/:' '__')_${TAG}"
record ngf "" "$(digest_of "${MANIFESTS}/$(repo ngf | tr '/:' '__')_${TAG}")"
run_assert
assert_rc "a manifest without a trailing newline still matches" 0

# A tag that moved after the build.
fresh
serve "$(repo plus):${TAG}-ubi" '{"moved":true}' >/dev/null
run_assert
assert_rc "a moved tag fails" 1
assert_says "the moved tag is named" "nginx-plus:${TAG}-ubi is sha256:"
assert_says "the other images are still checked" "7 do"

# A tag that is not there at all.
fresh
rm -f "${MANIFESTS}/$(repo nginx | tr '/:' '__')_${TAG}"
run_assert
assert_rc "a missing tag fails" 1
assert_says "the missing tag is named" "nginx-gateway-fabric/nginx:${TAG}: could not read"

# An image the tests pull with no record.
fresh
rm -f "${DIGESTS}/plus-nap-waf.json" "${DIGESTS}/plus-nap-waf-ubi.json"
run_assert
assert_rc "an image without a record fails" 1
assert_says "the unrecorded image is named" "no digest record for image 'plus-nap-waf'"

# The operator is not pulled by the tests, so its record is ignored by default.
fresh
record operator "" "sha256:$(printf '%064d' 0)"
run_assert
assert_rc "the operator record is ignored by default" 0
assert_not_inspected "the operator is not inspected" "operator"

# IMAGES narrows the check.
fresh
run_assert IMAGES="ngf"
assert_rc "IMAGES narrows the check" 0
assert_says "only the selected image is checked" "2 staged image(s) match"

# Bad input.
fresh
run_assert DIGEST_DIR=""
assert_rc "no DIGEST_DIR is a usage error" 2
run_assert REGISTRY=""
assert_rc "no REGISTRY is a usage error" 2
run_assert TAG=""
assert_rc "no TAG is a usage error" 2
run_assert IMAGES=" "
assert_rc "an empty IMAGES is a usage error" 2
run_assert DIGEST_DIR="$(mktemp -d "${TMP}/empty.XXXXXX")"
assert_rc "an empty DIGEST_DIR is a usage error" 2
assert_says "the empty directory is reported" "no digest records"

fresh
record ngf "" "sha256:short"
run_assert
assert_rc "a malformed digest is a usage error" 2

fresh
printf 'not json' >"${DIGESTS}/broken.json"
run_assert
assert_rc "a malformed record is a usage error" 2

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
