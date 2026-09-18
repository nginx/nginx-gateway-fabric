#!/usr/bin/env bash
#
# bump-nginx-versions.sh
#
# Updates the pinned NGINX, NGINX Plus, and F5 WAF for NGINX versions across
# the build files. The WAF module version is given once, in apk format, and
# also written in rpm format for the UBI images.
#
# Usage:
#   bump-nginx-versions.sh --show
#   bump-nginx-versions.sh [--nginx-oss X] [--nginx-source X] [--nginx-plus X]
#                          [--nap-waf-module X] [--nap-waf-release X]
#                          [--agent X] [--dry-run]
#
# Options:
#   --show               print the current pinned versions and exit
#   --nginx-oss X        NGINX OSS package version, e.g. 1.31.5
#   --nginx-source X     NGINX source version built for the Rust FFI bindings,
#                        e.g. 1.31.3
#   --nginx-plus X       NGINX Plus release, e.g. R37.1
#   --nap-waf-module X   WAF module version in apk format, e.g. 37.1.5.715
#   --nap-waf-release X  WAF release deployed as the sidecar tag, e.g. 5.15.0
#   --agent X            NGINX Agent version, e.g. v3.12.0
#   --dry-run            report what would change without writing
#
# Exit status:
#   0  done, or nothing to do
#   1  a file or pin was not found where expected
#   2  bad invocation, including a malformed version

set -euo pipefail

ROOT="${BUMP_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

NGINX_OSS=""
NGINX_SOURCE=""
NGINX_PLUS=""
NAP_WAF_MODULE=""
NAP_WAF_RELEASE=""
AGENT=""
SHOW=0
DRY_RUN=0

OSS_DOCKERFILES=("build/Dockerfile.nginx" "build/ubi/Dockerfile.nginx")
PLUS_DOCKERFILES=("build/Dockerfile.nginxplus" "build/ubi/Dockerfile.nginxplus")
ALL_DOCKERFILES=("${OSS_DOCKERFILES[@]}" "${PLUS_DOCKERFILES[@]}")
WAF_GO="internal/framework/waf/waf.go"

usage() {
    sed -n '/^# Usage:/,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

die() {
    echo "error: $*" >&2
    exit 2
}

while [ $# -gt 0 ]; do
    case "$1" in
    --show)
        SHOW=1
        shift
        ;;
    --dry-run)
        DRY_RUN=1
        shift
        ;;
    --nginx-oss | --nginx-source | --nginx-plus | --nap-waf-module | --nap-waf-release | --agent)
        [ $# -ge 2 ] || die "$1 needs a value"
        case "$1" in
        --nginx-oss) NGINX_OSS="$2" ;;
        --nginx-source) NGINX_SOURCE="$2" ;;
        --nginx-plus) NGINX_PLUS="$2" ;;
        --nap-waf-module) NAP_WAF_MODULE="$2" ;;
        --nap-waf-release) NAP_WAF_RELEASE="$2" ;;
        --agent) AGENT="$2" ;;
        esac
        shift 2
        ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        die "unknown argument '$1'"
        ;;
    esac
done

# A malformed version could silently select the wrong package, so reject it here.
validate() {
    local what="$1" value="$2" pattern="$3" example="$4"
    printf '%s' "${value}" | grep -Eq "${pattern}" ||
        die "${what} '${value}' is not in the expected format (e.g. ${example})"
}

[ -z "${NGINX_OSS}" ] || validate "NGINX OSS version" "${NGINX_OSS}" '^[0-9]+\.[0-9]+\.[0-9]+$' "1.31.5"
[ -z "${NGINX_SOURCE}" ] || validate "NGINX source version" "${NGINX_SOURCE}" '^[0-9]+\.[0-9]+\.[0-9]+$' "1.31.3"
[ -z "${NGINX_PLUS}" ] || validate "NGINX Plus release" "${NGINX_PLUS}" '^R[0-9]+(\.[0-9]+)?$' "R37.1"
[ -z "${NAP_WAF_RELEASE}" ] || validate "WAF release" "${NAP_WAF_RELEASE}" '^[0-9]+\.[0-9]+\.[0-9]+$' "5.15.0"
[ -z "${AGENT}" ] || validate "agent version" "${AGENT}" '^v[0-9]+\.[0-9]+\.[0-9]+$' "v3.12.0"

NAP_WAF_MODULE_RPM=""
if [ -n "${NAP_WAF_MODULE}" ]; then
    validate "WAF module version" "${NAP_WAF_MODULE}" \
        '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' "37.1.5.715"
    # 37.1.5.715 (apk) becomes 37.1+5.715 (rpm): the second separator differs.
    NAP_WAF_MODULE_RPM="$(printf '%s' "${NAP_WAF_MODULE}" |
        awk -F. '{ printf "%s.%s+%s.%s", $1, $2, $3, $4 }')"
fi

