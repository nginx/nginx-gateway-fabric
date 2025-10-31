package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceSpecSetter_PreservesExistingAnnotations(t *testing.T) {
	tests := []struct {
		existingAnnotations   map[string]string
		ngfManagedAnnotations map[string]string
		expectedAnnotations   map[string]string
		name                  string
	}{
		{
			name: "preserves external annotations and adds NGF annotations",
			existingAnnotations: map[string]string{
				"metallb.universe.tf/address-pool":          "production-public-ips",
				"external-dns.alpha.kubernetes.io/hostname": "example.com",
			},
			ngfManagedAnnotations: map[string]string{
				"gateway.nginx.org/managed": "true",
			},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/address-pool":          "production-public-ips",
				"external-dns.alpha.kubernetes.io/hostname": "example.com",
				"gateway.nginx.org/managed":                 "true",
			},
		},
		{
			name: "NGF annotations take precedence",
			existingAnnotations: map[string]string{
				"custom.annotation":                "old-value",
				"metallb.universe.tf/address-pool": "staging",
			},
			ngfManagedAnnotations: map[string]string{
				"custom.annotation": "new-value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation":                "new-value",
				"metallb.universe.tf/address-pool": "staging",
			},
		},
		{
			name:                "new service with no existing annotations",
			existingAnnotations: nil,
			ngfManagedAnnotations: map[string]string{
				"gateway.nginx.org/managed": "true",
			},
			expectedAnnotations: map[string]string{
				"gateway.nginx.org/managed": "true",
			},
		},
		{
			name: "preserves external annotations with patched annotations",
			existingAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "production",
				"metallb.universe.tf/loadBalancerIPs":        "192.168.1.100",
			},
			ngfManagedAnnotations: map[string]string{
				"gateway.nginx.org/managed":                         "true",
				"custom.patched.io/example":                         "patched-value",
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool":        "production",
				"metallb.universe.tf/loadBalancerIPs":               "192.168.1.100",
				"gateway.nginx.org/managed":                         "true",
				"custom.patched.io/example":                         "patched-value",
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create existing service with annotations
			existingService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-service",
					Namespace:   "default",
					Annotations: tt.existingAnnotations,
				},
			}

			// Create desired object metadata with NGF-managed annotations
			desiredMeta := metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "nginx-gateway",
				},
				Annotations: tt.ngfManagedAnnotations,
			}

			// Create desired spec
			desiredSpec := corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:     "http",
						Port:     80,
						Protocol: corev1.ProtocolTCP,
					},
				},
			}

			// Execute the setter
			setter := serviceSpecSetter(existingService, desiredSpec, desiredMeta)
			err := setter()

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(existingService.Annotations).To(Equal(tt.expectedAnnotations))
			g.Expect(existingService.Labels).To(Equal(desiredMeta.Labels))
			g.Expect(existingService.Spec).To(Equal(desiredSpec))
		})
	}
}
