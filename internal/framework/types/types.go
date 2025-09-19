package types //nolint:revive, nolintlint // ignoring “meaningless package name” and the unused-nolint warning

import "sigs.k8s.io/controller-runtime/pkg/client"

// ObjectType is used when we only care about the type of client.Object.
// The fields of the client.Object may be empty.
type ObjectType client.Object

// Fields used for communication with the EndpointPicker service when using the Inference Extension.
const (
	// EPPEndpointHostHeader is the HTTP header used to specify the EPP endpoint host.
	EPPEndpointHostHeader = "X-EPP-Host"
	// EPPEndpointPortHeader is the HTTP header used to specify the EPP endpoint port.
	EPPEndpointPortHeader = "X-EPP-Port"
	// GoShimPort is the default port for the Go EPP shim server to listen on. If collisions become a problem,
	// we can make this configurable via the NginxProxy resource.
	GoShimPort = 54800 // why 54800? Sum "nginx" in ASCII and multiply by 100.
)
