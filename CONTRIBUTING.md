# Contributing Guidelines

The following is a set of guidelines for contributing to NGINX Gateway Fabric. We really appreciate that you are
considering contributing!

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Ask a Question](#ask-a-question)
- [Getting Started](#getting-started)
  - [Project Structure](#project-structure)
- [Contributing](#contributing)
  - [Issues and Discussions](#issues-and-discussions)
    - [Open a Discussion](#open-a-discussion)
    - [Report a Bug](#report-a-bug)
    - [Suggest an Enhancement](#suggest-an-enhancement)
    - [Issue lifecycle](#issue-lifecycle)
  - [Development Guide](#development-guide)
- [F5 Contributor License Agreement (CLA)](#f5-contributor-license-agreement-cla)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Ask a Question

To ask a question, use [Github Discussions](https://github.com/nginx/nginx-gateway-fabric/discussions).

[NGINX Community Slack](https://community.nginx.org/joinslack) has a dedicated channel for this
project -- `#nginx-gateway-fabric`.

Reserve GitHub issues for feature requests and bugs rather than general questions.

## Getting Started

Follow our [Installation Instructions](https://docs.nginx.com/nginx-gateway-fabric/installation/) to get the NGINX Gateway Fabric up and running.

### Project Structure

- NGINX Gateway Fabric is written in Go and uses the open source NGINX software as the data plane.
- The project follows a standard Go project layout
  - The main code is found at `cmd/gateway/`
  - The internal code is found at `internal/`
  - Build files for Docker are found under `build/`
  - Deployment yaml files are found at `deploy/`
  - External APIs, clients, and SDKs can be found under `pkg/`
- We use [Go Modules](https://github.com/golang/go/wiki/Modules) for managing dependencies.
- We use [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) for our BDD style unit
  tests.
- The documentation is managed in [the NGINX documentation repository](https://github.com/nginx/documentation).

## Contributing

### Issues and Discussions

#### Open a Discussion

If you have any questions, ideas, or simply want to engage in a conversation with the community and maintainers, we
encourage you to open a [discussion](https://github.com/nginx/nginx-gateway-fabric/discussions) on GitHub.

#### Report a Bug

To report a bug, open an issue on GitHub with the label `bug` using the available bug report issue template. Before
reporting a bug, make sure the issue has not already been reported.

#### Suggest an Enhancement

To suggest an enhancement, [open an idea][idea] on GitHub discussions. We highly recommend that you open a discussion
about a potential enhancement before opening an issue. This enables the maintainers to gather valuable insights
regarding the idea and its use cases, while also giving the community an opportunity to provide valuable feedback.

In some cases, the maintainers may ask you to write an Enhancement Proposal. For details on this process, see
the [Enhancement Proposal](/docs/proposals/README.md) README.

[idea]: https://github.com/nginx/nginx-gateway-fabric/discussions/new?category=ideas

#### Issue lifecycle

When an issue or PR is created, it will be triaged by the maintainers and assigned a label to indicate the type of issue
it is (bug, proposal, etc) and to determine the milestone. See the [Issue Lifecycle](/ISSUE_LIFECYCLE.md) document for
more information.

### Development Guide

Before beginning development, familiarize yourself with the following documents:

- [Developer Quickstart](/docs/developer/quickstart.md): This guide provides a quick and easy walkthrough of setting up
  your development environment and executing tasks required when submitting a pull request.
- [Branching and Workflow](/docs/developer/branching-and-workflow.md): This document outlines the project's specific
  branching and workflow practices, including instructions on how to name a branch.
- [Implement a Feature](/docs/developer/implementing-a-feature.md): A step-by-step guide on how to implement a feature
  or bug.
- [Testing](/docs/developer/testing.md): The project's testing guidelines, includes both unit testing and manual testing
  procedures. This document explains how to write and run unit tests, and how to manually verify changes.
- [Pull Request Guidelines](/docs/developer/pull-request.md): A guide for both pull request submitters and reviewers,
  outlining guidelines and best practices to ensure smooth and efficient pull request processes.
- [Go Style Guide](/docs/developer/go-style-guide.md): A coding style guide for Go. Contains best practices and
  conventions to follow when writing Go code for the project.
- [Architecture](https://docs.nginx.com/nginx-gateway-fabric/overview/gateway-architecture/): A high-level overview of the project's architecture.
- [Design Principles](/docs/developer/design-principles.md): An overview of the project's design principles.
- [The NGINX documentation repository](https://github.com/nginx/documentation): The repository for NGINX documentation, which is used for the website.

## F5 Contributor License Agreement (CLA)

F5 requires all external contributors to agree to the terms of the F5 CLA (available [here](https://github.com/f5/.github/blob/main/CLA/cla-markdown.md)) before any of their changes can be incorporated into an F5 Open Source repository.

If you have not yet agreed to the F5 CLA terms and submit a PR to this repository, a bot will prompt you to view and agree to the F5 CLA. You will have to agree to the F5 CLA terms through a comment in the PR before any of your changes can be merged. Your agreement signature will be safely stored by F5 and no longer be required in future PRs.
