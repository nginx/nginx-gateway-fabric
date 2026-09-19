#!/usr/bin/env bash
#
# Tests for copy-images.sh, against a stubbed skopeo. Staging hostnames here
# are obvious fakes, as real ones are supplied in CI from repository variables.
#
# Run directly: bash .github/scripts/copy-images_test.sh

set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${DIR}/copy-images.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

# Stand-ins for the repository variables the workflow passes in.
FAKE_READ="staging-read.invalid"
FAKE_WRITE="staging-write.invalid"

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

# A skopeo that records its arguments instead of touching a registry.
cat >"${TMP}/skopeo" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${SKOPEO_LOG}"
STUB
chmod +x "${TMP}/skopeo"

# A skopeo that fails, to check the script reports a failed promotion.
cat >"${TMP}/skopeo-broken" <<'STUB'
#!/usr/bin/env bash
echo "stub failure" >&2
exit 1
STUB
chmod +x "${TMP}/skopeo-broken"

LOG="${TMP}/skopeo.log"
OUT=""
RC=0

# run_copy [VAR=VAL ...] <script> [args...]; staging variables always supplied.
run_copy() {
    : >"${LOG}"
    OUT=$(env SKOPEO="${TMP}/skopeo" SKOPEO_LOG="${LOG}" \
        STAGING_READ_REGISTRY="${FAKE_READ}" STAGING_WRITE_REGISTRY="${FAKE_WRITE}" \
        "$@" 2>&1)
    RC=$?
}

# run_copy_bare: same, but with the staging variables undefined.
run_copy_bare() {
    : >"${LOG}"
    OUT=$(env -u STAGING_READ_REGISTRY -u STAGING_WRITE_REGISTRY \
        SKOPEO="${TMP}/skopeo" SKOPEO_LOG="${LOG}" "$@" 2>&1)
    RC=$?
}

assert_copied() {
    if grep -qF -- "$2" "${LOG}"; then
        ok "$1"
    else
        no "$1" "expected a copy matching: $2" "log was:" "$(cat "${LOG}")"
    fi
}

