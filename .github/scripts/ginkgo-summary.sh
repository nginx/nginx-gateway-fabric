#!/usr/bin/env bash
# ginkgo-summary.sh
#
# Summarises the functional suites run by run-functional-suite.sh: markdown to
# the job summary, and one ::error annotation per failed spec.
#
# Usage: ginkgo-summary.sh <results-dir> [title]
#
# Reads <results-dir>/<suite>/report.json (Ginkgo's --json-report). A suite with
# no report ended before Ginkgo wrote one, and is said to have none.
#
# Reads from the environment:
#   GITHUB_STEP_SUMMARY  file the markdown is appended to. Default: stdout.
#   GITHUB_WORKSPACE     stripped from file names, which annotations need
#                        relative to the repository.
#
# Always exits 0: the test step decides whether the job fails.

set -euo pipefail

[ $# -ge 1 ] || {
    echo "usage: $0 <results-dir> [title]" >&2
    exit 2
}
dir="$1"
title="${2:-Functional tests}"
summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
ws="${GITHUB_WORKSPACE:+${GITHUB_WORKSPACE%/}/}"

# Shared by both passes. A suite-level node (BeforeSuite) has no leaf text.
defs='
def fullname: ((.ContainerHierarchyTexts // []) + [(.LeafNodeText | select(. != "")) // .LeafNodeType]) | join(" ");
def failed: select(.State | test("^(failed|panicked|timedout|interrupted|aborted)$"));
def file: ((.Failure.Location.FileName // "") | ltrimstr($ws));
# Gomega puts the cause on later lines ("Timed out after 5s." says nothing), so keep the
# whole message, on one line without the Go type noise ("<*url.Error | 0xc000>: ").
def reason: ((.Failure.Message // "") | gsub("\\s+"; " ") | gsub("<[^>]*>:? ?"; "") | split($ws) | join("") | .[0:300]);
'

{
    echo "### ${title}"
    echo
    for suite in "${dir}"/*/; do
        [ -d "${suite}" ] || continue
        name="$(basename "${suite}")"
        report="${suite}report.json"
        if [ ! -s "${report}" ]; then
            echo "- **${name}**: no report, the run ended before Ginkgo wrote one. See \`output.log\` in the artifact."
            continue
        fi
        jq -r --arg name "${name}" --arg ws "${ws}" "${defs}"'
            [.[].SpecReports[]] as $all
            | ($all | map(select(.LeafNodeType == "It"))) as $its
            | ($all | map(failed)) as $bad
            | ($its | map(select(.State == "passed" and .NumAttempts > 1))) as $flaky
            | "- **\($name)**: \($its | map(select(.State == "passed")) | length) passed, \($bad | length) failed, \($flaky | length) flaky, \($its | map(select(.State == "skipped" or .State == "pending")) | length) skipped"
              + (if ($bad | length) > 0 then
                  "\n\n| Failed spec | Location | Failure |\n|---|---|---|\n"
                  + ($bad | map("| \(fullname | gsub("\\|"; "\\|")) | `\(file):\(.Failure.Location.LineNumber)` | \(reason | gsub("\\|"; "\\|")) |") | join("\n"))
                  + "\n" else "" end)
              + (if ($flaky | length) > 0 then
                  "\nPassed only after a retry:\n" + ($flaky | map("- \(fullname) (\(.NumAttempts) attempts)") | join("\n")) + "\n" else "" end)
        ' "${report}"
    done
} >>"${summary}"

# Annotations are workflow commands, which only count on stdout.
for report in "${dir}"/*/report.json; do
    [ -s "${report}" ] || continue
    jq -r --arg ws "${ws}" "${defs}"'
        .[].SpecReports[] | failed
        | "::error file=\(file),line=\(.Failure.Location.LineNumber)::\(
            "\(fullname): \(reason)" | gsub("%"; "%25") | gsub("\r"; "%0D"))"
    ' "${report}"
done
