#!/usr/bin/env bash
set -euo pipefail

# Enforces that every directly-triggered workflow job says, in its own `if:`,
# which repository it runs in.
#
# The sync that feeds the internal release mirror force-pushes every public
# branch into it, workflow files included, so the same job definition runs in
# both repositories. `github.repository_owner` is `nginx`
# for both and cannot tell them apart; that is how a build of an embargoed
# internal branch would come to publish a public image. `github.repository`
# can tell them apart.
#
# A job is correctly gated when its `if:` has one of these shapes:
#
#     github.repository == 'nginx/nginx-gateway-fabric'
#     github.repository == vars.INTERNAL_REPOSITORY
#
# each optionally followed by ` && ( <extra conditions> )`. The gate must come
# first, and any extra conditions must be one balanced parenthesis group:
# GitHub Actions binds `&&` tighter than `||`, so "gate && (a) || (b)" parses
# as "(gate && (a)) || (b)" and runs ungated in the mirror.
#
# The mirror is named by a repository variable rather than a literal so that
# this public tree does not carry its name. The variable is defined only in
# the mirror, so in the public repository and in forks it expands to the empty
# string and the gate is false -- an internal-only job stays switched off,
# which is the safe direction to fail.
#
# The cost is that a mirror job whose variable is missing or misspelt never
# runs and never errors, which no static check can see. Anything gated to the
# mirror needs a live smoke test the first time it is wired up.
#
# Reusable workflows -- those whose only trigger is `workflow_call` -- are
# exempt. They cannot be triggered directly, so they run under the caller's
# gate. This is read from each file's own `on:` rather than from a list of
# names, so converting a workflow to or from reusable cannot leave a list
# stale.
#
# Two config files carry the exceptions, and a job may appear in at most one:
#
#   .github/config/workflow-gating-runs-in-mirror.txt
#       Jobs that legitimately run in BOTH repositories -- cherry-pick pull
#       requests are raised against internal/**, so the cheap checks have to
#       run there too. The grammar above cannot express "either repository"
#       without a top-level `||`, which is precisely the bypass it exists to
#       reject, so the decision is recorded here instead where it is reviewed
#       as a diff. A job listed here must NOT also carry a gate.
#
#   .github/config/workflow-gating-baseline.txt
#       Jobs that are not gated yet. The work list for the release pipeline
#       split: delete a line when you gate its job, or move it to the mirror
#       file when you decide it runs in both.
#
# An entry in either file that no longer describes its job is an error, so
# neither list can drift from the tree.
#
# Run directly, or via `make lint-workflow-gating`.

PUBLIC_REPO="nginx/nginx-gateway-fabric"
# Deliberately an expression, not a repository name: see the note above.
INTERNAL_REPO_EXPR="vars\.INTERNAL_REPOSITORY"
BASELINE_FILE="${BASELINE_FILE:-.github/config/workflow-gating-baseline.txt}"
MIRROR_FILE="${MIRROR_FILE:-.github/config/workflow-gating-runs-in-mirror.txt}"
WORKFLOW_DIR="${WORKFLOW_DIR:-.github/workflows}"

