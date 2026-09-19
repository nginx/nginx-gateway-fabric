#!/usr/bin/env bash
#
# Tests for validate-workflow-gating.sh.
#
# Each case builds a throwaway workflow directory containing one synthetic
# workflow, runs the validator against it, and asserts the exit status and
# (where it matters) that a specific finding was or was not reported.
#
# Usage: validate-workflow-gating_test.sh
# Exit status: 0 all passed, 1 one or more failed.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/validate-workflow-gating.sh"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "${TMP_ROOT}"' EXIT

PASSED=0
FAILED=0

# ---------------------------------------------------------------------------
# Harness
# ---------------------------------------------------------------------------

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

# ---------------------------------------------------------------------------
# A registry login must be gated
# ---------------------------------------------------------------------------
d="$(new_case ungated-login)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "ungated registry login fails" 1 "${d}" "w.yml::publish::login:ghcr.io"

d="$(new_case step-gated-login)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "step-level repository gate passes" 0 "${d}"

d="$(new_case job-gated-login)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "job-level repository gate passes" 0 "${d}"

# ---------------------------------------------------------------------------
# repository_owner is not a gate: it is identical in both repositories.
# This is the whole reason the script exists.
# ---------------------------------------------------------------------------
d="$(new_case owner-is-not-a-gate)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ github.repository_owner == 'nginx' }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "repository_owner does not count as a gate" 1 "${d}" "w.yml::publish::login:ghcr.io"

# ---------------------------------------------------------------------------
# A gate a top-level `||` can bypass is not a gate.
#
# Actions binds `&&` tighter than `||`, so "gate && (a) || (b)" parses as
# "(gate && (a)) || (b)" and publishes from any repository whenever (b) holds.
# ---------------------------------------------------------------------------
d="$(new_case toplevel-or-bypass)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' && (github.ref == 'refs/heads/main') || (github.event_name == 'schedule') }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "a gate a top-level || can bypass is not a gate" 1 "${d}" "w.yml::publish::login:ghcr.io"

# The same conditions, kept inside the group, cannot escape the gate.
d="$(new_case or-inside-group)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' && (github.ref == 'refs/heads/main' || github.event_name == 'schedule') }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "an || inside the group is fine" 0 "${d}"

# Mentioning github.repository without comparing it is not a gate either.
d="$(new_case repository-not-compared)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ startsWith(github.repository, 'nginx/') }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "github.repository must be compared, not merely mentioned" 1 "${d}" "w.yml::publish::login:ghcr.io"

# ---------------------------------------------------------------------------
# The in-workflow service registry is not a shared destination
# ---------------------------------------------------------------------------
d="$(new_case localhost-registry)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: localhost:5000
EOF
expect "localhost registry is ignored" 0 "${d}"

# ---------------------------------------------------------------------------
# metadata-action image targets: the enable= expression is the gate
# ---------------------------------------------------------------------------
d="$(new_case image-target-ungated)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Docker meta
        uses: docker/metadata-action@v6
        with:
          images: |
            name=ghcr.io/${{ github.repository_owner }}/ngf,enable=${{ github.event_name != 'pull_request' }}
            name=localhost:5000/ngf
EOF
expect "image target gated only on event name fails" 1 "${d}" "w.yml::build::image-target:ghcr.io"

d="$(new_case image-target-gated)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Docker meta
        uses: docker/metadata-action@v6
        with:
          images: |
            name=ghcr.io/${{ github.repository_owner }}/ngf,enable=${{ github.event_name != 'pull_request' && github.repository == 'nginx/nginx-gateway-fabric' }}
            name=localhost:5000/ngf
EOF
expect "image target with repository in enable passes" 0 "${d}"

# ---------------------------------------------------------------------------
# A destination computed at run time still needs a gate, and must produce a
# baseline key that does not churn when the expression is edited.
# ---------------------------------------------------------------------------
d="$(new_case computed-registry)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ${{ steps.target.outputs.host }}
EOF
expect "a computed registry is named <computed>" 1 "${d}" "w.yml::build::login:<computed>"

d="$(new_case computed-image-target)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Docker meta
        uses: docker/metadata-action@v6
        with:
          images: |
            name=${{ steps.target.outputs.target }},enable=${{ github.event_name != 'pull_request' }}
            name=localhost:5000/ngf
EOF
expect "a computed image target is named <computed>" 1 "${d}" "w.yml::build::image-target:<computed>"

d="$(new_case computed-registry-gated)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
        uses: docker/login-action@v4
        with:
          registry: ${{ steps.target.outputs.host }}
