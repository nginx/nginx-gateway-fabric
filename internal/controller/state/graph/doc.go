/*
Package graph translates the cluster state (Gateway API and Kubernetes resources) into a graph-like representation,
for which:
- Resources are validated. For example, if a Gateway listener is invalid, the graph representation will reflect that.
- Connections between resources are found. For example, if an HTTPRoute attaches to a Gateway, the graph representation
reflects that.
- Any validation or connections-related errors are captured.

Those three points make it easier to generate intermediate data plane configuration and statuses for resources.

The package includes the types to represent the graph and the functions to convert resources into their graph
representation.

The validation of the resource fields consists of two parts:
- Data-plane specific validation. For example, validating the value of an HTTP header. Such validation is delegated
to the data-plane specific implementation of a Validator.
- Data-plane agnostic validation. For such validation, the values either don't affect the data-plane configuration
directly or they must be validated to process a resource. For example, hostnames must be validated to be able to bind
an HTTPRoute to a Listener.
*/
package graph
