[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/nginx/nginx-gateway-fabric/badge)](https://scorecard.dev/viewer/?uri=github.com/nginx/nginx-gateway-fabric)
[![Continuous Integration](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/ci.yml/badge.svg)](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nginx/nginx-gateway-fabric)](https://goreportcard.com/report/github.com/nginx/nginx-gateway-fabric)
[![codecov](https://codecov.io/gh/nginx/nginx-gateway-fabric/graph/badge.svg?token=32ULC8F13Z)](https://codecov.io/gh/nginx/nginx-gateway-fabric)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/nginx/nginx-gateway-fabric?logo=github&sort=semver)](https://github.com/nginx/nginx-gateway-fabric/releases/latest)
[![Forum](https://img.shields.io/badge/nginx--community--forum-green)](https://community.nginx.org/c/projects/nginx-gateway-fabric/25)
[![Project Status: Active – The project has reached a stable, usable state and is being actively developed.](https://www.repostatus.org/badges/latest/active.svg)](https://www.repostatus.org/#active)

# NGINX Gateway Fabric

NGINX Gateway Fabric is an open-source project that provides an implementation of
the [Gateway API](https://gateway-api.sigs.k8s.io/) using [NGINX](https://nginx.org/) as the data plane. The goal of
this project is to implement the core Gateway APIs -- `Gateway`, `GatewayClass`, `HTTPRoute`, `GRPCRoute`, `TCPRoute`, `TLSRoute`,
and `UDPRoute` -- to configure an HTTP or TCP/UDP load balancer, reverse-proxy, or API gateway for applications running
on Kubernetes.

For a list of supported Gateway API resources and features, see
the [Gateway API Compatibility](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-api-compatibility/) doc.

Learn about our [design principles](/docs/developer/design-principles.md) and [architecture](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-architecture/).

NGINX Gateway Fabric uses [NGINX Agent](https://github.com/nginx/agent) to configure NGINX.

## Getting Started

1. [Get started using a kind cluster](https://docs.nginx.com/nginx-gateway-fabric/get-started/).
2. [Install](https://docs.nginx.com/nginx-gateway-fabric/install/) NGINX Gateway Fabric.
3. Deploy various [examples](examples).
4. Follow instructions for common use cases such as [routing](https://docs.nginx.com/nginx-gateway-fabric/traffic-management/) and [securing](https://docs.nginx.com/nginx-gateway-fabric/traffic-security/) traffic, or [monitoring](https://docs.nginx.com/nginx-gateway-fabric//monitoring/) your cluster.

You can find the comprehensive NGINX Gateway Fabric user documentation on the [NGINX Documentation](https://docs.nginx.com/nginx-gateway-fabric/) website.

## NGINX Gateway Fabric Releases

We publish NGINX Gateway Fabric releases on GitHub. See
our [releases page](https://github.com/nginx/nginx-gateway-fabric/releases).

The latest release is [2.4.1](https://github.com/nginx/nginx-gateway-fabric/releases/tag/v2.4.1).

The edge version is useful for experimenting with new features that are not yet published in a release. To use, choose
the _edge_ version built from the [latest commit](https://github.com/nginx/nginx-gateway-fabric/commits/main)
from the main branch.

The table below summarizes the options regarding the images, manifests, documentation and examples and gives your links
to the correct versions:

| Version | Description | Installation Manifests | Documentation and Examples |
| ------- | ----------- | ---------------------- | -------------------------- |
| Latest release | For production use | [Manifests](https://github.com/nginx/nginx-gateway-fabric/tree/v2.4.1/deploy). | [Documentation](https://docs.nginx.com/nginx-gateway-fabric). [Examples](https://github.com/nginx/nginx-gateway-fabric/tree/v2.4.1/examples). |
| Edge | For experimental use and latest features | [Manifests](https://github.com/nginx/nginx-gateway-fabric/tree/main/deploy). | [Examples](https://github.com/nginx/nginx-gateway-fabric/tree/main/examples). |

### Versioning

NGF uses semantic versioning for its releases. For more information, see https://semver.org.

> Major version zero `(0.Y.Z)` is reserved for development, anything MAY change at any time. The public API is not stable.

### Release Planning and Development

The features that will go into the next release are reflected in the
corresponding [milestone](https://github.com/nginx/nginx-gateway-fabric/milestones). Refer to
the [Issue Lifecycle](ISSUE_LIFECYCLE.md) document for information on issue creation and assignment to releases.

## Technical Specifications

The following table lists the software versions NGINX Gateway Fabric supports.

| NGINX Gateway Fabric | Gateway API | Kubernetes | NGINX OSS | NGINX Plus | NGINX Agent |
|----------------------|-------------|------------|-----------|------------|-------------|
| Edge                 | 1.4.1       | 1.25+      | 1.29.5    | R36        | v3.7.0      |
| 2.4.1                | 1.4.1       | 1.25+      | 1.29.5    | R36        | v3.7.0      |
| 2.4.0                | 1.4.1       | 1.25+      | 1.29.4    | R36        | v3.6.2      |
| 2.3.0                | 1.4.1       | 1.25+      | 1.29.3    | R36        | v3.6.0      |
| 2.2.2                | 1.3.0       | 1.25+      | 1.29.2    | R35        | v3.6.0      |
| 2.2.1                | 1.3.0       | 1.25+      | 1.29.2    | R35        | v3.5.0      |
| 2.2.0                | 1.3.0       | 1.25+      | 1.29.2    | R35        | v3.3.2      |
| 2.1.4                | 1.3.0       | 1.25+      | 1.29.1    | R35        | v3.3.1      |
| 2.1.3                | 1.3.0       | 1.25+      | 1.29.1    | R35        | v3.3.1      |
| 2.1.2                | 1.3.0       | 1.25+      | 1.29.1    | R35        | v3.3.1      |
| 2.1.1                | 1.3.0       | 1.25+      | 1.29.1    | R35        | v3.2.1      |
| 2.1.0                | 1.3.0       | 1.25+      | 1.29.1    | R35        | v3.2.1      |
| 2.0.2                | 1.3.0       | 1.25+      | 1.28.0    | R34        | v3.0.1      |
| 2.0.1                | 1.3.0       | 1.25+      | 1.28.0    | R34        | v3.0.1      |
| 2.0.0                | 1.3.0       | 1.25+      | 1.28.0    | R34        | v3.0.0      |
| 1.6.2                | 1.2.1       | 1.25+      | 1.27.4    | R33        | ---         |
| 1.6.1                | 1.2.1       | 1.25+      | 1.27.4    | R33        | ---         |
| 1.6.0                | 1.2.1       | 1.25+      | 1.27.3    | R33        | ---         |
| 1.5.1                | 1.2.0       | 1.25+      | 1.27.2    | R33        | ---         |
| 1.5.0                | 1.2.0       | 1.25+      | 1.27.2    | R33        | ---         |
| 1.4.0                | 1.1.0       | 1.25+      | 1.27.1    | R32        | ---         |
| 1.3.0                | 1.1.0       | 1.25+      | 1.27.0    | R32        | ---         |
| 1.2.0                | 1.0.0       | 1.23+      | 1.25.4    | R31        | ---         |

## SBOM (Software Bill of Materials)

We generate SBOMs for the binaries and the Docker image.

### Binaries

The SBOMs for the binaries are available in the releases page. The SBOMs are generated
using [syft](https://github.com/anchore/syft) and are available in SPDX format.

### Docker Images

The SBOM for the Docker image is available in
the [GitHub Container](https://github.com/nginx/nginx-gateway-fabric/pkgs/container/nginx-gateway-fabric)
repository. The SBOM is generated using [syft](https://github.com/anchore/syft) and stored as an attestation in the
image manifest.

For example to retrieve the SBOM for `linux/amd64` and analyze it using [grype](https://github.com/anchore/grype) you
can run the following command:

```shell
docker buildx imagetools inspect ghcr.io/nginx/nginx-gateway-fabric:edge --format '{{ json (index .SBOM "linux/amd64").SPDX }}' | grype
```

## Troubleshooting

For troubleshooting help, see the [Troubleshooting](https://docs.nginx.com/nginx-gateway-fabric/troubleshooting/) document.

## Contacts

We’d like to hear your feedback! If you experience issues with our Gateway Controller, please [open a bug][bug] in
GitHub. If you have any suggestions or enhancement requests, please [open an idea][idea] on GitHub discussions. You can
contact us directly on the [NGINX Community Forum][forum].

[bug]: https://github.com/nginx/nginx-gateway-fabric/issues/new?assignees=&labels=&projects=&template=bug_report.md&title=
[idea]: https://github.com/nginx/nginx-gateway-fabric/discussions/categories/ideas
[forum]: https://community.nginx.org/c/projects/nginx-gateway-fabric/25

## Community Meetings

Every Tuesday at 9:30AM Pacific / 5:30PM GMT

For the meeting link, updates, agenda, and meeting notes, check the calendar below:

[NGINX Gateway Fabric Meeting Calendar](https://calendar.google.com/calendar/embed?src=901278554a5187740a1cef8c6fd92351eb592550cb5936ba04d318f8859888f4%40group.calendar.google.com&ctz=America%2FLos_Angeles)

If you have a use case for NGINX Gateway Fabric that the project can't quite meet yet, bugs, problems, success stories, or just want to be more involved with the project, come by and say hi!

## Contributing

Please read our [Contributing guide](CONTRIBUTING.md) if you'd like to contribute to the project.

## Support and NGINX Plus

If your team needs dedicated support for NGINX Gateway Fabric in your environment, or you would like to leverage our [advanced NGINX Plus features](https://docs.nginx.com/nginx-gateway-fabric/overview/nginx-plus/), you can reach out [here](https://www.f5.com/content/f5-com/en_us/products/get-f5).

To try NGINX Gateway Fabric with NGINX Plus, you can start your free [30-day trial](https://www.f5.com/trials), then follow the [installation guide](https://docs.nginx.com/nginx-gateway-fabric/install/helm/) for installing with NGINX Plus.