EOF
expect "a gated computed registry passes" 0 "${d}"

# ---------------------------------------------------------------------------
# GoReleaser: only a publishing invocation counts
# ---------------------------------------------------------------------------
d="$(new_case goreleaser-snapshot)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Build binary
        uses: goreleaser/goreleaser-action@v7
        with:
          version: v2.18.1
          args: build --single-target --snapshot --clean
EOF
expect "goreleaser snapshot build passes" 0 "${d}"

d="$(new_case goreleaser-release)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Build binary
        uses: goreleaser/goreleaser-action@v7
        with:
          version: v2.18.1
          args: ${{ inputs.is_production_release && 'release' || 'build --snapshot' }} --clean
EOF
expect "goreleaser conditional release fails" 1 "${d}" "w.yml::build::goreleaser"

# `release --snapshot` builds, archives, and signs, but does not publish, so
# it is not a publishing step even though the word release appears.
d="$(new_case goreleaser-snapshot-release)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Build binary
        uses: goreleaser/goreleaser-action@v7
        with:
          version: v2.18.1
          args: ${{ inputs.is_production_release && 'release --snapshot' || 'build --snapshot' }} --clean
EOF
expect "goreleaser release --snapshot is not a publish" 0 "${d}"

# But a publishing release alongside a snapshot alternative must still fail:
# this is the shape that would otherwise slip through.
d="$(new_case goreleaser-mixed)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - name: Build binary
        uses: goreleaser/goreleaser-action@v7
        with:
          version: v2.18.1
          args: ${{ inputs.is_production_release && 'release' || 'build --snapshot' }} --clean
EOF
expect "a real release beside a snapshot alternative still fails" 1 "${d}" "w.yml::build::goreleaser"

# ---------------------------------------------------------------------------
# Other publishing destinations
# ---------------------------------------------------------------------------
d="$(new_case gh-release)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  report:
    runs-on: ubuntu-24.04
    steps:
      - name: Upload
        run: gh release upload "${TAG}" report.md
EOF
expect "ungated gh release upload fails" 1 "${d}" "w.yml::report::gh-release"

d="$(new_case helm-push)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  chart:
    runs-on: ubuntu-24.04
    steps:
      - name: Push chart
        run: helm push chart.tgz oci://ghcr.io/nginx/charts
EOF
expect "ungated helm push fails" 1 "${d}" "w.yml::chart::helm-push"

d="$(new_case skopeo-copy)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  promote:
    runs-on: ubuntu-24.04
    steps:
      - name: Promote
        run: skopeo copy --all docker://src docker://dst
EOF
expect "ungated skopeo copy fails" 1 "${d}" "w.yml::promote::skopeo-copy"

# ---------------------------------------------------------------------------
# Triggers are irrelevant: any repository holding the file can run it
# ---------------------------------------------------------------------------
d="$(new_case scheduled-workflow)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on:
  schedule:
    - cron: "0 3 * * *"
jobs:
  nightly:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
expect "a scheduled publish is still a publish" 1 "${d}" "w.yml::nightly::login:ghcr.io"

# ---------------------------------------------------------------------------
# Baseline behaviour
# ---------------------------------------------------------------------------
d="$(new_case baseline-suppresses)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a baselined finding does not fail" 0 "${d}"

d="$(new_case baseline-partial)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login ghcr
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
      - name: Login mgmt
        uses: docker/login-action@v4
        with:
          registry: docker-mgmt.nginx.com
EOF
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a new finding beside a baselined one still fails" 1 "${d}" "w.yml::publish::login:docker-mgmt.nginx.com"

d="$(new_case baseline-partial-no-noise)"
cp "${TMP_ROOT}/baseline-partial/workflows/w.yml" "${d}/workflows/w.yml"
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect_absent "the baselined entry is not reported again" 1 "${d}" "publish::login:ghcr.io"

d="$(new_case baseline-stale)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
echo "w.yml::publish::login:ghcr.io" >"${d}/baseline.txt"
expect "a fixed finding makes its baseline entry stale" 1 "${d}" "stale baseline"

# ---------------------------------------------------------------------------
# Allowlist
# ---------------------------------------------------------------------------
d="$(new_case allowlisted)"
cat >"${d}/workflows/shared.yml" <<'EOF'
name: shared
on:
  workflow_call:
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
printf '# gated by its caller\nshared.yml\n' >"${d}/allowlist.txt"
expect "an allowlisted workflow is skipped" 0 "${d}"

