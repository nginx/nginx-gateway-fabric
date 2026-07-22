# Demo — NGF + F5 BIG-IP (ExternalLoadBalancer)

Run everything on vm-1, in `~/demo`. Deploy and attach the clientssl profile
before the audience is watching, so you open on a ready system.

Note: CIS does not attach the TLS profile from the IngressLink, so
`attach-clientssl.sh` does it. Re-run it after any reset/re-apply — CIS recreates
the virtual server and drops the profile.

## 0. Before they're watching

```shell
cd ~/demo
./apply.sh
BIGIP_PASS=<pass> ./attach-clientssl.sh   # attach clientssl to the 443 VS
./wait-ready.sh                            # until VIP + ELB are green
./curl-demo.sh                             # sanity check: HTTP + HTTPS both 200
```

## 1. Frame it

NGF is a controller: it watches Gateway API resources and provisions the NGINX data plane pods that handle traffic.

Client traffic reaches those pods through F5 BIG-IP out front. The ExternalLoadBalancer CRD tells NGF to configure BIG-IP as that front door — the
virtual server, VIP, health monitor, TLS, and pool — from one Kubernetes resource. This demo walks through that configuration end to end.

One prerequisite: the operator sets up BIG-IP and installs F5 CIS (Container
Ingress Services) in the cluster. CIS is what watches the resources NGF creates
and programs BIG-IP; NGF drives it, but CIS and BIG-IP must already be in place.

## 2. Show the config

```shell
cat externalloadbalancers.yaml
```

One ExternalLoadBalancer points at the Gateway and sets the virtual server address, monitor, PROXY-protocol iRule, and client SSL profile. NGF turns it into an IngressLink; F5 CIS programs BIG-IP from that.

## 3. Show it's live

```shell
./status.sh
```

Gateway has the BIG-IP VIP as its address; the ExternalLoadBalancer is Accepted;
the IngressLink exists (NGF owns it — deleting the Gateway cleans it up).

## 4. Traffic — HTTP and HTTPS

```shell
./curl-demo.sh
```

Both ports return 200 through the BIG-IP VIP. On 443, BIG-IP terminates TLS with
its client SSL profile and forwards plain HTTP to NGINX.

## 5. Client IP preserved (PROXY protocol)

```shell
POD=$(sudo k3s kubectl get pods -n default -l gateway.networking.k8s.io/gateway-name=my-gateway -o jsonpath='{.items[0].metadata.name}')
sudo k3s kubectl logs -n default $POD -c nginx --tail=15
```

The access log shows the request coming from the VM's IP (10.145.47.84), not
BIG-IP's self-IP:

```text
10.145.47.84 - - [22/Jul/2026:05:13:37 +0000] "GET /coffee HTTP/1.1" 200 161 "-" "curl/8.14.1"
10.145.47.84 - - [22/Jul/2026:05:13:37 +0000] "GET /tea HTTP/1.1" 200 155 "-" "curl/8.14.1"
```

That real client IP is preserved by PROXY protocol — the iRule on the virtual
server sends it, and rewriteClientIP on the NginxProxy makes NGINX trust and log
it. Without it, every request would show BIG-IP's self-IP instead.

## 6. Scale — BIG-IP pool follows

Pane 1:

```shell
./watch-pool.sh
```

Pane 2:

```shell
./scale.sh 4
```


Wait for 20 seconds

```shell
BIGIP_PASS=<pass> ./bigip-pool.sh 
./scale.sh 1
```

Cluster mode: pool members are the NGINX pod IPs, so the pool tracks pods 1:1 as you scale.

## 7. Close

One Kubernetes resource gave us a BIG-IP virtual server + VIP, health monitor, TLS
at BIG-IP, client-IP preservation, and pool membership that tracks pods as we scale.

## Notes

- Nodeport mode is also supported for Big-IP. 
  
- VIP/status blank: wait a few seconds (`./wait-ready.sh`); don't re-apply mid-demo.
  
- HTTPS error `curl: (35) TLS connect error: error:0A00010B:SSL routines::wrong
  version number`: this means the clientssl profile is NOT on the 443 virtual
  server, so BIG-IP is passing plain TCP instead of terminating TLS. It happens
  after any reset/apply — CIS recreates the virtual server and drops the profile.
  Fix: re-run `BIGIP_PASS=<pass> ./attach-clientssl.sh`. Note the 443 VS takes
  ~60-90s to appear after apply; if attach 404s, wait and re-run.

- Reset: `./reset.sh`, then `./apply.sh`, wait ~60-90s, then `./attach-clientssl.sh`.



## Extra Notes:

- To build fresh NGF images and SCP to the VM run


```shell
make build-images TAG=$(whoami) GOARCH=amd64
docker save -o ngf.tar   nginx-gateway-fabric:sa.choudhary
docker save -o nginx.tar nginx-gateway-fabric/nginx:sa.choudhary
```