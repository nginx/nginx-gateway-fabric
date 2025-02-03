# Implementing a Feature

This document provides guidance on implementing new features in the project. Follow the recommended steps and best
practices to ensure a successful feature development process.

> **Note**
>
> If you’d like to implement a new feature, please open a discussion about the feature
> before creating an issue or opening a PR.

1. **Assign yourself to the GitHub issue for the feature**: Each feature must have a corresponding GitHub issue to track
   its progress.
2. **Post any questions or comments about the feature on the corresponding issue**: This allows for better tracking and
   visibility. If any discussions regarding the issue occur outside the issue thread, provide a summary of the
   conversation as a comment on the issue itself. This ensures that all relevant information and discussions are
   consolidated in one place for easy reference.
3. **Fork the repo**: NGF follows a fork workflow, which you can learn more about in
   the [branching and workflow](/docs/developer/branching-and-workflow.md) documentation.
4. **Branch**: Create a branch following
   the [naming conventions](/docs/developer/branching-and-workflow.md#branch-naming-conventions).
5. **Review style guide** Review the [Go style guide](/docs/developer/go-style-guide.md) to familiarize yourself with
   the project's coding practices.
6. **Make changes**: Make the necessary changes for the feature.
7. **Consider opening a draft PR**: If your feature involves substantial architecture changes, or you would like to
   receive early feedback, consider opening a draft PR and requesting reviews from the maintainers. Draft PRs are an
   effective means to solicit feedback and ensure that major architectural changes align with the project's goals and
   standards.
8. **Add or update unit tests** All features **must** be accompanied by unit tests that verify the functionality. Make
   sure to thoroughly test the different scenarios and edge cases related to the feature to ensure robustness and
   reliability. Additionally, open the code coverage report to ensure that the code you added has sufficient test
   coverage. For instructions on writing and running unit tests, refer to
   the [testing](/docs/developer/testing.md#unit-test-guidelines) documentation.
9. **Manually verify your changes**: Refer to the [manual testing](/docs/developer/testing.md#manual-testing) section of
   the testing documentation for instructions on how to manually test your changes.
10. **Update any relevant documentation**: See the [documentation](https://github.com/nginx/nginx-gateway-fabric/blob/main/site/README.md) guide for in-depth information about the workflow to update the docs and how we publish them.
   Here are some basic guidelines for updating documentation:
    - **Gateway API Feature**: If you are implementing a Gateway API feature, make sure to update
      the [Gateway API Compatibility](/site/content/concepts/gateway-api-compatibility.md) documentation.
    - **New Use Case:** If your feature introduces a new use case, add an example of how to use it in
      the [examples](/examples) directory. This example will help users understand how to leverage the new feature.
      > For security, a Docker image used in an example must be either managed by F5/NGINX or be an [official image](https://docs.docker.com/docker-hub/official_images/).
    - **Installation Changes**: If your feature involves changes to the installation process of NGF, update
      the [installation](/site/content/how-to/installation/installation.md) documentation.
    - **Helm Changes**: If your feature introduces or changes any values of the NGF Helm Chart, update the
      [Helm README](/charts/nginx-gateway-fabric/README.md).
    - **Command-line Changes**: If your feature introduces or changes a command-line flag or subcommand, update
      the [cli help](/site/content/reference/cli-help.md) documentation.
    - **Other Documentation Updates**: For any other changes that affect the behavior, usage, or configuration of NGF,
      review the existing documentation and update it as necessary. Ensure that the documentation remains accurate and
      up to date with the latest changes.
11. **Lint code**: See the [run the linter](/docs/developer/quickstart.md#run-the-linter) section of the quickstart
    guide for instructions.
12. **Run generators**: See the [Run go generate](/docs/developer/quickstart.md#run-go-generate) and the
    [Update Generated Manifests](/docs/developer/quickstart.md#update-generated-manifests) sections of the
    quickstart guide for instructions.
13. **Open pull request**: Open a pull request targeting the `main` branch of
    the [nginx-gateway-fabric](https://github.com/nginx/nginx-gateway-fabric/tree/main) repository. The
    entire `nginx-gateway-fabric` group will be automatically requested for review. If you have a specific or
    different reviewer in mind, you can request them as well. Refer to
    the [pull request](/docs/developer/pull-request.md) documentation for expectations and guidelines.
14. **Obtain the necessary approvals**: Work with code reviewers to maintain the required number of approvals.
15. **Ensure the product telemetry works**. If you made any changes to the product telemetry data points, it is
    necessary to push the generated scheme (`.avdl`, generated in Step 12) to the scheme registry. After that, manually
    verify that the product telemetry data is successfully pushed to the telemetry service by confirming that the data
    has been received.
16. **Squash and merge**: Squash your commits locally, or use the GitHub UI to squash and merge. Only one commit per
    pull request should be merged. Make sure the first line of the final commit message includes the pull request
    number. For example, Fix supported gateway conditions in compatibility doc (#674).
    > **Note**:
    When you squash commits, make sure to not include any commit messages related to the code review
    (for example, Fixed a typo). If you changed the code as a result of the code review in way that the
    > original commit message no longer describes it well, make sure to update the message.

## Fixing a Bug

When fixing a bug, follow the same process as [implementing a feature](#implementing-a-feature) with one additional
requirement:

All bug fixes should be reproduced with a unit test before submitting any code. Once the bug is reproduced in a unit
test, make the necessary code changes to address the issue and ensure that the unit test passes successfully. This
systematic approach helps ensure that the bug is properly understood, effectively resolved, and prevents regression.
