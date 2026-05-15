# UpstreamSettingsPolicy

This directory contains the YAML files used in the [UpstreamSettingsPolicy](https://docs.nginx.com/nginx-gateway-fabric/traffic-management/upstream-settings/) guide.

## UseClusterIP

By default, NGF routes traffic to individual Pod IPs resolved from EndpointSlices. Setting
`useClusterIP: true` on an `UpstreamSettingsPolicy` instructs NGF to route to the Service's
ClusterIP instead.

This is useful when:
- **Service mesh compatibility**: Sidecars (e.g. Linkerd, Istio) intercept traffic at the
  Service IP level and require routing through the ClusterIP to apply mesh policies correctly.
- **Headless service exclusion**: This field has no effect on headless Services (ClusterIP:
  None) or ExternalName Services — NGF will continue resolving endpoints normally for those.
- **L4/stream upstreams**: This field is not supported for TCP/UDP (L4) routes.

### Example

Apply the example policy from this directory:

```shell
kubectl apply -f use-cluster-ip.yaml
```

This creates an `UpstreamSettingsPolicy` that routes all traffic destined for the `coffee`
Service through its ClusterIP:

```yaml
apiVersion: gateway.nginx.org/v1alpha1
kind: UpstreamSettingsPolicy
metadata:
  name: use-cluster-ip
spec:
  targetRefs:
    - group: core
      kind: Service
      name: coffee
  useClusterIP: true
```

When accepted, the NGINX upstream for the `coffee` Service will contain a single
`server <clusterIP>:<port>` entry instead of individual Pod IP entries.

