#!/usr/bin/env bash
#
# Unit tests for validate-job-gating.sh.
#
# The condition tests source the script and exercise validate_if_condition and
# is_single_group directly; the source guard stops main() running, so they need
# no yq. The baseline tests do run main(), against fixture workflows in a temp
# directory, and therefore do need yq.
#
# Run directly: bash .github/scripts/validate-job-gating_test.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/validate-job-gating.sh"

# Sourcing the script enables `set -e`; disable it so every assertion runs even
# after an expected failure.
set +e

pass=0
fail=0

GATE_SQ="github.repository == 'nginx/nginx-gateway-fabric'"
GATE_DQ='github.repository == "nginx/nginx-gateway-fabric"'
GATE_INT="github.repository == vars.INTERNAL_REPOSITORY"

ok() { pass=$((pass + 1)); }
bad() {
  fail=$((fail + 1))
  echo "FAIL $*"
}

# assert_if <pass|fail> <description> <condition>
assert_if() {
  local want="$1" desc="$2" cond="$3" got
  if validate_if_condition "test-job" "$cond" >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi
  if [[ "$got" == "$want" ]]; then ok; else
    bad "[validate_if_condition] $desc (want=$want got=$got): '$cond'"
  fi
}

# assert_group <pass|fail> <description> <string>
assert_group() {
  local want="$1" desc="$2" str="$3" got
  if is_single_group "$str" >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi
  if [[ "$got" == "$want" ]]; then ok; else
    bad "[is_single_group] $desc (want=$want got=$got): '$str'"
  fi
}

# --- validate_if_condition: valid gates ---------------------------------------
assert_if pass "bare gate, single quotes" "$GATE_SQ"
assert_if pass "bare gate, double quotes" "$GATE_DQ"
assert_if pass "internal repository gate, by variable" "$GATE_INT"
assert_if pass "gate with simple parenthesised tail" "$GATE_SQ && (!cancelled())"
assert_if pass "gate with nested single group" "$GATE_SQ && (inputs.force && (github.ref_name == 'main' || startsWith(github.ref_name, 'release-')))"
assert_if pass "leading/trailing/extra whitespace normalized" "   $GATE_SQ    &&   (!cancelled())   "
assert_if pass "embedded newline normalized" "$GATE_SQ &&
(!cancelled())"
# GitHub treats a wrapped expression as equivalent to the bare one, and NGF
# workflows are written both ways.
assert_if pass "wrapped in \${{ }}" "\${{ $GATE_SQ }}"
assert_if pass "wrapped in \${{ }} with tail" "\${{ $GATE_SQ && (!cancelled()) }}"

# --- validate_if_condition: invalid gates -------------------------------------
assert_if fail "empty condition" ""
assert_if fail "whitespace-only condition" "   "
assert_if fail "no gate at all" "!cancelled()"
assert_if fail "gate not first" "foo && $GATE_SQ"
assert_if fail "mismatched quotes" "github.repository == 'nginx/nginx-gateway-fabric\""
assert_if fail "wrong repository" "github.repository == 'nginx/other-repo'"
# The owner is 'nginx' for the public repository and the internal mirror alike,
# which is the whole reason this check exists.
assert_if fail "owner-level gate is not enough" "github.repository_owner == 'nginx'"
# The mirror is named by a variable so that this public tree does not carry
# its name. Rejecting any literal other than the public repository is what
# keeps that true as jobs are added -- which is also why the placeholder
# below is a stand-in and the real name must never be written here.
assert_if fail "a hardcoded mirror name is rejected" \
    "github.repository == 'nginx/example-internal-mirror'"
assert_if fail "a near-miss variable name is rejected" "github.repository == vars.INTERNAL_REPO"
# `.` is a regex metacharacter, so the expression has to be escaped.
assert_if fail "the variable expression is matched literally" \
    "github.repository == varsXINTERNAL_REPOSITORY"
assert_if fail "top-level || bypass" "$GATE_SQ && (a) || (b)"
assert_if fail "two separate groups" "$GATE_SQ && (a) && (b)"
assert_if fail "unparenthesised tail" "$GATE_SQ && a"
assert_if fail "unparenthesised tail with ||" "$GATE_SQ && a || b"
# Two wrappers are not a single expression, so the condition is left as-is and
# fails rather than being silently rewritten into something that passes.
assert_if fail "two separate \${{ }} expressions" "\${{ $GATE_SQ }} && \${{ true }}"

# --- is_single_group ----------------------------------------------------------
assert_group pass "single group" "(a)"
assert_group pass "nested groups" "(a && (b || c))"
assert_group pass "double-wrapped" "((a))"
assert_group fail "two top-level groups with ||" "(a) || (b)"
assert_group fail "adjacent groups" "(a)(b)"
assert_group fail "unbalanced open" "(a"
assert_group fail "unbalanced close" "a)"
assert_group fail "no parentheses" "a"
assert_group fail "empty string" ""

# --- main(): reusable-workflow exemption and baseline handling ----------------
#
# These prove the check can fail, not just that it passes on the tree as it
# stands: a green run against an already-clean tree says nothing.

