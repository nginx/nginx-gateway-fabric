#!/usr/bin/env bash
set -euo pipefail

# Enforces that every directly-triggered workflow job's `if:` gates on
# github.repository, not repository_owner (identical in the public repo and
# the internal mirror). Reusable (workflow_call-only) workflows are exempt.
#
# Config (a job may appear in at most one):
#   .github/config/job-gating-runs-in-mirror.txt  -- runs in both repos
#   .github/config/job-gating-baseline.txt        -- not gated yet
#
# Run directly, or via `make lint-job-gating`.

PUBLIC_REPO="nginx/nginx-gateway-fabric"
# An expression, not a literal name, so this public tree never names the
# mirror repository.
INTERNAL_REPO_EXPR="vars\.INTERNAL_REPOSITORY"
BASELINE_FILE="${BASELINE_FILE:-.github/config/job-gating-baseline.txt}"
MIRROR_FILE="${MIRROR_FILE:-.github/config/job-gating-runs-in-mirror.txt}"
WORKFLOW_DIR="${WORKFLOW_DIR:-.github/workflows}"

# True only if the whole string is one balanced parenthesis group, e.g.
# "(a && (b || c))"; rejects "(a) || (b)" and unbalanced input.
is_single_group() {
    local s="$1" depth=0 i
    [[ $s == "("* ]] || return 1
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

# Collapses whitespace and strips one wrapping `${{ ... }}` (GitHub treats it
# as equivalent to the bare expression); "${{ a }} && ${{ b }}" is left alone.
normalize_condition() {
    local cond="$1"
    cond=$(printf '%s' "$cond" | tr -s '[:space:]' ' ' | sed 's/^ *//;s/ *$//')
    # shellcheck disable=SC2016  # '${{' is matched literally, not expanded.
    if [[ $cond == '${{'* && $cond == *'}}' ]]; then
        local inner="${cond:3:${#cond}-5}"
        # shellcheck disable=SC2016  # as above.
        if [[ $inner != *'${{'* ]]; then
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

    # <gate> or <gate> && ( ... ); the alternation forces quote-matching (no
    # mixed '..." branch). Group 2 is the optional tail, group 3 its contents.
    local gate_re="^github\.repository == ('${PUBLIC_REPO}'|\"${PUBLIC_REPO}\"|${INTERNAL_REPO_EXPR})( && \\((.+)\\))?$"
    if [[ ! $cond =~ $gate_re ]]; then
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
        echo "❌ Job gating validation failed with $errors problem(s)."
        echo "   Every job outside a reusable workflow must be gated on"
        echo "   github.repository, listed as running in both repositories, or"
        echo "   baselined as outstanding."
        exit 1
    fi

    echo "✅ Job gating is consistent ($mirrored run in both, $baselined still to gate)."
    exit 0
}

# Only run the driver when executed directly, not when sourced (e.g. by tests).
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
    main "$@"
fi
