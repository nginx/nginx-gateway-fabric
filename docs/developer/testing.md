# Testing

This document provides guidelines for testing, including instructions on running the unit tests, accessing the code
coverage report, performing manual testing, and running the conformance tests. By following these guidelines, you will
gain a thorough understanding of the project's approach to unit testing, enabling you to ensure code quality, validate
functionality, and maintain robust test coverage.

## Unit Test Guidelines

In our testing approach, we employ a combination of Behavior-Driven Development (BDD) style tests and traditional unit
tests. The choice of testing style depends on the nature of the code being tested. Here's a breakdown of our testing
practices:

**All exported interfaces must be covered by BDD style tests**: We use the [Ginkgo](https://onsi.github.io/ginkgo/)
testing framework, which enables us to write expressive and readable tests that focus on the behavior of our code.
Additionally, we use the [Gomega](https://onsi.github.io/gomega/) matcher library to provide powerful assertions and
expectations within our BDD style tests.

**Most testing should be done at the exported interface layer**: By writing tests at the exported interface layer, we
ensure that the tests remain decoupled from the internal implementation details. This allows us to refactor and modify
the implementation of the interface without needing to extensively update the tests. It promotes flexibility and
maintainability in our codebase.

**Use standard unit tests to cover gaps**:  While we primarily focus on testing at the exported interface layer, we
acknowledge the importance of comprehensive test coverage. If we identify areas that require additional testing or if
the exported interface layer doesn't provide sufficient coverage, we typically use unit tests to cover these gaps. This
ensures that critical code paths and edge cases are thoroughly tested. We still use
the [Gomega](https://onsi.github.io/gomega/) matcher library for assertions within these tests.

**Use table-driven tests**: When testing multiple cases for a single function or method, we prefer to use table-driven
tests. For BDD style tests, use [DescribeTable](https://onsi.github.io/ginkgo/#table-specs) and for standard unit tests,
use [subtests](https://go.dev/blog/subtests).

**Generate test mocks**: To facilitate the generation of mocks for testing, we use
the [Counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) tool. Counterfeiter helps us create mock
implementations of internal and public interfaces, allowing us to isolate and control dependencies during testing. It
simplifies the process of mocking and stubbing, making our tests more robust and flexible.

**Parallelize unit tests**: In general, all tests should be designed to run in parallel for faster execution and help uncover bugs. For standard Go tests, this requires adding `t.Parallel()` to every test and subtest. Ginkgo tests, on the other hand, automatically run in parallel without the need for additional configuration. If a component under test requires sequential execution, you can run tests sequentially by using an [ordered container](https://onsi.github.io/ginkgo/#ordered-containers) for Ginkgo tests or by omitting `t.Parallel()` from the go test or subtest. In such cases, it’s essential to include a comment explaining why parallel execution is not possible.

By combining BDD style tests, unit tests, and mock generation, we aim to achieve a comprehensive and maintainable
testing strategy. This approach enables us to ensure the correctness, reliability, and flexibility of our codebase while
promoting efficient refactoring and continuous development.

### Running the Unit Tests

To run the unit tests, run the make unit-test command from the project's root directory:

```makefile
make unit-test
```

This command runs the unit tests with the `-race` flag, enabling the detection of potential data races.

#### Viewing Code Coverage Report

The unit tests generate a code coverage file named `cover.html` in the project's root directory. To view the code
coverage report, open the `cover.html` file in your browser. The report provides insights into which parts of the code
are covered by the tests and helps identify areas that may require additional testing.

## Manual Testing

To ensure the quality and correctness of your changes, it is essential to perform manual testing in a Kubernetes
cluster. Manual testing helps validate the functionality and behavior of your changes in a real-world environment.
Follow the steps below for manual testing:

1. Follow the instructions to [deploy on kind](/docs/developer/quickstart.md#deploy-on-kind).
2. Test your changes. Make sure to check the following:
   - Logs of the `nginx-gateway` container. Look out for unexpected error logs or panics.

     ```shell
     kubectl -n nginx-gateway logs <ngf-pod-name>
     ```

   - Logs of the `nginx` container. Look for unexpected error logs and verify the access logs are correct.

     ```shell
     kubectl -n <nginx-pod-namespace> logs <nginx-pod-name>
     ```

   - The generated nginx config. Make sure it's correct.

     ```shell
     kubectl exec -it -n <nginx-pod-namespace> <nginx-pod-name> -- nginx -T
     ```

   - The statuses of the Gateway API Resources. Make sure they look correct.

     ```shell
     kubectl describe <resource> <resource name>
     ```

   - NGINX proxies traffic successfully (when applicable).
   - [Examples](/examples) work correctly. This will ensure that your changes have not introduced any regressions.

> **Note**
>
> Don't limit yourself to happy path testing. Make an effort to cover various scenarios,
> including edge cases and potential error conditions. By testing a wide range of scenarios,
> you can uncover hidden issues and ensure the robustness of your changes.

Performing manual testing helps guarantee the stability, reliability, and effectiveness of your changes before
submitting them for review and integration into the project.


## Gateway API Conformance Testing

To run Gateway API conformance tests, please follow the instructions on [this](/tests/README.md#conformance-testing) page.
