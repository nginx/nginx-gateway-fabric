#!/usr/bin/env bash
# Attach the BIG-IP clientssl profile to the CIS-built 443 virtual server.
#
# Why this exists: CIS does NOT apply the IngressLink tls profiles to the virtual
# server (a CIS/IngressLink limitation, verified live — see externalloadbalancers.yaml).
# So BIG-IP terminates TLS only after we attach the clientssl profile to the VS by
# hand via the BIG-IP REST API. CIS recreates the VS on any reset / re-apply / churn,
# which drops the profile, so RE-RUN this after every apply.
#
# Prereqs: the clientssl profile ($CLIENTSSL_PROFILE) already exists on BIG-IP
# (create it in the UI and upload the cert), and BIGIP_PASS is set in the env.
#   BIGIP_PASS='...' ./attach-clientssl.sh
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

: "${BIGIP_PASS:?set BIGIP_PASS in your shell first}"
CLIENTSSL_PROFILE="${CLIENTSSL_PROFILE:-/Common/cafe-clientssl}"

# The CIS-built 443 VS name is derived from the virtual server address.
VIP="$(get_vip)"
[ -n "$VIP" ] || { echo "Gateway has no address yet — is the ELB Accepted? Run ./status.sh"; exit 1; }
VS="ingress_link_crd_$(echo "$VIP" | tr '.' '_')_443"
VS_PATH="~${BIGIP_PARTITION}~Shared~${VS}"
B="https://${BIGIP}/mgmt/tm/ltm/virtual/${VS_PATH}/profiles"
AUTH=(-sk -u "${BIGIP_USER}:${BIGIP_PASS}")

echo "Attaching ${CLIENTSSL_PROFILE} (clientside) to ${VS} ..."
resp=$(curl "${AUTH[@]}" -X POST -H "Content-Type: application/json" \
  -d "{\"name\": \"${CLIENTSSL_PROFILE}\", \"context\": \"clientside\"}" "$B")

if echo "$resp" | grep -q '"context":"clientside"'; then
  echo "OK — clientssl attached. Test: curl -k https://cafe.example.com/coffee --resolve cafe.example.com:443:${VIP}"
elif echo "$resp" | grep -qi "already exists\|0x01020066"; then
  echo "Already attached (nothing to do)."
else
  echo "Response: $resp"
fi