arg_value() {
    local file="$1" name="$2"
    awk -v name="${name}" '
    $0 ~ "^ARG " name "=" { sub("^ARG " name "=", ""); sub(/[[:space:]]*#.*$/, ""); print; exit }
  ' "${ROOT}/${file}"
}

go_const_value() {
    sed -n 's/^const Release = "\(.*\)"$/\1/p' "${ROOT}/${WAF_GO}"
}

CHANGED=0
MISSING=0

# set_arg <file> <arg-name> <new-value>
set_arg() {
    local file="$1" name="$2" want="$3" path="${ROOT}/$1" current
    current="$(arg_value "${file}" "${name}")"

    if [ -z "${current}" ]; then
        echo "MISSING: ${file} has no 'ARG ${name}='" >&2
        MISSING=$((MISSING + 1))
        return
    fi

    if [ "${current}" = "${want}" ]; then
        printf '  %-34s %-28s unchanged\n' "${name}" "${current}"
        return
    fi

    printf '  %-34s %-28s -> %s\n' "${name}" "${current}" "${want}"
    CHANGED=$((CHANGED + 1))

    [ "${DRY_RUN}" -eq 1 ] && return

    # Anchored on the ARG line so that a version appearing elsewhere in the file,
    # such as inside a package name, is left alone.
    awk -v name="${name}" -v want="${want}" '
    $0 ~ "^ARG " name "=" {
      comment = ""
      if (match($0, /[[:space:]]*#.*$/)) comment = substr($0, RSTART)
      print "ARG " name "=" want comment
      next
    }
    { print }
  ' "${path}" >"${path}.tmp" && mv "${path}.tmp" "${path}"
}

set_go_const() {
    local want="$1" path="${ROOT}/${WAF_GO}" current
    current="$(go_const_value)"

    if [ -z "${current}" ]; then
        echo "MISSING: ${WAF_GO} has no 'const Release = \"...\"'" >&2
        MISSING=$((MISSING + 1))
        return
    fi

    if [ "${current}" = "${want}" ]; then
        printf '  %-34s %-28s unchanged\n' "waf.Release" "${current}"
        return
    fi

    printf '  %-34s %-28s -> %s\n' "waf.Release" "${current}" "${want}"
    CHANGED=$((CHANGED + 1))

    [ "${DRY_RUN}" -eq 1 ] && return

    sed "s/^const Release = \".*\"$/const Release = \"${want}\"/" "${path}" >"${path}.tmp" &&
        mv "${path}.tmp" "${path}"
}

if [ "${SHOW}" -eq 1 ]; then
    for f in "${ALL_DOCKERFILES[@]}"; do
        echo "${f}"
        for name in NGINX_VERSION NGINX_PLUS_VERSION APP_PROTECT_VERSION NGINX_AGENT_VERSION; do
            v="$(arg_value "${f}" "${name}")"
            [ -n "${v}" ] && printf '  %-24s %s\n' "${name}" "${v}"
        done
    done
    echo "${WAF_GO}"
    printf '  %-24s %s\n' "Release" "$(go_const_value)"
    exit 0
fi

if [ -z "${NGINX_OSS}${NGINX_SOURCE}${NGINX_PLUS}${NAP_WAF_MODULE}${NAP_WAF_RELEASE}${AGENT}" ]; then
    echo "error: nothing to do; pass at least one version, or --show" >&2
    usage >&2
    exit 2
fi

[ "${DRY_RUN}" -eq 1 ] && echo "Dry run, no files will be written."

if [ -n "${NGINX_OSS}" ]; then
    echo "NGINX OSS:"
    for f in "${OSS_DOCKERFILES[@]}"; do
        echo " ${f}"
        set_arg "${f}" NGINX_VERSION "${NGINX_OSS}"
    done
fi

if [ -n "${NGINX_SOURCE}" ]; then
    # The Plus images build the Rust guardrails module against NGINX source,
    # which is pinned separately from the Plus package version.
    echo "NGINX source:"
    for f in "${PLUS_DOCKERFILES[@]}"; do
        echo " ${f}"
        set_arg "${f}" NGINX_VERSION "${NGINX_SOURCE}"
    done
fi

if [ -n "${NGINX_PLUS}" ]; then
    echo "NGINX Plus:"
    for f in "${PLUS_DOCKERFILES[@]}"; do
        echo " ${f}"
        set_arg "${f}" NGINX_PLUS_VERSION "${NGINX_PLUS}"
    done
fi

if [ -n "${NAP_WAF_MODULE}" ]; then
    echo "F5 WAF module (two formats from one version):"
    echo " build/Dockerfile.nginxplus (apk)"
    set_arg "build/Dockerfile.nginxplus" APP_PROTECT_VERSION "${NAP_WAF_MODULE}"
    echo " build/ubi/Dockerfile.nginxplus (rpm)"
    set_arg "build/ubi/Dockerfile.nginxplus" APP_PROTECT_VERSION "${NAP_WAF_MODULE_RPM}"
fi

if [ -n "${NAP_WAF_RELEASE}" ]; then
    echo "F5 WAF release:"
    echo " ${WAF_GO}"
    set_go_const "${NAP_WAF_RELEASE}"
fi

if [ -n "${AGENT}" ]; then
    echo "NGINX Agent:"
    for f in "${ALL_DOCKERFILES[@]}"; do
        echo " ${f}"
        set_arg "${f}" NGINX_AGENT_VERSION "${AGENT}"
    done
fi

echo
if [ "${MISSING}" -gt 0 ]; then
    echo "${MISSING} pin(s) were not found. The build files may have been"
    echo "restructured; this script needs updating rather than working around."
    exit 1
fi

if [ "${CHANGED}" -eq 0 ]; then
    echo "Nothing to change."
else
    echo "${CHANGED} pin(s) $([ "${DRY_RUN}" -eq 1 ] && echo "would change" || echo "changed")."
fi
