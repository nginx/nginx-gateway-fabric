# CRD Versioning

This document discusses how we version our CustomResourceDefinitions (CRDs) that define Policies, Filters, or other configuration objects. NGINX Gateway Fabric distributes a few CRDs, such as NginxGateway, NginxProxy, ClientSettingsPolicy, and ObservabilityPolicy. More will inevitably be added over time.

## Initial Version

When creating a new CRD, the initial version should be `v1alpha1`. This version indicates a new API that needs time for use and feedback. By starting at the alpha version, it also allows us a little more flexibility in changing the API.

## Changing an API

When changing an API in a CRD, it's important to understand the impact of those changes. This will determine if and how we need to update the API version.

**No change in version required**

- Adding new optional fields.
- Documentation updates.

**Compatible changes, requires a version change**

- When making schema changes (in other words, renaming or changing the structure of an existing field), a change of version is required (`v1alpha1` to `v1alpha2`) and a [conversion webhook](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) is utilized to preserve compatibility for a user and allow our controller to support both versions.
  - As part of this, the old API version should be marked as deprecated, and then removed after 3 releases.
  - The new API should not be marked as the preferred or `stored` version until old API is removed.
  - To reduce complexity, we should only support a maximum of 2 API versions at the same time.

For more in depth information on compatible changes, see the Kubernetes [API changes doc](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#on-compatibility).

**Breaking changes, requires a version change**

The following API changes are incompatible with previous API versions, and therefore not only require a version bump, but cannot use a conversion webhook. These types of changes should be avoided if at all possible due to the disruption for users. A user will need to update their configurations when upgrading NGF. These types of changes need clear messaging in release notes and docs.

We'll allow a bit more flexibility for this case when dealing with alpha APIs.

- Adding new required fields.
- Removing fields. If possible, we should try to deprecate fields for 3 releases to slowly phase them out before removing.
- Changing validation and allowed values of a field.

For more in depth information on how to handle API changes, see the Kubernetes [API changes doc](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#backward-compatibility-gotchas).


The NGINX Gateway Fabric version should not need to change when CRD versions change. CRDs are not embedded into NGF; they have their own versions.

NGINX Gateway Fabric releases are directly tied to the CRD version they support and users are required to upgrade CRD versions when upgrading NGF. This ensures that the CRDs remain compatible with NGINX Gateway Fabric.

However, the user may need to fix any incompatible CRDs.

## Graduating to v1 (stable)

An alpha API allows us more flexibility in making breaking API changes as we further design and craft the API to fit the necessary use cases. However, at some point these APIs need to become stable and no longer have breaking changes, so that a user can rely on them without the worry of things not working from release to release.

When we've determined that an API is stable and should no longer have breaking changes or refactors, we can promote it to `v1`. Once promoted, this API **should not** be subject to changes that require a version bump. Only when absolutely necessary should we consider changes that would lead to a `v2alpha1` or `v2` version.

We are skipping the usual alpha -> beta -> v1 graduation for a couple of reasons.

1. It matches the pattern of Gateway API graduation.
2. Our APIs are likely going to be simple enough that we don't need to go through 3 stages of graduation. There is overhead in this that is probably not necessary for our project.

As NGF API adoption and feedback is being evaluated, the timeline remains flexible.
