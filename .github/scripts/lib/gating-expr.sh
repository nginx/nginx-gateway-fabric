#!/usr/bin/env bash
# gating-expr.sh -- sourced, not run.
#
# Reads GitHub Actions `if:` and `enable=` expressions for the two gating
# validators, so that both agree on what a repository gate is.
#
# A gate is the comparison
#
#     github.repository == '<public repository>'
#     github.repository == vars.INTERNAL_REPOSITORY
#
# as one term of a top-level `&&` chain with no `||` at top level: then the
# expression cannot be true unless the comparison is. Actions binds `&&`
# tighter than `||`, so a top-level `||` anywhere lets it be true without the
# gate; and a term that is anything other than the exact comparison -- a
# negation, a parenthesised group, github.repository compared with itself --
# is not a gate however it reads. A job's `if:` is held to a stricter form,
# the gate first, so the gating reads the same in every workflow.
#
# The expression is walked character by character, skipping string literals,
# so a bracket or `||` inside quotes is never mistaken for structure. Actions
# expressions quote with ' and escape it by doubling; " is accepted the same
# way for compatibility with gates written that way.

GATING_PUBLIC_REPO="${GATING_PUBLIC_REPO:-nginx/nginx-gateway-fabric}"

# Collapses whitespace and strips one wrapping `${{ ... }}`, which Actions
# treats as the bare expression. "${{ a }} && ${{ b }}" is left alone: it is
# a string interpolation, not an expression, and never a gate.
gating_normalize() {
    local cond="$1"
    cond=$(printf '%s' "${cond}" | tr -s '[:space:]' ' ' | sed 's/^ *//;s/ *$//')
    # shellcheck disable=SC2016 # '${{' is matched literally, not expanded.
    if [[ ${cond} == '${{'* && ${cond} == *'}}' ]]; then
        local inner="${cond:3:${#cond}-5}"
        # shellcheck disable=SC2016 # as above.
        if [[ ${inner} != *'${{'* && ${inner} != *'}}'* ]]; then
            cond=$(printf '%s' "${inner}" | sed 's/^ *//;s/ *$//')
        fi
    fi
    printf '%s' "${cond}"
}

# Splits an expression on top-level `&&`, printing one trimmed term per line.
# Returns 1, printing nothing, when the expression has a top-level `||`, an
# unbalanced bracket, or an unterminated string: none of those can be a gate.
gating_split_and() {
    local s="$1" depth=0 i c q="" start=0 n=${#1}
    local -a terms=()
    for ((i = 0; i < n; i++)); do
        c="${s:i:1}"
        if [ -n "${q}" ]; then
            if [ "${c}" = "${q}" ]; then
                # A doubled quote is an escaped quote, not the end.
                if [ "${s:i+1:1}" = "${q}" ]; then
                    i=$((i + 1))
                else
                    q=""
                fi
            fi
            continue
        fi
        case "${c}" in
        "'" | '"') q="${c}" ;;
        "(" | "[") depth=$((depth + 1)) ;;
        ")" | "]")
            depth=$((depth - 1))
            ((depth >= 0)) || return 1
            ;;
        "|")
            if ((depth == 0)) && [ "${s:i+1:1}" = "|" ]; then
                return 1
            fi
            ;;
        "&")
            if ((depth == 0)) && [ "${s:i+1:1}" = "&" ]; then
                terms+=("${s:start:i-start}")
                i=$((i + 1))
                start=$((i + 1))
            fi
            ;;
        esac
    done
    [ -z "${q}" ] && ((depth == 0)) || return 1
    terms+=("${s:start}")
    local t
    for t in "${terms[@]}"; do
        t="${t#"${t%%[![:space:]]*}"}"
        t="${t%"${t##*[![:space:]]}"}"
        printf '%s\n' "${t}"
    done
}

# True when one term is exactly the repository comparison.
gating_is_gate_term() {
    local t="$1"
    local re="^github\\.repository ?== ?('${GATING_PUBLIC_REPO}'|\"${GATING_PUBLIC_REPO}\"|vars\\.INTERNAL_REPOSITORY)\$"
    [[ ${t} =~ ${re} ]]
}

# True when the whole string is one balanced bracket group, "(a && (b || c))",
# with string literals skipped. Rejects "(a) || (b)" and "(a)(b)".
gating_is_single_group() {
    local s="$1" depth=0 i c q="" n=${#1}
    [[ ${s} == "("* ]] || return 1
    for ((i = 0; i < n; i++)); do
        c="${s:i:1}"
        if [ -n "${q}" ]; then
            if [ "${c}" = "${q}" ]; then
                if [ "${s:i+1:1}" = "${q}" ]; then i=$((i + 1)); else q=""; fi
            fi
            continue
        fi
        case "${c}" in
        "'" | '"') q="${c}" ;;
        "(") depth=$((depth + 1)) ;;
        ")") depth=$((depth - 1)) ;;
        esac
        if ((depth == 0 && i < n - 1)); then
            return 1
        fi
    done
    [ -z "${q}" ] && ((depth == 0))
}

# True when the expression is gated: some top-level `&&` term is the gate and
# nothing at top level could bypass it. For step and `enable=` conditions.
gating_is_gated() {
    local cond terms term
    cond="$(gating_normalize "$1")"
    [ -n "${cond}" ] || return 1
    terms="$(gating_split_and "${cond}")" || return 1
    while IFS= read -r term; do
        gating_is_gate_term "${term}" && return 0
    done <<<"${terms}"
    return 1
}

# The stricter form required of a job's `if:`: the gate alone, or the gate and
# exactly one bracket group holding everything else.
gating_is_strict_job_gate() {
    local cond terms count
    cond="$(gating_normalize "$1")"
    [ -n "${cond}" ] || return 1
    terms="$(gating_split_and "${cond}")" || return 1
    count="$(printf '%s\n' "${terms}" | wc -l | tr -d ' ')"
    gating_is_gate_term "$(printf '%s\n' "${terms}" | sed -n 1p)" || return 1
    case "${count}" in
    1) return 0 ;;
    2) gating_is_single_group "$(printf '%s\n' "${terms}" | sed -n 2p)" ;;
    *) return 1 ;;
    esac
}
