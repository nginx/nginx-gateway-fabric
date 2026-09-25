#!/usr/bin/env bash
# emit-release-manifest.sh
#
# Prints the release manifest to stdout: the signed contract between release
# prep (private mirror) and release publish (public repo). Every image is
# recorded by digest, not tag, so what publish promotes is bit-identical to
# what prep tested.
#
# Reads from the environment:
#   RELEASE_VERSION    e.g. v2.8.0. Required.
#   OPERATOR_VERSION   e.g. v0.3.0. Required.
#   INTERNAL_BRANCH    the branch prep built from. Required.
#   INTERNAL_SHA       the commit prep built. Required, 40 hex.
#   TREE_HASH          tree hash of that commit. Required, 40 hex.
#   DIGEST_DIR         directory of digest records written by build.yml. Required.
#   CHART_DIGEST       staged chart digest. Optional; null when unset.
#   NGINX_VERSIONS     JSON object of built package/image versions. Optional, default {}.
#   ASSET_REFS         JSON array of staged asset references. Optional, default [].
#
# Exit status: 0 emitted, 2 missing or malformed input.

set -euo pipefail

# Bump only when a change would make an older publish misread a newer
# manifest (a removed or re-meaning field) -- not when adding an optional one.
SCHEMA_VERSION=1

die() {
    echo "error: $*" >&2
    exit 2
}

require() {
    local name="$1" value="$2"
    [ -n "${value}" ] || die "${name} is required"
}

is_sha1() {
    printf '%s' "$1" | grep -Eq '^[0-9a-f]{40}$'
}

# A tag here would defeat the reason the manifest exists: publish would
# silently promote whatever bytes the tag currently points to.
is_digest() {
    printf '%s' "$1" | grep -Eq '^sha256:[0-9a-f]{64}$'
}

RELEASE_VERSION="${RELEASE_VERSION:-}"
OPERATOR_VERSION="${OPERATOR_VERSION:-}"
INTERNAL_BRANCH="${INTERNAL_BRANCH:-}"
INTERNAL_SHA="${INTERNAL_SHA:-}"
TREE_HASH="${TREE_HASH:-}"
DIGEST_DIR="${DIGEST_DIR:-}"
CHART_DIGEST="${CHART_DIGEST:-}"
# Defaulted on their own lines, not with ${VAR:-{}}: bash won't strip a
# backslash before a brace in double quotes, so that form yields literal \{\}.
NGINX_VERSIONS="${NGINX_VERSIONS:-}"
ASSET_REFS="${ASSET_REFS:-}"
[ -n "${NGINX_VERSIONS}" ] || NGINX_VERSIONS='{}'
[ -n "${ASSET_REFS}" ] || ASSET_REFS='[]'

require RELEASE_VERSION "${RELEASE_VERSION}"
require OPERATOR_VERSION "${OPERATOR_VERSION}"
require INTERNAL_BRANCH "${INTERNAL_BRANCH}"
require INTERNAL_SHA "${INTERNAL_SHA}"
require TREE_HASH "${TREE_HASH}"
require DIGEST_DIR "${DIGEST_DIR}"

is_sha1 "${INTERNAL_SHA}" || die "INTERNAL_SHA must be a 40-character hex SHA, got '${INTERNAL_SHA}'"
is_sha1 "${TREE_HASH}" || die "TREE_HASH must be a 40-character hex tree hash, got '${TREE_HASH}'"
[ -d "${DIGEST_DIR}" ] || die "DIGEST_DIR is not a directory: ${DIGEST_DIR}"

if [ -n "${CHART_DIGEST}" ]; then
    is_digest "${CHART_DIGEST}" || die "CHART_DIGEST must be a sha256 digest, got '${CHART_DIGEST}'"
fi

printf '%s' "${NGINX_VERSIONS}" | jq -e 'type == "object"' >/dev/null 2>&1 ||
    die "NGINX_VERSIONS must be a JSON object"
printf '%s' "${ASSET_REFS}" | jq -e 'type == "array"' >/dev/null 2>&1 ||
    die "ASSET_REFS must be a JSON array"

# One file per build job: a matrix job can't return an output, so every leg
# writes its own file rather than the same key.
records=()
while IFS= read -r f; do
    [ -n "${f}" ] || continue
    records+=("${f}")
done < <(find "${DIGEST_DIR}" -type f -name '*.json' | sort)

# An empty image set would produce a manifest that publish accepts and that
# promotes nothing, which looks like a successful release of no images.
[ "${#records[@]}" -gt 0 ] || die "no digest records found in ${DIGEST_DIR}"

images="$(jq -s '.' "${records[@]}")" || die "a digest record is not valid JSON"

for field in image target digest platforms; do
    printf '%s' "${images}" | jq -e --arg f "${field}" \
        'all(.[]; has($f) and (.[$f] | tostring | length > 0))' >/dev/null 2>&1 ||
        die "every digest record needs a non-empty '${field}'"
done

while IFS= read -r d; do
    is_digest "${d}" || die "image digest must be a sha256 digest, got '${d}'"
done < <(printf '%s' "${images}" | jq -r '.[].digest')

# Two records for the same image/OS/platform mean a race or re-run; there is
# no way to tell here which one was tested.
dupes="$(printf '%s' "${images}" |
    jq -r 'group_by(.image + "|" + (.["base-os"] // "") + "|" + .platforms)
           | map(select(length > 1) | .[0])
           | .[] | .image + " " + (.["base-os"] // "-") + " " + .platforms')"
[ -z "${dupes}" ] || die "duplicate digest records: ${dupes}"

jq -n \
    --argjson schema "${SCHEMA_VERSION}" \
    --arg release_version "${RELEASE_VERSION}" \
    --arg operator_version "${OPERATOR_VERSION}" \
    --arg internal_branch "${INTERNAL_BRANCH}" \
    --arg internal_sha "${INTERNAL_SHA}" \
    --arg tree_hash "${TREE_HASH}" \
    --arg chart_digest "${CHART_DIGEST}" \
    --argjson images "${images}" \
    --argjson nginx_versions "${NGINX_VERSIONS}" \
    --argjson assets "${ASSET_REFS}" \
    '{
      schema_version: $schema,
      release_version: $release_version,
      operator_version: $operator_version,
      source: {
        internal_branch: $internal_branch,
        internal_sha: $internal_sha,
        tree_hash: $tree_hash
      },
      images: ($images | sort_by(.image, (.["base-os"] // ""), .platforms)),
      chart: { digest: (if $chart_digest == "" then null else $chart_digest end) },
      nginx_versions: $nginx_versions,
      assets: $assets
    }'
