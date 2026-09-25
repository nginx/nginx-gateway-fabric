# Release Process

This document describes how NGINX Gateway Fabric (NGF) is released.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Versioning](#versioning)
- [Release Planning and Development](#release-planning-and-development)
- [How a Release Flows](#how-a-release-flows)
- [Major or Minor Release](#major-or-minor-release)
  - [Prepare the branches](#prepare-the-branches)
  - [Prepare the release in the mirror](#prepare-the-release-in-the-mirror)
  - [Build and test in the mirror](#build-and-test-in-the-mirror)
  - [If a fix has to go in after the cut](#if-a-fix-has-to-go-in-after-the-cut)
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

1. **Cut** `internal/release-X.Y` in the mirror from the synced `release-X.Y`, at least four days before release day. Public `release-X.Y` receives no commits after this point.
2. **Prepare** on the internal branch: bump the pinned NGINX versions and run the release pull request. None of it is public yet. A fix that has to go in after the cut comes across from `main` with the inward cherry-pick.
3. **Prep** (`Release Prep`, in the mirror) builds binaries and images from the internal branch, pushes the images to the staging registry, runs the functional, conformance and Helm suites against those exact images, and emits a **signed release manifest**: the commit and tree it built, the digest of every image, and the location and sha256 of every asset it staged.
4. **Promote** (`Promote Release Branch`, in the mirror) fetches that manifest, checks it was built from the commit about to ship, fast-forwards public `release-X.Y` to the internal branch, and dispatches publish.
5. **Publish** (`Release Publish`, in the public repository) verifies the manifest's signature and the tree, copies the images by digest, publishes the chart, attaches the staged assets, and creates the tag and GitHub release last.

The same workflow files exist in both repositories because the sync copies them. Every job is gated on the repository it belongs to, so dispatching prep in the public repository, or publish in the mirror, is a no-op. For the full breakdown of which jobs run where, on which event, and which images they use — and for the repository variables and secrets each side must hold — see [Workflow Matrix](/docs/developer/workflow-matrix.md).

## Major or Minor Release

`X.Y.Z` is the version being released. Each step names the repository it happens in.

### Prepare the branches

1. **Public.** Create an issue titled `Release X.Y.Z` to track the release.
2. **Public.** Create the `release-X.Y` branch from `main`. Ask the infra team to add it to the runner permissions. `main` is not frozen; work carries on there throughout the release.
3. **Mirror.** Wait for the sync (up to 15 minutes), then run **Cut Internal Release Branch** with `release_branch: release-X.Y`. It refuses if the synced copy is not identical to the public tip, or if `internal/release-X.Y` already exists.

   **Cut at least four days before release day.** The long-running tests run against the images prep stages, and longevity alone takes three days.

   **Public `release-X.Y` receives no commits from here until release day.** Promote fast-forwards it from the internal branch, which is only possible while the public branch holds nothing the internal one lacks. Every change below is made on `internal/release-X.Y`.

### Prepare the release in the mirror

All of this happens in the mirror, on `internal/release-X.Y`. Nothing here is visible publicly until promote fast-forwards the branch on release day.

The release ships what was on `main` when the branch was cut. If that turns out not to be everything — testing finds a bug, or something urgent lands — see [If a fix has to go in after the cut](#if-a-fix-has-to-go-in-after-the-cut).

1. **Bump the pinned NGINX versions** if the release moves them, using `.github/scripts/bump-nginx-versions.sh`. Read its `--help` first: the OSS package version, the NGINX source version the Plus images build against, and the WAF module version are separate flags, and one WAF flag writes both the apk and rpm spellings.
2. **Run the Release PR workflow**, dispatched in the mirror. Leave `base-branch` empty: it derives `internal/release-X.Y` from the version, which is where the release is prepared. Then finish the pull request:
   1. Update the [README](/README.md) with the release information.
   2. Update the [changelog](/CHANGELOG.md). The automation adds a blank section listing the pull requests labelled `release-notes`. Keep only changes that matter to users; the autogenerated full changelog is attached to the GitHub release later. Use the release notes written in each pull request where present. If the supported Gateway API minor version changed, say whether the previous version is still supported.
3. **Merge the Release PR** in the mirror.
4. **If the release ships a new operator version**, complete the operator's [manual checks](../../operators/README.md#manual-items-to-check) now. Prep takes `operator_version` as an input and the bundle is generated from what it builds, so these have to be settled before prep runs, not when the bundle is opened.

### Build and test in the mirror

1. **Mirror.** Run **Release Prep** with **"Use workflow from" set to `internal/release-X.Y`**. There is no branch input; the first step rejects any other ref. Inputs:

   | Input | Value |
   | --- | --- |
   | `release_version` | `vX.Y.Z` |
   | `operator_version` | the operator version, for example `v1.0.1` |
   | `dry_run` | `false` |

   Prep ends with a `release-manifest-vX.Y.Z` artifact only if every build and every suite passed. If anything failed, fix it on the internal branch and run prep again; nothing has been published.

2. **Mirror. Long-running tests, against the staged images.** These are not part of prep. Run them from the mirror once prep has staged the images, so they exercise the artifact that will ship rather than a rebuild of it. Each takes `image_source: registry` and `image_registry` set to the NGF repository prep pushed to; the data plane repositories are derived from it. Use `X.Y.Z` for `version` and prep's tag for `image_tag`. The cluster pulls with the registry JWT, which the workflow fetches from the vault for any registry source.

   Dispatch each of these from the **mirror's** Actions tab. They are not linked here on purpose: following a link to this repository's Actions tab and running it there tests the public images, not the ones being released.

   - **Longevity:** `Start Longevity Tests`, then `Stop Longevity Tests` three days later to collect results and tear down.
   - **NFR:** `Non Functional Testing`.
   - **GatewayLink:** `GatewayLink Integration Testing`. It stands up a BIG-IP alongside the cluster, so run it once per release rather than per attempt.
   - **IPv6:** from the `tests` directory, `IMAGE_SOURCE=registry RELEASE_REPO=<the repository prep pushed to> make ipv6-tests TAG=<prep's tag>`. It creates the pull secret itself from the JWT.
   - **OpenShift:** the [OpenShift tests](/tests/OPENSHIFT_CONFORMANCE.md).

   Longevity's three days are what set the four-day lead time on cutting the branch.

   **Results belong to `main`, not to the release.** The job that opens a results pull request runs only in the public repository, so a mirror run does not open one. Take the results from the run's artifacts and open that pull request against public `main` afterwards. They are test artifacts, not release content, so they do not go on the release branch.

   These all still run publicly against public images when dispatched there, and the NFR and GatewayLink schedules remain public-only.

### If a fix has to go in after the cut

The normal case is that everything the release ships is already on `main` before the branch is cut. This is the exception: a bug the testing above found, or something urgent that cannot wait for the next release. For a patch release it is the usual path rather than the exception.

Public `release-X.Y` still takes no direct commits. The fix reaches it only when promote fast-forwards the branch.

1. **Land it on `main` first**, with the `needs cherry pick` label, unless it has to go straight to the release branch — in which case commit it on `internal/release-X.Y` in the mirror and skip the next step. A fix that goes straight to the internal branch is not on `main`, so merge it back after the release; [After publishing](#after-publishing) covers that.
2. **Mirror.** Run **Cherry-pick Inward** with `release_branch: release-X.Y`. It opens one pull request for each public pull request carrying the label that is not yet on the internal branch, oldest merge first. Review and merge them in the mirror, and run it again whenever another labelled pull request merges.

**Then run prep again.** Anything that changes the internal branch after prep invalidates the images it staged: the manifest records the commit and tree it built, so promote refuses a manifest that no longer matches. Prep re-runs the functional, conformance and Helm suites itself, so those cost only the prep run.

The long-running suites are the awkward part. Longevity takes three days, so unless the fix lands within a day or so of the cut there is no room to repeat it before release day. That leaves a choice to make deliberately rather than by default: hold the release, or ship knowing longevity exercised a build that differs by this fix. Re-run NFR and GatewayLink, which fit in a day, and say in the release issue which suites were repeated and which were not.

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

   Promote refuses if the manifest was not built from the commit being promoted (re-run prep), if the public branch has commits the internal one lacks (see [Promote refuses to fast-forward](#redo-dry-runs-and-manual-fallbacks) below -- merge, do not cherry-pick), or if the push does not land. On success it fast-forwards public `release-X.Y` and dispatches **Release Publish** in the public repository.

2. **Public.** Watch the **Release Publish** run. In order, it verifies the manifest's signature and the merge-back tree, promotes the images by digest, publishes the Helm chart, submits the UBI images for RedHat preflight certification, fetches and verifies the staged binaries, SBOMs and signatures, and only then creates the tag on the verified commit, dispatches the operator bundle, and creates the GitHub release with the autogenerated changelog and closes the milestone. A tag is never created for a release whose images, chart or assets failed.

### Operator bundle

The operator's [manual checks](../../operators/README.md#manual-items-to-check) belong earlier, before prep runs; see [Prepare the release in the mirror](#prepare-the-release-in-the-mirror).

1. Publish dispatches the [Operator Bundle PR](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/operator-bundle-pr.yml) workflow itself when the manifest records an operator version, with `submit-to-redhat` taken from publish's `submit_operator_to_redhat` input (default `false`). The workflow generates the bundle manifests from image digests and opens a draft pull request in this repository; with `submit-to-redhat`, it also forks and opens a pull request against the [RedHat certified-operators repository](https://github.com/redhat-openshift-ecosystem/certified-operators).
2. Review and merge the bundle pull request. Monitor the RedHat pull request for feedback.

### After publishing

1. **Public.** Prepare and merge a pull request into `main` mirroring the release-branch changes: [README](/README.md), [changelog](/CHANGELOG.md), the Helm chart `version`, `GW_API_PREV_VERSION` in the tests Makefile if it changed, references to the previous release in the docs, and the installation instructions (in particular the Helm README) for the supported Gateway API and NGF versions.
2. **Public.** Merge back to `main` any fix that went straight to `internal/release-X.Y` rather than arriving through `main`. Anything that skipped `main` on the way in is missing from it until this lands, and the next release would regress it.
3. Prepare and merge a pull request into `main` of the [documentation repository](https://github.com/nginx/documentation) from its release branch, `ngf-release-next`:
    - Run `make generate-api-docs` in this repository and copy `docs/api/content.md` to `content/ngf/reference/api.md` there.
    - Update the version shortcodes under `layouts/shortcodes/` that this release moves. `version-ngf.html` always changes; the others follow their own dependency, so check each against what shipped — for example `version-inference-extension.html` against `sigs.k8s.io/gateway-api-inference-extension` in `go.mod`. Do not add an empty line to any of them.
    - Update the Technical Specifications table to match this repository's README.
    - Documentation builds and deploys automatically from `main`.
    - Create the branch for the next release, named `ngf-release-next`.
4. Close the release issue and confirm the [milestone](https://github.com/nginx/nginx-gateway-fabric/milestones) is closed.
5. Verify that the published artifacts install properly.
6. Submit the release's `conformance-profile.yaml` artifact to the [Gateway API repository](https://github.com/kubernetes-sigs/gateway-api/tree/main/conformance/reports): fork it, name the file as its [README](https://github.com/kubernetes-sigs/gateway-api/blob/main/conformance/reports/README.md) requires, update the README in the `ngf` directory and the site source if needed, check the NGF `version`, and open a pull request ([example](https://github.com/kubernetes-sigs/gateway-api/pull/3149)). A first-time contributor signs the CLA with F5, Inc. as the organization.
7. Do the same with `conformance-profile-inference.yaml` in the [Inference Extension repository](https://github.com/kubernetes-sigs/gateway-api-inference-extension/tree/main/conformance/reports), and update any NGF version references in that repository's docs.

## Patch Release

A patch release carries a fix that cannot wait for the next major or minor release. The `release-X.Y` and `internal/release-X.Y` branches already exist.

1. **Public.** Create an issue titled `Release X.Y.Z`.
2. Land the fix by following [If a fix has to go in after the cut](#if-a-fix-has-to-go-in-after-the-cut). For a patch that is the normal path, not the exception: the branches already exist, so the fix always arrives after the cut.
3. Follow [Prepare the release in the mirror](#prepare-the-release-in-the-mirror). If there is a new operator version, also update it in [operators/Makefile](/operators/Makefile) and [operators/Dockerfile](/operators/Dockerfile).
4. Follow [Build and test in the mirror](#build-and-test-in-the-mirror), [Release day](#release-day) and [Operator bundle](#operator-bundle).
5. Prepare and merge a pull request into `main` of the [documentation repository](https://github.com/nginx/documentation) updating `layouts/shortcodes/version-ngf.html`, and the API reference and Technical Specifications table if they changed (see [After publishing](#after-publishing)).
6. Close the issue and the milestone.

## Redo, Dry Runs and Manual Fallbacks

**Rehearse prep** with `dry_run: true`. Everything is built but nothing is pushed, so no manifest is produced; this checks the pipeline, not the release.

**Rehearse promote** by leaving `dry_run` at its default `true`. It reports what it would do and changes nothing. `publish_dry_run: true` promotes the branch but has publish only verify.

**Re-run publish by hand**, for a redo or to set `submit_operator_to_redhat`, by dispatching **Release Publish** in the public repository with `release_version`, `release_branch`, `dry_run`, and the `manifest` and `manifest_bundle` files from the prep run's `release-manifest-vX.Y.Z` artifact. An unsigned manifest is refused. The binaries are not an input: the manifest records where prep staged them and what they hash to, and publish fetches them itself.

**Regenerate the operator bundle** without a release by running the [Operator Bundle PR](https://github.com/nginx/nginx-gateway-fabric/actions/workflows/operator-bundle-pr.yml) workflow on the release branch with `operator-version` (without the `v`) and `submit-to-redhat`.

**Prep built the wrong commit.** Promote refuses when the manifest's commit is not the one being promoted. There is no override: fix the internal branch and run prep again.

**Promote refuses to fast-forward.** Something landed on public `release-X.Y` after the internal branch was cut, so the two have diverged. Cherry-picking it inward does not fix this: that leaves two different commits for one change and the branches still diverge. Merge public `release-X.Y` into `internal/release-X.Y`, run prep again, then promote.

**Publish refuses the tree.** The public branch's tree is not the one prep built, so the merge-back is not what was tested. Run prep again from the current internal branch and promote from that run.

## How the Two Stages Trust Each Other

Prep builds a commit that does not exist publicly. The internal branch carries changes that have not been merged back yet, so its commits have different parents, authors and messages from anything the public repository will hold. Publish therefore cannot work out what the release is from its own tree, and everything it does rests on the manifest.

- **The manifest is signed in the mirror** with cosign keyless. The certificate names the mirror, `release-prep.yml`, and the `internal/release-X.Y` branch it ran on. Publish verifies all three before it reads a byte, so the same workflow on any other branch, or any other signer, is refused.
- **The tree hash is the invariant.** The merge-back changes every commit's identity but not its tree. Publish compares the public branch's tree with the one in the manifest and refuses on any difference, which is why the public release branch must carry nothing the internal branch lacks.
- **The verifier does not come from the branch being verified.** `verify-merge-back` checks out the workflow's own ref and resolves the release branch as a git ref only, so a dispatch naming some other branch cannot supply the script that decides whether to trust it.
- **Images are promoted by digest**, copied from the staging registry with the manifest lists intact. A staging tag could be moved by a later prep run; a digest cannot. What a customer pulls is byte-for-byte what the suites tested.
- **Assets are pinned, not trusted.** Prep stages binaries, SBOMs and signatures in blob storage and records each one's path and sha256 in the manifest before signing it. Publish refuses any downloaded byte that does not match, verifies the checksums file's own signature against the same prep identity, and checks the archives against the checksums.
- **The tag is created last**, on the commit publish verified rather than on the branch head, and only after every artifact has published. A release that fails part way leaves no tag to clean up.
- **No tag is created in the mirror.** The sync carries tags, so a tag created there would either never reach publish or appear publicly before the release has finished.
- **Provenance is built in `min` mode.** `max` embeds the full event payload, including commit messages and the branch name, which would put the internal branch's history on every customer-pullable image.
