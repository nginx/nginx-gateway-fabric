// Package nap contains constants shared across components that interact with
// F5 NGINX App Protect (NAP) v5.
package waf

import "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"

// Release is the NAP v5 release version deployed by NGINX Gateway Fabric.
// Re-exported from the fetch package to avoid an import cycle.
const Release = fetch.Release
