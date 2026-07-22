#!/usr/bin/env bash
# Step 6: Verify the setup on vm-1 is healthy before the demo.
source "$(dirname "$0")/lib.sh"

banner "NGF on vm-1"
kc "$VM1_IP" "get pods -n $NGF_NS --no-headers | grep -v Running >/dev/null 2>&1" \
  && warn "some NGF pods not Running" || ok "NGF Running"
kc "$VM1_IP" "get crd externalloadbalancers.gateway.nginx.org >/dev/null 2>&1" \
  && ok "ExternalLoadBalancer CRD present" || warn "ExternalLoadBalancer CRD missing"
kc "$VM1_IP" "get crd ingresslinks.cis.f5.com >/dev/null 2>&1" \
  && ok "IngressLink CRD present" || warn "IngressLink CRD missing"

banner "CIS on vm-1"
kc "$VM1_IP" "get pods -n $CIS_NS -l app=f5-bigip-ctlr --no-headers | grep -q '1/1.*Running'" \
  && ok "CIS 1/1 Running" || warn "CIS not healthy"

banner "CIS flags (cluster mode + custom resources)"
kc "$VM1_IP" "get deploy f5-cis-f5-bigip-ctlr -n $CIS_NS -o jsonpath='{.spec.template.spec.containers[0].args}' | tr ',' '\n' | grep -iE 'pool-member-type|custom-resource-mode|bigip-partition'"

banner "No CIS panic in recent logs"
kc "$VM1_IP" "logs deploy/f5-cis-f5-bigip-ctlr -n $CIS_NS --tail=200 2>&1 | grep -iE 'panic|nil pointer' >/dev/null" \
  && warn "CIS log shows a panic — inspect logs" || ok "no panic in recent CIS logs"

banner "No stale demo resources"
kc "$VM1_IP" "get ingresslink,gateway,externalloadbalancer -A 2>/dev/null | grep -v 'No resources' | head"

echo
echo "If all OK: deploy the demo with ./run-demo.sh (or on vm-1: cd ~/demo && ./apply.sh)."
