# Workflows

This document describes how to file issues, propose features, submit pull requests, run tests before submitting, and respond to review feedback in NGINX Gateway Fabric.

## Filing an Issue

### Bug Reports

Open a GitHub issue using the **bug report** template. Before filing, check that the issue has not already been reported. Include:

- Steps to reproduce
- Expected vs. actual behavior
- NGF version, Kubernetes version, and NGINX version
- Relevant logs or configuration

### Questions

Use [GitHub Discussions](https://github.com/nginx/nginx-gateway-fabric/discussions) or the [NGINX Community Forum](https://community.nginx.org/c/projects/nginx-gateway-fabric/25). Do not use GitHub issues for general questions.

## Proposing a Feature

1. **Open a Discussion first.** Post an [idea](https://github.com/nginx/nginx-gateway-fabric/discussions/new?category=ideas) on GitHub Discussions describing the use case and desired behavior. This lets maintainers and the community give early feedback before any code is written.

2. **Enhancement Proposal (if requested).** For significant features, maintainers may ask you to write a formal proposal. See [docs/proposals/README.md](/docs/proposals/README.md) for the process.

3. **Issue creation.** Once the approach is agreed upon, a GitHub issue is created and assigned to a milestone.

## Submitting a Pull Request

### Setup

NGF uses a **fork-and-merge** workflow. All contributors (internal and external) work from their own forks:

1. Fork `github.com/nginx/nginx-gateway-fabric`
2. Clone your fork and add the upstream remote:
   ```shell
   git clone git@github.com:<YOUR-USER>/nginx-gateway-fabric.git
   git remote add upstream git@github.com:nginx/nginx-gateway-fabric.git
   ```
3. Keep your fork synced: `git pull upstream main`

### Branch Naming

Use prefixes that match the [labeler](/.github/labeler.yml) configuration, which drives automatic release note generation:

| Prefix | Use For |
|--------|---------|
| `bug/` or `fix/` | Bug fixes |
| `feat/` or `feature/` | New features |
| `docs/` | Documentation changes |
| `chore/` | Maintenance, CI, dependency updates |
| `enh/` | Enhancements to existing features |

### PR Description

Fill in the [PR template](/.github/PULL_REQUEST_TEMPLATE.md):

- **Problem:** Brief overview of the issue being addressed
- **Solution:** Explanation of the approach and key design decisions
- **Testing:** Description of testing performed
- **Please focus on (optional):** Areas where you want specific reviewer attention
- **Closes #ISSUE:** Link to the GitHub issue

### PR Title

Use customer-focused language. PR titles become release notes. Write from the user's perspective:

- Good: "Support configurable debug mode"
- Bad: "Add debug boolean to config struct"

### PR Checklist

Before creating the PR, verify:

- [ ] Read [CONTRIBUTING.md](/CONTRIBUTING.md)
- [ ] Added tests proving the fix or feature works
- [ ] All unit tests pass (`make unit-test`)
- [ ] Necessary documentation updated
- [ ] Branch rebased onto main
- [ ] PR targets main from your fork

### Commit Hygiene

- **During review:** Commit each change individually with descriptive messages so reviewers can follow incremental updates.
- **Before merge:** Squash all commits into a single commit following the [commit message format](/docs/developer/pull-request.md#commit-message-format):

```
<One line summary>

Problem: Why this change is needed.

Solution: What was done and key design decisions.

Testing: How it was validated.
```

- **Release notes:** If the change affects users, add a summary in the `release-note` block in the PR description. Write `NONE` if not applicable.

## Running Tests Before Submitting

At minimum, run the full development check before pushing:

```shell
make dev-all
```

This runs: `deps` -> `fmt` -> `njs-fmt` -> `vet` -> `lint` -> `unit-test` -> `njs-unit-test`.

For changes that affect NGINX configuration or routing behavior, also run conformance tests locally:

```shell
# From the tests/ directory:
make install-ngf-local-build
make build-test-runner-image
make run-conformance-tests
```

For NGF-specific behavior changes, run relevant functional tests:

```shell
# From the tests/ directory:
make test TAG=$(whoami) GINKGO_LABEL=<relevant-label> GINKGO_PROCS=1
```

If you changed CRDs, regenerate and verify:

```shell
make generate-all
```

## Responding to Review Feedback

### Resolving Comments

- **Simple, unambiguous comments:** Resolve them yourself (prioritize speed).
- **Design-level changes:** Leave resolution for the reviewer to confirm.
- **Long discussions (3-5+ rounds):** Move to Slack, Zoom, or an in-person conversation, then update the PR with the decision:
  ```
  Spoke offline, decided to move A to package B. Rewrite tests and API docs.
  ```

### Pushing Updates

- Commit each review change individually with a descriptive message
- Do not force-push during review (reviewers need to see incremental changes)
- Squash to a single commit only after approval and all comments are resolved

### Required Approvals and CI

- **Minimum 2 approvals** from maintainers
- **All CI checks must pass** (lint, unit tests, build, conformance, functional)
- **F5 CLA** must be signed (a bot will prompt on first PR)
- Merges use **squash-and-merge** (the commit history on main shows one commit per PR with the PR number appended)

## Key References

- [CONTRIBUTING.md](/CONTRIBUTING.md) -- contribution guidelines
- [docs/developer/branching-and-workflow.md](/docs/developer/branching-and-workflow.md) -- fork-and-merge workflow details
- [docs/developer/pull-request.md](/docs/developer/pull-request.md) -- PR guidelines, commit message format, comment tags
- [docs/developer/implementing-a-feature.md](/docs/developer/implementing-a-feature.md) -- step-by-step feature implementation guide
- [ISSUE_LIFECYCLE.md](/ISSUE_LIFECYCLE.md) -- issue triage and milestone assignment process
