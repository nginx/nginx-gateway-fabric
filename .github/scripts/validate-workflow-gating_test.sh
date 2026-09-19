#!/usr/bin/env bash
#
# Tests for validate-workflow-gating.sh. Each case builds a throwaway
# workflow directory and asserts the validator's exit status and output.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/validate-workflow-gating.sh"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "${TMP_ROOT}"' EXIT

PASSED=0
FAILED=0

# Harness

# new_case <name> -- creates a fixture dir and echoes its path
new_case() {
    local dir="${TMP_ROOT}/$1"
    mkdir -p "${dir}/workflows"
    : >"${dir}/allowlist.txt"
    : >"${dir}/baseline.txt"
    printf '%s' "${dir}"
}

run_validator() {
    local dir="$1"
    "${VALIDATOR}" \
        --workflows "${dir}/workflows" \
        --allowlist "${dir}/allowlist.txt" \
        --baseline "${dir}/baseline.txt" \
        2>&1
}

# expect <name> <expected-status> <dir> [substring-that-must-appear]
expect() {
    local name="$1" want="$2" dir="$3" must="${4:-}"
    local out status
    out="$(run_validator "${dir}")"
    status=$?

    if [ "${status}" -ne "${want}" ]; then
        printf 'FAIL  %s\n      expected exit %s, got %s\n' "${name}" "${want}" "${status}"
        printf '%s\n' "${out}" | sed 's/^/      | /'
        FAILED=$((FAILED + 1))
        return
    fi

    if [ -n "${must}" ] && ! printf '%s' "${out}" | grep -Fq -e "${must}"; then
        printf 'FAIL  %s\n      expected output to contain: %s\n' "${name}" "${must}"
        printf '%s\n' "${out}" | sed 's/^/      | /'
        FAILED=$((FAILED + 1))
        return
    fi

    printf 'ok    %s\n' "${name}"
    PASSED=$((PASSED + 1))
}

# expect_absent <name> <expected-status> <dir> <substring-that-must-not-appear>
expect_absent() {
    local name="$1" want="$2" dir="$3" absent="$4"
    local out status
    out="$(run_validator "${dir}")"
    status=$?

    if [ "${status}" -ne "${want}" ]; then
        printf 'FAIL  %s\n      expected exit %s, got %s\n' "${name}" "${want}" "${status}"
        printf '%s\n' "${out}" | sed 's/^/      | /'
        FAILED=$((FAILED + 1))
        return
    fi

    if printf '%s' "${out}" | grep -Fq -e "${absent}"; then
        printf 'FAIL  %s\n      expected output NOT to contain: %s\n' "${name}" "${absent}"
        printf '%s\n' "${out}" | sed 's/^/      | /'
        FAILED=$((FAILED + 1))
        return
    fi

    printf 'ok    %s\n' "${name}"
    PASSED=$((PASSED + 1))
}

# Fixtures
#
# Almost every case is "one job, optionally gated, holding one step", so the
# scaffolding is built here and each case supplies only the part under test.
# A case whose shape is itself the subject -- an unusual trigger, deliberately
# wrong indentation -- is written out in full instead.

# wf <dir> <file> <job> [job-if] -- step lines are read from stdin
wf() {
    local dir="$1" file="$2" job="$3" job_if="${4:-}"
    {
        printf 'name: w\non: [push]\njobs:\n  %s:\n' "${job}"
        if [ -n "${job_if}" ]; then
            printf '    if: %s\n' "${job_if}"
        fi
        printf '    runs-on: ubuntu-26.04\n    steps:\n'
        cat
    } >"${dir}/workflows/${file}"
}

# login_step <registry> [step-if]
login_step() {
    printf '      - name: Login\n'
    if [ -n "${2:-}" ]; then
        printf '        if: %s\n' "$2"
    fi
    printf '        uses: docker/login-action@v4\n'
    printf '        with:\n          registry: %s\n' "$1"
}

# meta_step <image-name> <enable-expression> -- the localhost target is always
# present, as it is in the real workflows, so it cannot be what trips a finding.
meta_step() {
    printf '      - name: Docker meta\n'
    printf '        uses: docker/metadata-action@v6\n'
    printf '        with:\n          images: |\n'
    printf '            name=%s,enable=%s\n' "$1" "$2"
    printf '            name=localhost:5000/ngf\n'
}

# goreleaser_step <args>
goreleaser_step() {
    printf '      - name: Build binary\n'
    printf '        uses: goreleaser/goreleaser-action@v7\n'
    printf '        with:\n          version: v2.18.1\n          args: %s\n' "$1"
}

