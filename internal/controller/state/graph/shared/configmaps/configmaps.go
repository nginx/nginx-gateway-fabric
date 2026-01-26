package configmaps

import (
	v1 "k8s.io/api/core/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

// CaCertConfigMap represents a ConfigMap resource that holds CA Cert data.
type CaCertConfigMap struct {
	// Source holds the actual ConfigMap resource. Can be nil if the ConfigMap does not exist.
	Source     *v1.ConfigMap
	CertBundle *secrets.CertificateBundle
}
