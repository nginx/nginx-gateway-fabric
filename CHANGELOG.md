# Changelog

This document includes a curated changelog for each release. We also publish a changelog as the description of
a [GitHub release](https://github.com/nginx/nginx-gateway-fabric/releases), which, by contrast, is auto-generated
and includes links to all PRs that went into the release.

## Release 2.2.1

_November 12, 2025_

FEATURES:

- Add support to make endpoint picker communication settings configurable. [4105](https://github.com/nginx/nginx-gateway-fabric/pull/4105)

BUG FIXES:

- Support mixed externalname and local services in single route. [4188](https://github.com/nginx/nginx-gateway-fabric/pull/4188)
- Preserve HPA replicas on deployment. [4214](https://github.com/nginx/nginx-gateway-fabric/pull/4214)
- Preserve external controller annotations. [4182](https://github.com/nginx/nginx-gateway-fabric/pull/4182)
- Fix bug where user mounted files were being removed by NGINX. [4178](https://github.com/nginx/nginx-gateway-fabric/pull/4178)
- Fix bug that prevented HPA from working when Inference Extension was enabled. [4247](https://github.com/nginx/nginx-gateway-fabric/pull/4247)

HELM CHART:

- Allow disabling cert-generator with a Helm value. [4186](https://github.com/nginx/nginx-gateway-fabric/pull/4186). Thanks to [fabian4](https://github.com/fabian4)
- The version of the Helm chart is now 2.2.1


KNOWN ISSUES:

- Data plane sometimes does not sync upstream server IPs. [3626](https://github.com/nginx/nginx-gateway-fabric/issues/3626)

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.2`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.5.0`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.2.1`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.2.1`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.2.1`

## Release 2.2.0

_October 22, 2025_

FEATURES:

- Add first class OpenShift support, including UBI based images and an Operator. [4001](https://github.com/nginx/nginx-gateway-fabric/pull/4001)
- Add regex for path matching. [3874](https://github.com/nginx/nginx-gateway-fabric/pull/3874). Thanks to [fabian4](https://github.com/fabian4)
- Add support for Inference Extension. [4091](https://github.com/nginx/nginx-gateway-fabric/pull/4091)
- Support ExternalName Services. [3759](https://github.com/nginx/nginx-gateway-fabric/pull/3759)
- Add support for Gateway addresses field. [3896](https://github.com/nginx/nginx-gateway-fabric/pull/3896)
- Add support for Port in ParentReference. [3778](https://github.com/nginx/nginx-gateway-fabric/pull/3778)

BUG FIXES:

- Handle duplicate kinds in listener allowed kinds. [3810](https://github.com/nginx/nginx-gateway-fabric/pull/3810)
- Align with BackendTLSPolicy validation. [3871](https://github.com/nginx/nginx-gateway-fabric/pull/3871)

HELM CHART:

- The version of the Helm chart is now 2.2.0
- Set NGINX Plus default image. [3919](https://github.com/nginx/nginx-gateway-fabric/pull/3919)

KNOWN ISSUES:

- Data plane sometimes does not sync upstream server IPs. [3626](https://github.com/nginx/nginx-gateway-fabric/issues/3626)

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.2`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.3.2`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.2.0`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.2.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.2.0`
- Operator: `ghcr.io/nginx/nginx-gateway-fabric/operator:1.0.0`

## Release 2.1.4

_October 1, 2025_

BUG FIXES:

- Update Alpine packages libcrpyto3 and libssl3 to fix cves. [3993](https://github.com/nginx/nginx-gateway-fabric/pull/3993)

HELM CHART:

- The version of the Helm chart is now 2.1.4

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.1`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.3.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.1.4`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.1.4`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.1.4`

## Release 2.1.3

_October 1, 2025_

BUG FIXES:

- Update Alpine packages libexpat and tiff to fix cves. [3973](https://github.com/nginx/nginx-gateway-fabric/pull/3973)

HELM CHART:

- The version of the Helm chart is now 2.1.3

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.1`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.3.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.1.3`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.1.3`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.1.3`

## Release 2.1.2

_September 25, 2025_

BUG FIXES:

- Fixes a bug where the subscribe method was incorrectly intercepting initial configuration operations after a ServiceAccountToken rotation and signaling broadcast completion. [3905](https://github.com/nginx/nginx-gateway-fabric/pull/3905)
- Fixes an issue where a failed configuration reload caused all HTTPRoutes to be marked as invalid (Accepted: false). This led to external-dns removing DNS records even though the configuration had been rolled back. Routes now retain their valid state during reload failures, preventing unnecessary DNS disruptions. [3936](https://github.com/nginx/nginx-gateway-fabric/pull/3936)
- Adds NGINX image version validation during agent connections to prevent newer config being sent to pods running previous image versions during upgrades. [3928](https://github.com/nginx/nginx-gateway-fabric/pull/3928)

NGINX AGENT BUG FIXES:

- The NGINX Agent v3.3.1 update fixes connection reset issues during config apply by reusing existing connections, adds rollback on failures, and improves logging for more reliable configuration updates. [1261](https://github.com/nginx/agent/pull/1261)

HELM CHART:

- The version of the Helm chart is now 2.1.2

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.1`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.3.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.1.2`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.1.2`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.1.2`

## Release 2.1.1

_September 3, 2025_

BUG FIXES:

- Fix remove patch label/ annotation cross contamination. [3754](https://github.com/nginx/nginx-gateway-fabric/pull/3754)
- Fix host readiness probe on ipv6 in addition to ipv4. [3765](https://github.com/nginx/nginx-gateway-fabric/pull/3765). Thanks to [lucasl0st](https://github.com/lucasl0st).
- Prevent policy `includes` duplication in advanced routing configuration. [3799](https://github.com/nginx/nginx-gateway-fabric/pull/3799)
- Adjust nginx agent backoff settings and revert request timeout. [3820](https://github.com/nginx/nginx-gateway-fabric/pull/3820)

HELM CHART:

- The version of the Helm chart is now 2.1.1
- Add patches to helm chart spec. [3773](https://github.com/nginx/nginx-gateway-fabric/pull/3773)

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.1`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.2.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.1.1`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.1.1`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.1.1`

## Release 2.1.0

_August 14, 2025_

FEATURES:

- Add policyAffected status for policy target refs. [3535](https://github.com/nginx/nginx-gateway-fabric/pull/3535)
- Add support for appProtocol on BackendRefs which point to a Kubernetes Service with a specified appProtocol on the target Service Port. [3511](https://github.com/nginx/nginx-gateway-fabric/pull/3511)
- Support configurable hostPorts in NGINX container. [3321](https://github.com/nginx/nginx-gateway-fabric/pull/3321)
- Add support for configuring NGINX `worker_connections` directive. [3611](https://github.com/nginx/nginx-gateway-fabric/pull/3611)
- Add support for percentage based request mirroring. [3627](https://github.com/nginx/nginx-gateway-fabric/pull/3627)
- NGINX Pod now has a configurable readiness probe. [3629](https://github.com/nginx/nginx-gateway-fabric/pull/3629)
- Add the ability to patch the dataplane Service, Deployment, and DaemonSet resources through NginxProxy. [3630](https://github.com/nginx/nginx-gateway-fabric/pull/3630)
- Add disableSNIHostValidation field to NginxProxy CRD to resolve HTTP/2 connection reuse issues with wildcard certificates, with documented security trade-offs. [3659](https://github.com/nginx/nginx-gateway-fabric/pull/3659)
- Enable connection to NGINX One Console. [3676](https://github.com/nginx/nginx-gateway-fabric/pull/3676)
- Add HorizontalPodAutoscaling support for both control plane and data plane deployments. [3702](https://github.com/nginx/nginx-gateway-fabric/pull/3702). Thanks to [nowjean](https://github.com/nowjean).
- Set the OverlappingTLSConfig condition on Listeners with overlapping TLS hostnames. [3709](https://github.com/nginx/nginx-gateway-fabric/pull/3709)

BUG FIXES:

- Fix an issue where the NGINX Pod couldn't connect to control plane if multiple Pods shared the NGINX Pod's IP address. [3673](https://github.com/nginx/nginx-gateway-fabric/pull/3673)

DOCUMENTATION:

- Add simplified architecture diagrams for traffic flow and config changes. [3557](https://github.com/nginx/nginx-gateway-fabric/pull/3557)

HELM CHART:

- The version of the Helm chart is now 2.1.0.
- Allow users to specify nginxGateway.name to configure names of deployments. [3528](https://github.com/nginx/nginx-gateway-fabric/pull/3528)
- Fix Helm Schema for data plane volume mounts. [3588](https://github.com/nginx/nginx-gateway-fabric/pull/3588). Thanks to [vazkarvishal](https://github.com/vazkarvishal).
- Update Helm Chart README to recommend using server-side apply when applying NGF CRDs. [3589](https://github.com/nginx/nginx-gateway-fabric/pull/3589)

UPGRADE:

- **Important:** Upgrading from v2.0.x to 2.1 requires the NGINX Gateway Fabric control plane to be uninstalled and then reinstalled to avoid any downtime to user traffic. CRDs do not need to be removed. The NGINX data plane deployment is not affected by this process, and traffic should still flow uninterrupted. For more information on how to do so, view our guide on [Upgrading NGINX Gateway Fabric](https://docs.nginx.com/nginx-gateway-fabric/install/upgrade-version/)

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.29.0`
- NGINX Plus version: `R35`
- NGINX Agent version: `v3.2.0`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.1.0`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.1.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.1.0`

## Release 2.0.2

_July 8, 2025_

Bug Fixes:

- Fix crash when BackendRef filter is specified. [3508](https://github.com/nginx/nginx-gateway-fabric/pull/3508)
- Fix index out of bounds error when building status. [3513](https://github.com/nginx/nginx-gateway-fabric/pull/3513)
- Disable automountServiceAccountToken on ServiceAccount, and instead enable on the Pods directly. [3573](https://github.com/nginx/nginx-gateway-fabric/pull/3573)
- Fix port binding with reduced privileges. [3574](https://github.com/nginx/nginx-gateway-fabric/pull/3574)
- Fixed an issue where SnippetsFilters were being included in NGINX configuration for all gateways regardless of whether routes attached to those specific gateways actually referenced the filters. [3586](https://github.com/nginx/nginx-gateway-fabric/pull/3586)

HELM CHART:

- The version of the Helm chart is now 2.0.2.
- Removes Extra keyword from volumeMounts and Volumes for the data plane. [3588](https://github.com/nginx/nginx-gateway-fabric/pull/3588). Thanks to [vazkarvishal](https://github.com/vazkarvishal).

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.28.0`
- NGINX Plus version: `R34`
- NGINX Agent version: `v3.0.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.0.2`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.0.2`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.0.2`

## Release 2.0.1

_June 11, 2025_

Bug Fixes:

- Set proper IP family policy on NGINX LB Service. [3475](https://github.com/nginx/nginx-gateway-fabric/pull/3475).

HELM CHART:

- The version of the Helm chart is now 2.0.1.
- Add support to configure `cert-generator` job pod placement. [3493](https://github.com/nginx/nginx-gateway-fabric/pull/3493). Thanks to [Baburciu](https://github.com/baburciu).
- Add support for configuring the `ttlSecondsAfterFinished` field for the cert-generator job. The default is set to 30s. [3487](https://github.com/nginx/nginx-gateway-fabric/pull/3487). Thanks to [Ab-andresc](https://github.com/ab-andresc).
- Adds support for all additional labels for the control plane service. [3499](https://github.com/nginx/nginx-gateway-fabric/pull/3499). Thanks to [MichasHL](https://github.com/michasHL).

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.28.0`
- NGINX Plus version: `R34`
- NGINX Agent version: `v3.0.1`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.0.1`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.0.1`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.0.1`

## Release 2.0.0

_June 5, 2025_

BREAKING CHANGES:

[How to upgrade to 2.0.0](https://docs.nginx.com/nginx-gateway-fabric/install/upgrade-version/#upgrade-from-v1x-to-v2x).

The following changes are breaking and require users to fully uninstall NGINX Gateway Fabric (including NGINX Gateway Fabric CRDs) before re-installing the new version. Gateway API resources (such as Gateway, HTTPRoute, etc) are unaffected and can be left alone. [3318](https://github.com/nginx/nginx-gateway-fabric/pull/3318)

- Control plane and data plane have been separated into different Deployments. The control plane will provision an NGINX data plane Deployment and Service when a Gateway object is created.
- NginxProxy CRD resource is now namespace-scoped (was cluster-scoped).
- NginxProxy resource controls infrastructure fields for the NGINX Deployment and Service, such as replicas, loadBalancerIP, serviceType, etc. Users who want to set or update these fields must do so either at installation time through the helm chart (which sets them globally), or per Gateway. Updating these fields directly on a provisioned nginx Deployment or Service will not take effect. This does not apply to the the NGINX Gateway Fabric control plane Deployment.
- Helm values structure has changed slightly to better support the separate Deployments.
- `nginxGateway.replicaCount` Helm value has been renamed to `nginxGateway.replicas`.

FEATURES:

- Support for creating and deploying multiple Gateways. [3318](https://github.com/nginx/nginx-gateway-fabric/pull/3318)
- NginxProxy resource can now additionally be attached to a Gateway, and will overwrite any settings that are attached at the GatewayClass level, for the Gateway that it's attached to. [3318](https://github.com/nginx/nginx-gateway-fabric/pull/3318)
- Listener isolation supported for all routes. [3067](https://github.com/nginx/nginx-gateway-fabric/pull/3067)
- Allow configuration of NGINX Plus API access. [3066](https://github.com/nginx/nginx-gateway-fabric/pull/3066)
- Adds regex matching for headers and query params for HTTPRoutes and headers for GRPCRoutes. [3093](https://github.com/nginx/nginx-gateway-fabric/pull/3093)
- Add support for request mirroring using the RequestMirror filter. [3066](https://github.com/nginx/nginx-gateway-fabric/pull/3306)
- Adds support for Secrets to be used in BackendTLSPolicy for TLS certificates and CA certificates. [3084](https://github.com/nginx/nginx-gateway-fabric/pull/3084). Thanks to [porthorian](https://github.com/porthorian).

BUG FIXES:

- Fix an issue where default headers were still being set when overwritten by a user. [3249](https://github.com/nginx/nginx-gateway-fabric/pull/3249)
- Add 503 status code when there are zero upstream endpoints. [3406](https://github.com/nginx/nginx-gateway-fabric/pull/3406)
- Fixed bug that occurred when a route's ParentRef does not include a sectionName and the Gateway's listeners have duplicate hostnames. This would cause conflicts when the route tries to attach to all the listeners and falsely trigger validation checks around overlapping routes. [3418](https://github.com/nginx/nginx-gateway-fabric/pull/3418)

DOCUMENTATION:

- Migrated the documentation website into the [NGINX documentation repository](https://github.com/nginx/documentation). [3047](https://github.com/nginx/nginx-gateway-fabric/pull/3047)

HELM CHART:

- The version of the Helm chart is now 2.0.0
- Helm values structure has changed slightly to better support the separate Deployments.
- `nginxGateway.replicaCount` Helm value has been renamed to `nginxGateway.replicas`.
- Add support for control plane Deployment labels. [3194](https://github.com/nginx/nginx-gateway-fabric/pull/3194). Thanks to [Butterneck](https://github.com/Butterneck).

UPGRADE:

- [Upgrade to 2.0.0](https://docs.nginx.com/nginx-gateway-fabric/install/upgrade-version/#upgrade-from-v1x-to-v2x)

KNOWN ISSUES:

- NGINX LoadBalancer Service IPFamily uses Kubernetes default (likely SingleStack IPv4) instead of matching the ipFamily set in the NginxProxy resource, which defaults to DualStack. [3473](https://github.com/nginx/nginx-gateway-fabric/issues/3473)

DEPENDENCIES:

- NGINX Plus was updated to R34. [3281](https://github.com/nginx/nginx-gateway-fabric/pull/3281)
- Update to v1.3.0 of the Gateway API. [3348](https://github.com/nginx/nginx-gateway-fabric/pull/3348)

COMPATIBILITY:

- Gateway API version: `1.3.0`
- NGINX version: `1.28.0`
- NGINX Plus version: `R34`
- NGINX Agent version: `v3.0.0`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:2.0.0`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:2.0.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:2.0.0`

## Release 1.6.2

_March 11, 2025_

HELM CHART:

- The version of the Helm chart is now 1.6.2

UPGRADE:

- Update golang.org/x/oauth2. [3185](https://github.com/nginx/nginx-gateway-fabric/pull/3185)

COMPATIBILITY:

- Gateway API version: `1.2.1`
- NGINX version: `1.27.4`
- NGINX Plus version: `R33`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:1.6.2`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:1.6.2`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.6.2`

## Release 1.6.1

_February 6, 2025_

HELM CHART:

- The version of the Helm chart is now 1.6.1

UPGRADE:

- Update nginx to version 1.27.4. [3102](https://github.com/nginx/nginx-gateway-fabric/pull/3102)

COMPATIBILITY:

- Gateway API version: `1.2.1`
- NGINX version: `1.27.4`
- NGINX Plus version: `R33`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:1.6.1`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:1.6.1`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.6.1`

## Release 1.6.0

_January 15, 2025_

FEATURES:

- Add UpstreamSettingsPolicy to allow users to configure upstream settings for Services. [2941](https://github.com/nginx/nginx-gateway-fabric/pull/2941)
- Add path support for RequestRedirect Filter. [2979](https://github.com/nginx/nginx-gateway-fabric/pull/2979)
- Use state files for NGINX Plus upstream servers instead of the NGINX config. [2897](https://github.com/nginx/nginx-gateway-fabric/pull/2897)

BUG FIXES:

- Fix rewrite path for ReplacePrefixMatch to parse request arguments correctly. [2951](https://github.com/nginx/nginx-gateway-fabric/pull/2951)
- Fix an issue where updating upstreams with the NGINX Plus API would not occur if metrics were disabled. [2897](https://github.com/nginx/nginx-gateway-fabric/pull/2897)
- Support updating stream upstreams with the NGINX Plus API instead of reloading NGINX. [2897](https://github.com/nginx/nginx-gateway-fabric/pull/2897)

DOCUMENTATION:

- Docs: Update support referencing support package tool. [2789](https://github.com/nginx/nginx-gateway-fabric/pull/2789). Thanks to [mrajagopal](https://github.com/mrajagopal).
- Add how-to guide for configuring upstream settings for services using the UpstreamSettingsPolicy API. Find it [here](https://docs.nginx.com/nginx-gateway-fabric/how-to/traffic-management/upstream-settings/). [2987](https://github.com/nginx/nginx-gateway-fabric/pull/2987)

HELM CHART:

- The version of the Helm chart is now 1.6.0

UPGRADE:

- The Gateway API version has been updated to 1.2.1. [2868](https://github.com/nginx/nginx-gateway-fabric/pull/2868)
- ObservabilityPolicy API version has been increased to `v1alpha2` due to a strengthening of validation rules. [2998](https://github.com/nginx/nginx-gateway-fabric/pull/2998)

COMPATIBILITY:

- Gateway API version: `1.2.1`
- NGINX version: `1.27.3`
- NGINX Plus version: `R33`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:1.6.0`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:1.6.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.6.0`

## Release 1.5.1

_December 16, 2024_

BUG FIXES:

- Write NGINX Plus R33 deployment context in init container. [2905](https://github.com/nginx/nginx-gateway-fabric/pull/2905)
- SnippetsFilter CRD missing from CRDs manifest. [2822](https://github.com/nginx/nginx-gateway-fabric/pull/2822)
- Omit empty deployment context fields in NGINX Plus R33 reporting. [2910](https://github.com/nginx/nginx-gateway-fabric/pull/2910)

HELM CHART:

- The version of the Helm chart is now 1.5.1

COMPATIBILITY:

- Gateway API version: `1.2.0`
- NGINX version: `1.27.2`
- NGINX Plus version: `R33`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginx/nginx-gateway-fabric:1.5.1`
- Data plane: `ghcr.io/nginx/nginx-gateway-fabric/nginx:1.5.1`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.5.1`

## Release 1.5.0

_November 20, 2024_

BREAKING CHANGES:

- NGINX Plus R33 support added. The NGINX Plus release now requires a valid JSON Web Token (JWT) in order to run. Users of NGINX Plus _must_ have this JWT added to a Secret before installing NGINX Gateway Fabric v1.5.0. See the [NGINX Plus JWT](https://docs.nginx.com/nginx-gateway-fabric/installation/nginx-plus-jwt/) guide for information on setting this up. [2760](https://github.com/nginx/nginx-gateway-fabric/pull/2760)

FEATURES:

- Add support to retain client IP information. [2284](https://github.com/nginx/nginx-gateway-fabric/pull/2284)
- Add the ability to configure data plane error log level. [2603](https://github.com/nginx/nginx-gateway-fabric/pull/2603)
- Introduce SnippetsFilter API, which allows users to inject custom NGINX configuration via an HTTPRoute or GRPCRoute filter. See the [SnippetsFilter guide](https://docs.nginx.com/nginx-gateway-fabric/how-to/traffic-management/snippets/) for information on how to use SnippetsFilters. [2604](https://github.com/nginx/nginx-gateway-fabric/pull/2604)
- Reduce logging verbosity of default Info log level. [2455](https://github.com/nginx/nginx-gateway-fabric/pull/2455)

BUG FIXES:

- Stream status_zone directive is no longer set if its value is empty. [2684](https://github.com/nginx/nginx-gateway-fabric/pull/2684)
- Fix an issue with upstream names when split clients are used with a namespace name that starts with a number. [2730](https://github.com/nginx/nginx-gateway-fabric/pull/2730)
- A 503 http response code is now returned to the client when a service has no ready endpoints. [2696](https://github.com/nginx/nginx-gateway-fabric/pull/2696)

DOCUMENTATION:

- Add a [guide](https://docs.nginx.com/nginx-gateway-fabric/how-to/traffic-management/snippets) for SnippetsFilter. [2721](https://github.com/nginx/nginx-gateway-fabric/pull/2721)
- Add a new [Get started](https://docs.nginx.com/nginx-gateway-fabric/get-started/) document. [2721](https://github.com/nginx/nginx-gateway-fabric/pull/2717)
- Add documentation for [proxyProtocol and rewriteClientIP](https://docs.nginx.com/nginx-gateway-fabric/how-to/data-plane-configuration/#configure-proxy-protocol-and-rewriteclientip-settings) settings. [2701](https://github.com/nginx/nginx-gateway-fabric/pull/2701)
- Fix indentation in lifecycle examples. [2588](https://github.com/nginx/nginx-gateway-fabric/pull/2588). Thanks to [Derek F](https://github.com/defrank).

HELM CHART:

- The version of the Helm chart is now 1.5.0
- Add `loadBalancerSourceRanges` to helm parameters to use during install/upgrade. [2773](https://github.com/nginx/nginx-gateway-fabric/pull/2773)
- Add `loadBalancerIP` as a helm parameter to use during install/upgrade. [2766](https://github.com/nginx/nginx-gateway-fabric/pull/2766)
- Add Helm schema. [2492](https://github.com/nginx/nginx-gateway-fabric/pull/2492)
- Add capability to configure `topologySpreadConstraints`. [2703](https://github.com/nginx/nginx-gateway-fabric/pull/2703). Thanks to [Robsta86](https://github.com/Robsta86)

DEPENDENCIES:

- NGINX Plus was updated to R33. [2760](https://github.com/nginx/nginx-gateway-fabric/pull/2760)
- Update to v1.2.0 of the Gateway API. [2694](https://github.com/nginx/nginx-gateway-fabric/pull/2694)

COMPATIBILITY:

- Gateway API version: `1.2.0`
- NGINX version: `1.27.2`
- NGINX Plus version: `R33`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.5.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.5.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.5.0`

## Release 1.4.0

_August 20, 2024_

FEATURES:

- Support IPv6. [2190](https://github.com/nginx/nginx-gateway-fabric/pull/2190)
- Add support for TLS Passthrough using TLSRoutes. [2356](https://github.com/nginx/nginx-gateway-fabric/pull/2356)
- Support cross-namespace routing with TLSRoutes. [2379](https://github.com/nginx/nginx-gateway-fabric/pull/2379)
- Added server_zone metrics for NGINX Plus users. [2360](https://github.com/nginx/nginx-gateway-fabric/pull/2360)

BUG FIXES:

- Fixed issue where NGF Pod cannot recover if NGINX master process fails without cleaning up. [2131](https://github.com/nginx/nginx-gateway-fabric/pull/2131)
- Leader election couldn't be disabled. [2307](https://github.com/nginx/nginx-gateway-fabric/pull/2307)
- Disallow routes from attaching to listeners if not present in allowed routes. [2314](https://github.com/nginx/nginx-gateway-fabric/pull/2314)
- Honor ReferenceGrants that allow GRPCRoutes to reference Services in different namespaces. [2337](https://github.com/nginx/nginx-gateway-fabric/pull/2337)
- Fixed an issue that prevented ClientSettingsPolicies and ObservabilityPolicies from working when attached to an HTTPRoute where matching conditions were defined. [2318](https://github.com/nginx/nginx-gateway-fabric/pull/2318)
- Replace TODO route condition with an Accepted/False condition. [2228](https://github.com/nginx/nginx-gateway-fabric/pull/2228)

DOCUMENTATION:

- Enhanced the troubleshooting guide with more details and scenarios. [2141](https://github.com/nginx/nginx-gateway-fabric/pull/2141)
- Update kubectl exec syntax to remove deprecation warning. [2218](https://github.com/nginx/nginx-gateway-fabric/pull/2218). Thanks [aknot242](https://github.com/aknot242).
- Add info on setting up host network access. [2263](https://github.com/nginx/nginx-gateway-fabric/pull/2263). Thanks [fardarter](https://github.com/fardarter).

HELM CHART:

- The version of the Helm chart is now 1.4.0
- Add capability to set resource requests and limits on nginx-gateway deployment. [2216](https://github.com/nginx/nginx-gateway-fabric/pull/2216). Thanks to [anwbtom](https://github.com/anwbtom).
- Add capability to configure custom annotations for the nginx-gateway-fabric pod(s). [2250](https://github.com/nginx/nginx-gateway-fabric/pull/2250). Thanks to [Robsta86](https://github.com/Robsta86).
- Add helm chart examples. [2292](https://github.com/nginx/nginx-gateway-fabric/pull/2292)
- Add seccompProfile. [2323](https://github.com/nginx/nginx-gateway-fabric/pull/2323)

COMPATIBILITY:

- Gateway API version: `1.1.0`
- NGINX version: `1.27.1`
- NGINX Plus version: `R32`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.4.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.4.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.4.0`

## Release 1.3.0

_June 11, 2024_

FEATURES:

- Support for [GRPCRoute](https://gateway-api.sigs.k8s.io/api-types/grpcroute/):
  - Exact Method Matching, Header Matching, and Listener Hostname Matching. [1835](https://github.com/nginx/nginx-gateway-fabric/pull/1835)
  - RequestHeaderModifier Filter. [1909](https://github.com/nginx/nginx-gateway-fabric/pull/1909)
  - ResponseHeaderModifier filter. [1983](https://github.com/nginx/nginx-gateway-fabric/pull/1983)
- Support tracing via the ObservabilityPolicy CRD. [2004](https://github.com/nginx/nginx-gateway-fabric/pull/2004)
- NginxProxy CRD added to configure global settings (such as tracing endpoint) at the GatewayClass level. [1870](https://github.com/nginx/nginx-gateway-fabric/pull/1870)
  - Add configuration option to disable HTTP2 to the NginxProxy CRD. [1925](https://github.com/nginx/nginx-gateway-fabric/pull/1925)
- Introduce ClientSettingsPolicy CRD. This CRD allows users to configure the behavior of the connection between the client and NGINX. [1940](https://github.com/nginx/nginx-gateway-fabric/pull/1940)
- Introduce support for the HTTP filter `ResponseHeaderModifier`, enabling the modification of response headers within HTTPRoutes. [1880](https://github.com/nginx/nginx-gateway-fabric/pull/1880). With help from [Kai-Hsun Chen](https://github.com/kevin85421).
- Collect BackendTLSPolicy and GRPCRoute counts configured with NGINX Gateway Fabric. [1954](https://github.com/nginx/nginx-gateway-fabric/pull/1954)

BUG FIXES:

- Remove zone size for invalid backend ref. [1931](https://github.com/nginx/nginx-gateway-fabric/pull/1931)
- Fixed issue when using BackendTLSPolicy that led to failed connections. [1934](https://github.com/nginx/nginx-gateway-fabric/pull/1934).
- Update secrets on resource version change only. [2047](https://github.com/nginx/nginx-gateway-fabric/pull/2047)
- Fix reload errors due to long matching conditions. [1829](https://github.com/nginx/nginx-gateway-fabric/pull/1829).
- Add SecurityContextConstraints so NGF can run on Openshift. [1976](https://github.com/nginx/nginx-gateway-fabric/pull/1976)

DOCUMENTATION:

- Helm docs are now automatically generated. [2058](https://github.com/nginx/nginx-gateway-fabric/pull/2058)
- Add [guide](https://docs.nginx.com/nginx-gateway-fabric/how-to/monitoring/tracing/) on how to configure tracing for HTTPRoutes and GRPCRoutes. [2026](https://github.com/nginx/nginx-gateway-fabric/pull/2026).
- Add [guide](https://docs.nginx.com/nginx-gateway-fabric/how-to/traffic-management/client-settings/) on how to use the ClientSettingsPolicy API. [2071](https://github.com/nginx/nginx-gateway-fabric/pull/2071).
- Document how to upgrade from Open Source NGINX to NGINX Plus. [2104](https://github.com/nginx/nginx-gateway-fabric/pull/2104)
- Add [overview](https://docs.nginx.com/nginx-gateway-fabric/overview/custom-policies) of how custom policies work in NGINX Gateway Fabric. [2088](https://github.com/nginx/nginx-gateway-fabric/pull/2088)

HELM CHART:

- The version of the Helm chart is now 1.3.0
- Specify minimum Kubernetes version in Helm chart. [1885](https://github.com/nginx/nginx-gateway-fabric/pull/1885)
- Use kustomize to install Gateway API and NGINX Gateway Fabric CRDs. [1886](https://github.com/nginx/nginx-gateway-fabric/pull/1886) and [2011](https://github.com/nginx/nginx-gateway-fabric/pull/2011)
- Annotations for GatewayClass and NginxGateway are now configurable. [1993](https://github.com/nginx/nginx-gateway-fabric/pull/1993). Thanks to [sgavrylenko](https://github.com/sgavrylenko).
- Fix RBAC ServiceAccount ImagePullSecrets template which caused errors when running NGF with NGINX+. [1953](https://github.com/nginx/nginx-gateway-fabric/pull/1953)

DEPENDENCIES:

- The minimum supported version of Kubernetes is now 1.25. [1885](https://github.com/nginx/nginx-gateway-fabric/pull/1885)
- NGINX Plus was updated to R32. [2057](https://github.com/nginx/nginx-gateway-fabric/pull/2057)
- Update to v1.1.0 of the Gateway API. This includes a breaking change to BackendTLSPolicies - see [the release notes](https://github.com/kubernetes-sigs/gateway-api/releases/tag/v1.1.0) for further details. [1975](https://github.com/nginx/nginx-gateway-fabric/pull/1975)

UPGRADE:

- This version of NGINX Gateway Fabric is not compatible with v1.0.0 of the Gateway API. You must upgrade the Gateway API CRDs to v1.1.0 before upgrading NGINX Gateway Fabric. For instructions, see the upgrade documentation for [helm](https://docs.nginx.com/nginx-gateway-fabric/installation/installing-ngf/helm/#upgrade-nginx-gateway-fabric) or [manifests](https://docs.nginx.com/nginx-gateway-fabric/installation/installing-ngf/manifests/#upgrade-nginx-gateway-fabric). If you are using the v1.0.0 or earlier experimental versions of GRPCRoute or BackendTLSPolicy, see [v1.1.0 Upgrade Notes](https://gateway-api.sigs.k8s.io/guides/#v11-upgrade-notes) for instructions on upgrading the Gateway API CRDs.

KNOWN ISSUES:

- Tracing does not work on HTTPRoutes with matching conditions. [2105](https://github.com/nginx/nginx-gateway-fabric/issues/2105)
- ClientSettingsPolicy does not work on HTTPRoutes with matching conditions. [2079](https://github.com/nginx/nginx-gateway-fabric/issues/2079)
- In restrictive environments, the NGF Pod may fail to become ready due to a permissions issue that causes nginx reloads to fail. [1695](https://github.com/nginx/nginx-gateway-fabric/issues/1695)

COMPATIBILITY:

- Gateway API version: `1.1.0`. This release is not compatible with v1.0.0 of the Gateway API. See the UPGRADE section above for instructions on how to upgrade.
- NGINX version: `1.27.0`
- NGINX Plus version: `R32`
- Kubernetes version: `1.25+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.3.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.3.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.3.0`

## Release 1.2.0

_March 21, 2024_

FEATURES:

- [NGINX Plus](https://docs.nginx.com/nginx-gateway-fabric/overview/nginx-plus) can now be used as the data plane. [PR-1394](https://github.com/nginx/nginx-gateway-fabric/pull/1394)
  - Supports dynamic upstream reloads. [PR-1469](https://github.com/nginx/nginx-gateway-fabric/pull/1469)
  - Contains advanced Prometheus metrics. [PR-1394](https://github.com/nginx/nginx-gateway-fabric/pull/1394)
  - Includes the NGINX Plus monitoring dashboard. [PR-1488](https://github.com/nginx/nginx-gateway-fabric/pull/1488)
- Support for [BackendTLSPolicy](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/). [PR-1487](https://github.com/nginx/nginx-gateway-fabric/pull/1487)
- Support for URLRewrite HTTPRoute Filter. [PR-1396](https://github.com/nginx/nginx-gateway-fabric/pull/1396)
- NGINX Gateway Fabric will collect and report product telemetry to an F5 telemetry service every 24h. Read https://docs.nginx.com/nginx-gateway-fabric/overview/product-telemetry/ for more info, including what gets collected and how to opt out. [PR-1699](https://github.com/nginx/nginx-gateway-fabric/pull/1699)

ENHANCEMENTS:

- Stop processing resources that haven't changed. [PR-1422](https://github.com/nginx/nginx-gateway-fabric/pull/1422) Thanks to [Kai-Hsun Chen](https://github.com/kevin85421).
- Maintain Gateway Status order. [PR-1324](https://github.com/nginx/nginx-gateway-fabric/pull/1324) Thanks to [Kai-Hsun Chen](https://github.com/kevin85421).

BUG FIXES:

- Prevent paths in HTTPRoute matches from conflicting with internal locations in NGINX. [PR-1445](https://github.com/nginx/nginx-gateway-fabric/pull/1445)

DOCUMENTATION:

- Sample Grafana dashboard added. [PR-1620](https://github.com/nginx/nginx-gateway-fabric/pull/1620)
- Add a document about how to get support. [PR-1388](https://github.com/nginx/nginx-gateway-fabric/pull/1388)
- [Documentation](https://docs.nginx.com/nginx-gateway-fabric/installation/ngf-images) on how to build or install the NGINX Plus image.

HELM CHART:

- The version of the Helm chart is now 1.2.0
- nodeSelector is now configurable. [PR-1531](https://github.com/nginx/nginx-gateway-fabric/pull/1531) Thanks to [Leandro Martins](https://github.com/leandrocostam)

KNOWN ISSUES:

- Shutdown of non-leader Pods starts leader jobs. [1738](https://github.com/nginx/nginx-gateway-fabric/issues/1738)
- Too many matching conditions can cause reload errors. [1107](https://github.com/nginx/nginx-gateway-fabric/issues/1107)
- NGF Pod fails to become ready due to nginx reload failure. [1695](https://github.com/nginx/nginx-gateway-fabric/issues/1695)

COMPATIBILITY:

- Gateway API version: `1.0.0`
- NGINX version: `1.25.4`
- NGINX Plus version: `R31`
- Kubernetes version: `1.23+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.2.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.2.0`
- Data plane with NGINX Plus: `private-registry.nginx.com/nginx-gateway-fabric/nginx-plus:1.2.0`

## Release 1.1.0

_December 14, 2023_

This release updates NGINX Gateway Fabric to support version 1.0.0 of the Gateway API in addition to bug fixes and documentation updates. Our docs are now available at https://docs.nginx.com/nginx-gateway-fabric.

FEATURES:

- Update to v1.0.0 of the Gateway API. [PR-1250](https://github.com/nginx/nginx-gateway-fabric/pull/1250)
- Set the SupportedVersion Condition on GatewayClass. [PR-1301](https://github.com/nginx/nginx-gateway-fabric/pull/1301)

BUG FIXES:

- Merge HTTPRoute conditions from all Gateway controllers. [PR-1220](https://github.com/nginx/nginx-gateway-fabric/pull/1220)
- Validate header names and report validation errors in the HTTPRoute status. [PR-1239](https://github.com/nginx/nginx-gateway-fabric/pull/1239)
- Remove usage info from log output. [PR-1242](https://github.com/nginx/nginx-gateway-fabric/pull/1242)
- Set the Gateway Listener status AttachedRoutes field to the number of Routes associated with a Listener regardless of Gateway or Route status. [PR-1275](https://github.com/nginx/nginx-gateway-fabric/pull/1275)
- Set file mode explicitly for regular NGINX configuration files. [PR-1323](https://github.com/nginx/nginx-gateway-fabric/pull/1323). Thanks to [Kai-Hsun Chen](https://github.com/kevin85421).

DOCUMENTATION:

- Documentation is now available on docs.nginx.com. [Link](https://docs.nginx.com/nginx-gateway-fabric/)
- Update the resource validation documents to cover CEL validation. [Link](https://docs.nginx.com/nginx-gateway-fabric/overview/resource-validation/)
- Non-functional testing results. [Link](https://github.com/nginx/nginx-gateway-fabric/tree/main/tests)

HELM CHART:

- The version of the Helm chart is now 1.1.0.
- Add tolerations to the helm chart. [PR-1192](https://github.com/nginx/nginx-gateway-fabric/pull/1192). Thanks to [Jerome Brown](https://github.com/oWretch).
- Add extra volume mounts to the helm chart. [PR-1193](https://github.com/nginx/nginx-gateway-fabric/pull/1193). Thanks to [Jerome Brown](https://github.com/oWretch).
- Fix broken helm chart icon links. [PR-1290](https://github.com/nginx/nginx-gateway-fabric/pull/1290). Thanks to [arukiidou](https://github.com/arukiidou).

UPGRADE:

- This version of NGINX Gateway Fabric is not compatible with v0.8.0 of the Gateway API. You must upgrade the Gateway API CRDs to v1.0.0 before upgrading NGINX Gateway Fabric. For instructions, see the upgrade documentation for [helm](https://docs.nginx.com/nginx-gateway-fabric/installation/installing-ngf/helm/#upgrade-nginx-gateway-fabric) or [manifests](https://docs.nginx.com/nginx-gateway-fabric/installation/installing-ngf/manifests/#upgrade-nginx-gateway-fabric).

COMPATIBILITY:

- Gateway API version: `1.0.0`
- NGINX version: `1.25.3`
- Kubernetes version: `1.23+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.1.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.1.0`

## Release 1.0.0

_October 24, 2023_

This is the official v1.0.0 release of NGINX Gateway Fabric.

BREAKING CHANGES:

- Rename the product from NGINX Kubernetes Gateway to NGINX Gateway Fabric. [PR-1070](https://github.com/nginx/nginx-gateway-fabric/pull/1070)

FEATURES:

- Add readiness probe. [PR-1047](https://github.com/nginx/nginx-gateway-fabric/pull/1047)
- Support horizontal scaling. [PR-1048](https://github.com/nginx/nginx-gateway-fabric/pull/1048)
- Add NGINX reload metrics. [PR-1049](https://github.com/nginx/nginx-gateway-fabric/pull/1049)
- Retry status updater on failures. [PR-1062](https://github.com/nginx/nginx-gateway-fabric/pull/1062)
- Add event processing histogram metric. [PR-1134](https://github.com/nginx/nginx-gateway-fabric/pull/1134)
- Set Service address in Gateway Status. [PR-1141](https://github.com/nginx/nginx-gateway-fabric/pull/1141)

BUG FIXES:

- Optimize default NGINX config. [PR-1040](https://github.com/nginx/nginx-gateway-fabric/pull/1040)
- Ensure NGINX reload occurs. [PR-1033](https://github.com/nginx/nginx-gateway-fabric/pull/1033)
- Fix failure to recover if conf files are unexpectedly removed. [PR-1132](https://github.com/nginx/nginx-gateway-fabric/pull/1132)
- Only update a resource's status if it has changed. [PR-1151](https://github.com/nginx/nginx-gateway-fabric/pull/1151)

DOCUMENTATION:

- Non-functional testing guides and results. [Link](https://github.com/nginx/nginx-gateway-fabric/tree/main/tests)

COMPATIBILITY:

- Gateway API version: `0.8.1`
- NGINX version: `1.25.2`
- Kubernetes version: `1.23+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-gateway-fabric:1.0.0`
- Data plane: `ghcr.io/nginxinc/nginx-gateway-fabric/nginx:1.0.0`

## Release 0.6.0

_August 31, 2023_

This release adds a Helm chart, dynamic control plane logging, Prometheus metrics, and in-depth guides for various use cases.

FEATURES:

- Helm chart. [PR-840](https://github.com/nginx/nginx-gateway-fabric/pull/840)
- Use custom nginx container. [PR-934](https://github.com/nginx/nginx-gateway-fabric/pull/934)
- Support dynamic control plane logging. [PR-943](https://github.com/nginx/nginx-gateway-fabric/pull/943)
- Support websocket connections. [PR-962](https://github.com/nginx/nginx-gateway-fabric/pull/962)
- Support Prometheus metrics. [PR-999](https://github.com/nginx/nginx-gateway-fabric/pull/999)

BUG FIXES:

- Ensure NGINX Kubernetes Gateway has least privileges. [PR-1004](https://github.com/nginx/nginx-gateway-fabric/pull/1004)

DOCUMENTATION:

- Use case guides: https://github.com/nginx/nginx-gateway-fabric/tree/v0.6.0/docs/guides

COMPATIBILITY:

- Gateway API version: `0.8.0`
- NGINX version: `1.25.2`
- Kubernetes version: `1.23+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.6.0`
- Data plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway/nginx:0.6.0`

## Release 0.5.0

_July 17, 2023_

This release completes all v1beta1 Core features of the Gateway API resources. See the [Gateway Compatibility doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.5.0/docs/gateway-api-compatibility.md)

FEATURES:

- Support cross-namespace BackendRefs in HTTPRoutes. [PR-806](https://github.com/nginx/nginx-gateway-fabric/pull/806)
- Support dynamic certificate rotation with Kubernetes Secrets. [PR-807](https://github.com/nginx/nginx-gateway-fabric/pull/807)
- Support SupportedKinds in ListenerStatus. [PR-809](https://github.com/nginx/nginx-gateway-fabric/pull/809)

BUG FIXES:

- Set redirect port in location header according to the scheme. [PR-801](https://github.com/nginx/nginx-gateway-fabric/pull/801)
- Set proxy host header to the exact value of the request host header. [PR-827](https://github.com/nginx/nginx-gateway-fabric/pull/827)
- Ensure Prefix matching requires trailing slash. [PR-817](https://github.com/nginx/nginx-gateway-fabric/pull/817)

COMPATIBILITY:

- Gateway API version: `0.7.1`
- NGINX version: `1.25.x` \*
- Kubernetes version: `1.21+`

\*the installation manifests use the `nginx:1.25` image, which always points to the latest version of 1.25.x releases.

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.5.0`

## Release 0.4.0

_July 6, 2023_

This release brings:

- Support for more features of the Gateway API resources. See the [Gateway Compatibility doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.4.0/docs/gateway-api-compatibility.md)
- Support for running the conformance test suite. See the [Conformance tests README](https://github.com/nginx/nginx-gateway-fabric/blob/v0.4.0/conformance/README.md).
- Defined Enhancement Proposal process for NGINX Kubernetes Gateway project. See the [Enhancement Proposal README](https://github.com/nginx/nginx-gateway-fabric/blob/v0.4.0/docs/proposals/README.md).
- Improved developer documentation for contributing to the project. See the [Development quickstart](https://github.com/nginx/nginx-gateway-fabric/blob/v0.4.0/docs/developer/quickstart.md).
- Architecture document that explains how NGINX Kubernetes Gateway works at a high level. See the [Architecture doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.4.0/docs/architecture.md)
- Miscellaneous enhancements and bug fixes.

FEATURES:

- Allow empty sectionName in HTTPRoute parentRef. [PR-626](https://github.com/nginx/nginx-gateway-fabric/pull/626)
- Exact PathMatch support for HTTPRoutes. [PR-603](https://github.com/nginx/nginx-gateway-fabric/pull/603)
- Set ResolvedRefs condition to true on HTTPRoutes. [PR-645](https://github.com/nginx/nginx-gateway-fabric/pull/645)
- Set gateway Pod IP as GatewayStatus address. [PR-638](https://github.com/nginx/nginx-gateway-fabric/pull/638)
- Set Accepted condition type on Gateway status. [PR-633](https://github.com/nginx/nginx-gateway-fabric/pull/633)
- Drop unrequired capabilities from containers. [PR-677](https://github.com/nginx/nginx-gateway-fabric/pull/677)
- Update route condition where listener is not found. [PR-675](https://github.com/nginx/nginx-gateway-fabric/pull/675)
- Set Gateway Programmed condition. [PR-658](https://github.com/nginx/nginx-gateway-fabric/pull/658)
- AllowedRoutes support for Listeners. [PR-721](https://github.com/nginx/nginx-gateway-fabric/pull/721)
- Support custom listener ports. [PR-745](https://github.com/nginx/nginx-gateway-fabric/pull/745)
- Add support for RequestHeaderModifier for HTTPRouteRule objects. [PR-717](https://github.com/nginx/nginx-gateway-fabric/pull/717)
- Add wildcard hostname support. [PR-769](https://github.com/nginx/nginx-gateway-fabric/pull/769)
- Add Programmed status for listener. [PR-786](https://github.com/nginx/nginx-gateway-fabric/pull/786)
- ReferenceGrant from Gateway to Secret. [PR-791](https://github.com/nginx/nginx-gateway-fabric/pull/791)

BUG FIXES:

- Set upstream zone size to 512k. [PR-609](https://github.com/nginx/nginx-gateway-fabric/pull/609)
- Allow empty HTTPRoute hostnames. [PR-650](https://github.com/nginx/nginx-gateway-fabric/pull/650)
- Allow long server names. [PR-651](https://github.com/nginx/nginx-gateway-fabric/pull/651)
- Add in required capabilities for writing TLS secrets. [PR-718](https://github.com/nginx/nginx-gateway-fabric/pull/718)
- Fix binding to multiple listeners with empty section name. [PR-730](https://github.com/nginx/nginx-gateway-fabric/pull/730)
- Add timeout and retry logic for finding NGINX PID file. [PR-676](https://github.com/nginx/nginx-gateway-fabric/pull/676)
- Prioritize method matching. [PR-789](https://github.com/nginx/nginx-gateway-fabric/pull/789)
- Add NewListenerInvalidRouteKinds condition. [PR-799](https://github.com/nginx/nginx-gateway-fabric/pull/799)
- Set ResolvedRefs/False/InvalidKind condition on the HTTPRoute if a BackendRef specifies an unknown kind. [PR-800](https://github.com/nginx/nginx-gateway-fabric/pull/800)
- Set GatewayClass status for ignored GatewayClasses. [PR-804](https://github.com/nginx/nginx-gateway-fabric/pull/804)

DEPENDENCIES:

- Bump sigs.k8s.io/gateway-api from 0.7.0 to 0.7.1. [PR-711](https://github.com/nginx/nginx-gateway-fabric/pull/711)

COMPATIBILITY:

- Gateway API version: `0.7.1`
- NGINX version: `1.25.x` \*
- Kubernetes version: `1.21+`

\*the installation manifests use the `nginx:1.25` image, which always points to the latest version of 1.25.x releases.

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.4.0`

## Release 0.3.0

_April 24, 2023_

This release brings:

- Extensive validation of Gateway API resources for robustness, security and correctness. See the [validation doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.3.0/docs/resource-validation.md)
  for more details.
- Defined open-source development process for NGINX Kubernetes Gateway project. See the [Issue lifecycle doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.3.0/ISSUE_LIFECYCLE.md).
- Miscellaneous enhancements and bug fixes.

FEATURES:

- Report proper Conditions in status of HTTPRoute and Gateway when GatewayClass is invalid or doesn't exist. [PR-576](https://github.com/nginx/nginx-gateway-fabric/pull/576)
- Implement NKG-specific field validation for GatewayClasses. [PR-295](https://github.com/nginx/nginx-gateway-fabric/pull/495)
- Implement NKG-specific field validation for HTTPRoutes. [PR-455](https://github.com/nginx/nginx-gateway-fabric/pull/455)
- Implement NKG-specific field validation for Gateways. [PR-407](https://github.com/nginx/nginx-gateway-fabric/pull/407)
- Run webhook validation rules inside NKG control plane. [PR-388](https://github.com/nginx/nginx-gateway-fabric/pull/388)
- Make NGINX error log visible in NGINX container logs. [PR-319](https://github.com/nginx/nginx-gateway-fabric/pull/319)
- Always generate a root "/" location block in NGINX config to handle unmatched requests with 404 response. [PR-356](https://github.com/nginx/nginx-gateway-fabric/pull/356)

BUG FIXES:

- Fix HTTPRoute section name related bugs. [PR-568](https://github.com/nginx/nginx-gateway-fabric/pull/568)
- Fix Observed Generation for Gateway Status. [PR-351](https://github.com/nginx/nginx-gateway-fabric/pull/351)
- Fix status for parentRef with invalid listener in HTTPRoute. [PR-350](https://github.com/nginx/nginx-gateway-fabric/pull/350)
- Fix initContainer failure during pod restart. [PR-337](https://github.com/nginx/nginx-gateway-fabric/pull/337).
  Thanks to [Tom Plant](https://github.com/pl4nty)
- Generate default http server in NGINX if http listener exists in Gateway. [PR-320](https://github.com/nginx/nginx-gateway-fabric/pull/320)

DEPENDENCIES:

- Bump sigs.k8s.io/gateway-api from 0.6.0 to 0.6.2. [PR-471](https://github.com/nginx/nginx-gateway-fabric/pull/471)

COMPATIBILITY:

- Gateway API version: `0.6.2`
- NGINX version: `1.23.x` \*
- Kubernetes version: `1.21+`

\*the installation manifests use the `nginx:1.23` image, which always points to the latest version of 1.23.x releases.

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.3.0`

## Release 0.2.0

_October 25, 2022_

This release extends the support of the features of the Gateway API resources.

FEATURES:

- Support the Pod IPs instead of the virtual IP of a Service in the NGINX upstream. Additionally, NGINX Gateway
  Fabric will pick up any changes to the Pod IPs and update the NGINX upstream
  accordingly. [PR-221](https://github.com/nginx/nginx-gateway-fabric/pull/221)
- Support the redirect filter in an HTTPRoute rule. [PR-218](https://github.com/nginx/nginx-gateway-fabric/pull/218)
- Support weights in backendRefs in the HTTPRoute (traffic splitting). [PR-261](https://github.com/nginx/nginx-gateway-fabric/pull/261)
- Support the ObservedGeneration field in the HTTPRoute status. [PR-254](https://github.com/nginx/nginx-gateway-fabric/pull/254)

BUG FIXES:

- Do not require the namespace in the `--gateway-ctlr-name` cli argument. [PR-235](https://github.com/nginx/nginx-gateway-fabric/pull/235)
- Ensure NGINX Kubernetes Gateway exits gracefully during shutdown. [PR-250](https://github.com/nginx/nginx-gateway-fabric/pull/250)
- Handle query param names in case-sensitive way. [PR-220](https://github.com/nginx/nginx-gateway-fabric/pull/220)

DEPENDENCIES:

- Use the latest NGINX 1.23 image. [PR-275](https://github.com/nginx/nginx-gateway-fabric/pull/275)
- Bump sigs.k8s.io/gateway-api from 0.5.0 to 0.5.1 [PR-251](https://github.com/nginx/nginx-gateway-fabric/pull/251)

COMPATIBILITY:

- Gateway API version: `0.5.1`
- NGINX version: `1.21.x` \*
- Kubernetes version: `1.21+`

\*the installation manifests use the `nginx:1.21` image, which always points to the latest version of 1.21.x releases.

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.2.0`

## Release 0.1.0

_August 22, 2022_

This is an initial release of NGINX Kubernetes Gateway project.

The release includes:

- A control plane agent (a Kubernetes controller) that updates date plane (NGINX) configuration based on the state of
  the resources in the cluster.
- Support for NGINX as a data plane.
- Kubernetes manifests for a Deployment with a single Pod with the control plane and data plane containers as well as
  Services to enable external connectivity to that Pod.
- Support for a subset of features of GatewayClass, Gateway and HTTPRoute resources (see the [Gateway API Compatibility doc](https://github.com/nginx/nginx-gateway-fabric/blob/v0.1.0/README.md)).

We expect that the architecture of NGINX Kubernetes Gateway -- the number of pods and containers and their
interaction -- will change as the project evolves.

NGINX Kubernetes Gateway is ready for experimental usage. We included
the [docs](https://github.com/nginx/nginx-gateway-fabric/tree/v0.1.0/docs) as well
as [examples](https://github.com/nginx/nginx-gateway-fabric/tree/v0.1.0/examples).

If you'd like to give us feedback or get involved, see
the [README](https://github.com/nginx/nginx-gateway-fabric) to learn how.

COMPATIBILITY:

- Gateway API version: `0.5.0`
- NGINX version: `1.21.3`
- Kubernetes version: `1.19+`

CONTAINER IMAGES:

- Control plane: `ghcr.io/nginxinc/nginx-kubernetes-gateway:0.1.0`