# run_step <name> <command>
run_step() {
    printf '      - name: %s\n        run: %s\n' "$1" "$2"
}

# Registry logins

d="$(new_case ungated-login)"
login_step ghcr.io | wf "${d}" w.yml publish
expect "ungated registry login fails" 1 "${d}" "w.yml::publish::login:ghcr.io"

d="$(new_case step-gated-login)"
login_step ghcr.io "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}" | wf "${d}" w.yml publish
expect "step-level repository gate passes" 0 "${d}"

d="$(new_case job-gated-login)"
login_step ghcr.io | wf "${d}" w.yml publish "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}"
expect "job-level repository gate passes" 0 "${d}"

# repository_owner is not a gate: it is identical in both repositories.
d="$(new_case owner-is-not-a-gate)"
login_step ghcr.io | wf "${d}" w.yml publish "\${{ github.repository_owner == 'nginx' }}"
expect "repository_owner does not count as a gate" 1 "${d}" "w.yml::publish::login:ghcr.io"

# A top-level `||` (Actions binds `&&` tighter) can bypass a gate.
d="$(new_case toplevel-or-bypass)"
login_step ghcr.io |
    wf "${d}" w.yml publish "\${{ github.repository == 'nginx/nginx-gateway-fabric' && (github.ref == 'refs/heads/main') || (github.event_name == 'schedule') }}"
expect "a gate a top-level || can bypass is not a gate" 1 "${d}" "w.yml::publish::login:ghcr.io"

# The same conditions, kept inside the group, cannot escape the gate.
d="$(new_case or-inside-group)"
login_step ghcr.io |
    wf "${d}" w.yml publish "\${{ github.repository == 'nginx/nginx-gateway-fabric' && (github.ref == 'refs/heads/main' || github.event_name == 'schedule') }}"
expect "an || inside the group is fine" 0 "${d}"

# Mentioning github.repository without comparing it is not a gate either.
d="$(new_case repository-not-compared)"
login_step ghcr.io | wf "${d}" w.yml publish "\${{ startsWith(github.repository, 'nginx/') }}"
expect "github.repository must be compared, not merely mentioned" 1 "${d}" "w.yml::publish::login:ghcr.io"

# The in-workflow service registry is not a shared destination
d="$(new_case localhost-registry)"
login_step localhost:5000 | wf "${d}" w.yml build
expect "localhost registry is ignored" 0 "${d}"

# metadata-action image targets: the enable= expression is the gate

d="$(new_case image-target-ungated)"
meta_step 'ghcr.io/${{ github.repository_owner }}/ngf' "\${{ github.event_name != 'pull_request' }}" | wf "${d}" w.yml build
expect "image target gated only on event name fails" 1 "${d}" "w.yml::build::image-target:ghcr.io"

d="$(new_case image-target-gated)"
meta_step 'ghcr.io/${{ github.repository_owner }}/ngf' \
    "\${{ github.event_name != 'pull_request' && github.repository == 'nginx/nginx-gateway-fabric' }}" | wf "${d}" w.yml build
expect "image target with repository in enable passes" 0 "${d}"

# A computed destination still needs a gate, and its baseline key must not churn.

d="$(new_case computed-registry)"
login_step '${{ steps.target.outputs.host }}' | wf "${d}" w.yml build
expect "a computed registry is named <computed>" 1 "${d}" "w.yml::build::login:<computed>"

d="$(new_case computed-image-target)"
meta_step '${{ steps.target.outputs.target }}' "\${{ github.event_name != 'pull_request' }}" | wf "${d}" w.yml build
expect "a computed image target is named <computed>" 1 "${d}" "w.yml::build::image-target:<computed>"

d="$(new_case computed-registry-gated)"
login_step '${{ steps.target.outputs.host }}' "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}" | wf "${d}" w.yml build
expect "a gated computed registry passes" 0 "${d}"

# GoReleaser: only a publishing invocation counts

d="$(new_case goreleaser-snapshot)"
goreleaser_step "build --single-target --snapshot --clean" | wf "${d}" w.yml build
expect "goreleaser snapshot build passes" 0 "${d}"

d="$(new_case goreleaser-release)"
goreleaser_step "\${{ inputs.is_production_release && 'release' || 'build --snapshot' }} --clean" | wf "${d}" w.yml build
expect "goreleaser conditional release fails" 1 "${d}" "w.yml::build::goreleaser"

# `release --snapshot` builds and signs but does not publish.
d="$(new_case goreleaser-snapshot-release)"
goreleaser_step "\${{ inputs.is_production_release && 'release --snapshot' || 'build --snapshot' }} --clean" | wf "${d}" w.yml build
expect "goreleaser release --snapshot is not a publish" 0 "${d}"

