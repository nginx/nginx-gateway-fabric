#!/usr/bin/env bash
# Send traffic through the BIG-IP virtual server, on both HTTP (80) and HTTPS (443).
# Prints each curl command before running it, then shows the status line + the
# X-Nginx-Pod header (which NGINX pod served it — run repeatedly after scaling to
# watch it rotate across pods).
#
# HTTPS is terminated by BIG-IP (cafe-clientssl). If 443 fails but 80 works, the
# clientssl profile likely isn't attached to the VS — run ./attach-clientssl.sh.
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

VIP="$(get_vip)"
[ -n "$VIP" ] || { echo "Gateway has no address yet — is the ELB Accepted? Run ./status.sh"; exit 1; }
echo "VIP (from Gateway status) = $VIP"
echo

# Print the clean command, then run it showing the full response (headers + body).
hit() {
  local scheme="$1" port="$2" path="$3" k=""
  [ "$scheme" = https ] && k="-k"
  echo "\$ curl $k $scheme://cafe.example.com$path --resolve cafe.example.com:$port:$VIP"
  curl $k -sS -i --max-time 8 \
    --resolve "cafe.example.com:$port:$VIP" "$scheme://cafe.example.com$path" \
    || echo "  (no response)"
  echo
}

for path in /coffee /tea; do
  hit http  80  "$path"
  hit https 443 "$path"
done
