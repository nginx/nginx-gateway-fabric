package mirror

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

func TestRouteName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := RouteName("route1", "service1", "namespace1", 1)
	g.Expect(result).To(Equal("_ngf-internal-mirror-route1-namespace1/service1-1"))
}

func TestPathWithBackendRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		backendRef  v1.BackendObjectReference
		expected    *string
		name        string
		routeNsName types.NamespacedName
		idx         int
	}{
		{
			name: "with backendRef namespace",
			idx:  1,
			backendRef: v1.BackendObjectReference{
				Name:      "service1",
				Namespace: helpers.GetPointer[v1.Namespace]("namespace1"),
			},
			routeNsName: types.NamespacedName{Namespace: "routeNs", Name: "routeName1"},
			expected:    helpers.GetPointer("/_ngf-internal-mirror-namespace1/service1-routeNs/routeName1-1"),
		},
		{
			name: "without backendRef namespace",
			idx:  2,
			backendRef: v1.BackendObjectReference{
				Name: "service2",
			},
			routeNsName: types.NamespacedName{Namespace: "routeNs", Name: "routeName1"},
			expected:    helpers.GetPointer("/_ngf-internal-mirror-service2-routeNs/routeName1-2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := PathWithBackendRef(tt.idx, tt.backendRef, tt.routeNsName)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestBackendPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		namespace   *string
		expected    *string
		name        string
		service     string
		routeNsName types.NamespacedName
		idx         int
	}{
		{
			name:        "With backendRef namespace",
			idx:         1,
			namespace:   helpers.GetPointer("namespace1"),
			service:     "service1",
			routeNsName: types.NamespacedName{Namespace: "routeNs", Name: "routeName1"},
			expected:    helpers.GetPointer("/_ngf-internal-mirror-namespace1/service1-routeNs/routeName1-1"),
		},
		{
			name:        "Without backendRef namespace",
			idx:         2,
			namespace:   nil,
			service:     "service2",
			routeNsName: types.NamespacedName{Namespace: "routeNs", Name: "routeName1"},
			expected:    helpers.GetPointer("/_ngf-internal-mirror-service2-routeNs/routeName1-2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := BackendPath(tt.idx, tt.namespace, tt.service, tt.routeNsName)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
