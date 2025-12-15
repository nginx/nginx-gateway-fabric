package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceSpecSetter_PreservesExternalAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		existingAnnotations map[string]string
		desiredAnnotations  map[string]string
		expectedAnnotations map[string]string
		name                string
	}{
		{
			name: "preserves MetalLB annotations while adding NGF annotations",
			existingAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "production-public-ips",
				"metallb.universe.tf/loadBalancerIPs":        "192.168.1.100",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "from-gateway-infrastructure",
			},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool":         "production-public-ips",
				"metallb.universe.tf/loadBalancerIPs":                "192.168.1.100",
				"custom.annotation":                                  "from-gateway-infrastructure",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "NGF annotations take precedence on conflicts",
			existingAnnotations: map[string]string{
				"custom.annotation":                "old-value",
				"metallb.universe.tf/address-pool": "staging",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "new-value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation":                                  "new-value",
				"metallb.universe.tf/address-pool":                   "staging",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name:                "creates new service with annotations",
			existingAnnotations: nil,
			desiredAnnotations: map[string]string{
				"custom.annotation": "value",
			},
			expectedAnnotations: map[string]string{
				"custom.annotation": "value",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "removes NGF-managed annotations when no longer desired",
			existingAnnotations: map[string]string{
				"custom.annotation":                                  "should-be-removed",
				"metallb.universe.tf/ip-allocated-from-pool":         "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
			desiredAnnotations: map[string]string{},
			expectedAnnotations: map[string]string{
				"metallb.universe.tf/ip-allocated-from-pool": "production",
			},
		},
		{
			name: "preserves cloud provider annotations",
			existingAnnotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
			},
			desiredAnnotations: map[string]string{
				"custom.annotation": "from-nginxproxy-patch",
			},
			expectedAnnotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
				"custom.annotation": "from-nginxproxy-patch",
				"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
			},
		},
		{
			name: "updates tracking annotation when managed keys change",
			existingAnnotations: map[string]string{
				"annotation-to-keep":                                 "value1",
				"annotation-to-remove":                               "value2",
				"metallb.universe.tf/address-pool":                   "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep,annotation-to-remove",
			},
			desiredAnnotations: map[string]string{
				"annotation-to-keep": "value1",
			},
			expectedAnnotations: map[string]string{
				"annotation-to-keep":                                 "value1",
				"metallb.universe.tf/address-pool":                   "production",
				"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
				Annotations: tt.desiredAnnotations,
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

var testCasesDeploymentAndDaemonSet = []struct {
	existingAnnotations map[string]string
	desiredAnnotations  map[string]string
	expectedAnnotations map[string]string
	name                string
}{
	{
		name: "preserves external annotations while adding NGF annotations",
		existingAnnotations: map[string]string{
			"deployment.kubernetes.io/revision": "1",
			"field.cattle.io/publicEndpoints":   "192.61.0.19",
			"field.cattle.io/ports":             "80/tcp",
		},
		desiredAnnotations: map[string]string{
			"custom.annotation": "from-ngf",
		},
		expectedAnnotations: map[string]string{
			"deployment.kubernetes.io/revision":                  "1",
			"field.cattle.io/publicEndpoints":                    "192.61.0.19",
			"field.cattle.io/ports":                              "80/tcp",
			"custom.annotation":                                  "from-ngf",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
	},
	{
		name: "preserves existing NGF-managed annotations when still desired",
		existingAnnotations: map[string]string{
			"custom.annotation":                                  "keep-me",
			"argocd.argoproj.io/sync-options":                    "Prune=false",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
		desiredAnnotations: map[string]string{
			"custom.annotation": "keep-me",
		},
		expectedAnnotations: map[string]string{
			"custom.annotation":                                  "keep-me",
			"argocd.argoproj.io/sync-options":                    "Prune=false",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
	},
	{
		name: "NGF annotations take precedence on conflicts",
		existingAnnotations: map[string]string{
			"custom.annotation":                "old-value",
			"daemonSet.kubernetes.io/revision": "7",
		},
		desiredAnnotations: map[string]string{
			"custom.annotation": "new-value",
		},
		expectedAnnotations: map[string]string{
			"custom.annotation":                                  "new-value",
			"daemonSet.kubernetes.io/revision":                   "7",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
	},
	{
		name:                "creates new deployment with annotations",
		existingAnnotations: nil,
		desiredAnnotations: map[string]string{
			"custom.annotation": "value",
		},
		expectedAnnotations: map[string]string{
			"custom.annotation": "value",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
	},
	{
		name: "removes NGF-managed annotations when no longer desired",
		existingAnnotations: map[string]string{
			"custom.annotation":                                  "should-be-removed",
			"deployment.kubernetes.io/revision":                  "2",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
		desiredAnnotations: map[string]string{},
		expectedAnnotations: map[string]string{
			"deployment.kubernetes.io/revision": "2",
		},
	},
	{
		name: "updates tracking annotation when managed keys change",
		existingAnnotations: map[string]string{
			"annotation-to-keep":                                 "keep-value",
			"annotation-to-remove":                               "remove-value",
			"argocd.argoproj.io/sync-options":                    "Validate=true",
			"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep,annotation-to-remove",
		},
		desiredAnnotations: map[string]string{
			"annotation-to-keep": "updated-keep-value",
		},
		expectedAnnotations: map[string]string{
			"annotation-to-keep":                                 "updated-keep-value",
			"argocd.argoproj.io/sync-options":                    "Validate=true",
			"gateway.nginx.org/internal-managed-annotation-keys": "annotation-to-keep",
		},
	},
	{
		name: "preserves external annotations while adding NGF annotations",
		existingAnnotations: map[string]string{
			"field.cattle.io/publicEndpoints": "192.61.0.19",
			"field.cattle.io/ports":           "80/tcp",
		},
		desiredAnnotations: map[string]string{
			"custom.annotation": "from-ngf",
		},
		expectedAnnotations: map[string]string{
			"field.cattle.io/publicEndpoints":                    "192.61.0.19",
			"field.cattle.io/ports":                              "80/tcp",
			"custom.annotation":                                  "from-ngf",
			"gateway.nginx.org/internal-managed-annotation-keys": "custom.annotation",
		},
	},
}

func TestDeploymentSpecSetter_PreservesExternalObjectAnnotations(t *testing.T) {
	t.Parallel()

	for _, tt := range testCasesDeploymentAndDaemonSet {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "nginx-gateway",
					Namespace:   "nginx-gateway",
					Annotations: tt.existingAnnotations,
				},
			}

			desiredMeta := metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "nginx-gateway-fabric",
					"app.kubernetes.io/instance": "nginx-gateway",
				},
				Annotations: tt.desiredAnnotations,
			}

			inputSpec := appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name":     "nginx-gateway-fabric",
						"app.kubernetes.io/instance": "nginx-gateway",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":     "nginx-gateway-fabric",
							"app.kubernetes.io/instance": "nginx-gateway",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "nginx-gateway",
							Image: "nginx:1.25",
						}},
					},
				},
			}

			setter := deploymentSpecSetter(existing, inputSpec, desiredMeta)
			err := setter()

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(existing.Annotations).To(Equal(tt.expectedAnnotations))
			g.Expect(existing.Labels).To(Equal(desiredMeta.Labels))
			g.Expect(existing.Spec).To(Equal(inputSpec))
		})
	}
}

func TestDaemonSetSpecSetter_PreservesExternalObjectAnnotations(t *testing.T) {
	t.Parallel()

	for _, tt := range testCasesDeploymentAndDaemonSet {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			existing := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "nginx-gateway",
					Namespace:   "nginx-gateway",
					Annotations: tt.existingAnnotations,
				},
			}

			desiredMeta := metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "nginx-gateway-fabric",
					"app.kubernetes.io/instance": "nginx-gateway",
				},
				Annotations: tt.desiredAnnotations,
			}

			inputSpec := appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name":     "nginx-gateway-fabric",
						"app.kubernetes.io/instance": "nginx-gateway",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":     "nginx-gateway-fabric",
							"app.kubernetes.io/instance": "nginx-gateway",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "nginx-gateway",
							Image: "nginx:1.25",
						}},
					},
				},
			}

			setter := daemonSetSpecSetter(existing, inputSpec, desiredMeta)
			err := setter()

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(existing.Annotations).To(Equal(tt.expectedAnnotations))
			g.Expect(existing.Labels).To(Equal(desiredMeta.Labels))
			g.Expect(existing.Spec).To(Equal(inputSpec))
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }
