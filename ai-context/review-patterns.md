# Review Patterns

This document describes patterns observed in recent NGINX Gateway Fabric pull request reviews. It is based on analysis of the last 100 merged PRs, their review comments, discussion threads, and CI outcomes. The goal is to help future contributors (human or AI) anticipate feedback before submitting.

The tone here is descriptive, not prescriptive. These are observed patterns, not additional rules.

## Common Feedback Themes

Reviewers most frequently ask contributors to address the following:

### Unnecessary Code and Dead Checks

Reviewers consistently flag redundant nil checks, unnecessary conditionals, and duplicate validation. A common pattern is performing a check in a function and then re-checking the same condition in the caller (or vice versa).

### Code Clarity and Simplification

Reviewers push for the simplest possible implementation. Complex branching, inconsistent patterns within the same function, and workarounds are called out.

### Test Coverage

New code paths and branches are expected to have corresponding test coverage. Bug fixes specifically should include a test that reproduces the bug before the fix.

### Correctness and Edge Cases

Reviewers examine boundary conditions, error handling paths, and semantic correctness of API decisions.

### Kubernetes API and NGINX Semantics

Reviewers expect contributors to understand Kubernetes defaults, NGINX behavior, and Gateway API semantics.

## Implicit Standards

These patterns are not in CONTRIBUTING.md but show up consistently in reviews:

### Copilot Bot Reviews Exist but Don't Gate Merge

GitHub Copilot generates automated review comments on PRs. These are informational -- human reviewers (maintainers) make all merge decisions. The bot occasionally raises valid concerns, but its suggestions are evaluated case-by-case by the team.

### PR Titles Are Release Notes

PR titles use customer-facing language because they appear directly in generated release notes. Implementation-detail titles are revised during review. Examples from merged PRs:
- "Add additional configuration options to `nginxGateway.readinessProbe`"
- "Fix CORS preflight bypass when used with URLRewrite"
- "Support for Gateway FrontendTLS"

### Squash-and-Merge Is the Norm

The main branch history shows one squashed commit per PR, formatted as `<title> (#NNNN)`. During review, contributors commit changes incrementally so reviewers can follow progress. Final squashing happens at merge time.

### Renovate Bot Handles Dependency Updates

Approximately 40-50% of recent merged PRs are automated Renovate dependency bumps. These follow a standard pattern ("Update X to vY"), receive 1-2 quick approvals, and merge with minimal discussion.

### Feature Branches for Large Work

Large features (e.g., WAF, ListenerSets, FrontendTLS) develop on long-lived feature branches (`feat/nap-waf`, `feat/support-listener-sets`) and merge individual PRs to main as they complete.

## PR Structure Expectations

### Size Distribution

Based on the last 100 merged PRs:

| Category | Lines Changed | Typical Commits | Review Rounds | Examples |
|----------|--------------|-----------------|---------------|----------|
| Small fix/config | 1-50 | 1-2 | 1 (2 approvals) | #5174, #5122 |
| Medium feature/fix | 50-500 | 3-10 | 1-2 | #5200, #5194, #5095 |
| Large feature | 500-4500+ | 10-55 during review | 3-10+ | #5127, #5138, #5091 |
| Dependency bumps | 1-140 | 1 | 1 (auto-approve) | #5197, #5191 |

### Description Quality

Well-received PRs fill in the template completely: Problem, Solution, Testing, and a linked issue (`Closes #NNN`). Reviewers expect enough context to understand the change without reading every line of code first.

## Required Checks and Gates

| Gate | Description |
|------|-------------|
| CI build | Go binary and Docker images must build |
| Unit tests | `make unit-test` must pass |
| Linting | golangci-lint, markdownlint, yamllint must pass |
| Conformance tests | Gateway API conformance suite runs in CI |
| Functional tests | NGF-specific functional tests run in CI |
| Helm lint | Chart validation must pass |
| F5 CLA | External contributors must sign the F5 Contributor License Agreement (bot-enforced) |
| Approvals | Minimum 2 approvals from maintainers |

## Examples of Well-Received PRs

These PRs went through cleanly with minimal revision:

1. **[#5174](https://github.com/nginx/nginx-gateway-fabric/pull/5174) -- Set the default SSL verify depth to 4**
   Small, focused change (+10/-0). Single commit, 2 quick approvals, no revision needed. Clear problem statement.

2. **[#5122](https://github.com/nginx/nginx-gateway-fabric/pull/5122) -- Escape $ in rewrites**
   Targeted bugfix (+24/-2). 2 commits, 2 approvals. Fixed a specific escaping issue with a test.

3. **[#5095](https://github.com/nginx/nginx-gateway-fabric/pull/5095) -- Simplify HTTP response assertions in tests using Gomega matchers**
   Clean refactor (+19/-56). Reduced test code while improving readability. 2 commits, 3 reviews.

4. **[#5194](https://github.com/nginx/nginx-gateway-fabric/pull/5194) -- Fix CORS preflight bypass when used with URLRewrite**
   Well-scoped bugfix (+41/-4). Included test coverage. 3 commits, 2 approvals.

5. **[#5200](https://github.com/nginx/nginx-gateway-fabric/pull/5200) -- Add additional configuration options to `nginxGateway.readinessProbe`**
   Medium feature (+151/-16). Conversation about correct Kubernetes defaults was resolved constructively. 6 commits, 2 approvals (plus discussion).

What these share: focused scope, clear description, tests included, and an understanding of the project's conventions.

## Takeaways for Contributors

Based on these patterns, contributors can reduce revision cycles by:

- Eliminating redundant checks before submitting -- if a caller validates input, the callee does not need to re-validate
- Using consistent conditional patterns within a function (don't mix `> 0` and `== 0` / `continue` and no-continue for the same logic)
- Including tests for every new branch and edge case, especially error paths
- Running `make dev-all` locally before pushing
- Providing clear justification for NGINX template changes and CRD API decisions
- Keeping PRs as focused as possible -- smaller PRs merge faster

## Key References

- [docs/developer/pull-request.md](/docs/developer/pull-request.md) -- PR guidelines (submitter and reviewer)
- [.github/PULL_REQUEST_TEMPLATE.md](/.github/PULL_REQUEST_TEMPLATE.md) -- PR template
- [CONTRIBUTING.md](/CONTRIBUTING.md) -- contribution guidelines
