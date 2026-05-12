# AI Context for NGINX Gateway Fabric

This folder contains structured context for AI assistants and human contributors onboarding to the project. Each file provides a concise summary of a specific topic, with references to the canonical documentation for full details.

**The canonical docs remain authoritative.** The files here are summaries intended to accelerate understanding. If any content here conflicts with the source docs, the source docs take precedence. Keep these files in sync when the canonical docs change.

## File Index

| File | Description |
|------|-------------|
| [overview.md](overview.md) | What NGINX Gateway Fabric is, the problem it solves, who it's for, and where it fits in the Kubernetes Gateway API ecosystem |
| [architecture.md](architecture.md) | Major components (control plane, data plane, controllers, NGINX integration), how they interact, and the reconciliation flow |
| [repo-structure.md](repo-structure.md) | Each significant top-level folder with a one-to-two sentence description |
| [development.md](development.md) | Prerequisites, how to build, how to run locally against a Kind cluster, and common Make targets |
| [testing.md](testing.md) | Test types (unit, conformance, functional, scale), how to run each, and what each verifies |
| [conventions.md](conventions.md) | Go style choices, error handling, logging, naming, file organization, and commit message format |
| [glossary.md](glossary.md) | Concise definitions of project-specific and Gateway API terminology |
| [workflows.md](workflows.md) | How to file issues, propose features, submit PRs, run conformance tests, and respond to review feedback |
| [dependencies.md](dependencies.md) | Major external dependencies, what each is used for, and version constraints |
| [review-patterns.md](review-patterns.md) | Common PR review feedback themes, implicit standards, and examples of well-received contributions |

## Keeping This Folder Current

When updating canonical docs (such as `ARCHITECTURE.md`, `CONTRIBUTING.md`, or files under `docs/`), check whether the corresponding summary here needs updating. These files are intentionally brief; avoid duplicating detailed content that belongs in the source docs.
