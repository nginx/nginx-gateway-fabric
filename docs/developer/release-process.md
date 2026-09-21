# Release Process

This document describes how NGINX Gateway Fabric (NGF) is released.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Versioning](#versioning)
- [Release Planning and Development](#release-planning-and-development)
- [How a Release Flows](#how-a-release-flows)
- [One-time Prerequisites](#one-time-prerequisites)
- [Major or Minor Release](#major-or-minor-release)
  - [Prepare the branches](#prepare-the-branches)
  - [Update the release files](#update-the-release-files)
  - [Build and test in the mirror](#build-and-test-in-the-mirror)
  - [Release day](#release-day)
  - [Operator bundle](#operator-bundle)
  - [After publishing](#after-publishing)
- [Patch Release](#patch-release)
- [Redo, Dry Runs and Manual Fallbacks](#redo-dry-runs-and-manual-fallbacks)
- [How the Two Stages Trust Each Other](#how-the-two-stages-trust-each-other)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Versioning

NGF uses semantic versioning for its releases. For more information, see https://semver.org.

> Major version zero `(0.Y.Z)` is reserved for development, anything MAY change at any time. The public API is not stable.

## Release Planning and Development

The features that will go into the next release are reflected in the
corresponding [milestone](https://github.com/nginx/nginx-gateway-fabric/milestones). Refer to
the [Issue Lifecycle](/ISSUE_LIFECYCLE.md) document for information on issue creation and assignment to releases.

## How a Release Flows

Two repositories take part:

| | Public repository | Internal mirror |
| --- | --- | --- |
| Name | `nginx/nginx-gateway-fabric` | Set by the `INTERNAL_REPOSITORY` variable; never written in this tree |
| Holds | `main`, `release-X.Y`, tags, GitHub releases | A synced copy of every public branch, plus `internal/release-X.Y` |
| Sync | -- | A workflow owned outside this repository force-pushes every public branch (and `.github/`) into the mirror every 15 minutes |
| Can reach | Public registries, GitHub releases | Staging registry, internal package hosts, blob storage |
| Credential direction | Holds nothing that can read the mirror | Holds `PUBLIC_REPO_TOKEN`, which can push the release branch and dispatch publish |

A release is built in the mirror and published from the public repository. Nothing is rebuilt in between:

1. **Cut** `internal/release-X.Y` in the mirror from the synced `release-X.Y`.
2. **Fix** on the internal branch. Public pull requests reach it through the inward cherry-pick; embargoed fixes are made there directly.
3. **Prep** (`Release Prep`, in the mirror) builds binaries and images from the internal branch, pushes the images to the staging registry, runs the functional, conformance and Helm suites against those exact images, and emits a **signed release manifest**: the commit and tree it built, the digest of every image, and the location and sha256 of every asset it staged.
4. **Promote** (`Promote Release Branch`, in the mirror) fetches that manifest, checks it was built from the commit about to ship, fast-forwards public `release-X.Y` to the internal branch, and dispatches publish.
5. **Publish** (`Release Publish`, in the public repository) verifies the manifest's signature and the tree, copies the images by digest, publishes the chart, attaches the staged assets, and creates the tag and GitHub release last.

The same workflow files exist in both repositories because the sync copies them. Every job is gated on the repository it belongs to, so dispatching prep in the public repository, or publish in the mirror, is a no-op.

> **Status.** The two stages have not yet been exercised end to end. `cosign` and `az` have only run as test stubs, and the prerequisites below are not all in place. Treat the first prep dry run as the test of this document.

## One-time Prerequisites

These are set once and reviewed rarely. A missing variable fails closed: prep refuses to start, and a gated job stays switched off.

**In the mirror**

| Kind | Name | Purpose |
| --- | --- | --- |
| Variable | `INTERNAL_REPOSITORY` | The mirror's own `owner/name`. Every internal-only job is gated on it. **Must never be defined in the public repository**, or those jobs would run there. |
| Variable | `STAGING_READ_REGISTRY`, `STAGING_WRITE_REGISTRY` | Staging registry hosts prep pushes to and the suites pull from |
| Variable | `STAGING_PKG_HOST` | Internal NGINX package host for the image builds |
| Variable | `INTERNAL_RUNNER` | Runner label with a route to the internal hosts (optional; defaults to a GitHub runner) |
| Secret | `PUBLIC_REPO_TOKEN` | Token with `contents:write` and `actions:write` on the public repository, used only by promote |
| Secret | `AZ_VAULT_CLIENT_ID`, `AZ_VAULT_TENANT_ID`, `AZ_VAULT_NAME`, `AZ_COMMON_VAULT_NAME` | Vault access, as in the public repository |
| Secret | `AZURE_UPLOAD_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` | Federated identity that can write to the release asset store |

**In the public repository**

| Kind | Name | Purpose |
| --- | --- | --- |
| Variable | `RELEASE_SIGNER_REPOSITORY` | The mirror's `owner/name`, used only to verify manifest signatures. Deliberately a different name from `INTERNAL_REPOSITORY`. |
| Secret | `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` | Federated identity that can read the release asset store |

**In the NGF vault:** `azure-storage-account` and `azure-storage-bucket`, naming the asset store.

**Runner permissions:** when a new `release-X.Y` branch is created, ask the infra team to add it to the runner permissions.

## Major or Minor Release

`X.Y.Z` is the version being released. Each step names the repository it happens in.

### Prepare the branches

1. **Public.** Create an issue titled `Release X.Y.Z` to track the release.
2. **Public.** Stop merging new work into `main`.
3. **Public.** Create the `release-X.Y` branch from `main`. Ask the infra team to add it to the runner permissions.
4. **Mirror.** Wait for the sync (up to 15 minutes), then run **Cut Internal Release Branch** with `release_branch: release-X.Y`. It refuses if the synced copy is not identical to the public tip, or if `internal/release-X.Y` already exists.

   From here until publish, **every commit that lands on public `release-X.Y` must also land on `internal/release-X.Y`**, or promote will refuse to fast-forward. Label such pull requests `needs cherry pick` and run the inward cherry-pick (see [Update the release files](#update-the-release-files)). Embargoed fixes go to the internal branch only.

### Update the release files

1. **Public.** Run the [Release PR](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/release-pr.yml) workflow on `release-X.Y`, then finish the pull request by hand:
   1. Update the [README](/README.md) with the release information.
   2. Update the [changelog](/CHANGELOG.md). The automation adds a blank section listing the pull requests labelled `release-notes`. Keep only changes that matter to users; the autogenerated full changelog is attached to the GitHub release later. Use the release notes written in each pull request where present. If the supported Gateway API minor version changed, say whether the previous version is still supported.
2. **Public.** Merge the Release PR with the `needs cherry pick` label.
3. **Mirror.** Run **Cherry-pick Inward** with `release_branch: release-X.Y`. It opens one pull request per labelled public pull request not yet on the internal branch, oldest first. Review and merge them in the mirror.

### Build and test in the mirror

1. **Mirror.** Run **Release Prep** with **"Use workflow from" set to `internal/release-X.Y`**. There is no branch input; the first step rejects any other ref. Inputs:

   | Input | Value |
   | --- | --- |
   | `release_version` | `vX.Y.Z` |
   | `operator_version` | the operator version, for example `v1.0.1` |
   | `dry_run` | `false` |
   | `skip_step` | leave empty. Skipping any build or test job means no manifest, and a prep without a manifest cannot be published. |

   Prep ends with a `release-manifest-vX.Y.Z` artifact only if every build and every suite passed. If anything failed, fix it on the internal branch and run prep again; nothing has been published.

2. **Long-running tests.** Longevity, NFR, IPv6 and OpenShift tests are not part of prep and still need a home in this process.

   > **Open.** These used to run against `release-X.Y-rc` images built from the public release branch. That build path was removed with the split, so those images no longer exist. The staged images prep pushed are the candidates, but they live in the staging registry and the suites' image inputs have not been wired to it. Decide this before the first real release.

### Release day

1. **Mirror.** Run **Promote Release Branch**:

   | Input | Value |
   | --- | --- |
   | `release_branch` | `release-X.Y` |
   | `release_version` | `vX.Y.Z`, the same as prep |
   | `dry_run` | `false` (defaults to `true`) |
   | `publish_dry_run` | `false` |
   | `prep_run_id` | leave empty; the newest prep for the version is used |
   | `dispatch_publish` | `true` |

   Promote refuses if the manifest was not built from the commit being promoted (re-run prep), if the public branch has commits the internal one lacks (cherry-pick inward and re-run prep), or if the push does not land. On success it fast-forwards public `release-X.Y` and dispatches **Release Publish** in the public repository.

2. **Public.** Watch the **Release Publish** run. In order, it verifies the manifest's signature and the merge-back tree, promotes the images by digest, publishes the Helm chart, submits the UBI images for RedHat preflight certification, fetches and verifies the staged binaries, SBOMs and signatures, and only then creates the tag on the verified commit, dispatches the operator bundle, and creates the GitHub release with the autogenerated changelog and closes the milestone. A tag is never created for a release whose images, chart or assets failed.

### Operator bundle

1. Before the operator ships, complete the [manual checks](../../operators/README.md#manual-items-to-check).
2. Publish dispatches the [Operator Bundle PR](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/operator-bundle-pr.yml) workflow itself when the manifest records an operator version, with `submit-to-redhat` taken from publish's `submit_operator_to_redhat` input (default `false`). The workflow generates the bundle manifests from image digests and opens a draft pull request in this repository; with `submit-to-redhat`, it also forks and opens a pull request against the [RedHat certified-operators repository](https://github.com/redhat-openshift-ecosystem/certified-operators).
3. Review and merge the bundle pull request. Monitor the RedHat pull request for feedback.

### After publishing

1. **Public.** Prepare and merge a pull request into `main` mirroring the release-branch changes: [README](/README.md), [changelog](/CHANGELOG.md), the Helm chart `version`, `GW_API_PREV_VERSION` in the tests Makefile if it changed, references to the previous release in the docs, and the installation instructions (in particular the Helm README) for the supported Gateway API and NGF versions.
2. Prepare and merge a pull request into `main` of the [documentation repository](https://github.com/nginx/documentation) from its release branch, for example `ngf-release-2.0`:
    - Run `make generate-api-docs` in this repository and copy `docs/api/content.md` to `content/ngf/reference/api.md` there.
    - Update `layouts/shortcodes/version-ngf.html` with the new version, and `layouts/shortcodes/version-inference-extension.html` if the supported Gateway API Inference Extension version changed (see `sigs.k8s.io/gateway-api-inference-extension` in `go.mod`). Do not add an empty line to either file.
    - Update the Technical Specifications table to match this repository's README.
    - Documentation builds and deploys automatically from `main`.
    - Create the branch for the next release, named `ngf-release-<major>.<minor>`.
3. Close the release issue and confirm the [milestone](https://github.com/nginx/nginx-gateway-fabric/milestones) is closed.
4. Verify that the published artifacts install properly.
5. Submit the release's `conformance-profile.yaml` artifact to the [Gateway API repository](https://github.com/kubernetes-sigs/gateway-api/tree/main/conformance/reports): fork it, name the file as its [README](https://github.com/kubernetes-sigs/gateway-api/blob/main/conformance/reports/README.md) requires, update the README in the `ngf` directory and the site source if needed, check the NGF `version`, and open a pull request ([example](https://github.com/kubernetes-sigs/gateway-api/pull/3149)). A first-time contributor signs the CLA with F5, Inc. as the organization.
6. Do the same with `conformance-profile-inference.yaml` in the [Inference Extension repository](https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/conformance/reports), and update any NGF version references in that repository's docs.

## Patch Release

A patch release carries a fix that cannot wait for the next major or minor release. The `release-X.Y` and `internal/release-X.Y` branches already exist.

1. **Public.** Create an issue titled `Release X.Y.Z`.
2. Land the fix:
   - **Not embargoed, applies to `main` too:** merge the fix into `main` with the `needs cherry pick` label. The public cherry-pick workflow lands it on `release-X.Y`; the inward cherry-pick (step 3) lands it on the internal branch.
   - **Not embargoed, release branch only:** merge the fix into `release-X.Y` with the `needs cherry pick` label so the inward cherry-pick can follow.
   - **Embargoed (a CVE fix):** merge the fix into `internal/release-X.Y` in the mirror only. It reaches the public branch when promote fast-forwards it on release day, and not before. Do not open a public pull request until then.
3. **Mirror.** Run **Cherry-pick Inward** with `release_branch: release-X.Y` and merge the pull requests it opens.
4. Follow [Update the release files](#update-the-release-files). If there is a new operator version, also update it in [operators/Makefile](/operators/Makefile) and [operators/Dockerfile](/operators/Dockerfile).
5. Follow [Build and test in the mirror](#build-and-test-in-the-mirror), [Release day](#release-day) and [Operator bundle](#operator-bundle).
6. Prepare and merge a pull request into `main` of the [documentation repository](https://github.com/nginx/documentation) updating `layouts/shortcodes/version-ngf.html`, and the API reference and Technical Specifications table if they changed (see [After publishing](#after-publishing)).
7. Close the issue and the milestone.

## Redo, Dry Runs and Manual Fallbacks

**Rehearse prep** with `dry_run: true`. Everything is built but nothing is pushed, so no manifest is produced; this checks the pipeline, not the release.

**Rehearse promote** by leaving `dry_run` at its default `true`. It reports what it would do and changes nothing. `publish_dry_run: true` promotes the branch but has publish only verify.

**Re-run publish by hand**, for a redo or to set `submit_operator_to_redhat`, by dispatching **Release Publish** in the public repository with `release_version`, `release_branch`, `dry_run`, and the `manifest` and `manifest_bundle` files from the prep run's `release-manifest-vX.Y.Z` artifact. An unsigned manifest is refused. The binaries are not an input: the manifest records where prep staged them and what they hash to, and publish fetches them itself.

**Regenerate the operator bundle** without a release by running the [Operator Bundle PR](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/operator-bundle-pr.yml) workflow on the release branch with `operator-version` (without the `v`) and `submit-to-redhat`.

**Prep built the wrong commit.** Promote refuses when the manifest's commit is not the one being promoted. There is no override: fix the internal branch and run prep again.

**Publish refuses the tree.** The public release branch differs from what prep built, usually a commit that landed on `release-X.Y` after the internal branch was cut and was not picked inward. Pick it inward, run prep again, then promote again.

## How the Two Stages Trust Each Other

Prep builds a commit that does not exist publicly. The internal branch carries fixes that are embargoed until release day, so its commits have different parents, authors and messages from anything the public repository will ever hold. Publish therefore cannot work out what the release is from its own tree, and everything it does rests on the manifest.

- **The manifest is signed in the mirror** with cosign keyless. The certificate names the mirror, `release-prep.yml`, and the `internal/release-X.Y` branch it ran on. Publish verifies all three before it reads a byte, so the same workflow on any other branch, or any other signer, is refused.
- **The tree hash is the invariant.** The merge-back changes every commit's identity but not its tree. Publish compares the public branch's tree with the one in the manifest and refuses on any difference, which is why the public release branch must carry nothing the internal branch lacks.
- **Images are promoted by digest**, copied from the staging registry with the manifest lists intact. A staging tag could be moved by a later prep run; a digest cannot. What a customer pulls is byte-for-byte what the suites tested.
- **Assets are pinned, not trusted.** Prep stages binaries, SBOMs and signatures in blob storage and records each one's path and sha256 in the manifest before signing it. Publish refuses any downloaded byte that does not match, verifies the checksums file's own signature against the same prep identity, and checks the archives against the checksums.
- **The tag is created last**, on the commit publish verified rather than on the branch head, and only after every artifact has published. The previous pipeline pushed the tag first and left it behind on failure.
- **No tag is created in the mirror.** The sync carries tags, so a tag created there would either never reach publish or appear publicly ahead of release day, which on a CVE release is itself a disclosure.
- **Provenance is built in `min` mode.** `max` embeds the full event payload, including commit messages and the branch name, which from the mirror would put withheld CVE descriptions on every customer-pullable image.
