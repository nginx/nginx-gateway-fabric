# Logging Guidelines

This document describes the logging guidelines for the control plane of NGINX Gateway Fabric (NGF).

> The data plane logging is not covered here: such a concern is owned by NGINX developers, and NGF developers
> don't have control over it.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Requirements](#requirements)
  - [User Stories](#user-stories)
    - [Common Stories](#common-stories)
    - [Stories For User](#stories-for-user)
    - [Stories For Developers](#stories-for-developers)
- [Logging Library Choice](#logging-library-choice)
- [Guidelines](#guidelines)
  - [How to Log](#how-to-log)
    - [Log Levels](#log-levels)
    - [Log Messages](#log-messages)
    - [Context in Messages](#context-in-messages)
    - [Examples of Log Messages](#examples-of-log-messages)
    - [Message Guidelines](#message-guidelines)
  - [Log Formatting](#log-formatting)
  - [When to Log](#when-to-log)
  - [What not to Log](#what-not-to-log)
  - [Performance](#performance)
  - [Logger Dependency in Code](#logger-dependency-in-code)
    - [Logger Initialization](#logger-initialization)
    - [Logger Injection](#logger-injection)
    - [Special Case - Reconciler](#special-case---reconciler)
    - [Unit Tests](#unit-tests)
  - [External Libraries](#external-libraries)
  - [Evolution](#evolution)
- [External Resources](#external-resources)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Requirements

Before delving into the guidelines, we must first establish the high-level requirements for logging to ensure a clear
understanding of what these guidelines intend to address:

> We don't anticipate these high-level requirements to change for the duration of the project. If changes occur,
> we will need to update this document. However, we don't need to document enhancements based on these requirements,
> unless they directly impact the guidelines.

### User Stories

There are two distinct personas of NGF:

- *User*, who uses NGF. For logging, the user persona maps to the cluster operator from
  the [Gateway API personas](https://gateway-api.sigs.k8s.io/concepts/security-model/#roles-and-personas).
- *Developer*, who develops NGF.

Below are common user stories for both personas and also persona-specific stories.

#### Common Stories

As a user/developer, I want to:

- Have confidence the control plane works properly by looking at the logs.
- Control the amount/level of detail of logged information.
- Use logs to troubleshoot an issue with the control plane, either in real-time or by looking at the log history.
- In case of errors, see what event or Kubernetes resource caused it.

#### Stories For User

As a user, I want to:

- Export the logs into my logging infrastructure and parse them for further processing (like querying).
- Share the logs with developers to ask for help or report a bug.

#### Stories For Developers

As a developer, I want to:

- See human-readable (text) as opposed to machine-readable (JSON) format.
- Use logs to troubleshoot an issue collected from a user.

## Logging Library Choice

To make the logs easily parseable/processable by other programs, we use structured logging - i.e. logging data in a
structured
way (like JSON) as opposed to a non-structured way (like a long string from a `printf` invocation).

The code logs via the [logr][logr] library, which abstracts away
the logger implementation. This library covers our needs, is well adopted, and many logger implementations implement its
interfaces.

As an implementation, we use [zap](https://github.com/uber-go/zap/), which:

- supports logr via https://github.com/go-logr/zapr
- supports both JSON and text format
- supports log levels
- demonstrates good [performance](https://github.com/uber-go/zap/#performance)
- is stable and well-adopted.

We initialize zap via controller-runtime
framework [helpers][cr-zap].

## Guidelines

### How to Log

#### Log Levels

The table below describes the log verbosity levels. A level includes all messages of the levels above.

| Level            | Contents                                                                             | Target persona |
|------------------|--------------------------------------------------------------------------------------|----------------|
| `error`          | errors                                                                               | user           |
| `info` (default) | information about how the control plane operates                                     | user           |
| `debug`          | extra information about how the control plane operates to help troubleshoot an issue | developer      |

> The levels match the levels of
> the [controller-runtime framework][cr-log-levels] without any custom levels.

#### Log Messages

We log two kinds of messages (this is the extent of the [logr][logr] API):

- *Error*, which describes a non-critical error in the control plane (for which it will not terminate) that
  requires the user's attention.
  > Logging an error is a way of handling it. If you handle the error
  > differently, log it as info, or don't log it at all.
  >
  > See [What not to Log](#what-not-to-log) section about handling critical errors.
- *Info*, which describes anything that's not an error.
  - Messages logged at the `info` level target the user persona.
  - Messages logged at the `debug` level target the developer persona, and we don't expect the
      user to fully understand them.

> The message kind is not the same as the log level.

#### Context in Messages

In addition to the actual message, the log message should include the context:

- *Timestamp* (handled by the logging library) - to make the user/developer aware *when* something occurs.
- *Kubernetes resource* (if any) - to correlate to the corresponding Kubernetes resource.
- *Component name* - to correlate with the component (for example, eventHandler), which logs the message.
- *Relevant extra context* - additional context depending on the location of the log line in the code.

Additionally, specifically for the developer persona:

- *Stacktrace* - to pinpoint where in the code the message was logged.

The next section shows how to add context.

#### Examples of Log Messages

1. A simple info message with a timestamp with the default `info` level:

   ```go
   logger.Info("Info message")
   ```

   ```json
   {"level":"info","ts":"2023-07-20T15:10:03-04:00","msg":"Info message"}
   ```

2. An info message with the `debug` level:

   ```go
    logger.V(1).Info("Info message")
   ```

   ```json
   {"level":"debug","ts":"2023-07-20T15:10:03-04:00","msg":"Info message"}
   ```

3. An error:

   ```go
    logger.Error(errors.New("test error"), "Error message")
   ```

   ```json
    {"level":"error","ts":"2023-07-20T15:10:03-04:00","msg":"Error message","error":"test error","stacktrace":"main.main\n\t/<REDACTED for the document>.go:73\nruntime.main\n\t/usr/local/go/src/runtime/proc.go:250"}
   ```

   > An error message includes the stack trace by default.
4. A message with a Kubernetes resource:

   ```go
   // hr implements client.Object (controller-runtime)
   // hr is *v1.HTTPRoute
    logger.Info(
        "Processed resource",
        "resource", hr,
    )
   ```

   ```json
   {"level":"info","ts":"2023-07-20T15:10:03-04:00","msg":"Processed resource","resource":{"apiVersion":"gateway.networking.k8s.io/v1","kind":"HTTPRoute","namespace":"test","name":"hr-1"}}
   ```

   > The resource must include `TypeMeta`, otherwise its `apiVersion` and `kind` will not be printed.
5. A message with a component name:

   ```go
    logger = logger.WithName("component-a")
    logger.Info("Hello")
   ```

   ```json
    {"level":"info","ts":"2023-07-20T15:10:03-04:00","logger":"component-a","msg":"Hello"}
   ```

6. A message with extra context:

   ```go
    logger.Info(
        "Info message",
        "status", "ok",
        "generation", 123,
    )
   ```

   ```json
   {"level":"info","ts":"2023-07-20T15:10:03-04:00","msg":"Info message","status":"ok","generation":123}
   ```

7. An info message with a stack trace configured for the `info` level (done separately during the logger initialization):

   ```go
    logger.Info("Info message")
   ```

   ```json
   {"level":"info","ts":"2023-07-20T15:10:03-04:00","msg":"Info message","stacktrace":"main.main\n\t/<REDACTED for the document>.go:100\nruntime.main\n\t/usr/local/go/src/runtime/proc.go:250"}
   ```

#### Message Guidelines

1. Start a message with an uppercase letter. For example, `"Reloaded nginx"`.
2. Do not put dynamic values in the message, add them as key values (extra context):

   DO NOT:

   ```go
    logger.Info(fmt.Sprintf("Got response with status %q", "ok"))
   ```

   DO:

   ```go
    logger.Info(
        "Got response",
        "status", "ok",
    )
   ```

3. For readability, put each key-value pair on a separate line, as in the previous example.
4. Use camelCase for key names.
5. When dealing with a Kubernetes resource, use `resource` as a key:

   ```go
    logger.Info(
        "Processed resource",
        "resource", hr,
    )
   ```

6. Don't put structs as values except for Kubernetes resources, to prevent messy and potentially unbounded output.

### Log Formatting

By default, log messages are formatted as JSON, so that external systems can easily parse them.

For the developer, log messages are formatted as text strings (except key/values), for human-friendly parsing. For
example:

```text
2023-07-21T12:41:37.640-0400    INFO    Processed resource      {"resource": {"apiVersion": "gateway.networking.k8s.io/v1", "kind": "HTTPRoute", "namespace": "test", "name": "hr-1"}}
```

The formatting is controlled during the logger initialization.

### When to Log

Below are some examples of when to log a message:

- Lifecycle events
  - The control plane starts or shuts down
  - Lifecycle of the components.
- Handling a transaction/request
  - The main transaction in the control plane is processing a batch of events related to Kubernetes resources taken
      from the main event loop.
  - Additional transactions could be out of the event loop requests to auxiliary control plane endpoints (APIs).
- Interaction with external components. When the control plane interacts with an external component, log the stages of
  that interaction as well as the result of it.

For those cases:

- Log an error only if the code doesn't handle it (performs some action). If it is handled, but it is still important to
  record the fact of that
  error, log it as an info message.
- Consider including performance-related information. For example, the duration of an operation.
- Log extra details (or steps) to facilitate troubleshooting under the `debug` level.

> It is important to not overload the `info` level with too many details and messages.

### What not to Log

- *Sensitive information*, including the contents of the secrets, passwords, etc.
- *Returned errors*. If a function returns an error, do not log it inside that function, because it will be logged or
  handled up in the stack.
- *Critical errors*, which causes control plane to terminate.
  - For non-recoverable or programming errors, we [panic](go-style-guide.md#use-panics-for-unrecoverable-errors-or-programming-errors).
  - An error that is not handled will propagate to the `main` package, where the control plane will print it and
    immediately terminate.

### Performance

Logging too frequently can lead to poor performance, because of:

- Too many I/O operations.
- Too many allocations related to preparing the log message.

If we diligently follow the guidelines from this document, the `error` and `info` log levels will not have many messages
(many thousands per second). At the same time, such a case might still be possible, for example, in the case of a bug.
To prevent that, the default settings in the controller-runtime helpers for zap
[configure sampling][sampling].
The sampling is based on the message (the `msg` parameter) of the log-related methods. Read more about sampling
[here](https://github.com/uber-go/zap/blob/master/FAQ.md#why-sample-application-logs).

[sampling]:https://github.com/kubernetes-sigs/controller-runtime/blob/b1d6919d3e12fa85a119dd9792bdfdc17bdf8c3b/pkg/log/zap/zap.go#L211-L215

### Logger Dependency in Code

#### Logger Initialization

The logger should be initialized in the `main` package and passed to the components as `logr.Logger`. In the `main`
package
we set the parameters of the logger, such as the level, stack trace level, and format. The control-plane components
should
not change them.

#### Logger Injection

For asynchronous components, inject the logger during the component creation, setting up the name of the logger to
match the name of the component:

```go
asyncComp := newComponent(logger.WithName("async-component"))
```

For synchronous components (meaning another component will call it synchronously), inject the logger during the
method call, adding extra context when necessary:

```go
l := logger.WithName("sync-component").WithValues("operationID", 1)
syncComp.Process(l)
```

#### Special Case - Reconciler

The Reconciler [gets its logger][reconciler-logger] from the controller-runtime. We use it (as opposed to injecting ours)
because the runtime adds context to that logger with the group, kind, namespace and name of the resource, and a few
more key-value pairs. Note that the runtime creates that logger from the one we inject into it during the runtime
initialization (see [External Libraries](#external-libraries)). Also note that logger is compatible with our logging guidelines.

[reconciler-logger]:https://github.com/nginx/nginx-gateway-fabric/blob/5547fe5472d1742a937c8adbbd399893ee30f9e1/internal/framework/controller/reconciler.go#L63

#### Unit Tests

In unit tests, we don't test what we log.

To initialize a logger, use `zap.New()` from [controller-runtime helpers][cr-zap].

### External Libraries

There are two critical libraries for NGF that log:

- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime).
  - The [log levels][cr-log-levels] are compatible with our project.
  - We [inject][inject] logger into the library as `logr.Logger` from [logr][logr].
  - See also
      the [logging guidelines][cr-logging-guidelines]
      of that project.
- [client-go](https://github.com/kubernetes/client-go).
  - It uses [klog](https://github.com/kubernetes/klog) for logging.
  - We inject the above logger into klog to ensure it uses the same formatting.
  - Most of the logging is done at increased klog-specific
      verbosity. However, errors are logged at the default verbosity like
      in [this line](https://github.com/kubernetes/client-go/blob/c5b1c13ccbedeb03c00ba162ef27566b0dfb512d/tools/record/event.go#L240).

[inject]:https://github.com/nginx/nginx-gateway-fabric/blob/9b3ae2c7c59f28213a7690e049d9996443dbd3fc/internal/mode/static/manager.go#L54

[cr-logging-guidelines]:https://github.com/kubernetes-sigs/controller-runtime/blob/b1d6919d3e12fa85a119dd9792bdfdc17bdf8c3b/TMP-LOGGING.md

When adding a new library, evaluate if it does logging and how. If it is possible, configure the library logging to
be compatible with NGF logging. If not, document the logs in the user documentation for control plane logging, so that
the users are prepared for them.

### Evolution

As NGF evolves, we might change the logging. For example:

- Change the meaning of levels.
- Change the default key-value pairs and their representation.

Such changes should be considered a breaking change and handled according
to our [release process](/docs/developer/release-process.md) because they will require users to update their log processing
pipelines.

At the same time, changes to individual log messages are not breaking changes.

## External Resources

- [KEP-1602: Structured Logging](https://github.com/kubernetes/enhancements/blob/master/keps/sig-instrumentation/1602-structured-logging/README.md).
- [KEP-3077: contextual logging](https://github.com/kubernetes/enhancements/tree/master/keps/sig-instrumentation/3077-contextual-logging).

<!-- common links -->

[logr]:https://github.com/go-logr/logr/

[cr-log-levels]:https://github.com/kubernetes-sigs/controller-runtime/blob/b1d6919d3e12fa85a119dd9792bdfdc17bdf8c3b/pkg/log/zap/zap.go#L258-L259

[cr-zap]:https://github.com/kubernetes-sigs/controller-runtime/tree/main/pkg/log/zap
<!-- common links -->
