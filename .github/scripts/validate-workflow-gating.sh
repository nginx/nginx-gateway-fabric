#!/usr/bin/env bash
#
# Fails when a workflow step that publishes to a shared destination is not
# gated on github.repository. Neither github.repository_owner nor the event
# name can tell repositories apart; only github.repository can.
#
# A gate is `github.repository == '<repo>'` or `== vars.INTERNAL_REPOSITORY`
# as the first term of a top-level && chain with no top-level || (see
# lib/gating-expr.sh), on the job, the step, or a metadata-action image's
# enable=. Requires yq (mikefarah, v4) and jq.
#
# Usage:
#   validate-workflow-gating.sh [--workflows DIR] [--update-baseline] [--quiet]
#
# Exit status:
#   0  no ungated publishing steps outside the baseline
#   1  at least one ungated publishing step, or a stale baseline entry
#   2  bad invocation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKFLOW_DIR="${SCRIPT_DIR}/../workflows"
ALLOWLIST="${SCRIPT_DIR}/../config/workflow-gating-allowlist.txt"
BASELINE="${SCRIPT_DIR}/../config/workflow-gating-baseline.txt"
UPDATE_BASELINE=0
QUIET=0

usage() {
    sed -n '/^# Usage:/,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
    case "$1" in
    --workflows)
        [ $# -ge 2 ] || {
            echo "error: --workflows needs a directory" >&2
            exit 2
        }
        WORKFLOW_DIR="$2"
        shift 2
        ;;
    --allowlist)
        [ $# -ge 2 ] || {
            echo "error: --allowlist needs a file" >&2
            exit 2
        }
        ALLOWLIST="$2"
        shift 2
        ;;
    --baseline)
        [ $# -ge 2 ] || {
            echo "error: --baseline needs a file" >&2
            exit 2
        }
        BASELINE="$2"
        shift 2
        ;;
    --update-baseline)
        UPDATE_BASELINE=1
        shift
        ;;
    --quiet)
        QUIET=1
        shift
        ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        echo "error: unknown argument '$1'" >&2
        usage >&2
        exit 2
        ;;
    esac
done

[ -d "${WORKFLOW_DIR}" ] || {
    echo "error: workflow directory not found: ${WORKFLOW_DIR}" >&2
    exit 2
}

say() { [ "${QUIET}" -eq 1 ] || printf '%s\n' "$*"; }

# shellcheck source=lib/gating-expr.sh
. "${SCRIPT_DIR}/lib/gating-expr.sh"

command -v yq >/dev/null 2>&1 || {
    echo "error: yq is required" >&2
    exit 2
}

# Separates record fields. Not a tab: bash's `read` merges runs of whitespace
# separators, so an empty field would shift every field after it.
SEP=$'\x1f'

