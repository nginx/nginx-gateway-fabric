package configmaps

import (
	v1 "k8s.io/api/core/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

// CaCertConfigMap represents a ConfigMap resource that holds CA Cert data.
type CaCertConfigMap struct {
	// Source holds the actual ConfigMap resource. Can be nil if the ConfigMap does not exist.
	Source *v1.ConfigMap
	// CertBundle holds the certificate bundle from the ConfigMap data.
	CertBundle *secrets.CertificateBundle
}

const (
	// EventsConfKey is the key in the bootstrap ConfigMap data for events configuration.
	EventsConfKey = "events.conf"
	// MainConfKey is the key in the bootstrap ConfigMap data for main configuration.
	MainConfKey = "main.conf"
	// MgmtConfKey is the key in the bootstrap ConfigMap data for mgmt configuration.
	MgmtConfKey = "mgmt.conf"
	// AgentConfKey is the key in the agent ConfigMap data for agent configuration.
	AgentConfKey = "nginx-agent.conf"
)
