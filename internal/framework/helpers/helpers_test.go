package helpers_test

import (
	"testing"
	"text/template"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestMustCastObject(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var obj client.Object = &gatewayv1.Gateway{}

	g.Expect(func() {
		_ = helpers.MustCastObject[*gatewayv1.Gateway](obj)
	}).ToNot(Panic())

	g.Expect(func() {
		_ = helpers.MustCastObject[*gatewayv1alpha3.BackendTLSPolicy](obj)
	}).To(Panic())
}

func TestEqualPointers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p1       *string
		p2       *string
		name     string
		expEqual bool
	}{
		{
			name:     "first pointer nil; second has non-empty value",
			p1:       nil,
			p2:       helpers.GetPointer("test"),
			expEqual: false,
		},
		{
			name:     "second pointer nil; first has non-empty value",
			p1:       helpers.GetPointer("test"),
			p2:       nil,
			expEqual: false,
		},
		{
			name:     "different values",
			p1:       helpers.GetPointer("test"),
			p2:       helpers.GetPointer("different"),
			expEqual: false,
		},
		{
			name:     "both pointers nil",
			p1:       nil,
			p2:       nil,
			expEqual: true,
		},
		{
			name:     "first pointer nil; second empty",
			p1:       nil,
			p2:       helpers.GetPointer(""),
			expEqual: true,
		},
		{
			name:     "second pointer nil; first empty",
			p1:       helpers.GetPointer(""),
			p2:       nil,
			expEqual: true,
		},
		{
			name:     "same value",
			p1:       helpers.GetPointer("test"),
			p2:       helpers.GetPointer("test"),
			expEqual: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			val := helpers.EqualPointers(test.p1, test.p2)
			g.Expect(val).To(Equal(test.expEqual))
		})
	}
}

func TestMustExecuteTemplate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tmpl := template.Must(template.New("test").Parse(`Hello {{.}}`))
	bytes := helpers.MustExecuteTemplate(tmpl, "you")
	g.Expect(string(bytes)).To(Equal("Hello you"))
}

func TestMustExecuteTemplatePanics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	execute := func() {
		helpers.MustExecuteTemplate(nil, nil)
	}

	g.Expect(execute).To(Panic())
}

func TestMustGenerateRandomPrimeNumer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// This test is expected to panic if it fails to generate a random prime number.
	g.Expect(func() {
		_ = helpers.RandomPrimeNumber()
	}).ToNot(Panic())
}

func TestMustReturnUniqueResourceName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	name := "test-resource"
	uniqueName := helpers.UniqueResourceName(name)

	g.Expect(uniqueName).To(HavePrefix(name))
	g.Expect(len(uniqueName)).To(BeNumerically(">", len(name)))
}

func TestMustCreateKubernetesClient(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := helpers.GetKubernetesClient()

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(k8sClient).ToNot(BeNil())
}
