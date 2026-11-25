package resolver

import (
	"errors"
	"fmt"

	certificatesv1 "k8s.io/api/certificates/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/clustertrustbundles"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type clusterTrustBundleEntry struct {
	// err holds the corresponding error if the ClusterTrustBundle is invalid or does not exist.
	err                error
	clusterTrustBundle clustertrustbundles.CaCertClusterTrustBundle
}

func (c *clusterTrustBundleEntry) setError(err error) {
	c.err = err
}

func (c *clusterTrustBundleEntry) error() error {
	return c.err
}

func (c *clusterTrustBundleEntry) needsRevalidation(_ *resolveOptions) bool { return false }

func (c *clusterTrustBundleEntry) revalidate(_ *resolveOptions, _ client.Object) error { return nil }

func (c *clusterTrustBundleEntry) validate(obj client.Object) {
	ctb, ok := obj.(*certificatesv1.ClusterTrustBundle)
	if !ok {
		panic(fmt.Sprintf("expected ClusterTrustBundle object, got %T", obj))
	}

	var validationErr error
	trustBundle := []byte(ctb.Spec.TrustBundle)

	if len(trustBundle) == 0 {
		validationErr = errors.New("ClusterTrustBundle has an empty spec.trustBundle")
	} else {
		validationErr = secrets.ValidateCA(trustBundle)
	}

	c.clusterTrustBundle = clustertrustbundles.CaCertClusterTrustBundle{
		Source: ctb,
		CertBundle: secrets.NewCertificateBundle(
			client.ObjectKeyFromObject(ctb),
			kinds.ClusterTrustBundle,
			&secrets.Certificate{CACert: trustBundle},
		),
	}
	c.setError(validationErr)
}
