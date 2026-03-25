package state

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

var scheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := gatewayv1.Install(s); err != nil {
		panic(err)
	}
	if err := gatewayv1beta1.Install(s); err != nil {
		panic(err)
	}
	return s
}()

func TestConvertingReferenceGrantStore(t *testing.T) {
	t.Parallel()

	nsname := types.NamespacedName{Namespace: "test-ns", Name: "test-grant"}

	betaGrant := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsname.Namespace,
			Name:      nsname.Name,
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group("gateway.networking.k8s.io"),
					Kind:      gatewayv1beta1.Kind("HTTPRoute"),
					Namespace: gatewayv1beta1.Namespace("route-ns"),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(""),
					Kind:  gatewayv1beta1.Kind("Service"),
					Name:  ptrTo(gatewayv1beta1.ObjectName("my-svc")),
				},
				{
					Group: gatewayv1beta1.Group(""),
					Kind:  gatewayv1beta1.Kind("Secret"),
					// Name nil - allows all secrets in namespace
				},
			},
		},
	}

	t.Run("upsert converts v1beta1 to v1", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		v1Grants := make(map[types.NamespacedName]*gatewayv1.ReferenceGrant)
		store := newConvertingReferenceGrantStore(v1Grants)

		store.upsert(betaGrant)

		g.Expect(v1Grants).To(HaveKey(nsname))

		converted := v1Grants[nsname]
		g.Expect(converted.Name).To(Equal(nsname.Name))
		g.Expect(converted.Namespace).To(Equal(nsname.Namespace))

		// Check From
		g.Expect(converted.Spec.From).To(HaveLen(1))
		g.Expect(string(converted.Spec.From[0].Group)).To(Equal("gateway.networking.k8s.io"))
		g.Expect(string(converted.Spec.From[0].Kind)).To(Equal("HTTPRoute"))
		g.Expect(string(converted.Spec.From[0].Namespace)).To(Equal("route-ns"))

		// Check To
		g.Expect(converted.Spec.To).To(HaveLen(2))
		g.Expect(string(converted.Spec.To[0].Group)).To(Equal(""))
		g.Expect(string(converted.Spec.To[0].Kind)).To(Equal("Service"))
		g.Expect(converted.Spec.To[0].Name).ToNot(BeNil())
		g.Expect(string(*converted.Spec.To[0].Name)).To(Equal("my-svc"))
		g.Expect(converted.Spec.To[1].Name).To(BeNil())
	})

	t.Run("get returns stored v1 object", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		v1Grants := make(map[types.NamespacedName]*gatewayv1.ReferenceGrant)
		store := newConvertingReferenceGrantStore(v1Grants)

		// Get on empty store returns nil
		obj := store.get(&gatewayv1.ReferenceGrant{}, nsname)
		g.Expect(obj).To(BeNil())

		// Upsert then get
		store.upsert(betaGrant)
		obj = store.get(&gatewayv1.ReferenceGrant{}, nsname)
		g.Expect(obj).ToNot(BeNil())
		rg, ok := obj.(*gatewayv1.ReferenceGrant)
		g.Expect(ok).To(BeTrue())
		g.Expect(rg.Name).To(Equal(nsname.Name))
	})

	t.Run("delete removes the object", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		v1Grants := make(map[types.NamespacedName]*gatewayv1.ReferenceGrant)
		store := newConvertingReferenceGrantStore(v1Grants)

		store.upsert(betaGrant)
		g.Expect(v1Grants).To(HaveKey(nsname))

		store.delete(&gatewayv1.ReferenceGrant{}, nsname)
		g.Expect(v1Grants).ToNot(HaveKey(nsname))
	})

	t.Run("upsert panics on wrong type", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		v1Grants := make(map[types.NamespacedName]*gatewayv1.ReferenceGrant)
		store := newConvertingReferenceGrantStore(v1Grants)

		g.Expect(func() {
			store.upsert(&gatewayv1.ReferenceGrant{})
		}).To(Panic())
	})
}

func TestRefGrantTrackingCfg(t *testing.T) {
	t.Parallel()

	mustExtractGVK := kinds.NewMustExtractGKV(scheme)

	tests := []struct {
		discoveredCRDs  map[string]bool
		name            string
		expectedVersion string
	}{
		{
			name: "v1 ReferenceGrant discovered",
			discoveredCRDs: map[string]bool{
				kinds.ReferenceGrant: true,
			},
			expectedVersion: "v1",
		},
		{
			name: "v1 ReferenceGrant not discovered falls back to v1beta1",
			discoveredCRDs: map[string]bool{
				kinds.ReferenceGrant: false,
			},
			expectedVersion: "v1beta1",
		},
		{
			name:            "empty discoveredCRDs defaults to v1",
			discoveredCRDs:  map[string]bool{},
			expectedVersion: "v1",
		},
		{
			name:            "nil discoveredCRDs defaults to v1",
			discoveredCRDs:  nil,
			expectedVersion: "v1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			refGrants := make(map[types.NamespacedName]*gatewayv1.ReferenceGrant)

			cfg := refGrantTrackingCfg(mustExtractGVK, test.discoveredCRDs, refGrants)
			g.Expect(cfg.gvk.Version).To(Equal(test.expectedVersion))
			g.Expect(cfg.gvk.Kind).To(Equal(kinds.ReferenceGrant))
			g.Expect(cfg.predicate).To(BeNil())
		})
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