assert_not_copied() {
    if grep -qF -- "$2" "${LOG}"; then
        no "$1" "did not expect a copy matching: $2" "log was:" "$(cat "${LOG}")"
    else
        ok "$1"
    fi
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

# The public-registry guard, unit-tested directly against constructed near-misses.
# shellcheck source=/dev/null
source "${SCRIPT}" # the source guard stops main() running

set +eu # sourcing brings `set -euo pipefail`; assertions expect non-zero exits

assert_public() {
    if is_public_registry "$2"; then
        ok "$1"
    else
        no "$1" "expected '$2' to be treated as public"
    fi
}

assert_not_public() {
    if is_public_registry "$2"; then
        no "$1" "expected '$2' NOT to be treated as public"
    else
        ok "$1"
    fi
}

assert_public "ghcr.io is public" "ghcr.io"
assert_public "ghcr.io with a path is public" "ghcr.io/nginx"
assert_public "the production write endpoint is public" "docker-mgmt.nginx.com"
assert_public "the production read mirror is public" "private-registry.nginx.com"
assert_public "Artifact Registry is public" "us-docker.pkg.dev"
assert_public "gcr.io is public" "gcr.io"

# A host that merely contains a public one is a different host.
assert_not_public "a longer host is not the public one" "docker-mgmt-x.nginx.com"
assert_not_public "a longer read-mirror host is not the public one" "private-registry-x.nginx.com"
assert_not_public "a subdomain of a public host is not it" "inner.docker-mgmt.nginx.com"
assert_not_public "an unrelated host is not public" "registry.invalid"
assert_not_public "an empty host is not public" ""

# The mapping is not duplicated: it comes from resolve-image-target.sh.
run_copy "${SCRIPT}" --config production --source-tag 2.7.0 --target-tag 2.7.0
assert_rc "a production promotion succeeds" 0
assert_copied "plus keeps its built repository path" \
    "docker://docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus:2.7.0"
assert_copied "plus-nap-waf keeps its built repository path" \
    "docker://docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus-f5waf:2.7.0"
assert_copied "ngf promotes to the OSS registry" \
    "docker://ghcr.io/nginx/nginx-gateway-fabric:2.7.0"
assert_copied "nginx promotes to the OSS registry" \
    "docker://ghcr.io/nginx/nginx-gateway-fabric/nginx:2.7.0"
assert_copied "operator promotes to the OSS registry" \
    "docker://ghcr.io/nginx/nginx-gateway-fabric/operator:2.7.0"
assert_copied "promotion reads from the staging mirror, not the write endpoint" \
    "docker://${FAKE_READ}/nginx-gateway-fabric/nginx-plus:2.7.0"

# Every architecture must come across, and a flaky registry must be retried.
assert_copied "copies the whole manifest list" "copy --all --retry-times 5"

# Each image has a build-os variant except the operator.
assert_copied "the ubi variant is promoted" \
    "docker://docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus:2.7.0-ubi"
assert_not_copied "the operator has no ubi variant" \
    "nginx-gateway-fabric/operator:2.7.0-ubi"

# Precedence: CLI > config > environment > default.
run_copy "${SCRIPT}" --config production --images plus --variants default \
    --source-tag t --target-tag t --target-plus-registry cli.invalid
assert_copied "CLI beats the config file" "docker://cli.invalid/"
assert_not_copied "CLI beats the config file, production not used" \
    "docker://docker-mgmt.nginx.com/"

run_copy TARGET_PLUS_REGISTRY=env.invalid "${SCRIPT}" --config production \
    --images plus --variants default --source-tag t --target-tag t
assert_copied "the config file beats the environment" "docker://docker-mgmt.nginx.com/"
assert_not_copied "the config file beats the environment, env not used" \
    "docker://env.invalid/"

run_copy TARGET_PLUS_REGISTRY=env.invalid SOURCE_PLUS_REGISTRY=envsrc.invalid \
    TARGET_OSS_REGISTRY=env.invalid SOURCE_OSS_REGISTRY=envsrc.invalid \
    "${SCRIPT}" --images plus --variants default --source-tag t --target-tag t
assert_copied "the environment is used when no config is named" "docker://env.invalid/"

# Nothing is guessed: a run naming neither a config nor a registry fails.
run_copy_bare "${SCRIPT}" --images plus --variants default
assert_rc "a run with no registries at all fails" 2
assert_says "it names the missing value" "SOURCE_OSS_REGISTRY is not set"
assert_not_copied "it copies nothing" "docker://"

# An undefined repository variable leaves the config empty, and must fail loudly.
run_copy_bare "${SCRIPT}" --config staging --images plus --variants default
assert_rc "a config whose variables are undefined fails" 2
assert_says "it says the config left the value empty" "left it empty"

# A named config that does not exist is an error, never a fallback.
run_copy "${SCRIPT}" --config nonexistent --images plus
assert_rc "a missing named config fails" 2
assert_says "a missing named config says which one" "'nonexistent' was requested"
assert_not_copied "a missing named config copies nothing" "docker://"

# Staging cannot publish publicly, however the registry arrived.
run_copy "${SCRIPT}" --config staging --images plus --target-plus-registry ghcr.io/nginx
assert_rc "staging refuses a public target passed on the CLI" 2
assert_says "staging says which registry it refused" "ghcr.io/nginx"
assert_not_copied "staging copies nothing when it refuses" "docker://"

run_copy "${SCRIPT}" --config staging --images plus \
    --target-plus-registry docker-mgmt.nginx.com
assert_rc "staging refuses the production write endpoint" 2

run_copy STAGING_WRITE_REGISTRY=ghcr.io "${SCRIPT}" --config staging --images plus \
    --variants default --source-tag t --target-tag t
assert_rc "staging refuses a public target arriving from its variable" 2

run_copy "${SCRIPT}" --config staging --images plus --variants default \
    --source-tag t --target-tag t
assert_rc "staging itself resolves" 0
assert_copied "staging writes to the staging endpoint" "docker://${FAKE_WRITE}/"

run_copy "${SCRIPT}" --config production --images nope
assert_rc "an unrecognised image fails" 2

# Asking for nothing must not quietly mean everything, or quietly succeed.
run_copy "${SCRIPT}" --config production --images "" --source-tag t --target-tag t
assert_rc "an empty --images is an error, not 'all'" 2
assert_not_copied "an empty --images copies nothing" "docker://"

run_copy "${SCRIPT}" --config production --images "   " --source-tag t --target-tag t
assert_rc "a whitespace --images is an error, not a silent no-op" 2
assert_not_copied "a whitespace --images copies nothing" "docker://"

# Reached through the environment, since the CLI guard above intercepts the flag.
run_copy IMAGES="   " "${SCRIPT}" --config production --source-tag t --target-tag t
assert_rc "an empty IMAGES from the environment is an error" 2
assert_says "it says nothing was selected" "matched nothing"
assert_not_copied "an empty IMAGES copies nothing" "docker://"

run_copy "${SCRIPT}" --config production --images plus --variants default \
    --source-tag t --target-tag t --dry-run
assert_rc "a dry run succeeds" 0
assert_not_copied "a dry run copies nothing" "docker://"
assert_says "a dry run says what it would have done" "would copy"

run_copy "${SCRIPT}" --config production --images ngf --variants default \
    --source-tag 2.7.0 --target-tag 2.7.0 --additional-target-tag latest
assert_copied "an additional target tag is written" \
    "docker://ghcr.io/nginx/nginx-gateway-fabric:latest"
assert_copied "the additional tag comes from the same source" \
    "docker://${FAKE_READ}/nginx-gateway-fabric:2.7.0"

: >"${LOG}"
OUT=$(env SKOPEO="${TMP}/skopeo-broken" SKOPEO_LOG="${LOG}" \
    STAGING_READ_REGISTRY="${FAKE_READ}" STAGING_WRITE_REGISTRY="${FAKE_WRITE}" \
    "${SCRIPT}" --config staging --images plus --variants default \
    --source-tag t --target-tag t 2>&1)
RC=$?
assert_rc "a failed copy fails the promotion" 1
assert_says "a failed copy is reported" "FAILED"

# Sourcing by digest: promotes what prep recorded, not whatever a tag points
# at now, since a tag can move between stages on a prep re-run.
DIG_NGF="sha256:$(printf '1%.0s' {1..64})"
DIG_NGF_UBI="sha256:$(printf '2%.0s' {1..64})"
DIG_PLUS="sha256:$(printf '3%.0s' {1..64})"

cat >"${TMP}/manifest.json" <<EOF
{"schema_version":1,"release_version":"v2.8.0","images":[
 {"image":"ngf","base-os":"","target":"x/ngf","digest":"${DIG_NGF}","platforms":"linux/amd64"},
 {"image":"ngf","base-os":"ubi","target":"x/ngf","digest":"${DIG_NGF_UBI}","platforms":"linux/amd64"},
 {"image":"plus","base-os":"","target":"x/plus","digest":"${DIG_PLUS}","platforms":"linux/amd64"}]}
EOF

run_copy "${SCRIPT}" --config production --images ngf --variants default \
    --source-digests "${TMP}/manifest.json" --target-tag 2.8.0
assert_rc "a digest-sourced promotion succeeds" 0
assert_copied "the source is the recorded digest, not a tag" "@${DIG_NGF}"
assert_copied "the target is still the production tag" "ghcr.io/nginx/nginx-gateway-fabric:2.8.0"
assert_not_copied "no tag is used on the source side" "staging-read.invalid/nginx-gateway-fabric:"

# The manifest keys by base OS; copy-images speaks in variants.
run_copy "${SCRIPT}" --config production --images ngf --variants ubi \
    --source-digests "${TMP}/manifest.json" --target-tag 2.8.0
assert_copied "the ubi variant takes the ubi digest" "@${DIG_NGF_UBI}"
assert_not_copied "the ubi variant does not take the default digest" "@${DIG_NGF}"

run_copy "${SCRIPT}" --config production --images ngf --variants default \
    --source-digests "${TMP}/manifest.json" --target-tag 2.8.0
assert_copied "the default variant takes the empty-base-os digest" "@${DIG_NGF}"
assert_not_copied "the default variant does not take the ubi digest" "@${DIG_NGF_UBI}"

# An image the manifest does not record must stop the run, not fall back to a tag.
run_copy "${SCRIPT}" --config production --images operator --variants default \
    --source-digests "${TMP}/manifest.json" --target-tag 2.8.0
assert_rc "an image missing from the manifest is refused" 2
assert_says "the refusal names the image and the manifest" "no digest recorded for 'operator'"
assert_not_copied "nothing is copied when a digest is missing" "operator"

run_copy "${SCRIPT}" --config production --images ngf --variants ubi,default \
    --source-digests "${TMP}/nope.json" --target-tag 2.8.0
assert_rc "a missing manifest file is refused" 2
assert_says "the refusal names the missing file" "--source-digests file not found"

printf 'not json\n' >"${TMP}/bad.json"
run_copy "${SCRIPT}" --config production --images ngf --variants default \
    --source-digests "${TMP}/bad.json" --target-tag 2.8.0
assert_rc "a malformed manifest is refused" 2
assert_says "the refusal says it is not JSON" "not valid JSON"

# Without the flag, the tag path is still the default.
run_copy "${SCRIPT}" --config production --images ngf --variants default \
    --source-tag edge --target-tag 2.8.0
assert_copied "without --source-digests the source is still a tag" "staging-read.invalid/nginx-gateway-fabric:edge"

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
