package resolver_test

import (
	"testing"

	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func newClusterTrustBundle(name, trustBundle string) *certificatesv1.ClusterTrustBundle {
	return &certificatesv1.ClusterTrustBundle{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       certificatesv1.ClusterTrustBundleSpec{TrustBundle: trustBundle},
	}
}

func ctbKey(name string) resolver.ResourceKey {
	return resolver.ResourceKey{
		ResourceType:   resolver.ResourceTypeClusterTrustBundle,
		NamespacedName: types.NamespacedName{Name: name},
	}
}

func TestResolveClusterTrustBundle(t *testing.T) {
	t.Parallel()

	resources := map[resolver.ResourceKey]client.Object{
		ctbKey("valid"):   newClusterTrustBundle("valid", caBlock),
		ctbKey("invalid"): newClusterTrustBundle("invalid", "invalid"),
		ctbKey("empty"):   newClusterTrustBundle("empty", ""),
	}

	tests := []struct {
		name           string
		bundleName     string
		expectedErrMsg string
		apiAvailable   bool
		unsupported    bool
	}{
		{
			name:         "valid bundle",
			bundleName:   "valid",
			apiAvailable: true,
		},
		{
			name:           "invalid bundle",
			bundleName:     "invalid",
			apiAvailable:   true,
			expectedErrMsg: "the data field \"ca.crt\" must hold a valid CERTIFICATE PEM block",
		},
		{
			name:           "empty bundle",
			bundleName:     "empty",
			apiAvailable:   true,
			expectedErrMsg: "ClusterTrustBundle has an empty spec.trustBundle",
		},
		{
			name:           "non-existent bundle",
			bundleName:     "non-existent",
			apiAvailable:   true,
			expectedErrMsg: "ClusterTrustBundle non-existent does not exist",
		},
		{
			name:         "API not available",
			bundleName:   "valid",
			apiAvailable: false,
			unsupported:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resourceResolver := resolver.NewResourceResolver(
				resources,
				resolver.WithClusterTrustBundleAPI(test.apiAvailable),
			)

			err := resourceResolver.Resolve(
				resolver.ResourceTypeClusterTrustBundle,
				types.NamespacedName{Name: test.bundleName},
			)

			switch {
			case test.unsupported:
				g.Expect(err).To(MatchError(resolver.ErrUnsupportedKind))
				g.Expect(resourceResolver.GetClusterTrustBundles()).To(BeNil())
			case test.expectedErrMsg == "":
				g.Expect(err).ToNot(HaveOccurred())
			default:
				g.Expect(err).To(MatchError(test.expectedErrMsg))
			}
		})
	}
}

func TestGetClusterTrustBundles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	resources := map[resolver.ResourceKey]client.Object{
		ctbKey("valid"): newClusterTrustBundle("valid", caBlock),
	}

	resourceResolver := resolver.NewResourceResolver(resources, resolver.WithClusterTrustBundleAPI(true))
	g.Expect(resourceResolver.GetClusterTrustBundles()).To(BeNil())

	g.Expect(resourceResolver.Resolve(
		resolver.ResourceTypeClusterTrustBundle,
		types.NamespacedName{Name: "valid"},
	)).To(Succeed())
	// A missing bundle is still recorded so that its later creation triggers a graph rebuild.
	g.Expect(resourceResolver.Resolve(
		resolver.ResourceTypeClusterTrustBundle,
		types.NamespacedName{Name: "missing"},
	)).ToNot(Succeed())

	bundles := resourceResolver.GetClusterTrustBundles()
	g.Expect(bundles).To(HaveLen(2))

	valid := bundles[types.NamespacedName{Name: "valid"}]
	g.Expect(valid).ToNot(BeNil())
	g.Expect(valid.Source).To(Equal(resources[ctbKey("valid")]))
	g.Expect(valid.CertBundle).ToNot(BeNil())
	g.Expect(string(valid.CertBundle.Kind)).To(Equal(kinds.ClusterTrustBundle))
	g.Expect(valid.CertBundle.Name).To(Equal(types.NamespacedName{Name: "valid"}))
	g.Expect(string(valid.CertBundle.Cert.CACert)).To(Equal(caBlock))

	missing := bundles[types.NamespacedName{Name: "missing"}]
	g.Expect(missing).ToNot(BeNil())
	g.Expect(missing.Source).To(BeNil())
	g.Expect(missing.CertBundle).To(BeNil())
}
