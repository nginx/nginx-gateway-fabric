#!/usr/bin/env bash
#
# Tests for copy-images.sh, against a stubbed skopeo.
#
# What is worth pinning here is not that a copy happens -- skopeo's job -- but
# the things that would be expensive to get wrong: that the repository mapping
# still comes from resolve-image-target.sh, that the precedence rules hold,
# that nothing is guessed when a registry is missing, and that a staging run
# cannot reach a public registry.
#
# The staging hostnames are supplied here as obvious fakes, exactly as the
# real ones are supplied in CI from repository variables. Nothing in this file
# needs to know them, and the public-registry guard is unit-tested directly
# against constructed near-misses instead.
#
# Run directly: bash .github/scripts/copy-images_test.sh
# Exit status: 0 all passed, 1 one or more failed.

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

# run_copy [VAR=VAL ...] <script> [args...]
# Leading VAR=VAL pairs become the script's environment, so precedence between
# the environment, a config file and the CLI can be exercised directly. The
# staging variables are always supplied, as the calling workflow supplies them.
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

# ---------------------------------------------------------------------------
# The public-registry guard, unit-tested directly.
#
# Whole-host matching is the property that matters, and it is checked against
# constructed near-misses rather than real staging hostnames so that this file
# does not name internal infrastructure either.
# ---------------------------------------------------------------------------
# shellcheck source=/dev/null
source "${SCRIPT}" # the source guard stops main() running

# Sourcing brings `set -euo pipefail` with it. Turn -e off again so that an
# assertion which expects a non-zero exit does not end the run.
set +eu

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

# A host that merely contains a public one is a different host. Were the
# match loose, every internal registry sharing a prefix would be refused, or
# worse a production one would be waved through.
assert_not_public "a longer host is not the public one" "docker-mgmt-x.nginx.com"
assert_not_public "a longer read-mirror host is not the public one" "private-registry-x.nginx.com"
assert_not_public "a subdomain of a public host is not it" "inner.docker-mgmt.nginx.com"
assert_not_public "an unrelated host is not public" "registry.invalid"
assert_not_public "an empty host is not public" ""

# ---------------------------------------------------------------------------
# The mapping is not duplicated: it comes from resolve-image-target.sh, so a
# change to where an image is built moves the promotion with it.
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Variants. Each image is built with and without a build-os except the
# operator, which has no build-os matrix.
# ---------------------------------------------------------------------------
assert_copied "the ubi variant is promoted" \
    "docker://docker-mgmt.nginx.com/nginx-gateway-fabric/nginx-plus:2.7.0-ubi"
assert_not_copied "the operator has no ubi variant" \
    "nginx-gateway-fabric/operator:2.7.0-ubi"

# ---------------------------------------------------------------------------
# Precedence: CLI > config > environment > default.
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Nothing is guessed. No registry has a default, so a run that names neither
# a config nor an explicit registry fails -- which is also what stops an
# argument-free invocation reaching production.
# ---------------------------------------------------------------------------
run_copy_bare "${SCRIPT}" --images plus --variants default
assert_rc "a run with no registries at all fails" 2
assert_says "it names the missing value" "SOURCE_OSS_REGISTRY is not set"
assert_not_copied "it copies nothing" "docker://"

# An undefined repository variable leaves the config empty, and that must
# fail loudly rather than fall back to anything.
run_copy_bare "${SCRIPT}" --config staging --images plus --variants default
assert_rc "a config whose variables are undefined fails" 2
assert_says "it says the config left the value empty" "left it empty"

# ---------------------------------------------------------------------------
# A named config that does not exist is an error, never a fallback.
# ---------------------------------------------------------------------------
run_copy "${SCRIPT}" --config nonexistent --images plus
assert_rc "a missing named config fails" 2
assert_says "a missing named config says which one" "'nonexistent' was requested"
assert_not_copied "a missing named config copies nothing" "docker://"

# ---------------------------------------------------------------------------
# Staging cannot publish publicly, however the registry arrived.
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Odds and ends.
# ---------------------------------------------------------------------------
run_copy "${SCRIPT}" --config production --images nope
assert_rc "an unrecognised image fails" 2

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

printf '\npassed=%d failed=%d\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
