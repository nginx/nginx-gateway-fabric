# GatewayLink Demo — NGINX Gateway Fabric fronted by F5 BIG-IP

NGINX Gateway Fabric behind F5 BIG-IP via the `ExternalLoadBalancer` CRD. NGF turns
the `ExternalLoadBalancer` into an F5 `IngressLink`, and F5 CIS programs BIG-IP: a
virtual server on the demo VIP, pools whose members are the NGINX data plane pods
(cluster mode), health monitor, iRule, and TLS.

The demo shows:

1. **Static VIP + pool** — traffic through the BIG-IP VIP to coffee/tea over HTTP.
2. **Pod scaling reflected 1:1 on BIG-IP** — scale the data plane, watch pool members
   follow (cluster mode: members are the NGINX pod IPs).
3. **Proxy protocol** — BIG-IP prepends a PROXY header (iRule) and NGINX accepts it
   (`rewriteClientIP: ProxyProtocol`), so NGINX sees the real client IP.
4. **TLS terminated at BIG-IP** — BIG-IP terminates client TLS with a clientssl profile
   and forwards plain HTTP to NGINX.

## Topology

```
client --HTTP:80 --> BIG-IP VS (10.145.34.101:80)  --HTTP--> NGINX pods
client --TLS:443 --> BIG-IP VS (10.145.34.101:443) --HTTP--> NGINX pods
                       (cafe-clientssl terminates TLS)   (pool = pod IPs, cluster mode)
```

The Gateway has two listeners, **both `protocol: HTTP`** (80 and 443). NGINX serves plain
HTTP on both; BIG-IP owns TLS on 443. That is why there is no HTTPS listener / cert Secret
in NGF here.

## Prerequisites on BIG-IP (must already exist)

- Partition `k8s`
- iRule `/Common/Proxy_Protocol_iRule`
- Health monitor `/Common/nginx_health_8081` (HTTP monitor for the NGINX readiness port)
- Client SSL profile `/Common/cafe-clientssl` (create in the BIG-IP UI, upload the cert)

CIS must run with `--custom-resource-mode=true` and `--pool-member-type=cluster`, and the
NGINX data plane Service must be `ClusterIP` (set in `nginxproxy.yaml`).

## Run it

From the demo dir (on the VM):

```shell
./apply.sh                              # deploy app + Gateway + routes + ExternalLoadBalancer
BIGIP_PASS='...' ./attach-clientssl.sh  # attach clientssl to the 443 VS (see note below)
./status.sh                             # Gateway address, ELB Accepted, IngressLink, pods
./curl-demo.sh                          # traffic on HTTP 80 and HTTPS 443
```

### Important: attach the clientssl profile manually

**CIS does not apply the `tls` profiles from the IngressLink to the virtual server** (verified
limitation). NGF/CIS build the 443 virtual server, but the clientssl is not attached, so BIG-IP
will not terminate TLS until you attach it by hand:

```shell
BIGIP_PASS='...' ./attach-clientssl.sh
```

CIS **recreates the virtual server** on any reset / re-apply, which drops the profile, so
**re-run `attach-clientssl.sh` after every apply**.

## Scaling

```shell
./watch-pool.sh          # pane 1: data plane pods
./scale.sh 4             # pane 2: scale the data plane
BIGIP_PASS='...' ./bigip-pool.sh   # BIG-IP now shows 4 pod-IP pool members
./scale.sh 1
```

## Teardown

```shell
./reset.sh   # delete k8s resources; CIS removes the BIG-IP virtual server + pool
```

## Scripts

- `apply.sh` / `reset.sh` — deploy / tear down (CIS removes the BIG-IP objects on teardown)
- `attach-clientssl.sh` — attach the clientssl profile to the 443 VS (run after each apply)
- `status.sh` — one-screen status pipeline
- `wait-ready.sh` — poll until the Gateway has its VIP and the ELB is Accepted
- `curl-demo.sh` — send traffic on HTTP + HTTPS
- `scale.sh` / `watch-pool.sh` — scale the data plane and watch pods
- `bigip-pool.sh` — read virtual servers + pool members directly from BIG-IP
- `preflight.sh` — pre-demo checks (cluster, CRDs, NGF, CIS)
- `env.sh` — shared config (VIP, names, BIG-IP endpoint); sourced by the others

`DEMO-SCRIPT.md` is the talk-through narration for running the demo live.
`setup/` provisions NGF + CIS on a fresh cluster (run once, separate from the demo).
