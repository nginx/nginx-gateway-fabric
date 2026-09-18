#!/usr/bin/env bash
#
# validate-workflow-gating.sh
#
# Fails when a workflow step that publishes an artifact to a shared destination
# is not gated on repository identity.
#
# Why this exists: neither `github.repository_owner` nor the event name can
# identify which repository a workflow is running in. Several repositories in
# an organisation can share these workflow files, and they all report the same
# owner and fire the same events. A publishing step gated on either will
# publish from any repository that runs it.
#
# The gate that works is `github.repository`, on the step or on its job.
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
    sed -n '3,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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

# ---------------------------------------------------------------------------
# The scanner.
#
# Emits one tab-separated record per publishing step:
#   <workflow-basename> <job> <step> <destination> <gate>
# where <gate> is "job", "step", or "none".
#
# GitHub workflow files in this repository are uniform: jobs at indent 2, job
# keys at indent 4, steps at indent 6. The scanner relies on that rather than
# on a YAML parser, so it has no dependencies beyond awk. `validate` checks the
# assumption holds and fails loudly if a file breaks it.
# ---------------------------------------------------------------------------
scan_workflow() {
    awk -v fname="$(basename "$1")" '
    function flush_step() {
      if (step_name == "") return
      # GoReleaser only publishes when asked to. Its arguments are on a
      # different line from the action reference, so the decision has to wait
      # until the whole step has been read: a step that asks for `release` can
      # publish, one that only ever asks for a snapshot build cannot.
      if (gr && (gr_release || !gr_snapshot)) dests[dest_n++] = "goreleaser"
      for (i = 0; i < dest_n; i++) {
        gate = "none"
        if (step_gated) gate = "step"
        else if (job_gated) gate = "job"
        printf "%s\t%s\t%s\t%s\t%s\n", fname, job_name, step_name, dests[i], gate
      }
      dest_n = 0
      step_name = ""
      step_gated = 0
      gr = 0
      gr_release = 0
      gr_snapshot = 0
    }

    # True when the string has a `||` outside any parentheses. GitHub Actions
    # binds `&&` tighter than `||`, so "gate && (a) || (b)" parses as
    # "(gate && (a)) || (b)" and runs ungated. A `||` inside parentheses
    # cannot escape the gate, so only the top level matters.
    function has_toplevel_or(s,   i, c, depth) {
      depth = 0
      for (i = 1; i <= length(s); i++) {
        c = substr(s, i, 1)
        if (c == "(") depth++
        else if (c == ")") depth--
        else if (c == "|" && depth == 0 && substr(s, i + 1, 1) == "|") return 1
      }
      return 0
    }

    # A gate counts only if it compares github.repository -- not
    # github.repository_owner, which is identical in both repositories -- and
    # only if nothing can bypass it.
    #
    # `github.repository == a || github.repository == b` is rejected too. It
    # would be safe, but allowing a top-level `||` at all means reading every
    # future one to decide, and the two-repository model does not need it.
    function is_repo_gate(s) {
      t = s
      gsub(/github\.repository_owner/, "OWNER", t)
      if (t !~ /github\.repository[[:space:]]*==/) return 0
      if (has_toplevel_or(t)) return 0
      return 1
    }

    # A destination that is computed at run time cannot be named here, and
    # embedding the expression would make the baseline key change every time
    # the expression is edited. Collapse it to a stable placeholder: which
    # registry it resolves to does not change whether the step needs a gate.
    function name_dest(d) {
      if (d ~ /\$\{\{/) return "<computed>"
      return d
    }

    # Detect publishing destinations on a single line and record them against
    # the current step.
    function detect(l) {
      # A registry login. localhost is the in-workflow service registry, not a
      # shared destination.
      if (l ~ /^ *registry:[[:space:]]*[^[:space:]]/ && l !~ /localhost/) {
        d = l
        sub(/^ *registry:[[:space:]]*/, "", d)
        sub(/[[:space:]]*#.*$/, "", d)
        if (d != "" && d !~ /^registry:[0-9]/) dests[dest_n++] = "login:" name_dest(d)
      }
      # A docker/metadata-action image target. Whether an image reaches a
      # shared registry is decided here, by the per-name `enable=`
      # expression -- not by `push:`, which pushes whatever the tag list
      # ended up containing. So this is the line that matters.
      if (l ~ /name=[^,]+,enable=/) {
        host = l
        sub(/^[^=]*name=/, "", host)
        sub(/[\/,].*$/, "", host)
        enable = l
        sub(/^.*,enable=/, "", enable)
        if (host !~ /localhost/ && host != "" && !is_repo_gate(enable)) {
          dests[dest_n++] = "image-target:" name_dest(host)
        }
      }
      if (l ~ /gh release (create|upload)/) dests[dest_n++] = "gh-release"
      if (l ~ /helm push/) dests[dest_n++] = "helm-push"
      if (l ~ /skopeo copy/) dests[dest_n++] = "skopeo-copy"
      # GoReleaser: note its presence and what it was asked to do, and decide
      # at the end of the step. See flush_step.
      if (l ~ /goreleaser\/goreleaser-action/ || l ~ /goreleaser (release|publish|build)/) gr = 1
      if (l ~ /--snapshot/) gr_snapshot = 1

      # `release` publishes, but `release --snapshot` does not: it builds,
      # archives, signs, and stops. The arguments are usually a conditional
      # expression holding several alternatives, so remove the snapshot
      # releases and see whether a publishing one remains. Checking only for
      # `--snapshot` anywhere on the line would miss the common shape of
      # `<publish> or <snapshot build>`.
      #
      # `release` has to be matched as a whole word. Identifiers such as
      # `inputs.is_production_release` end in it, and matching those means the
      # check keys on the name of a workflow input rather than on the command
      # being run.
      stripped = l
      gsub(/release[[:space:]]+--snapshot/, "", stripped)
      if (stripped ~ /goreleaser (release|publish)/ ||
          (stripped ~ /args:/ && stripped ~ /[^-_[:alnum:]](release|publish)([^-_[:alnum:]]|$)/)) {
        gr_release = 1
      }
    }

    /^jobs:[[:space:]]*$/ { in_jobs = 1; next }

    # Leaving the jobs block: any key at column 0.
    in_jobs && /^[^[:space:]#]/ { flush_step(); in_jobs = 0 }

    !in_jobs { next }

    # Job header at indent 2.
    /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ {
      flush_step()
      line = $0
      sub(/^  /, "", line)
      sub(/:[[:space:]]*$/, "", line)
      job_name = line
      job_gated = 0
      next
    }

    # Job-level if at indent 4.
    /^    if:/ {
      if (is_repo_gate($0)) job_gated = 1
      next
    }

    # Step boundary at indent 6.
    /^      - / {
      flush_step()
      step_name = "(unnamed)"
      if ($0 ~ /^      - name:/) {
        line = $0
        sub(/^      - name:[[:space:]]*/, "", line)
        gsub(/^["'"'"']|["'"'"']$/, "", line)
        step_name = line
      } else if ($0 ~ /^      - uses:/) {
        line = $0
        sub(/^      - uses:[[:space:]]*/, "", line)
        sub(/[[:space:]]*#.*$/, "", line)
        step_name = line
      }
      if (is_repo_gate($0)) step_gated = 1
      detect($0)
      next
    }

    # Inside a step: keys at indent 8 or deeper.
    step_name != "" {
      if ($0 ~ /^        if:/ && is_repo_gate($0)) step_gated = 1
      detect($0)
    }

    END { flush_step() }
  ' "$1"
}

# Structural assumption check: steps must be at indent 6, since the scanner
# keys on it. A file that indents differently would be silently skipped.
check_structure() {
    local file="$1" bad
    bad="$(awk '
    /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
    in_jobs && /^[^[:space:]#]/ { in_jobs = 0 }
    in_jobs && /^ *- (name|uses):/ {
      match($0, /[^ ]/)
      if (RSTART - 1 != 6) print FILENAME ":" NR ": step at indent " RSTART - 1 ", expected 6"
    }
  ' "${file}")"
    if [ -n "${bad}" ]; then
        printf '%s\n' "${bad}"
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Collect findings
# ---------------------------------------------------------------------------
# Two kinds of line. A bare basename exempts a whole workflow file, which is
# a blunt instrument: the file is never scanned, so a publishing step added to
# it later is never seen. A line containing "::" exempts one
# <workflow>::<job>::<destination> and nothing else, which is what a
# false positive needs -- the rest of the file stays checked.
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

structure_errors=0
findings=""

for wf in "${WORKFLOW_DIR}"/*.yml "${WORKFLOW_DIR}"/*.yaml; do
    [ -e "${wf}" ] || continue
    base="$(basename "${wf}")"

    if ! errs="$(check_structure "${wf}")"; then
        say "structure: ${base}"
        printf '%s\n' "${errs}" | sed 's/^/  /'
        structure_errors=$((structure_errors + 1))
        continue
    fi

    if is_allowlisted "${base}"; then
        continue
    fi

    # `step` is part of the record but not part of the finding key: the key is
    # the destination, so moving a publish between steps of the same job is
    # not a new finding.
    # shellcheck disable=SC2034
    while IFS="$(printf '\t')" read -r f job step dest gate; do
        [ -n "${f:-}" ] || continue
        [ "${gate}" = "none" ] || continue
        findings="${findings}${f}::${job}::${dest}"$'\n'
    done < <(scan_workflow "${wf}")
done

raw_findings="$(printf '%s' "${findings}" | grep -v '^$' | sort -u || true)"

# An exemption that matches nothing is reported rather than ignored. It means
# either the step was gated -- good news, delete the line -- or it was renamed
# or removed and the exemption is now covering nothing. Both want the same fix,
# and leaving it in place would silently exempt whatever next takes that key.
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

# ---------------------------------------------------------------------------
# Baseline reconciliation
# ---------------------------------------------------------------------------
if [ "${UPDATE_BASELINE}" -eq 1 ]; then
    {
        cat <<'EOF'
# Publishing steps that are not yet gated on github.repository.
#
# Each line is <workflow>::<job>::<destination>. Generated by
# validate-workflow-gating.sh --update-baseline; do not hand-edit.
#
# This file is technical debt with a known end state. Gating one of these jobs
# on github.repository makes its line here stale, and the check then fails
# until the line is removed. The file should shrink to nothing; it must never
# grow.
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

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
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

if [ "${structure_errors}" -gt 0 ]; then
    status=1
    echo "FAIL: ${structure_errors} workflow file(s) do not match the expected step indentation."
    echo "The scanner keys on steps being at indent 6 and cannot check these files."
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