# A publishing release beside a snapshot alternative must still fail.
d="$(new_case goreleaser-mixed)"
goreleaser_step "\${{ inputs.is_production_release && 'release' || 'build --snapshot' }} --clean" | wf "${d}" w.yml build
expect "a real release beside a snapshot alternative still fails" 1 "${d}" "w.yml::build::goreleaser"

# Other publishing destinations

d="$(new_case gh-release)"
run_step Upload 'gh release upload "${TAG}" report.md' | wf "${d}" w.yml report
expect "ungated gh release upload fails" 1 "${d}" "w.yml::report::gh-release"

d="$(new_case helm-push)"
run_step "Push chart" "helm push chart.tgz oci://ghcr.io/nginx/charts" | wf "${d}" w.yml chart
expect "ungated helm push fails" 1 "${d}" "w.yml::chart::helm-push"

d="$(new_case skopeo-copy)"
run_step Promote "skopeo copy --all docker://src docker://dst" | wf "${d}" w.yml promote
expect "ungated skopeo copy fails" 1 "${d}" "w.yml::promote::skopeo-copy"

# Triggers are irrelevant: any repository holding the file can run it. Written
# out in full because the trigger is the subject.
d="$(new_case scheduled-workflow)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on:
  schedule:
    - cron: "0 3 * * *"
jobs:
  nightly:
    runs-on: ubuntu-26.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "a scheduled publish is still a publish" 1 "${d}" "w.yml::nightly::login:ghcr.io"

# Baseline behaviour

d="$(new_case baseline-suppresses)"
login_step ghcr.io | wf "${d}" w.yml publish
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a baselined finding does not fail" 0 "${d}"

d="$(new_case baseline-partial)"
{
    login_step ghcr.io
    login_step docker-mgmt.nginx.com
} | wf "${d}" w.yml publish
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a new finding beside a baselined one still fails" 1 "${d}" "w.yml::publish::login:docker-mgmt.nginx.com"

d="$(new_case baseline-partial-no-noise)"
cp "${TMP_ROOT}/baseline-partial/workflows/w.yml" "${d}/workflows/w.yml"
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect_absent "the baselined entry is not reported again" 1 "${d}" "publish::login:ghcr.io"

d="$(new_case baseline-stale)"
login_step ghcr.io | wf "${d}" w.yml publish "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}"
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a fixed finding makes its baseline entry stale" 1 "${d}" "stale baseline"

# Allowlist. Written out in full: workflow_call is the point of the case.
d="$(new_case allowlisted)"
cat >"${d}/workflows/shared.yml" <<'EOF'
name: shared
on:
  workflow_call:
jobs:
  publish:
    runs-on: ubuntu-26.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
printf '# gated by its caller\nshared.yml\n' >"${d}/allowlist.txt"
expect "an allowlisted workflow is skipped" 0 "${d}"

# release-publish.yml promotes via copy-images.sh; no literal `skopeo copy` appears.

d="$(new_case copy-images-script)"
run_step "Promote by digest" ".github/scripts/copy-images.sh --config production --target-tag 2.8.0" | wf "${d}" w.yml promote
expect "promotion via copy-images.sh is detected" 1 "${d}" "w.yml::promote::skopeo-copy"

d="$(new_case copy-images-script-gated)"
run_step "Promote by digest" ".github/scripts/copy-images.sh --config production --target-tag 2.8.0" |
    wf "${d}" w.yml promote "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}"
expect "a gated promotion via copy-images.sh passes" 0 "${d}"

# Two ungated destinations in one job distinguish "exempted one" from "skipped the file".
two_dest_workflow() {
    {
        login_step ghcr.io
        run_step "Push the chart" "helm push chart.tgz oci://ghcr.io/nginx/charts"
    } | wf "$1" two.yml publish
}

d="$(new_case entry-allowlist-suppresses)"
two_dest_workflow "${d}"
printf '# a reason\ntwo.yml::publish::login:ghcr.io\n' >"${d}/allowlist.txt"
expect "an entry-level exemption suppresses the finding it names" 1 "${d}" "two.yml::publish::helm-push"

out="$(run_validator "${d}")"
if printf '%s' "${out}" | grep -Fq "two.yml::publish::login:ghcr.io"; then
    printf 'FAIL  %s\n' "an entry-level exemption does not report the finding it names"
    FAILED=$((FAILED + 1))
else
    printf 'ok    %s\n' "an entry-level exemption does not report the finding it names"
    PASSED=$((PASSED + 1))
fi

