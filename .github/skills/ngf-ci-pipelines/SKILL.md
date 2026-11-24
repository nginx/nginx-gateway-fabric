---
name: ngf-ci-pipelines
description: 'CI/CD workflows, the public repo / internal mirror split and job gating, release flow, and the Docker image build for NGF. Use when working on workflows, release scripts, Dockerfiles, image builds, build matrices, or debugging CI failures.'
---

# NGF CI/CD and Image Builds

Authoritative docs: `docs/developer/workflow-matrix.md`, `docs/developer/release-process.md`, `docs/developer/branching-and-workflow.md`. Where they disagree with the workflows or scripts, trust the code (known mismatches: `ci.yml` has no `workflow_dispatch`; fork PRs skip Plus build jobs; the `release-prep.yml` concurrency comment is out of date).

## Repository Split -- read this first

Workflow files live in the public repo `nginx/nginx-gateway-fabric` and are force-synced every 15 minutes (including `.github/`) to an internal mirror whose name is only in the variable `vars.INTERNAL_REPOSITORY`. Every workflow exists in both; only each job's `if:` decides where it runs.

| Repo | Owns | Gate string |
| --- | --- | --- |
| Public | CI, publishing: copy images by digest, chart, RedHat certification, assets, tag, operator bundle, GitHub release | `github.repository == 'nginx/nginx-gateway-fabric'` |
| Mirror | Release branch cutting, release PR, release image builds, staging, tests against staged images, signed manifest, promotion, cherry-pick inward | `github.repository == vars.INTERNAL_REPOSITORY` |

### Gating rule

A job's `if:` starts with one of the two gate strings, then any extra conditions inside **one** `( ... )` group, e.g. `github.repository == 'nginx/nginx-gateway-fabric' && (always())`. A top-level `||` invalidates the gate; `github.repository_owner` is rejected (same in both repos). A job with no gate must be listed in `.github/config/job-gating-runs-in-mirror.txt` and must publish nothing. Workflows triggered only by `workflow_call` are exempt from the job check.

| Script / target | Checks |
| --- | --- |
| `.github/scripts/validate-job-gating.sh` (`make lint-job-gating`) | Every job in directly triggered workflows is gated; baseline file `job-gating-baseline.txt` |
| `.github/scripts/validate-workflow-gating.sh` (`make lint-workflow-gating`) | Every publishing step (registry login, push, `gh release`, `helm push`, skopeo, cosign, `git push`, goreleaser publish, ...) is gated at job, step or `enable=` level; exceptions in `workflow-gating-allowlist.txt` (reason above each line), `workflow-gating-baseline.txt` must only shrink |

Both run in the `gating` job of `lint.yml` along with their `*_test.sh`. Run them after editing any `if:`.

## Workflows

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `ci.yml` | push to main, PRs, nightly cron | Orchestrator (below) |
| `lint.yml` | push to main, PRs | Go, NJS, Rust, actionlint, release-script tests, gating, markdown, chart, yaml |
| `build.yml`, `functional.yml`, `conformance.yml`, `helm.yml`, `publish-helm.yml`, `redhat-certification.yml` | `workflow_call` | Reusable pieces |
| `codeql-analysis.yml`, `dependency-review.yml`, `scorecards.yml`, `f5-cla.yml`, `labeler.yml`, `stale.yml` | various | Security, CLA, labels, stale issues |
| `renovate-build.yml` | PR by `renovate[bot]` touching go.mod/go.sum/Makefile | Runs `make generate-all`, `go mod tidy` in `tests/`, auto-commits |
| `update-docker-images.yml` | cron, dispatch | Rebuild latest release images when a base image changes |
| `nfr.yml`, `longevity-*.yml`, `gatewaylink.yml`, `agent-branch-release.yml` | dispatch / cron | Long-running or Agent-branch test suites |
| `cut-internal-release-branch.yml`, `release-pr.yml`, `release-prep.yml`, `promote-release-branch.yml`, `cherry-pick-inward.yml`, `internal-pr.yml` | dispatch / PR | Mirror release flow |
| `release-publish.yml`, `operator-bundle-pr.yml`, `nkp-catalog-pr.yml` | dispatch | Public publishing |

`ci.yml` graph: `vars` (go mod tidy diff, `make generate-all && git diff --exit-code`, operator RBAC check) -> `unit-tests`, `njs-unit-tests`, `rust-unit-tests`, `cel-tests`; `binary` and `rust-modules` -> `build-oss`, `build-plus`, `build-plus-nap-waf`, `build-operator` -> `functional-tests`, `conformance-tests` -> `test-completion` (required check "Functional/Conformance Tests Pass"); `helm-tests` -> `publish-helm` (main only). PR builds are amd64, Alpine, latest Kubernetes only; main and nightly add arm64, UBI and the minimum Kubernetes version. Matrices are inline in `ci.yml`.

