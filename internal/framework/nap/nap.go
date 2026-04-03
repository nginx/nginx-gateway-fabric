// Package nap contains constants shared across components that interact with
// F5 NGINX App Protect (NAP) v5.
package nap

// Release is the NAP v5 release version deployed by NGINX Gateway Fabric.
// It is used both as the default image tag for the waf-enforcer and waf-config-mgr
// sidecar containers and as the nap_release query parameter when compiling
// policy bundles via the F5 NGINX One Console API.
const Release = "5.12.1"