# An entry-level line must not behave like a basename line (exempt the whole file).
d="$(new_case entry-allowlist-is-not-file-level)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\n' >"${d}/allowlist.txt"
expect "an entry-level exemption leaves the rest of the file checked" 1 "${d}" "helm-push"

# Exempting both destinations individually reaches the same outcome via two reviewed lines.
d="$(new_case entry-allowlist-both)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\ntwo.yml::publish::helm-push\n' >"${d}/allowlist.txt"
expect "exempting every finding individually passes" 0 "${d}"

# An exemption matching nothing is reported, not left to silently outlive the step.
d="$(new_case entry-allowlist-stale)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\ntwo.yml::publish::helm-push\ntwo.yml::publish::gh-release\n' >"${d}/allowlist.txt"
expect "an exemption matching nothing is reported" 1 "${d}" "two.yml::publish::gh-release"

d="$(new_case entry-allowlist-stale-after-gating)"
login_step ghcr.io | wf "${d}" two.yml publish "\${{ github.repository == 'nginx/nginx-gateway-fabric' }}"
printf 'two.yml::publish::login:ghcr.io\n' >"${d}/allowlist.txt"
expect "gating a step makes its exemption stale" 1 "${d}" "no longer match"

# An exempted finding must not be written to the baseline as debt to pay off.
d="$(new_case entry-allowlist-not-in-baseline)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\ntwo.yml::publish::helm-push\n' >"${d}/allowlist.txt"
"${VALIDATOR}" --workflows "${d}/workflows" --allowlist "${d}/allowlist.txt" \
    --baseline "${d}/baseline.txt" --update-baseline --quiet >/dev/null 2>&1
if grep -q '^two.yml' "${d}/baseline.txt"; then
    printf 'FAIL  %s\n' "--update-baseline does not record exempted findings"
    sed 's/^/      | /' "${d}/baseline.txt"
    FAILED=$((FAILED + 1))
else
    printf 'ok    %s\n' "--update-baseline does not record exempted findings"
    PASSED=$((PASSED + 1))
fi

# A basename line must keep working; entry keys are an addition, not a replacement.
d="$(new_case file-level-still-works)"
two_dest_workflow "${d}"
printf 'two.yml\n' >"${d}/allowlist.txt"
expect "a basename exemption still skips the whole file" 0 "${d}"

# The scanner's structural assumption must be checked, not assumed. Written out
# in full: the indentation is deliberately wrong, so it cannot come from wf.
d="$(new_case bad-indentation)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-26.04
    steps:
    - name: Login
      uses: docker/login-action@v4
      with:
        registry: ghcr.io
EOF
expect "unexpected step indentation is reported, not skipped" 1 "${d}" "expected 6"

# Things outside the jobs block must not be mistaken for steps. Written out in
# full: the expanded push trigger is the subject.
d="$(new_case push-trigger-not-a-push)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on:
  push:
    branches:
      - main
jobs:
  test:
    runs-on: ubuntu-26.04
    steps:
      - name: Test
        run: make unit-test
EOF
expect "an on.push trigger is not a publishing step" 0 "${d}"

# --update-baseline writes what the checker would otherwise report
d="$(new_case update-baseline)"
login_step ghcr.io | wf "${d}" w.yml publish
if "${VALIDATOR}" --workflows "${d}/workflows" --allowlist "${d}/allowlist.txt" \
    --baseline "${d}/baseline.txt" --update-baseline --quiet >/dev/null 2>&1 &&
    grep -Fxq "w.yml::publish::login:ghcr.io" "${d}/baseline.txt" &&
    run_validator "${d}" >/dev/null 2>&1; then
    printf 'ok    %s\n' "--update-baseline records the finding and then passes"
    PASSED=$((PASSED + 1))
else
    printf 'FAIL  %s\n' "--update-baseline records the finding and then passes"
    sed 's/^/      | /' "${d}/baseline.txt"
    FAILED=$((FAILED + 1))
fi

# Invocation errors
if "${VALIDATOR}" --workflows "${TMP_ROOT}/does-not-exist" >/dev/null 2>&1; then
    printf 'FAIL  %s\n' "a missing workflow directory is an error"
    FAILED=$((FAILED + 1))
else
    printf 'ok    %s\n' "a missing workflow directory is an error"
    PASSED=$((PASSED + 1))
fi

if "${VALIDATOR}" --nonsense >/dev/null 2>&1; then
    printf 'FAIL  %s\n' "an unknown argument is an error"
    FAILED=$((FAILED + 1))
else
    printf 'ok    %s\n' "an unknown argument is an error"
    PASSED=$((PASSED + 1))
fi

printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
