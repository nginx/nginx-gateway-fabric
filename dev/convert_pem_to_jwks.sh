#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat <<'USAGE'
Usage: convert_pem_to_jwks.sh [options]

Converts an RSA public key PEM into a JWKS file compatible with the dev JWT auth setup.

Options:
  --pem PATH      Path to RSA public key in PEM format (default: dev/public_key.pem)
  --out PATH      Output JWKS file path (default: dev/secrets/auth)
  --kid KID       Key ID to embed in JWKS (default: my-key-id)
  --alg ALG       JWS alg (default: RS256)
  --use USE       JWK use (default: sig)
  -h, --help      Show help

Environment variables (same names as options): PEM, OUT, KID, ALG, USE
USAGE
}

PEM=${PEM:-"dev/public_key.pem"}
OUT=${OUT:-"dev/auth"}
KID=${KID:-"my-key-id"}
ALG=${ALG:-"RS256"}
USE=${USE:-"sig"}

while [[ $# -gt 0 ]]; do
    case "$1" in
    --pem)
        PEM="$2"
        shift 2
        ;;
    --out)
        OUT="$2"
        shift 2
        ;;
    --kid)
        KID="$2"
        shift 2
        ;;
    --alg)
        ALG="$2"
        shift 2
        ;;
    --use)
        USE="$2"
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

if [[ ! -f $PEM ]]; then
    echo "Public key PEM not found: $PEM" >&2
    exit 1
fi

# Convert PEM public key to DER (SubjectPublicKeyInfo) and extract RSA modulus/exponent.
DER_HEX=$(
    openssl pkey -pubin -in "$PEM" -outform DER 2>/dev/null | xxd -p -c 100000
)

read -r N_B64URL E_B64URL < <(
    python3 - "$DER_HEX" <<'PY'
import sys, base64

hexstr = sys.argv[1].strip()
der = bytes.fromhex(hexstr)

def read_len(b, i):
    first = b[i]
    i += 1
    if first < 0x80:
        return first, i
    n = first & 0x7f
    ln = int.from_bytes(b[i:i+n], 'big')
    i += n
    return ln, i

def read_tlv(b, i, expected_tag=None):
    tag = b[i]
    i += 1
    if expected_tag is not None and tag != expected_tag:
        raise ValueError(f"unexpected tag 0x{tag:02x}, expected 0x{expected_tag:02x}")
    ln, i = read_len(b, i)
    val = b[i:i+ln]
    i += ln
    return tag, ln, val, i

def b64url_no_pad(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b'=').decode('ascii')

# SubjectPublicKeyInfo ::= SEQUENCE { algorithm AlgorithmIdentifier, subjectPublicKey BIT STRING }
_, _, spki, idx = read_tlv(der, 0, 0x30)

# AlgorithmIdentifier
_, _, _, idx2 = read_tlv(spki, 0, 0x30)

# subjectPublicKey BIT STRING
_, _, bitstr, idx3 = read_tlv(spki, idx2, 0x03)
if not bitstr:
    raise ValueError("empty BIT STRING")

unused_bits = bitstr[0]
if unused_bits != 0:
    raise ValueError("unsupported BIT STRING with unused bits")

pubkey = bitstr[1:]

# RSAPublicKey ::= SEQUENCE { modulus INTEGER, publicExponent INTEGER }
_, _, rsa_seq, _ = read_tlv(pubkey, 0, 0x30)

_, _, n_bytes, j = read_tlv(rsa_seq, 0, 0x02)
_, _, e_bytes, j = read_tlv(rsa_seq, j, 0x02)

# Strip leading 0x00 used to force positive INTEGER.
if len(n_bytes) > 1 and n_bytes[0] == 0x00:
    n_bytes = n_bytes[1:]
if len(e_bytes) > 1 and e_bytes[0] == 0x00:
    e_bytes = e_bytes[1:]

print(b64url_no_pad(n_bytes), b64url_no_pad(e_bytes))
PY
)

tmp="${OUT}.tmp.$$"

jq -n \
    --arg kty "RSA" \
    --arg n "$N_B64URL" \
    --arg e "$E_B64URL" \
    --arg alg "$ALG" \
    --arg use "$USE" \
    --arg kid "$KID" \
    '{keys: [{kty:$kty,n:$n,e:$e,alg:$alg,use:$use,kid:$kid}]}' \
    >"$tmp"

mv "$tmp" "$OUT"

echo "Wrote JWKS to $OUT"