# Finds the publishing operations in one workflow. The file is read by yq,
# not matched on indentation, so comments, quoting, flow-style mappings and
# folded `if:` blocks all arrive as the values Actions itself would see.
#
# Emits one record per publishing operation, fields separated by SEP:
#   <job> <destination> <job if> <step if> <extra condition>
# where <extra condition> is a metadata-action image's `enable=` expression,
# or empty. An operation is gated when any of the three conditions is.
#
# Destinations:
#   login:<host>        a registry login (docker/login-action with no
#                       registry: is Docker Hub, docker.io)
#   image-target:<host> a docker/metadata-action image name, in a job whose
#                       build-push-action can push
#   build-push          docker/build-push-action pushing tags it was given
#                       literally (tags from metadata-action are decided by
#                       that step's enable= instead)
#   gh-release          gh release create/upload/edit/delete, or gh api on
#                       a releases endpoint other than a read
#   helm-push           helm push
#   skopeo-copy         skopeo copy, or copy-images.sh, which wraps it
#   docker-push         docker push / docker image push / buildx --push
#   crane               a crane subcommand that writes to a registry
#   oras                an oras subcommand that writes to a registry
#   cosign-sign         cosign sign / attest (sign-blob writes a file)
#   git-push            any git push: a branch or tag in a shared repository
#   make-release        make release
#   goreleaser          GoReleaser invoked so that it can publish
# A destination computed by an expression is named <computed>, so the key
# does not change whenever the expression is edited.
scan_workflow() {
    yq -o=json '.' "$1" | jq -r --arg sep "${SEP}" '
      def flat: tostring | gsub("\\s+"; " ") | sub("^ "; "") | sub(" $"; "");
      def host_of($name):
        ($name | split("/")[0]) as $h
        | if ($h | test("\\$\\{\\{")) then "<computed>"
          elif ($name | test("/") | not) then "docker.io"
          elif ($h | test("[.:]") | not) and $h != "localhost" then "docker.io"
          else ($h | ascii_downcase) end;
      def registry_name:
        flat | gsub("^[\"'\'']|[\"'\'']$"; "")
        | if test("\\$\\{\\{") then "<computed>" else ascii_downcase end;
      def is_local: test("^(localhost|127\\.0\\.0\\.1)([:/]|$)");

      # True when build-push-action in this job is able to push.
      def pushes_images:
        any(.[]; ((.uses // "" | tostring) | test("^docker/build-push-action@"))
                 and (((.with // {}).push // false | tostring) | test("^(false)?$") | not));

      # [destination, extra condition] pairs for one step. $pushes says
      # whether the job pushes images at all: a metadata-action image name
      # only names tags, and is a destination only when something pushes them.
      def dests($pushes):
        (.uses // "" | tostring) as $uses
        | (.with // {}) as $with
        | (.run // "" | tostring) as $run
        | (($with.args // "" | tostring) + " " + $run) as $cmd
        | (
            # Logins: any step naming a registry, and docker/login-action
            # with none, which logs in to Docker Hub.
            ( if ($with | has("registry")) then
                ($with.registry | registry_name) as $r
                | if ($r | is_local) or $r == "" then empty else ["login:" + $r, ""] end
              elif ($uses | test("^docker/login-action@")) then ["login:docker.io", ""]
              else empty end ),

            # metadata-action: one target per image line, gated by enable=.
            ( if ($uses | test("^docker/metadata-action@")) and $pushes then
                ($with.images // "" | tostring | split("\n")[] | flat | select(. != "")
                 | . as $line
                 | (if test("(^|,)name=") then
                      (capture("(^|,)name=(?<n>\\$\\{\\{.*?\\}\\}[^,]*|[^,]*)").n)
                    else split(",")[0] end) as $name
                 | (if test(",enable=") then
                      (capture(",enable=(?<e>\\$\\{\\{.*?\\}\\}|[^,]*)").e)
                    else "" end) as $enable
                 | select($name | is_local | not)
                 | ["image-target:" + host_of($name), $enable])
              else empty end ),

            # build-push-action decides nothing itself when its tags come from
            # metadata-action; literal tags are a destination of its own.
            ( if ($uses | test("^docker/build-push-action@"))
                 and (($with.push // false | tostring) | test("^(false)?$") | not)
                 and (($with.tags // "" | tostring) | test("^\\s*\\$\\{\\{\\s*steps\\.[^.]+\\.outputs\\.tags\\s*\\}\\}\\s*$") | not)
              then ["build-push", ""] else empty end ),

            ( if ($run | test("\\bgh\\s+release\\s+(create|upload|edit|delete)\\b")) then ["gh-release", ""] else empty end ),
            # gh api reads unless told otherwise: it writes with -X/--method
            # other than GET, or implicitly as a POST once it has fields.
            ( if ($run | split("\n") | any(test("\\bgh\\s+api\\b.*/releases")
                   and (test("(-X|--method)[ =]?(POST|PATCH|PUT|DELETE)\\b"; "i")
                        or (test("\\s(-f|-F|--field|--raw-field|--input)[ =]") and (test("(-X|--method)[ =]?GET\\b"; "i") | not)))))
              then ["gh-release", ""] else empty end ),
            ( if ($run | test("\\bhelm\\s+push\\b")) then ["helm-push", ""] else empty end ),
            ( if ($run | test("\\bskopeo\\s+copy\\b|copy-images\\.sh")) then ["skopeo-copy", ""] else empty end ),
            ( if ($run | test("\\bdocker\\s+(image\\s+)?push\\b|\\bdocker\\s+buildx\\s+.*--push\\b")) then ["docker-push", ""] else empty end ),
            ( if ($run | test("\\bcrane\\s+(copy|cp|push|tag|append|mutate|delete|rebase|flatten|index)\\b")) then ["crane", ""] else empty end ),
            ( if ($run | test("\\boras\\s+(push|cp|copy|attach|tag|manifest\\s+push)\\b")) then ["oras", ""] else empty end ),
            ( if ($run | test("\\bcosign\\s+(sign|attest)(\\s|$)")) then ["cosign-sign", ""] else empty end ),
            ( if ($run | test("\\bgit\\s+push\\b")) then ["git-push", ""] else empty end ),
            ( if ($run | test("\\bmake\\s+([^\\n]*\\s)?release(\\s|$)")) then ["make-release", ""] else empty end ),

            # GoReleaser publishes on `release` or `publish` unless the run is a
            # snapshot; `release --snapshot` builds and signs but publishes nothing.
            ( if ($uses | test("^goreleaser/goreleaser-action@")) or ($run | test("\\bgoreleaser\\s+(release|publish|build)\\b")) then
                ($cmd | gsub("release\\s+--snapshot"; "")) as $stripped
                | if ($stripped | test("(^|[^-_A-Za-z0-9])(release|publish)([^-_A-Za-z0-9]|$)"))
                     or ($cmd | test("--snapshot") | not)
                  then ["goreleaser", ""] else empty end
              else empty end )
          );

      (.jobs // {}) | to_entries[]
      | .key as $job
      | (.value.if // "" | flat) as $job_if
      | (.value.steps // []) as $steps
      | ($steps | pushes_images) as $pushes
      | $steps[]
      | (.if // "" | flat) as $step_if
      | dests($pushes)
      | [$job, .[0], $job_if, $step_if, (.[1] | flat)] | join($sep)
    '
}

# Collect findings. A bare basename exempts a whole file (never scanned
# again); a "::" line exempts one <workflow>::<job>::<destination> only.
allowed=""
entry_allowed=""
if [ -f "${ALLOWLIST}" ]; then
    allowlist_lines="$(sed -e 's/#.*//' -e 's/[[:space:]]*$//' "${ALLOWLIST}" | grep -v '^$' || true)"
    allowed="$(printf '%s\n' "${allowlist_lines}" | grep -v '::' || true)"
    entry_allowed="$(printf '%s\n' "${allowlist_lines}" | grep -F '::' || true)"
fi

is_allowlisted() {
    [ -n "${allowed}" ] || return 1
    printf '%s\n' "${allowed}" | grep -Fxq "$1"
}

is_entry_allowlisted() {
    [ -n "${entry_allowed}" ] || return 1
    printf '%s\n' "${entry_allowed}" | grep -Fxq "$1"
}

parse_errors=0
findings=""

for wf in "${WORKFLOW_DIR}"/*.yml "${WORKFLOW_DIR}"/*.yaml; do
    [ -e "${wf}" ] || continue
    base="$(basename "${wf}")"

    if is_allowlisted "${base}"; then
        continue
    fi

    if ! records="$(scan_workflow "${wf}" 2>&1)"; then
        say "error: could not read ${base}:"
        printf '%s\n' "${records}" | sed 's/^/  /'
        parse_errors=$((parse_errors + 1))
        continue
    fi
    while IFS="${SEP}" read -r job dest job_if step_if extra_if; do
        [ -n "${dest:-}" ] || continue
        if gating_is_gated "${job_if}" || gating_is_gated "${step_if}" ||
            { [ -n "${extra_if}" ] && gating_is_gated "${extra_if}"; }; then
            continue
        fi
        findings="${findings}${base}::${job}::${dest}"$'\n'
    done <<<"${records}"
done

raw_findings="$(printf '%s' "${findings}" | grep -v '^$' | sort -u || true)"

# An exemption matching nothing is reported rather than ignored: leaving it
# would silently exempt whatever next takes that key.
findings=""
while IFS= read -r finding; do
    [ -n "${finding}" ] || continue
    is_entry_allowlisted "${finding}" && continue
    findings="${findings}${finding}"$'\n'
done < <(printf '%s\n' "${raw_findings}")
findings="$(printf '%s' "${findings}" | grep -v '^$' | sort -u || true)"

stale_exemptions=""
while IFS= read -r entry; do
    [ -n "${entry}" ] || continue
    if ! printf '%s\n' "${raw_findings}" | grep -Fxq "${entry}"; then
        stale_exemptions="${stale_exemptions}${entry}"$'\n'
    fi
done < <(printf '%s\n' "${entry_allowed}")

# Baseline reconciliation
if [ "${UPDATE_BASELINE}" -eq 1 ]; then
    {
        cat <<'EOF'
# Publishing steps not yet gated on github.repository.
# Format: <workflow>::<job>::<destination>. Generated by
# validate-workflow-gating.sh --update-baseline; do not hand-edit.
# Debt with a known end state: it must shrink to nothing, never grow.
EOF
        printf '%s\n' "${findings}" | grep -v '^$' || true
    } >"${BASELINE}"
    say "wrote baseline: ${BASELINE} ($(printf '%s\n' "${findings}" | grep -cv '^$' || true) entries)"
    exit 0
fi

baseline_entries=""
if [ -f "${BASELINE}" ]; then
    baseline_entries="$(sed -e 's/#.*//' -e 's/[[:space:]]*$//' "${BASELINE}" | grep -v '^$' || true)"
fi

new_violations=""
while IFS= read -r finding; do
    [ -n "${finding}" ] || continue
    if ! printf '%s\n' "${baseline_entries}" | grep -Fxq "${finding}"; then
        new_violations="${new_violations}${finding}"$'\n'
    fi
done < <(printf '%s\n' "${findings}")

stale_baseline=""
while IFS= read -r entry; do
    [ -n "${entry}" ] || continue
    if ! printf '%s\n' "${findings}" | grep -Fxq "${entry}"; then
        stale_baseline="${stale_baseline}${entry}"$'\n'
    fi
done < <(printf '%s\n' "${baseline_entries}")

# Report
status=0

if [ -n "$(printf '%s' "${new_violations}")" ]; then
    status=1
    echo "FAIL: publishing step not gated on github.repository"
    echo
    printf '%s\n' "${new_violations}" | grep -v '^$' | sed 's/^/  /'
    echo
    cat <<'EOF'
Each entry is <workflow>::<job>::<destination>.

This step publishes to a shared destination, but nothing stops it running from
any other repository holding this workflow file. Add a gate to the step or its
job:

    if: ${{ github.repository == 'nginx/nginx-gateway-fabric' }}

github.repository_owner does not work here: every repository in the
organisation reports the same owner. If the workflow is a reusable one that
several repositories call on purpose, add it to workflow-gating-allowlist.txt
with a reason.
EOF
fi

if [ -n "$(printf '%s' "${stale_baseline}")" ]; then
    status=1
    echo "FAIL: stale baseline entries"
    echo
    printf '%s\n' "${stale_baseline}" | grep -v '^$' | sed 's/^/  /'
    echo
    cat <<EOF
These are listed in $(basename "${BASELINE}") but no longer found. If you
gated or removed them, delete the lines:

    $(basename "$0") --update-baseline
EOF
fi

if [ -n "$(printf '%s' "${stale_exemptions}")" ]; then
    status=1
    echo "FAIL: allowlist entries that no longer match anything"
    echo
    printf '%s\n' "${stale_exemptions}" | grep -v '^$' | sed 's/^/  /'
    echo
    cat <<EOF
These are exempted in $(basename "${ALLOWLIST}") but the check no longer finds
them. If the step was gated or removed, delete the line. If the whole workflow
is also exempted by basename, the entry is redundant -- delete it too.
EOF
fi

if [ "${parse_errors}" -gt 0 ]; then
    status=1
    echo "FAIL: ${parse_errors} workflow file(s) could not be read, so could not be checked."
fi

if [ "${status}" -eq 0 ]; then
    n_baseline="$(printf '%s\n' "${baseline_entries}" | grep -cv '^$' || true)"
    if [ "${n_baseline}" -gt 0 ]; then
        say "OK: no new ungated publishing steps (${n_baseline} known, see $(basename "${BASELINE}"))"
    else
        say "OK: every publishing step is gated on github.repository"
    fi
fi

exit "${status}"