## Release Flow

1. Public: create `release-X.Y` from main.
2. Mirror: **Cut Internal Release Branch** creates `internal/release-X.Y` (at least 4 days before release; public `release-X.Y` then takes no commits).
3. Mirror: `make bump-nginx-versions` (`.github/scripts/bump-nginx-versions.sh`), then **Release PR**.
4. Mirror: **Release Prep**, only from `internal/release-X.Y`: build, push to staging, run functional, conformance and Helm tests against the staged images, emit a cosign-signed manifest.
5. Mirror: **Promote Release Branch** (`check`, `promote-and-verify`, `release`) fast-forwards public `release-X.Y` and dispatches publish with `manifest_sha256`.
6. Public: **Release Publish** (`verify`, `publish`): verify signature and commit, copy images by digest, publish chart and assets, tag last, then operator bundle and GitHub release.

Fixes after the cut: merge to main with the label `needs cherry pick`, then run **Cherry-pick Inward** in the mirror (uses `nginx-bot-pat`, because `GITHUB_TOKEN` PRs start no workflows). If public `release-X.Y` diverges, merge it into the internal branch; do not cherry-pick.

Versions: `Makefile` `VERSION`, `charts/nginx-gateway-fabric/Chart.yaml`, `operators/Makefile`; `release-pr.yml` rewrites them with the README and CHANGELOG. Do not hard-code versions in docs or skills.

## Patterns

- **Secrets:** Azure Key Vault via `nginx/ci-self-hosted/.github/actions/get-from-vault@<sha>` with OIDC (`id-token: write`). Hosts live in the vault so logs mask them. Do not add repository secrets (the exception is `CODECOV_TOKEN`).
- **Pinning:** actions as `@<40-char SHA> # vX.Y.Z`.
- **Renovate:** `renovate.json` extends `nginx/k8s-common`; inline `# renovate: datasource=... depName=...` comments in the Makefile, Dockerfiles and `ci.yml` mark pins. Do not bump them by hand.
- **Concurrency:** CI groups `${{ github.ref_name }}-<suffix>` with cancel-in-progress; `release-prep.yml` does not cancel.
- **Forks:** no vault access; steps are gated on `!github.event.pull_request.head.repo.fork`; no PR pushes to a real registry (`enable=` guards on metadata-action targets); Plus builds on a fork only succeed on a BuildKit cache hit.
- **Provenance:** use `provenance: mode=min`; `max` leaks internal branch names.
- A job whose `if` uses `always()` or `failure()` runs even when a dependency failed; assert dependencies explicitly.
- `contains()` is a substring match, not a token match.

## Image Builds

| Image | Alpine | UBI |
| --- | --- | --- |
| NGF control plane | `build/Dockerfile` (targets `container`, `local`, `goreleaser`; final stage `scratch`) | `build/ubi/Dockerfile` |
| NGINX OSS | `build/Dockerfile.nginx` | `build/ubi/Dockerfile.nginx` |
| NGINX Plus (and NAP WAF with `INCLUDE_NAP_WAF=true`) | `build/Dockerfile.nginxplus` | `build/ubi/Dockerfile.nginxplus` |

Also: `build/Dockerfile.rustmodules` (musl and glibc builds of the Rust modules), `build/entrypoint.sh`, `debug/Dockerfile` (dlv), `operators/Dockerfile`.

- Targets: `build-ngf-image`, `build-nginx-image` (builds Rust OSS modules first), `build-nginx-plus-image`, `build-nginx-plus-image-with-nap-waf` (amd64 only), `build-nginx-plus-image-with-nap-waf-dev` (needs `NAP_WAF_REPO_URL`), `build-prod-*`, `debug-build-images`. `BUILD_OS=ubi` selects `build/ubi/`. `GOARCH` defaults to `amd64`.
- **Plus credentials** are BuildKit `--secret` mounts (`NGINX_DOCKER_BUILD_PLUS_ARGS` in the `Makefile`, `secret-envs` in `build.yml`), never `COPY`. An optional `nginx-pkg-hosts` secret overrides package hosts so they stay out of image history.
- Runtime user is `USER 101:1001`. NAP WAF is amd64 only. GoReleaser (`.goreleaser.yml`) builds the `gateway` binary for amd64 and arm64 plus SBOMs and a cosign checksum signature.
- NGINX, Plus, Agent and WAF versions are pinned in the Dockerfiles; read them with `.github/scripts/bump-nginx-versions.sh --show` and change them with `make bump-nginx-versions ARGS=...`.
- UBI bases are digest-pinned; Alpine and Go bases are tag-pinned.

## Gotchas

- Release images are never built in the public repo; never add a build step to a publish-stage workflow.
- A wrong gate string skips the job forever with no error.
- Never put credentials in a layer, build arg or log.
