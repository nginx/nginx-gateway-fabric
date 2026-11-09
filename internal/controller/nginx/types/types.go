package types

const (
	// Nginx503Server is used as a backend for services that cannot be resolved (have no IP address).
	Nginx503Server = "unix:/var/run/nginx/nginx-503-server.sock"
)

const (
	// AgentOwnerNameLabel is the label key used to store the owner name of the nginx agent.
	AgentOwnerNameLabel = "owner-name"
	// AgentOwnerTypeLabel is the label key used to store the owner type of the nginx agent.
	AgentOwnerTypeLabel = "owner-type"
	// DaemonSetType is the value used to represent a DaemonSet owner type.
	DaemonSetType = "DaemonSet"
	// DeploymentType is the value used to represent a Deployment owner type.
	DeploymentType = "Deployment"
	// NginxImageAnnotation records the nginx image a Pod's template asked for, so image validation
	// still works when an admission webhook rewrites the Pod's image.
	NginxImageAnnotation = "gateway.nginx.org/nginx-image"
)
