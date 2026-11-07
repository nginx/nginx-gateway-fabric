package clustertrustbundles

import (
	certificatesv1 "k8s.io/api/certificates/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

// CaCertClusterTrustBundle represents a ClusterTrustBundle resource that holds CA Cert data.
type CaCertClusterTrustBundle struct {
	// Source holds the actual ClusterTrustBundle resource. Can be nil if the ClusterTrustBundle does not exist.
	Source *certificatesv1.ClusterTrustBundle
	// CertBundle holds the certificate bundle from the ClusterTrustBundle spec.trustBundle.
	CertBundle *secrets.CertificateBundle
}
