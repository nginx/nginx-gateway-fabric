
# Enhancement Proposal-4052: External AuthFilter

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4052
- Status: Provisional

## Summary

[GEP-1494](https://gateway-api.sigs.k8s.io/geps/gep-1494/) defines an API for HTTPRoute to standardize Authentication and Authorization within the Gateway API.

This proposal aim to provider users of the Gateway API with a native form of Authenticaiton through Gateway API's [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter)

## Goals

- Integrate with the [HTTPExternalAuthFilter](https://gateway-api.sigs.k8s.io/reference/spec/#httpexternalauthfilter) for HTTPRoute in the Gateway API
- Ensure this capability is available only when users enable experimental features in NGF
- Support only [HTTPAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#httpauthconfig)
- Provide users with a helm chart to deploy, manage and configure NGINX for Authentication use cases. i.e. the External Service
- Allow users to configure an exteranl IdP such as Keycloak or AzureAD

## Non-Goals

- Support for [GRPCAuthConfig](https://gateway-api.sigs.k8s.io/reference/spec/#grpcauthconfig)