# Returns 0 only if the entire string is one balanced parenthesis group,
# e.g. "(a && (b || c))". Rejects "(a) || (b)" (the first '(' closes before the
# end) and any unbalanced input.
is_single_group() {
  local s="$1" depth=0 i
  [[ "$s" == "("* ]] || return 1
  for ((i = 0; i < ${#s}; i++)); do
    case "${s:i:1}" in
    "(") depth=$((depth + 1)) ;;
    ")") depth=$((depth - 1)) ;;
    esac
    # Returning to depth 0 before the final char means it is not a single group.
    if ((depth == 0 && i < ${#s} - 1)); then
      return 1
    fi
  done
  ((depth == 0))
}

# Collapses whitespace and removes a wrapping `${{ ... }}`, which GitHub treats
# as equivalent to the bare expression. Only a single wrapper spanning the
# whole condition is removed; "${{ a }} && ${{ b }}" is left alone so that it
# fails rather than being silently rewritten.
normalize_condition() {
  local cond="$1"
  cond=$(printf '%s' "$cond" | tr -s '[:space:]' ' ' | sed 's/^ *//;s/ *$//')
  # shellcheck disable=SC2016  # '${{' is matched literally, not expanded.
  if [[ "$cond" == '${{'* && "$cond" == *'}}' ]]; then
    local inner="${cond:3:${#cond}-5}"
    # shellcheck disable=SC2016  # as above.
    if [[ "$inner" != *'${{'* ]]; then
      cond=$(printf '%s' "$inner" | sed 's/^ *//;s/ *$//')
    fi
  fi
  printf '%s' "$cond"
}

# Validates a single job's `if:` condition. Prints why it failed and returns 1
# when the job is not correctly gated; returns 0 otherwise.
validate_if_condition() {
  local job_name="$1" cond
  cond=$(normalize_condition "$2")

  if [ -z "$cond" ]; then
    echo "  - Job '$job_name' has no 'if:' condition (ungated)."
    return 1
  fi

  # <gate>  or  <gate> && ( ... ). The alternation forces the quotes to match
  # (there is no mixed '..." branch). Capture group 2 is the optional tail and
  # group 3 is the tail's inner parenthesised content.
  local gate_re="^github\.repository == ('${PUBLIC_REPO}'|\"${PUBLIC_REPO}\"|${INTERNAL_REPO_EXPR})( && \\((.+)\\))?$"
  if [[ ! "$cond" =~ $gate_re ]]; then
    echo "  - Job '$job_name' is not correctly gated: '$cond'"
    echo "    Expected: github.repository == '${PUBLIC_REPO}'  [ && ( ... ) ]"
    echo "          or: github.repository == vars.INTERNAL_REPOSITORY  [ && ( ... ) ]"
    return 1
  fi

  # If there is a parenthesised tail, it must be one balanced group so that a
  # top-level '||' cannot bypass the gate (rejects e.g. "gate && (a) || (b)").
  if [ -n "${BASH_REMATCH[2]}" ] && ! is_single_group "(${BASH_REMATCH[3]})"; then
    echo "  - Job '$job_name' extra conditions must be a single ( ... ) group: '$cond'"
    return 1
  fi

  return 0
}

# Resolves the yq binary, preferring one on PATH.
resolve_yq() {
  if command -v yq &>/dev/null; then
    echo "yq"
  elif [ -x "/tmp/yq" ]; then
    echo "/tmp/yq"
  else
    echo "❌ Error: yq is not installed." >&2
    return 1
  fi
}

# True when a workflow's only trigger is workflow_call, i.e. it is reusable and
# runs under its caller's gate.
is_reusable_workflow() {
  local yq_bin="$1" file="$2" triggers
  triggers=$("$yq_bin" -r '.on | keys | join(",")' "$file" 2>/dev/null || echo "")
  [ "$triggers" = "workflow_call" ]
}

# Reads "<file>:<job>" entries into the named associative array, each mapped to
# 0 for "not yet matched". Blank lines and # comments are ignored.
# shellcheck disable=SC2034  # `dest` is a nameref: writes land in the caller's array.
read_entries() {
  local -n dest="$1"
  local path="$2" line
  [ -f "$path" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%%#*}"
    line=$(printf '%s' "$line" | sed 's/^ *//;s/ *$//')
    [ -z "$line" ] && continue
    dest["$line"]=0
  done <"$path"
}

main() {
  local yq_bin
  yq_bin=$(resolve_yq) || exit 1

  # Each entry starts unmatched; anything still unmatched at the end is stale.
  local -A baseline_seen=() mirror_seen=()
  read_entries baseline_seen "$BASELINE_FILE"
  read_entries mirror_seen "$MIRROR_FILE"

  local errors=0 baselined=0 mirrored=0
  local file jobs_data file_has_errors job_name if_cond key line
  local in_mirror in_baseline gated

  for file in "$WORKFLOW_DIR"/*.yml "$WORKFLOW_DIR"/*.yaml; do
    [ -e "$file" ] || continue

    if is_reusable_workflow "$yq_bin" "$file"; then
      continue
    fi

    # Job ids and their if conditions, one per line, internal newlines removed.
    if ! jobs_data=$("$yq_bin" -r '.jobs | to_entries | .[] | .key + ":::" + (.value.if // "" | split("\n") | join(" "))' "$file" 2>&1); then
      echo "❌ Error: Failed to parse or evaluate '$file' with yq:"
      echo "   $jobs_data"
      errors=$((errors + 1))
      continue
    fi

    file_has_errors=0
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      job_name="${line%%:::*}"
      if_cond="${line#*:::}"
      key="$file:$job_name"

      in_mirror=0
      in_baseline=0
      [ -n "${mirror_seen[$key]+set}" ] && in_mirror=1
      [ -n "${baseline_seen[$key]+set}" ] && in_baseline=1

      # "Runs in both" and "not done yet" are different claims about the same
      # job, so holding both means one of them is wrong.
      if [ "$in_mirror" -eq 1 ] && [ "$in_baseline" -eq 1 ]; then
        echo "❌ '$key' is listed in both $MIRROR_FILE and $BASELINE_FILE; remove one."
        mirror_seen["$key"]=1
        baseline_seen["$key"]=1
        errors=$((errors + 1))
        continue
      fi

      gated=0
      validate_if_condition "$job_name" "$if_cond" >/dev/null && gated=1

      if [ "$in_mirror" -eq 1 ]; then
        mirror_seen["$key"]=1
        if [ "$gated" -eq 1 ]; then
          echo "❌ '$key' is gated to one repository but listed as running in both; remove one."
          errors=$((errors + 1))
        else
          mirrored=$((mirrored + 1))
        fi
        continue
      fi

      if [ "$gated" -eq 1 ]; then
        if [ "$in_baseline" -eq 1 ]; then
          echo "❌ Stale baseline entry '$key': the job is gated now, remove the line."
          errors=$((errors + 1))
        fi
        continue
      fi

      if [ "$in_baseline" -eq 1 ]; then
        baseline_seen["$key"]=1
        baselined=$((baselined + 1))
        continue
      fi

      if [ "$file_has_errors" -eq 0 ]; then
        echo "❌ File '$file' has ungated or poorly gated jobs:"
        file_has_errors=1
      fi
      validate_if_condition "$job_name" "$if_cond" || true
      errors=$((errors + 1))
    done <<<"$jobs_data"
  done

  # Entries that matched no job: renamed, removed, or already gated. Left in
  # place they would silently excuse a future job of the same name.
  for key in "${!baseline_seen[@]}"; do
    if [ "${baseline_seen[$key]}" -eq 0 ]; then
      echo "❌ Stale baseline entry '$key': no such ungated job, remove the line."
      errors=$((errors + 1))
    fi
  done
  for key in "${!mirror_seen[@]}"; do
    if [ "${mirror_seen[$key]}" -eq 0 ]; then
      echo "❌ Stale entry '$key' in $MIRROR_FILE: no such job, remove the line."
      errors=$((errors + 1))
    fi
  done

  if [ "$errors" -ne 0 ]; then
    echo "❌ Workflow gating validation failed with $errors problem(s)."
    echo "   Every job outside a reusable workflow must be gated on"
    echo "   github.repository, listed as running in both repositories, or"
    echo "   baselined as outstanding."
    exit 1
  fi

  echo "✅ Workflow gating is consistent ($mirrored run in both, $baselined still to gate)."
  exit 0
}

# Only run the driver when executed directly, not when sourced (e.g. by tests).
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
