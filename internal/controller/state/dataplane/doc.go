/*
Package dataplane translates Graph representation of the cluster state into an intermediate representation of
data plane configuration. We can think of it as an intermediate state between the cluster resources
and NGINX configuration files.

The package includes:
- The types to hold the intermediate representation.
- The functions to translate the Graph into the representation.
*/
package dataplane
