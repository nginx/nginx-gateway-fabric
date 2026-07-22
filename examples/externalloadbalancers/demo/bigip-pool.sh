#!/usr/bin/env bash
# Read-only view of what BIG-IP ACTUALLY has, straight from its iControl REST API
# This is ground truth for "did CIS program BIG-IP".
# Three GET calls, each returns JSON that an inline python3 parses:
#   GET /mgmt/tm/ltm/virtual                      -> virtual servers
#   GET /mgmt/tm/ltm/pool                         -> pools
#   GET /mgmt/tm/ltm/pool/<name>/members/stats    -> per-member health
#
# CIS creates these objects under the Shared folder of the partition, so their
# fullPath is /<partition>/Shared/<name> (e.g. /k8s/Shared/ingress_link_crd_...).
# We select on the fullPath prefix /<partition>/ rather than the 'partition'
# field: fullPath is stable, whereas how partition/subPath split the path can
# vary, and a partition-field filter is what makes the GUI list look empty.
# In the members URL, '/' is encoded as '~' (BIG-IP REST path convention).
#
# Requires VPN reachability to BIG-IP and BIGIP_PASS in the environment:
#   BIGIP_PASS='...' ./bigip-pool.sh
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

: "${BIGIP_PASS:?set BIGIP_PASS in your shell first}"
B="https://${BIGIP}/mgmt/tm/ltm"
AUTH=(-sk -u "${BIGIP_USER}:${BIGIP_PASS}")
PART="/${BIGIP_PARTITION}/"

echo "== Virtual servers under ${PART} (from BIG-IP) =="
curl "${AUTH[@]}" "${B}/virtual" | python3 -c "
import sys,json
part='${PART}'
for v in json.load(sys.stdin).get('items',[]):
    if (v.get('fullPath') or '').startswith(part):
        print('  ', v['fullPath'], '->', v.get('destination'), '| pool=', v.get('pool'), '| rules=', v.get('rules'))
"

echo
echo "== Pools + members under ${PART} (from BIG-IP) =="
curl "${AUTH[@]}" "${B}/pool" | python3 -c "
import sys,json
part='${PART}'
print('\n'.join(p['fullPath'] for p in json.load(sys.stdin).get('items',[]) if (p.get('fullPath') or '').startswith(part)))
" | while read -r pool; do
  [ -z "$pool" ] && continue
  enc=$(echo "$pool" | sed 's#/#~#g')
  echo "-- $pool --"
  curl "${AUTH[@]}" "${B}/pool/${enc}/members/stats" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  entries=d.get('entries',{})
  if not entries: print('     (no members)')
  for k,v in entries.items():
    st=v['nestedStats']['entries']
    addr=st.get('addr',{}).get('description','?')
    avail=st.get('status.availabilityState',{}).get('description','?')
    reason=st.get('status.statusReason',{}).get('description','')
    if ':' not in addr:   # skip IPv6 noise
      print(f'     {addr}: {avail} ({reason})')
except Exception as e: print('     (error reading members:', e, ')')
"
done
