# Enhancement Proposal-4051: Session Persistence for NGINX Plus and OSS

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/4051
- Status: Provisional

## Summary

This enhancement proposal extends the Upstream Settings Policy API to support session persistence for both NGINX Plus and NGINX OSS. It enables application developers to configure basic session persistence using `ip_hash` for OSS and cookie-based session persistence for NGINX Plus.

## Goals

- Extend Upstream Settings Policy API to support session persistence.

## Non-Goals

- Provide implementation details for enabling session persistence.
- Support session persistence for TLSRoute or other Layer 4 routes.

## Introduction

### Extension of Upstream Settings Policy API


- explain API
- How it works for OSS and Plus
- Describe directives
- directive contraints
- sample config

Apps using session persistence must account for aspects like load shedding, draining, and session migration as a part of their application design.

