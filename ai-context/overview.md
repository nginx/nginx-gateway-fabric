# Project Overview

NGINX Gateway Fabric (NGF) is an open-source implementation of the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/) that uses [NGINX](https://nginx.org/) as its data plane. It translates Gateway API resources into NGINX configuration, providing HTTP and TCP/UDP load balancing, reverse proxying, and API gateway functionality for applications running on Kubernetes.

## The Problem It Solves

Kubernetes does not ship with a built-in traffic gateway. The older Ingress API offers limited routing capabilities and lacks standardization across implementations, leading to vendor-specific annotations and configuration drift. The Gateway API specification addresses this by defining a richer, role-oriented, and portable API for managing ingress traffic. NGF implements this specification, giving teams a standards-based gateway backed by NGINX's proven request-handling engine.

## Who It Is For

NGF targets platform engineers, cluster operators, and DevOps teams who manage Kubernetes infrastructure and need production-grade traffic management. Its role-oriented design aligns with the Gateway API security model: infrastructure providers manage GatewayClass resources, cluster operators configure Gateways and policies, and application developers attach Routes to expose their services. Organizations already using NGINX benefit from a familiar data plane with known performance and stability characteristics.

## Where It Fits in the Gateway API Ecosystem

The Gateway API is an official Kubernetes SIG-Network project with multiple conformant implementations, including Istio, Envoy Gateway, Contour, and Traefik, among others. NGF is distinguished by its use of NGINX (both OSS and Plus) as the data plane, its control plane / data plane separation architecture, and its support for per-Gateway dynamic provisioning. The control plane watches Kubernetes resources and generates NGINX configuration; the data plane pods run NGINX with an agent that receives configuration updates over mTLS gRPC. This separation provides security isolation, independent scaling, and blast-radius containment.

NGF implements the core Gateway API resources -- `Gateway`, `GatewayClass`, `HTTPRoute`, `GRPCRoute`, `TLSRoute`, `TCPRoute`, and `UDPRoute` -- and extends the API with custom resources for NGINX-specific capabilities such as rate limiting, client settings, upstream tuning, observability, and raw NGINX snippet injection. It passes the Gateway API conformance test suite, validating compliance with the specification.

The project is Apache 2.0 licensed, maintained by NGINX (F5), and follows semantic versioning. The current stable release is v2.5.1, supporting Gateway API v1.5.1 and Kubernetes 1.31+. An optional NGINX Plus subscription adds features such as dynamic upstreams and advanced metrics.

## Key References

- [README.md](/README.md) -- project overview, getting started, version matrix
- [ARCHITECTURE.md](/ARCHITECTURE.md) -- comprehensive architecture documentation
- [Design Principles](/docs/developer/design-principles.md) -- security, availability, performance, resilience, observability, ease of use
- [Gateway API Compatibility](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-api-compatibility/) -- supported resources and features
