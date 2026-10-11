# Workflow Matrix

Which jobs run in which repository, on which event, and where their images come from.

Read this when you need to answer "will this run in the mirror?", "does a pull request
publish anything?", or "which images did that test actually use?". For how to carry out a
release, see the [release process](/docs/developer/release-process.md) documentation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [The rule that decides everything](#the-rule-that-decides-everything)
- [Everyday CI](#everyday-ci)
  - [What each event builds and publishes](#what-each-event-builds-and-publishes)
  - [What is actually fork-specific](#what-is-actually-fork-specific)
  - [Why a fork can build Plus images without the certificate](#why-a-fork-can-build-plus-images-without-the-certificate)
  - [Two different fork cases](#two-different-fork-cases)
- [The release path](#the-release-path)
- [Scheduled work](#scheduled-work)
- [Pull request housekeeping](#pull-request-housekeeping)
- [Where images come from, in one place](#where-images-come-from-in-one-place)
  - [What a published release does and does not reveal](#what-a-published-release-does-and-does-not-reveal)
  - [Why prep pushes to one staging endpoint and reads from another](#why-prep-pushes-to-one-staging-endpoint-and-reads-from-another)
  - [What Artifact Registry is for](#what-artifact-registry-is-for)
- [Repository variables and secrets](#repository-variables-and-secrets)
- [The operator bundle, and the one thing it can break](#the-operator-bundle-and-the-one-thing-it-can-break)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## The rule that decides everything

Every workflow file exists in both repositories: the sync force-pushes `.github/` into the
mirror, so a workflow cannot be kept out by not copying it. What a job does is decided
entirely by its `if:`, and there are three permitted shapes.

| Gate | Meaning |
| --- | --- |
| `github.repository == 'nginx/nginx-gateway-fabric'` | Public only |
| `github.repository == vars.INTERNAL_REPOSITORY` | Mirror only |
| No repository gate | Runs in both, and must publish nothing |

`.github/scripts/validate-job-gating.sh` fails the build unless every directly-triggered
job carries one of these. The ungated ones are listed by name, with a reason, in
`.github/config/job-gating-runs-in-mirror.txt`; `validate-workflow-gating.sh` separately
checks that no publishing step slips through. Reusable (`workflow_call`-only) workflows are
exempt because they run under their caller's gate.

## Everyday CI

`ci.yml` triggers on push to `main`, pull requests to any branch, a nightly run at 03:00
UTC, and a manual dispatch that takes no inputs. There is no longer a push trigger for
`release-*`: release builds happen in the mirror.

| Job | Public | Mirror |
| --- | --- | --- |
| `vars`, `unit-tests`, `njs-unit-tests`, `rust-unit-tests`, `cel-tests` | Yes | **Yes** |
| `binary`, `build-oss`, `build-plus`, `build-plus-nap-waf`, `build-operator` | Yes | No |
| `functional-tests`, `conformance-tests`, `helm-tests`, `test-completion` | Yes | No |
| `publish-helm` | Yes, on `main` only | No |

The mirror therefore runs the cheap checks and nothing else, which is what catches a broken
merge inward. **No image is ever built in the mirror by `ci.yml`** -- the mirror's only
builds come from release prep.

All nine `lint.yml` jobs are ungated and run in both repositories. None can reach a
registry.

### What each event builds and publishes

| Event | Builds | Pushes to | Tests pull from |
| --- | --- | --- | --- |
| Pull request | Every image, `linux/amd64` only, Alpine only | `localhost:5000` only, an ephemeral service container | `kind load` of the local build |
| Pull request from a fork | The same set, but without registry or package credentials | `localhost:5000` only | `kind load`; **all Plus suites skipped** |
| Push to `main` | Every image, both architectures, Alpine and UBI | `ghcr.io/nginx/nginx-gateway-fabric{,/nginx,/operator}`, `docker-mgmt.nginx.com` for Plus, and the GAR test mirrors, tagged `edge` | `kind load` of the local build |
| Nightly | The same as `main` | The same repositories, `type=schedule` tag | `kind load` of the local build |

Note that the narrower pull request build is set by the matrices in `ci.yml`, which select
`platforms: ["linux/amd64"]` and `build-os: [""]` for `pull_request`. It applies to every
pull request, not only to forks.

Every registry entry in the `metadata-action` images block carries
`github.event_name != 'pull_request'`, so **no pull request can push to a real registry**,
whoever opened it. Only the `localhost:5000` entry is unconditional, and that registry is a
service container that dies with the job.

### What is actually fork-specific

Only two things, both inherited from `main` and unchanged by the release split.

- **No credentials.** Both vault steps in `build.yml` are gated on
  `!github.event.pull_request.head.repo.fork`, so a fork build has no Docker Hub login and
  no `nginx-repo.crt` or `nginx-repo.key`. The Plus build jobs still run -- see below for
  why they usually succeed anyway.
- **No Plus testing.** `functional.yml`, `conformance.yml` and `helm.yml` each skip their
  Plus legs for a fork, because there is no image to test and no secret to pull it with.

`build.yml` also carries a guard skipping the UBI Plus build for forks. It cannot fire from
`ci.yml`, because no pull request matrix ever passes `build-os: ubi` in the first place.

### Why a fork can build Plus images without the certificate

This surprises people, so it is worth writing down. The certificate is consumed at
`build/Dockerfile.nginxplus:58` as `--mount=type=secret`, and BuildKit deliberately keeps
secret *content* out of the layer cache key -- that is the difference between a secret and
a build argument. `build.yml` sets `no-cache` to `github.event_name != 'pull_request'`, so
every pull request builds *with* the cache while every push and scheduled run rebuilds from
scratch and re-exports it. `main` and the nightly therefore keep the `plus` cache scope
warm, and a fork's build reuses that layer rather than running the `apk add nginx-plus`
that would need the certificate.

**This only works on a cache hit, and a miss is a hard failure.** `build.yml` also sets
`pull: true`, so a new Alpine base image -- or an edit to the Dockerfile above line 58, or
an NGINX Plus version bump -- invalidates the layer. The `apk add` then runs against
`pkgs.nginx.com` with empty files where the certificate and key should be, and the job
fails. Nothing marks it `continue-on-error`, so expect fork pull requests to fail their
Plus builds whenever the cache turns over.

### Two different fork cases

These are easy to confuse and behave differently.

- **A pull request from a fork** runs in the base repository's context, so
  `github.repository` is `nginx/nginx-gateway-fabric` and the repository gates pass. Only
  the fork-specific conditions apply.
- **A push inside a contributor's own fork** has `github.repository` set to their fork, so
  every gated job skips and they get the ungated jobs only. This is why forks never attempt
  to publish.

## The release path

Three dispatches, in order. Steps 3 to 5 are the reason the split exists: everything that
is tested is the artifact that ships, identified by digest through the signed manifest, and
nothing is rebuilt between test and release.

| Step | Workflow | Runs in | Runner | Images |
| --- | --- | --- | --- | --- |
| 1 | `cut-internal-release-branch.yml` | **Mirror** | GitHub-hosted | -- |
| 2 | `release-pr.yml` | **Mirror**; targets `internal/release-X.Y` | GitHub-hosted | -- |
| 3 | `release-prep.yml` | **Mirror** | `INTERNAL_RUNNER` | Builds and pushes to the staging write registry, tagged with the release version |
| 4 | `functional.yml`, `conformance.yml`, `helm.yml`, called by prep | **Mirror** | `INTERNAL_RUNNER` | Pulls from the staging read registry (`image-source: registry`) |
| 5 | `longevity-start.yml`, `longevity-stop.yml`, `nfr.yml`, `gatewaylink.yml` | Both; dispatched from the mirror | GitHub-hosted | Pulls the staged images, authenticated with the registry JWT |
| 6 | `promote-release-branch.yml` | **Mirror** | `INTERNAL_RUNNER` | -- |
| 7 | `release-publish.yml` | **Public** | `ubuntu-26.04-amd64` | Promotes by digest from staging to `ghcr.io/nginx` and `docker-mgmt.nginx.com` |

Release work runs on internal runners, as production builds always have: in the mirror on
the label in `INTERNAL_RUNNER` (`aw-ubuntu-24.04-amd64`), in the public repository on
`ubuntu-26.04-amd64`, the label `ci.yml` already uses for production builds. Prep and
promote refuse to start without `INTERNAL_RUNNER` rather than falling back to a
GitHub-hosted runner, and `build.yml` logs in to Docker Hub on any runner other than the
GitHub-hosted default, so it does so on either internal label.

The long-running suites in step 5 stay on GitHub-hosted runners because the runner pulls
nothing: it creates a GKE cluster, and the cluster's nodes pull the staged images. What
they need is a route from GKE to the staging read registry, which is a network question
for the registry, not a runner one.

`release-publish.yml` is the only workflow that writes to a public registry during a
release, and all nine of its jobs are public-gated. It creates the tag last, once every
artifact has published, so a failure leaves no tag behind.

The long-running suites in step 5 are ungated so a release can dispatch them from the
mirror, but the jobs that open a results pull request stay public-only: results are test
artifacts for `main`, not release content.

Not every release test is a workflow. **IPv6** is a `make` target and **OpenShift** is a
written procedure (see [OpenShift conformance](/tests/OPENSHIFT_CONFORMANCE.md)); both are
run by hand and neither is gated by anything. IPv6 takes the staged images through
`IMAGE_SOURCE=registry RELEASE_REPO=...`, which is how it tests what prep built; it runs
on your own machine, so nothing it prints reaches a public log.

## Scheduled work

| Workflow | Cadence | Runs in |
| --- | --- | --- |
| `ci.yml` nightly | Daily, 03:00 UTC | Public only, for the gated half |
| `nfr.yml` cron | 1st and 15th, 16:00 UTC | **Public only**; the manual dispatch runs in both |
| `gatewaylink.yml` cron | 5th and 20th, 16:00 UTC | **Public only**; the manual dispatch runs in both |
| `helm.yml` `helm-test-remote` | Schedule only | Public only |
| `codeql-analysis.yml` | Schedule, push, pull request | Public only |
| `scorecards.yml`, `stale.yml`, `update-docker-images.yml` | Various | Public only |

`nfr.yml` and `gatewaylink.yml` restrict only their **cron**, and by event rather than by
repository: `!(github.event_name == 'schedule' && github.repository != 'nginx/nginx-gateway-fabric')`.
The scheduled runs stay in the public repository so the mirror does not stand up GKE
clusters -- and, for gatewaylink, BIG-IP VMs -- on a timer. **A manual dispatch runs in
either repository**, which is the normal way both are used for a release; see step 5 of
the release path above.

## Pull request housekeeping

`f5-cla.yml`, `labeler.yml` and `renovate-build.yml` are all public-gated.
`dependency-review.yml` is ungated and runs in both: it writes nothing.

There is no longer a public cherry-pick workflow. `needs cherry pick` has exactly one
consumer, **Cherry-pick Inward**, which runs in the mirror and picks onto
`internal/release-X.Y`. Apply it to a fix that has to reach a release branch; nothing
applies it automatically.

## Where images come from, in one place

```text
Pull request       ->  localhost:5000                   ephemeral, never leaves the runner
main and nightly   ->  ghcr.io/nginx, docker-mgmt.nginx.com, GAR      :edge / :nightly
Release prep       ->  staging write registry (vault)                 :<release version>
All release tests  <-  staging read registry (vault)                  JWT-authenticated pull
Release publish    ->  ghcr.io/nginx, docker-mgmt.nginx.com           copied by digest
```

Staging hostnames are never written in this tree, and never printed in a log. They are
vault entries, not repository variables: a variable is printed in plain text wherever a
step uses it, while the vault action masks everything it fetches. That is why
`.github/config/registries-staging` reads them from the environment rather than naming
them, and why each job that needs one fetches it itself -- the runner drops a masked value
from a job output, so none can be passed between jobs.

### What a published release does and does not reveal

- **The staging registries and package host: nothing.** The hosts are masked in both
  repositories' logs. The manifest records each image by name and digest, not by the
  repository it was staged in. The Plus images take the staging package host as a
  BuildKit secret (`nginx-pkg-hosts`), not a build argument: a build argument is written
  into the image's history, where `docker history` shows it, and a secret is recorded in
  neither the image nor its provenance. The Alpine image installs from those package
  sources through a file of its own, so its `/etc/apk/repositories` does not name them
  either; the UBI image only ever bind-mounts its repository files. When checking a new
  base image or build change, run `docker history --no-trunc` and inspect the provenance
  attestation of a staged image for the host before the first release on it.
- **The mirror's name and the internal release branch: yes, by design.** Prep signs the
  manifest with cosign keyless, through the public Sigstore instance. The signing
  certificate names the workflow that signed, `release-prep.yml` in the mirror on
  `internal/release-X.Y`, and that certificate is recorded in the public Rekor
  transparency log when prep runs, before release day. The signature bundle is also a
  release asset. This is what lets anyone verify a release came from prep; hiding the
  mirror's name would mean signing with a managed key instead. The commit prep built is
  in the certificate too, but it does not exist publicly until promote has fast-forwarded
  the release branch.

### Why prep pushes to one staging endpoint and reads from another

Prep builds to the staging write registry and its suites pull from the staging read registry.
That is deliberate and confirmed with the registry team: **the read mirror is the supported
read path, and there is no replication delay between the two**, so no job waits or polls
between the push and the pull.

Two questions this settles, because both come up:

- **Why not rebuild the images in the test job, the way `ci.yml` does?** Because a rebuild
  is not the artifact that ships. It gets a different digest, `load: true` produces a
  single-architecture image rather than the multi-architecture index that was pushed, and
  the attestations differ. The signed manifest promotes the digest prep *tested*, so the
  tested bytes have to be the pushed bytes.
- **Why not read back from the write endpoint?** It is not the supported read path. The
  split is also why the credentials differ: a read mirror takes the licence JWT, which is
  what an image pull secret can hold, and in registry mode the kubelet does the pulling.

### What Artifact Registry is for

`us-docker.pkg.dev` carries three distinct things, and they are worth separating because
only one of them is irreplaceable.

1. **`nginx-crossplane`**, built locally and **pushed** per run by
   `tests/scripts/push-crossplane-image.sh`. It is a test helper that has to live somewhere
   the GKE cluster can pull from, and nothing else plays this role. All three GKE workflows
   reach it: the script is called by the `nfr-test` target, and `start-longevity-test` and
   `stop-longevity-test` both depend on `nfr-test`. So the GAR login in those workflows is
   a push as well as a pull -- **running them from the mirror needs GAR write access there,
   not just read**. Note also that `:latest` is a shared mutable tag, so a mirror release
   run and a public nightly can overwrite each other's copy.
2. **`nginx-plus` and `nginx-plus-f5waf` mirrors**, pushed by `build.yml` on non-pull-request
   public events. These duplicate images that already go to `docker-mgmt.nginx.com`. The
   GKE suites reach them through `NGINX_PLUS_PREFIX`.
3. **`formatNginxPlusEdgeImagePath`** (`tests/suite/system_suite_test.go:108`), used only on
   the published-chart (`oci://`) NFR path.

Items 2 and 3 exist because a GKE cluster pulls from Artifact Registry using workload
identity, with no image pull secret to manage. A release run already bypasses them: it
points `NGINX_PLUS_PREFIX` at the staging registry and pulls with the JWT.

**`docker-mgmt.nginx.com` is not a substitute, even though it holds the same images.** The
NGINX registries split by direction: read mirrors take the licence JWT as the username,
while write endpoints such as `docker-mgmt.nginx.com` take a short-lived OIDC token from
the runner. A cluster cannot hold a credential of the second kind in an image pull secret.
So a non-release GKE run needs either Artifact Registry, as today, or a read mirror that
carries edge Plus images -- and the staging registry only carries release versions.

## Repository variables and secrets

What each side must hold for the gates above to resolve and the release workflows to run.
Everything but the vault's own access and two mirror-only variables lives in the Azure key
vault, because the vault action masks what it fetches and a repository variable is printed
wherever a step uses it. These are set once and reviewed rarely. **Anything missing fails
closed**: a missing vault entry fails the step that fetches it, naming the entry, before
anything is built or published, and without `INTERNAL_REPOSITORY` every mirror-only job
simply stays switched off.

"Vault (NGF)" is the vault named by `AZ_VAULT_NAME`, "Vault (common)" the one named by
`AZ_COMMON_VAULT_NAME`. Entries marked *existing* are already used on `main` and need no
action.

**In the mirror**

| Kind | Name | Purpose |
| --- | --- | --- |
| Variable | `INTERNAL_REPOSITORY` | The mirror's own `owner/name`. Every mirror-only job is gated on it. It has to be a variable: a job's `if:` is evaluated before any step can read the vault. It is defined only in the mirror, so it never appears in a public log. |
| Variable | `INTERNAL_RUNNER` | `aw-ubuntu-24.04-amd64`, the mirror's internal runner label. **Required**: prep and promote fail without it rather than building on a GitHub-hosted runner. A variable for the same reason as above: `runs-on` is evaluated before any step |
| Secret | `AZ_VAULT_CLIENT_ID`, `AZ_VAULT_TENANT_ID`, `AZ_VAULT_NAME`, `AZ_COMMON_VAULT_NAME` | Vault access, the same names and values as in the public repository. These reach the vault, so they cannot live in it. The `AZ_VAULT_CLIENT_ID` identity must trust the mirror's OIDC tokens, including on `internal/release-*` branches, and it also uploads the release assets, so it needs write access to the asset container |
| Vault (NGF) | `staging-write-registry`, `staging-read-registry` | The staging registry prep pushes to, and the read mirror the suites pull from. The read host must end in `.nginx.com`: the suites attach the registry JWT only to NGINX registries |
| Vault (NGF) | `staging-pkg-host` | Internal NGINX package host for the Plus image builds. Reaches the build as a secret |
| Vault (NGF) | `jwt-plus-waf-registry` | Registry JWT: prep's staged-digests check and every registry-source suite pull with it |
| Vault (NGF) | `jwt-plus-reporting-endpoint`, `jwt-plus-exception-reporting` | Plus licensing for the functional, conformance and Helm suites; *existing* in the public repository's vault |
| Vault (NGF) | `azure-storage-account`, `azure-storage-bucket` | The storage account and blob container prep uploads the release binaries to. A GitHub artifact cannot cross between repositories, so this is how publish gets them |
| Vault (common) | `artifactory-service-user-nginx`, `artifactory-service-user-nginx-token`, `artifactory-go-url` | The Go module proxy prep's binary build uses |
| Vault (common) | `docker-username`, `docker-password`, `nginx-pkg-certificate`, `nginx-pkg-key` | Image builds: Docker Hub pulls and the licensed package repository; *existing* in the public repository's common vault |
| Vault (common) | `nginx-bot-pat` | *Existing*. Cherry-pick Inward pushes and opens its pull request with it; `GITHUB_TOKEN` would not do, as GitHub starts no workflows for a pull request it opens. Promote pushes public `release-X.Y` and dispatches publish with it, so the bot must be allowed to push to `release-*` by the branch rules, and the token needs the `workflow` scope: GitHub refuses a push that changes `.github/workflows/` from a token without it |
| Vault (NGF) | `gcp-workload-identity`, `gcp-service-account`, `gcp-project-id`, `gcp-nodes-service-account`, and `bigip-admin-password` for GatewayLink | *Existing* in the public repository's vault. The cluster suites create their GKE clusters from here. The workload identity must let the mirror **write** to Artifact Registry, not only read, because each run pushes the `nginx-crossplane` test helper |

**In the public repository**

| Kind | Name | Purpose |
| --- | --- | --- |
| Secret | `AZ_VAULT_CLIENT_ID`, `AZ_VAULT_TENANT_ID`, `AZ_VAULT_NAME`, `AZ_COMMON_VAULT_NAME` | Vault access; *existing*. The same identity downloads the release assets, so it needs read access to the asset container |
| Vault (NGF) | `release-signer-repository` | The mirror's `owner/name`, used only to verify manifest signatures. A vault entry so the public log masks it, and deliberately separate from the mirror's `INTERNAL_REPOSITORY` |
| Vault (NGF) | `staging-read-registry` | Publish promotes **from** here: it logs in to this registry and `registries-production` names it as the source |
| Vault (NGF) | `jwt-plus-waf-registry` | Publish logs in to the staging read registry with it to promote from there |
| Vault (NGF) | `azure-storage-account`, `azure-storage-bucket` | The release asset store publish downloads from |
| Vault (NGF) | `certification-component-id-ngf`, `certification-component-id-nginx`, `certification-component-id-operator`, `pyxis-api-token` | RedHat certification, which publish runs after promoting; *existing* |
| Vault (common) | `nginx-bot-pat` | The operator bundle pull request publish dispatches; *existing* |

The public repository holds no repository variables for the release.

**One vault, shared by both repositories.** Both sides read the same vaults, so a workflow
in the public repository can fetch any entry above, mirror-only ones included. Only
workflows can reach the vault, and pull requests from forks get neither an OIDC token nor
secrets, so this is exposure to people with write access to the public repository and to
the actions its workflows run, not to the public. For the hosts that is a name and no
more, and every log masks them either way. The vault identity is shared too, so it can
write to the asset store from either side; a file planted there from the public side is
refused by publish, because every asset must match a sha256 in the signed manifest.

The one entry that breaks the design's credential rule -- credentials point from private to
public, never the reverse -- is `nginx-bot-pat`, if the bot has access to the mirror:
Cherry-pick Inward needs it there, and the public `operator-bundle-pr.yml` already reads
it. That predates the release split. If it needs closing, the mirror's credential moves to
a secret held in the mirror repository itself, which the public repository cannot read.

## The operator bundle, and the one thing it can break

`operator-bundle-pr.yml` is public-only and should stay that way: the bundle records the
digests of the **published** images, which do not exist until publish has promoted them,
and it may open a pull request against the RedHat certified-operators repository. There is
nothing for the mirror to do.

The sequence on release day is:

1. Publish creates the tag, then dispatches `operator-bundle-pr.yml` on the release branch.
2. It generates the bundle from the published image digests and opens a draft pull
   request **into `main`**. With `submit-to-redhat` it also opens one against the RedHat
   certified-operators repository.
3. That pull request is where it ends. It is **not** labelled for the release branch.

The bundle never goes back to `release-X.Y`, and does not need to: nothing on that branch
reads `operators/bundle`, and the next release on the line regenerates it from its own
published digests with `make bundle-release`. Carrying it back would put a commit on
public `release-X.Y` outside a promote, which is the one thing that branch may not
receive -- it may only advance by the fast-forward promote performs, or promote refuses.

It follows that the bundle is not part of the release it describes. It records digests of
images that are already published, so nothing in the release waits on it.
