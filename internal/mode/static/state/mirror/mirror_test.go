package mirror

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

func TestRouteName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		namespace *string
		name      string
		routeName string
		service   string
		expected  string
		idx       int
	}{
		{
			name:      "with namespace",
			routeName: "route1",
			service:   "service1",
			namespace: helpers.GetPointer("namespace1"),
			idx:       1,
			expected:  "_ngf-internal-mirror-route1-service1-namespace1-1",
		},
		{
			name:      "without namespace",
			routeName: "route2",
			service:   "service2",
			namespace: nil,
			idx:       2,
			expected:  "_ngf-internal-mirror-route2-service2-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := RouteName(tt.routeName, tt.service, tt.namespace, tt.idx)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestPathWithBackendRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		backendRef v1.BackendObjectReference
		expected   *string
		name       string
		idx        int
	}{
		{
			name: "with namespace",
			idx:  1,
			backendRef: v1.BackendObjectReference{
				Name:      "service1",
				Namespace: helpers.GetPointer[v1.Namespace]("namespace1"),
			},
			expected: helpers.GetPointer("/_ngf-internal-mirror-namespace1-service1-1"),
		},
		{
			name: "without namespace",
			idx:  2,
			backendRef: v1.BackendObjectReference{
				Name: "service2",
			},
			expected: helpers.GetPointer("/_ngf-internal-mirror-service2-2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := PathWithBackendRef(tt.idx, tt.backendRef)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestBackendPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		namespace *string
		expected  *string
		name      string
		service   string
		idx       int
	}{
		{
			name:      "With namespace",
			idx:       1,
			namespace: helpers.GetPointer("namespace1"),
			service:   "service1",
			expected:  helpers.GetPointer("/_ngf-internal-mirror-namespace1-service1-1"),
		},
		{
			name:      "Without namespace",
			idx:       2,
			namespace: nil,
			service:   "service2",
			expected:  helpers.GetPointer("/_ngf-internal-mirror-service2-2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := BackendPath(tt.idx, tt.namespace, tt.service)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