# ---------------------------------------------------------------------------
# Promotion through the script, not inline skopeo
#
# release-publish.yml promotes by calling copy-images.sh. The literal
# `skopeo copy` never appears in the workflow, so detecting only that left an
# ungated promotion job invisible -- found by mutation-testing the publish
# workflow against this checker.
# ---------------------------------------------------------------------------
d="$(new_case copy-images-script)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  promote:
    runs-on: ubuntu-24.04
    steps:
      - name: Promote by digest
        run: .github/scripts/copy-images.sh --config production --target-tag 2.8.0
EOF
expect "promotion via copy-images.sh is detected" 1 "${d}" "w.yml::promote::skopeo-copy"

d="$(new_case copy-images-script-gated)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  promote:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
    runs-on: ubuntu-24.04
    steps:
      - name: Promote by digest
        run: .github/scripts/copy-images.sh --config production --target-tag 2.8.0
EOF
expect "a gated promotion via copy-images.sh passes" 0 "${d}"

# ---------------------------------------------------------------------------
# Entry-level allowlist
#
# The fixture has two ungated destinations in one job, so every case can
# distinguish "exempted the named one" from "stopped checking the file".
# ---------------------------------------------------------------------------
two_dest_workflow() {
    cat >"$1/workflows/two.yml" <<'EOF'
name: two
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
      - name: Push the chart
        run: helm push chart.tgz oci://ghcr.io/nginx/charts
EOF
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

# The mutation that matters: an entry-level line must not behave like a
# basename line. If it silently exempted the whole file, the helm push above
# would vanish too and the check would pass.
d="$(new_case entry-allowlist-is-not-file-level)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\n' >"${d}/allowlist.txt"
expect "an entry-level exemption leaves the rest of the file checked" 1 "${d}" "helm-push"

# Both destinations exempted individually is the same outcome as exempting the
# file, but it took two reviewed lines to get there.
d="$(new_case entry-allowlist-both)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\ntwo.yml::publish::helm-push\n' >"${d}/allowlist.txt"
expect "exempting every finding individually passes" 0 "${d}"

# An exemption that matches nothing is reported. Without this the line
# outlives the step and silently covers whatever next takes that key.
d="$(new_case entry-allowlist-stale)"
two_dest_workflow "${d}"
printf 'two.yml::publish::login:ghcr.io\ntwo.yml::publish::helm-push\ntwo.yml::publish::gh-release\n' >"${d}/allowlist.txt"
expect "an exemption matching nothing is reported" 1 "${d}" "two.yml::publish::gh-release"

d="$(new_case entry-allowlist-stale-after-gating)"
cat >"${d}/workflows/two.yml" <<'EOF'
name: two
on: [push]
jobs:
  publish:
    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
printf 'two.yml::publish::login:ghcr.io\n' >"${d}/allowlist.txt"
expect "gating a step makes its exemption stale" 1 "${d}" "no longer match"

# An exempted finding must not be written to the baseline: it would then be
# recorded as debt to pay off, which is the opposite of a decision that stays.
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

# A basename line must keep working; entry keys are an addition, not a
# replacement.
d="$(new_case file-level-still-works)"
two_dest_workflow "${d}"
printf 'two.yml\n' >"${d}/allowlist.txt"
expect "a basename exemption still skips the whole file" 0 "${d}"

# ---------------------------------------------------------------------------
# The scanner's structural assumption must be checked, not assumed
# ---------------------------------------------------------------------------
d="$(new_case bad-indentation)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
    - name: Login
      uses: docker/login-action@v4
      with:
        registry: ghcr.io
EOF
expect "unexpected step indentation is reported, not skipped" 1 "${d}" "expected 6"

# ---------------------------------------------------------------------------
# Things outside the jobs block must not be mistaken for steps
# ---------------------------------------------------------------------------
d="$(new_case push-trigger-not-a-push)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on:
  push:
    branches:
      - main
jobs:
  test:
    runs-on: ubuntu-24.04
    steps:
      - name: Test
        run: make unit-test
EOF
expect "an on.push trigger is not a publishing step" 0 "${d}"

# ---------------------------------------------------------------------------
# --update-baseline writes what the checker would otherwise report
# ---------------------------------------------------------------------------
d="$(new_case update-baseline)"
cat >"${d}/workflows/w.yml" <<'EOF'
name: w
on: [push]
jobs:
  publish:
    runs-on: ubuntu-24.04
    steps:
      - name: Login
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
EOF
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

# ---------------------------------------------------------------------------
# Invocation errors
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
printf '\n%s passed, %s failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ]
