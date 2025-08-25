# Enhancement Proposal-3716: Gateway API Inference Extension

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/3716
- Status: Provisional

## Summary

Enable NGINX Gateway Fabric to support the [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/), allowing dynamic routing to AI workloads. The goal for now is a basic implementation that meets the core functionality based on the API spec. There are likely many enhancements and improvements that can be made to this, but those should be considered after feedback around the usage and worth of this feature.

## Goals

- Define and implement the basic implementation to meet the API's core specifications.
- Determine which resources (e.g. InferencePool) NGF needs to watch, and what configuration should be built based upon this.
- Define the process in which NGF should integrate with the [Endpoint Picker](https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/pkg/epp) (EPP).
- Determine what NGINX needs to do in order to forward incoming traffic to an AI workload.

## Non-Goals

- Define new APIs.
- Determine how to integrate with AI Gateway.
- Any functionality beyond the core API specification.