if ! command -v yq &>/dev/null && [ ! -x /tmp/yq ]; then
  if [ -n "${CI:-}" ]; then
    echo "FAIL yq is required to run the baseline tests in CI"
    fail=$((fail + 1))
  else
    echo "NOTE: yq not found, skipping the baseline tests (they run in CI)"
  fi
else
  FIXTURE_DIR=$(mktemp -d)
  trap 'rm -rf "$FIXTURE_DIR"' EXIT

  # A reusable workflow with an ungated job: exempt, so never an error.
  cat >"$FIXTURE_DIR/reusable.yml" <<'YAML'
on:
  workflow_call:
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - run: "true"
YAML

  # A directly-triggered workflow with one gated and one ungated job.
  cat >"$FIXTURE_DIR/direct.yml" <<'YAML'
on:
  push:
jobs:
  gated:
    if: github.repository == 'nginx/nginx-gateway-fabric'
    runs-on: ubuntu-24.04
    steps:
      - run: "true"
  ungated:
    runs-on: ubuntu-24.04
    steps:
      - run: "true"
YAML

  BASELINE_TMP="$FIXTURE_DIR/baseline.txt"
  MIRROR_TMP="$FIXTURE_DIR/mirror.txt"
  : >"$MIRROR_TMP"
  # Read by main() from the sourced script, which shellcheck cannot see.
  # shellcheck disable=SC2034
  WORKFLOW_DIR="$FIXTURE_DIR"
  # shellcheck disable=SC2034
  BASELINE_FILE="$BASELINE_TMP"
  # shellcheck disable=SC2034
  MIRROR_FILE="$MIRROR_TMP"

  # Prefixes each line of captured output so a failure reads clearly.
  indent() { printf '%s\n' "$1" | while IFS= read -r l; do printf '      %s\n' "$l"; done; }

  # run_main <expected exit code> <description> [grep pattern that must appear]
  run_main() {
    local want="$1" desc="$2" pattern="${3:-}" out rc
    out=$( (main) 2>&1 )
    rc=$?
    if [ "$rc" -ne "$want" ]; then
      bad "[main] $desc (want exit=$want got=$rc)"
      indent "$out"
      return
    fi
    if [ -n "$pattern" ] && ! grep -q "$pattern" <<<"$out"; then
      bad "[main] $desc: output did not mention '$pattern'"
      indent "$out"
      return
    fi
    ok
  }

  # An ungated job with no baseline entry fails.
  : >"$BASELINE_TMP"
  run_main 1 "ungated job with empty baseline fails" "ungated"

  # Baselining it lets it through, and the reusable workflow's ungated job is
  # never reported at all.
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$BASELINE_TMP"
  run_main 0 "baselined job passes" "1 still to gate"

  # Comments and blank lines are ignored.
  printf '# a comment\n\n%s\n' "$FIXTURE_DIR/direct.yml:ungated" >"$BASELINE_TMP"
  run_main 0 "comments and blanks ignored" "1 still to gate"

  # A baseline entry for a job that is gated is stale and fails, so the list
  # cannot outlive the work it tracks.
  printf '%s\n%s\n' "$FIXTURE_DIR/direct.yml:ungated" "$FIXTURE_DIR/direct.yml:gated" >"$BASELINE_TMP"
  run_main 1 "baseline entry for a gated job is stale" "Stale baseline entry"

  # So is an entry naming a job that no longer exists.
  printf '%s\n%s\n' "$FIXTURE_DIR/direct.yml:ungated" "$FIXTURE_DIR/direct.yml:deleted-job" >"$BASELINE_TMP"
  run_main 1 "baseline entry for a missing job is stale" "no such ungated job"

  # --- the runs-in-mirror allowlist ------------------------------------------

  # A job listed as running in both repositories needs no gate.
  : >"$BASELINE_TMP"
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$MIRROR_TMP"
  run_main 0 "mirror-listed job passes without a gate" "1 run in both"

  # But it must not also carry one: gated and "runs in both" contradict.
  echo "$FIXTURE_DIR/direct.yml:gated" >"$MIRROR_TMP"
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$BASELINE_TMP"
  run_main 1 "gated job listed as running in both fails" "listed as running in both"

  # Nor may a job be both decided and outstanding.
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$MIRROR_TMP"
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$BASELINE_TMP"
  run_main 1 "job in both config files fails" "listed in both"

  # A mirror entry naming no job is stale, exactly as a baseline entry is.
  echo "$FIXTURE_DIR/direct.yml:deleted-job" >"$MIRROR_TMP"
  echo "$FIXTURE_DIR/direct.yml:ungated" >"$BASELINE_TMP"
  run_main 1 "stale mirror entry fails" "no such job"

  # A reusable workflow is exempt even when nothing is listed for it.
  rm "$FIXTURE_DIR/direct.yml"
  : >"$BASELINE_TMP"
  : >"$MIRROR_TMP"
  run_main 0 "reusable workflow jobs are exempt" "0 run in both"
fi

echo "passed=$pass failed=$fail"
[ "$fail" -eq 0 ]
