package cel

import (
	"testing"

	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestIngressLinkSchemaConformance guards against silent drift in the F5 CIS IngressLink schema.
// NGF writes the IngressLink spec as an unstructured object with hardcoded field keys, so a rename
// or type change on F5's side would not fail to compile; it would only surface at runtime on BIG-IP.
//
// This reads the IngressLink CRD back from the cluster where it was installed at the pinned
// CIS_VERSION (via `make install-gateway-link-crds`) and asserts that every field NGF writes still
// exists with the expected type. Because it validates against the freshly installed CRD rather than
// a committed copy, it cannot go stale: a CIS bump installs the new schema and this test runs against
// it, so a breaking change fails CI on the bump.
func TestIngressLinkSchemaConformance(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	specProps := ingressLinkSpecSchema(t, g)

	object := func(props map[string]apiextv1.JSONSchemaProps, name string) map[string]apiextv1.JSONSchemaProps {
		t.Helper()
		p, ok := props[name]
		g.Expect(ok).To(BeTrue(), "field %q missing from IngressLink schema", name)
		g.Expect(p.Type).To(Equal("object"), "field %q is not an object", name)
		return p.Properties
	}

	arrayItems := func(props map[string]apiextv1.JSONSchemaProps, name string) map[string]apiextv1.JSONSchemaProps {
		t.Helper()
		p, ok := props[name]
		g.Expect(ok).To(BeTrue(), "field %q missing from IngressLink schema", name)
		g.Expect(p.Type).To(Equal("array"), "field %q is not an array", name)
		g.Expect(p.Items).ToNot(BeNil(), "array field %q has no items schema", name)
		g.Expect(p.Items.Schema).ToNot(BeNil(), "array field %q has no items schema", name)
		return p.Items.Schema.Properties
	}

	expectField := func(props map[string]apiextv1.JSONSchemaProps, name, wantType string) {
		t.Helper()
		p, ok := props[name]
		g.Expect(ok).To(BeTrue(), "field %q missing from IngressLink schema", name)
		g.Expect(p.Type).To(Equal(wantType), "field %q has type %q, want %q", name, p.Type, wantType)
	}

	expectEnum := func(props map[string]apiextv1.JSONSchemaProps, name string, want ...string) {
		t.Helper()
		p, ok := props[name]
		g.Expect(ok).To(BeTrue(), "field %q missing from IngressLink schema", name)
		got := make([]string, 0, len(p.Enum))
		for _, e := range p.Enum {
			got = append(got, string(e.Raw))
		}
		for _, w := range want {
			g.Expect(got).To(ContainElement(`"`+w+`"`), "enum %q missing value %q", name, w)
		}
	}

	// Top-level spec fields NGF writes.
	expectField(specProps, "virtualServerAddress", "string")
	expectField(specProps, "virtualServerName", "string")
	expectField(specProps, "ipamLabel", "string")
	expectField(specProps, "host", "string")
	expectField(specProps, "partition", "string")
	expectField(specProps, "bigipRouteDomain", "integer")
	expectField(specProps, "iRules", "array")

	// monitors: array of objects with name and reference.
	monitor := arrayItems(specProps, "monitors")
	expectField(monitor, "name", "string")
	expectEnum(monitor, "reference", "bigip")

	// tls: object with clientSSLs, serverSSLs, reference.
	tls := object(specProps, "tls")
	expectField(tls, "clientSSLs", "array")
	expectField(tls, "serverSSLs", "array")
	expectEnum(tls, "reference", "bigip", "secret")

	// serviceAddress: array (maxItems 1) of objects with icmpEcho and trafficGroup.
	serviceAddress := arrayItems(specProps, "serviceAddress")
	expectEnum(serviceAddress, "icmpEcho", "enable", "disable", "selective")
	expectField(serviceAddress, "trafficGroup", "string")

	// multiClusterServices: array of objects with clusterName, namespace, service, weight.
	mcService := arrayItems(specProps, "multiClusterServices")
	expectField(mcService, "clusterName", "string")
	expectField(mcService, "namespace", "string")
	expectField(mcService, "service", "string")
	expectField(mcService, "weight", "integer")

	// selector: object with matchLabels.
	selector := object(specProps, "selector")
	expectField(selector, "matchLabels", "object")
}

// ingressLinkSpecSchema reads the installed IngressLink CRD from the cluster and returns its
// spec.properties. It requires the CIS CRDs to be installed.
func ingressLinkSpecSchema(t *testing.T, g *WithT) map[string]apiextv1.JSONSchemaProps {
	t.Helper()

	k8sConfig, err := controllerruntime.GetConfig()
	g.Expect(err).ToNot(HaveOccurred())

	scheme := runtime.NewScheme()
	g.Expect(apiextv1.AddToScheme(scheme)).To(Succeed())

	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme})
	g.Expect(err).ToNot(HaveOccurred())

	var crd apiextv1.CustomResourceDefinition
	err = k8sClient.Get(t.Context(), types.NamespacedName{Name: "ingresslinks.cis.f5.com"}, &crd)
	if apierrors.IsNotFound(err) {
		t.Skip("IngressLink CRD not installed; run make install-gateway-link-crds")
	}
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(crd.Spec.Versions).ToNot(BeEmpty())
	schema := crd.Spec.Versions[0].Schema
	g.Expect(schema).ToNot(BeNil())
	g.Expect(schema.OpenAPIV3Schema).ToNot(BeNil())

	spec, ok := schema.OpenAPIV3Schema.Properties["spec"]
	g.Expect(ok).To(BeTrue(), "IngressLink CRD schema has no spec")

	return spec.Properties
}
