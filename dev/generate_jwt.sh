#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat <<'USAGE'
Usage: generate_jwt.sh [options]

Generates an RS256 JWT signed by a PEM private key.

Options:
  --key PATH          Path to RSA private key in PEM format (default: dev/private_key.pem)
  --kid KID           Key ID to embed in JWT header (default: my-key-id)
  --sub SUB           JWT subject (default matches dev/generate_jwt.js)
  --name NAME         JWT name claim (default matches dev/generate_jwt.js)
  --exp-seconds N     Expiration in seconds from now (default: 3600)
  -h, --help          Show help

Environment variables (same names as options): KEY, KID, SUB, NAME, EXP_SECONDS
USAGE
}

KEY=${KEY:-"dev/private_key.pem"}
KID=${KID:-"my-key-id"}
SUB=${SUB:-"8534c6c2-bb27-4f50-a718-7178bc3d4ad0"}
NAME=${NAME:-"nginx-user"}
EXP_SECONDS=${EXP_SECONDS:-"3600"}

while [[ $# -gt 0 ]]; do
    case "$1" in
    --key)
        KEY="$2"
        shift 2
        ;;
    --kid)
        KID="$2"
        shift 2
        ;;
    --sub)
        SUB="$2"
        shift 2
        ;;
    --name)
        NAME="$2"
        shift 2
        ;;
    --exp-seconds)
        EXP_SECONDS="$2"
        shift 2
        ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        echo "Unknown argument: $1" >&2
        usage
        exit 2
        ;;
    esac
done

need() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "Missing required dependency: $1" >&2
        exit 1
    }
}
need openssl
need python3
need jq
need xxd

if [[ ! -f $KEY ]]; then
    echo "Private key PEM not found: $KEY" >&2
    exit 1
fi

now=$(date +%s)
exp=$((now + EXP_SECONDS))

header_json=$(jq -c -n --arg alg "RS256" --arg typ "JWT" --arg kid "$KID" '{alg:$alg,typ:$typ,kid:$kid}')
payload_json=$(jq -c -n --arg sub "$SUB" --arg name "$NAME" --argjson exp "$exp" '{sub:$sub,name:$name,exp:$exp}')

header_b64=$(printf '%s' "$header_json" | python3 -c 'import base64,sys; raw=sys.stdin.buffer.read(); print(base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii"))')

payload_b64=$(printf '%s' "$payload_json" | python3 -c 'import base64,sys; raw=sys.stdin.buffer.read(); print(base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii"))')

signing_input="${header_b64}.${payload_b64}"

sig_b64=$(
    printf '%s' "$signing_input" |
        openssl dgst -sha256 -sign "$KEY" |
        python3 -c 'import base64,sys; sig=sys.stdin.buffer.read(); print(base64.urlsafe_b64encode(sig).rstrip(b"=").decode("ascii"))'
)

token="${signing_input}.${sig_b64}"
export TOKEN="$token"
echo "Generated JWT and saved to TOKEN environment variable"
echo "$TOKEN"
