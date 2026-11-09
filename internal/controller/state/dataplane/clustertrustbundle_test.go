package dataplane

import (
	"testing"

	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/clustertrustbundles"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/configmaps"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func newTestCTB(name, data string) *clustertrustbundles.CaCertClusterTrustBundle {
	nsName := types.NamespacedName{Name: name}
	return &clustertrustbundles.CaCertClusterTrustBundle{
		Source: &certificatesv1.ClusterTrustBundle{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       certificatesv1.ClusterTrustBundleSpec{TrustBundle: data},
		},
		CertBundle: secrets.NewCertificateBundle(
			nsName,
			kinds.ClusterTrustBundle,
			&secrets.Certificate{CACert: []byte(data)},
		),
	}
}

func TestBuildRefCertificateBundlesIncludesClusterTrustBundles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundles := buildRefCertificateBundles(
		nil,
		nil,
		map[types.NamespacedName]*clustertrustbundles.CaCertClusterTrustBundle{
			{Name: "corp-root"}: newTestCTB("corp-root", "ctb-data"),
			// A missing bundle is recorded without a CertBundle and must be skipped.
			{Name: "missing"}: {},
		},
	)

	g.Expect(bundles).To(HaveLen(1))
	g.Expect(bundles[0].Name).To(Equal(types.NamespacedName{Name: "corp-root"}))
	g.Expect(string(bundles[0].Kind)).To(Equal(kinds.ClusterTrustBundle))
}

func TestGetFrontendTLSCertBundlesClusterTrustBundle(t *testing.T) {
	t.Parallel()

	gateway := &graph.Gateway{
		Source: &v1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "gateway-ns"}},
	}

	refCertBundles := buildRefCertificateBundles(
		nil,
		map[types.NamespacedName]*configmaps.CaCertConfigMap{
			{Namespace: "gateway-ns", Name: "client-ca"}: {
				CertBundle: secrets.NewCertificateBundle(
					types.NamespacedName{Namespace: "gateway-ns", Name: "client-ca"},
					kinds.ConfigMap,
					&secrets.Certificate{CACert: []byte("configmap-data")},
				),
			},
		},
		// corp-root is resolved elsewhere in the graph, e.g. by a valid BackendTLSPolicy reference.
		map[types.NamespacedName]*clustertrustbundles.CaCertClusterTrustBundle{
			{Name: "corp-root"}: newTestCTB("corp-root", "ctb-data"),
		},
	)
	index := indexRefCertBundles(refCertBundles)

	validCTBRef := v1.ObjectReference{
		Group: certificatesv1.GroupName,
		Kind:  kinds.ClusterTrustBundle,
		Name:  "corp-root",
	}
	wrongGroupCTBRef := v1.ObjectReference{
		Kind: kinds.ClusterTrustBundle,
		Name: "corp-root",
	}
	namespacedCTBRef := v1.ObjectReference{
		Group:     certificatesv1.GroupName,
		Kind:      kinds.ClusterTrustBundle,
		Name:      "corp-root",
		Namespace: helpers.GetPointer(v1.Namespace("other-ns")),
	}
	configMapRef := v1.ObjectReference{
		Kind: kinds.ConfigMap,
		Name: "client-ca",
	}

	tests := []struct {
		expected CertBundle
		name     string
		refs     []v1.ObjectReference
	}{
		{
			name:     "valid ClusterTrustBundle ref",
			refs:     []v1.ObjectReference{validCTBRef},
			expected: CertBundle("ctb-data"),
		},
		{
			name: "ClusterTrustBundle ref with the wrong group is skipped",
			refs: []v1.ObjectReference{wrongGroupCTBRef},
		},
		{
			name: "ClusterTrustBundle ref with a namespace is skipped",
			refs: []v1.ObjectReference{namespacedCTBRef},
		},
		{
			// Regression: the valid ConfigMap ref keeps the listener valid, but the rejected ClusterTrustBundle
			// refs must not add a bundle that happens to be resolved elsewhere in the graph.
			name:     "mixed valid and rejected refs only include the valid bundle",
			refs:     []v1.ObjectReference{configMapRef, wrongGroupCTBRef, namespacedCTBRef},
			expected: CertBundle("configmap-data"),
		},
		{
			name:     "valid ConfigMap and ClusterTrustBundle refs are combined",
			refs:     []v1.ObjectReference{configMapRef, validCTBRef},
			expected: CertBundle("configmap-datactb-data"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			id := CertBundleID("cert_bundle_gateway-ns_gw_443")
			result := getFrontendTLSCertBundles(
				id,
				make(map[CertBundleID]CertBundle),
				gateway,
				index,
				test.refs,
			)

			if test.expected == nil {
				g.Expect(result).ToNot(HaveKey(id))
				return
			}
			g.Expect(result).To(HaveKeyWithValue(id, test.expected))
		})
	}
}

func TestBackendTLSClusterTrustBundleCertBundle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gw"}
	btp := &graph.BackendTLSPolicy{
		Source: &v1.BackendTLSPolicy{
			Spec: v1.BackendTLSPolicySpec{
				Validation: v1.BackendTLSPolicyValidation{Hostname: "foo.example.com"},
			},
		},
		CaCertRef: types.NamespacedName{Name: "corp-root"},
		Gateways:  []types.NamespacedName{gwNsName},
		Valid:     true,
	}

	verify := convertBackendTLS(btp, gwNsName)
	g.Expect(verify).ToNot(BeNil())
	g.Expect(verify.CertBundleID).To(Equal(CertBundleID("cert_bundle__corp-root")))
	g.Expect(verify.RootCAPath).To(BeEmpty())

	refCertBundles := buildRefCertificateBundles(
		nil,
		nil,
		map[types.NamespacedName]*clustertrustbundles.CaCertClusterTrustBundle{
			{Name: "corp-root"}: newTestCTB("corp-root", "ctb-data"),
		},
	)
	backendGroups := []BackendGroup{
		{Backends: []Backend{{Valid: true, VerifyTLS: verify}}},
	}

	bundles := buildCertBundles(refCertBundles, backendGroups, nil, nil, nil)
	g.Expect(bundles).To(HaveKeyWithValue(verify.CertBundleID, CertBundle("ctb-data")))
}
